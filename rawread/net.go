package rawread

import (
	"fmt"
	"net"
	"sync"
	"syscall"

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
		if !forwardPkt(cc.backendConn, cc.frontendConn, buf[:], false) {
			return
		}
		for {
			// need to disable ssl
			if !forwardPkt(cc.frontendConn, cc.backendConn, buf[:], true) {
				return
			}
			if !forwardPkt(cc.backendConn, cc.frontendConn, buf[:], true) {
				return
			}
		}
	}()
}

func forwardPkt(from, to net.Conn, buf []byte, rawcall bool) bool {
	idx := 0
	done := false
	rawConn := util.Try(from.(syscall.Conn).SyscallConn()).(syscall.RawConn)
	for idx < 4 {
		var n int
		var err, readErr error
		if rawcall {
			err = rawConn.Read(func(fd uintptr) bool {
				if done {
					n, readErr = syscall.Read(int(fd), buf[idx:])
					return readErr != syscall.EAGAIN
				}
				done = true
				return false
			})
			if err == nil {
				err = readErr
			}
		} else {
			n, err = from.Read(buf[idx:])
		}
		if err != nil {
			return false
		}
		idx += n
	}

	length := int(buf[0]) | int(buf[1])<<8 | int(buf[2])<<16
	data := buf[:]
	if length+4 > len(buf) {
		data = make([]byte, length+4)
		copy(data[:], buf[:idx])
	}
	for idx < length+4 {
		n, err := from.Read(data[idx:])
		if err != nil {
			return false
		}
		idx += n
	}
	_, err := to.Write(data[:idx])
	return err == nil
}
