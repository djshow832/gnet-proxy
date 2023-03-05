package srvcli

import (
	"fmt"
	"github.com/djshow832/gnet-proxy/util"
	"github.com/panjf2000/gnet/v2"
	bbPool "github.com/panjf2000/gnet/v2/pkg/pool/bytebuffer"
	_ "net/http/pprof"
	"sync"
	"time"
)

func StartSrvCliMode(port int, backends []string) {
	p = newProxy(fmt.Sprintf("tcp://:%d", port), backends)
	p.Start()
}

var p *Proxy

type Proxy struct {
	sync.RWMutex
	listenAddr string
	curIndex   int
	backends   []string
	cli        *gnet.Client
}

func newProxy(listenAddr string, backends []string) *Proxy {
	cli := util.Try(gnet.NewClient(&backendHandler{}, gnet.WithTCPKeepAlive(time.Minute))).(*gnet.Client)
	return &Proxy{
		listenAddr: listenAddr,
		backends:   backends,
		cli:        cli,
	}
}

func (p *Proxy) Start() {
	util.Try(p.cli.Start())
	util.Try(gnet.Run(&frontendHandler{}, p.listenAddr, gnet.WithMulticore(true), gnet.WithTCPKeepAlive(time.Minute)))
}

func (p *Proxy) Stop() {
	util.Try(p.cli.Stop())
}

func (p *Proxy) GetBackend() string {
	if p.curIndex >= len(p.backends) {
		p.curIndex = 0
	}
	backend := p.backends[p.curIndex]
	p.curIndex++
	return backend
}

type connContext struct {
	sync.Mutex
	frontendConn gnet.Conn
	backendConn  gnet.Conn
}

type frontendHandler struct {
	*gnet.BuiltinEventEngine
}

func (fh *frontendHandler) OnOpen(frontendConn gnet.Conn) (out []byte, action gnet.Action) {
	p.Lock()
	backendConn := util.Try(p.cli.Dial("tcp", p.GetBackend())).(gnet.Conn)
	// OnTraffic may occur before setting the context.
	ctx := &connContext{
		frontendConn: frontendConn,
		backendConn:  backendConn,
	}
	frontendConn.SetContext(ctx)
	backendConn.SetContext(ctx)
	p.Unlock()
	return
}

func (fh *frontendHandler) OnTraffic(frontendConn gnet.Conn) (action gnet.Action) {
	buf := bbPool.Get()
	util.Try(frontendConn.WriteTo(buf))
	p.RLock()
	ctx := frontendConn.Context().(*connContext)
	p.RUnlock()
	ctx.Lock()
	util.Try(ctx.backendConn.AsyncWrite(buf.Bytes(), nil))
	ctx.Unlock()
	return
}

func (fh *frontendHandler) OnClosed(frontendConn gnet.Conn, _ error) (action gnet.Action) {
	p.Lock()
	ctx := frontendConn.Context().(*connContext)
	frontendConn.SetContext(nil)
	p.Unlock()
	// gnet.Conn is not concurrency-safe.
	ctx.Lock()
	util.Try(ctx.backendConn.Close())
	ctx.Unlock()
	return
}

type backendHandler struct {
	*gnet.BuiltinEventEngine
}

func (bh *backendHandler) OnTraffic(backendConn gnet.Conn) (action gnet.Action) {
	buf := bbPool.Get()
	util.Try(backendConn.WriteTo(buf))
	p.RLock()
	ctx := backendConn.Context().(*connContext)
	p.RUnlock()
	ctx.Lock()
	util.Try(ctx.frontendConn.AsyncWrite(buf.Bytes(), nil))
	ctx.Unlock()
	return
}

func (bh *backendHandler) OnClosed(backendConn gnet.Conn, _ error) (action gnet.Action) {
	p.Lock()
	ctx := backendConn.Context().(*connContext)
	backendConn.SetContext(nil)
	p.Unlock()
	ctx.Lock()
	util.Try(ctx.frontendConn.Close())
	ctx.Unlock()
	return
}
