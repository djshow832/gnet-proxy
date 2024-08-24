package rawread

import (
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"errors"

	"github.com/djshow832/gnet-proxy/util"
)

var p *Proxy

type Proxy struct {
	sync.RWMutex
	listener   net.Listener
	listenAddr string
	curIndex   int
	backends   []string
}

func StartNetMode(port int, backends []string) {
	p = newProxy(fmt.Sprintf(":%d", port), backends)
	p.Start()
}

func newProxy(listenAddr string, backends []string) *Proxy {
	return &Proxy{
		listenAddr: listenAddr,
		backends:   backends,
	}
}

func (p *Proxy) Start() {
	p.listener = util.Try(net.Listen("tcp", p.listenAddr)).(net.Listener)
	for {
		frontendConn, err := p.listener.Accept()
		if err != nil {
			return
		}
		ctx := &connContext{}
		backendConn := util.Try(net.Dial("tcp", p.GetBackend())).(net.Conn)
		ctx.backendConn = backendConn
		ctx.frontendConn = frontendConn
		ctx.onConn()
	}
}

func (p *Proxy) GetBackend() string {
	p.Lock()
	defer p.Unlock()
	if p.curIndex >= len(p.backends) {
		p.curIndex = 0
	}
	backend := p.backends[p.curIndex]
	p.curIndex++
	return backend
}

func (p *Proxy) Stop() {
	_ = p.listener.Close()
}

type connContext struct {
	frontendConn net.Conn
	backendConn  net.Conn
}

func (cc *connContext) onConn() {
	go func() {
		var buf [4096]byte
		var err error
		rawBackendConn := util.Try(cc.backendConn.(syscall.Conn).SyscallConn()).(syscall.RawConn)
		rawFrontendConn := util.Try(cc.frontendConn.(syscall.Conn).SyscallConn()).(syscall.RawConn)
		if err = forwardPkt(cc.backendConn, cc.frontendConn, rawBackendConn, buf[:], false); err != nil {
			return
		}
		for {
			// need to disable ssl
			if err = forwardPkt(cc.frontendConn, cc.backendConn, rawFrontendConn, buf[:], true); err != nil {
				println("relay fail", err.Error())
				return
			}
			if err = forwardPkt(cc.backendConn, cc.frontendConn, rawBackendConn, buf[:], true); err != nil {
				println("relay fail", err.Error())
				return
			}
		}
	}()
}

type temp struct {
	buf     []byte
	from    net.Conn
	readErr error
	idx     int
	n       int
	first   bool
}

func forwardPkt(from, to net.Conn, rawConn syscall.RawConn, buf []byte, rawcall bool) error {
	var tmp temp
	tmp.first = true
	tmp.from = from
	tmp.buf = buf
	for tmp.idx < 4 {
		var err error
		if rawcall {
			err = rawConn.Read(func(fd uintptr) bool {
				if tmp.first {
					tmp.first = false
					tmp.from.SetReadDeadline(time.Now().Add(3 * time.Millisecond))
					return false
				}
				tmp.n, tmp.readErr = syscall.Read(int(fd), tmp.buf[tmp.idx:])
				done := tmp.readErr != syscall.EAGAIN
				if tmp.readErr != nil && done {
					println("read error", tmp.readErr.Error())
				}
				return done
			})
			if errors.Is(err, os.ErrDeadlineExceeded) {
				tmp.from.SetReadDeadline(time.Time{})
				continue
			}
			if err == nil {
				err = tmp.readErr
			}
		} else {
			tmp.n, err = from.Read(buf[tmp.idx:])
		}
		if err != nil {
			return err
		}
		if tmp.n == 0 {
			return io.EOF
		}
		tmp.idx += tmp.n
	}

	length := int(buf[0]) | int(buf[1])<<8 | int(buf[2])<<16
	data := buf[:]
	if length+4 > len(buf) {
		data = make([]byte, length+4)
		copy(data[:], buf[:tmp.idx])
	}
	for tmp.idx < length+4 {
		n, err := from.Read(data[tmp.idx:])
		if err != nil {
			return err
		}
		tmp.idx += n
	}
	_, err := to.Write(data[:tmp.idx])
	return err
}
