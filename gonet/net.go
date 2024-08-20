package gonet

import (
	"fmt"
	"io"
	"net"
	"sync"

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
		io.Copy(cc.frontendConn, cc.backendConn)
	}()
	go func() {
		io.Copy(cc.backendConn, cc.frontendConn)
	}()
}
