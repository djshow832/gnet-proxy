package dcli

import (
	"fmt"
	"github.com/djshow832/gnet-proxy/util"
	"github.com/panjf2000/gnet/v2"
	bbPool "github.com/panjf2000/gnet/v2/pkg/pool/bytebuffer"
	"net"
	"sync"
	"time"
)

func StartDoubleCliMode(port int, backends []string) {
	p = newProxy(fmt.Sprintf(":%d", port), backends)
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
	cli := util.Try(gnet.NewClient(&handler{}, gnet.WithTCPKeepAlive(time.Minute))).(*gnet.Client)
	return &Proxy{
		listenAddr: listenAddr,
		backends:   backends,
		cli:        cli,
	}
}

func (p *Proxy) Start() {
	util.Try(p.cli.Start())
	ln := util.Try(net.Listen("tcp", p.listenAddr)).(net.Listener)
	for {
		conn := util.Try(ln.Accept()).(net.Conn)
		p.Lock()
		frontendConn := util.Try(p.cli.Enroll(conn)).(gnet.Conn)
		backendConn := util.Try(p.cli.Dial("tcp", p.GetBackend())).(gnet.Conn)
		ctx := &connContext{
			frontendConn: frontendConn,
			backendConn:  backendConn,
		}
		frontendConn.SetContext(ctx)
		backendConn.SetContext(ctx)
		p.Unlock()
	}
}

func (p *Proxy) GetBackend() string {
	if p.curIndex >= len(p.backends) {
		p.curIndex = 0
	}
	backend := p.backends[p.curIndex]
	p.curIndex++
	return backend
}

func (p *Proxy) Stop() {
	util.Try(p.cli.Stop())
}

type connContext struct {
	sync.Mutex
	frontendConn gnet.Conn
	backendConn  gnet.Conn
}

func (ctx *connContext) GetPeer(conn gnet.Conn) gnet.Conn {
	if conn == ctx.frontendConn {
		return ctx.backendConn
	}
	return ctx.frontendConn
}

type handler struct {
	*gnet.BuiltinEventEngine
}

func (fh *handler) OnTraffic(conn gnet.Conn) (action gnet.Action) {
	buf := bbPool.Get()
	util.Try(conn.WriteTo(buf))
	p.RLock()
	ctx := conn.Context().(*connContext)
	p.RUnlock()
	ctx.Lock()
	util.Try(ctx.GetPeer(conn).AsyncWrite(buf.Bytes(), func(c gnet.Conn, err error) error {
		bbPool.Put(buf)
		return err
	}))
	ctx.Unlock()
	return
}

func (fh *handler) OnClose(conn gnet.Conn, _ error) (action gnet.Action) {
	p.Lock()
	ctx := conn.Context().(*connContext)
	conn.SetContext(nil)
	p.Unlock()
	ctx.Lock()
	util.Try(ctx.GetPeer(conn).Close())
	ctx.Unlock()
	return
}
