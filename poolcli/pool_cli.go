package poolcli

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/djshow832/gnet-proxy/util"
	"github.com/panjf2000/gnet/v2"
	"github.com/tiancaiamao/gp"
)

func StartPoolCliMode(port int, backends []string) {
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
	gopool     *gp.Pool
}

func newProxy(listenAddr string, backends []string) *Proxy {
	cli := util.Try(gnet.NewClient(&handler{}, gnet.WithMulticore(true), gnet.WithTCPKeepAlive(time.Minute))).(*gnet.Client)
	return &Proxy{
		listenAddr: listenAddr,
		backends:   backends,
		cli:        cli,
		gopool:     gp.New(100, time.Minute),
	}
}

func (p *Proxy) Start() {
	util.Try(p.cli.Start())
	ln := util.Try(net.Listen("tcp", p.listenAddr)).(net.Listener)
	for {
		conn := util.Try(ln.Accept()).(net.Conn)
		ctx := &connContext{}
		// after v2.3.0, Enroll and Dial may call OnTraffic. Be careful about the lock.
		frontendConn := util.Try(p.cli.EnrollContext(conn, ctx)).(gnet.Conn)
		backendConn := util.Try(p.cli.DialContext("tcp", p.GetBackend(), ctx)).(gnet.Conn)
		ctx.Lock()
		ctx.backendConn = backendConn
		ctx.frontendConn = frontendConn
		ctx.Unlock()
	}
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

func (ctx *connContext) GetPeer(conn gnet.Conn) gnet.Conn {
	for {
		ctx.Lock()
		if ctx.frontendConn == nil || ctx.backendConn == nil {
			ctx.Unlock()
			continue
		}
		defer ctx.Unlock()
		if conn == ctx.frontendConn {
			return ctx.backendConn
		}
		return ctx.frontendConn
	}
}

type handler struct {
	*gnet.BuiltinEventEngine
}

func (fh *handler) OnTraffic(conn gnet.Conn) (action gnet.Action) {
	p.gopool.Go(
		func() {
			ctx := conn.Context().(*connContext)
			p.Lock()
			buf := util.Try(conn.Next(-1)).([]byte)
			p.Unlock()
			peer := ctx.GetPeer(conn)
			util.Try(peer.Write(buf))
		})
	return
}

func (fh *handler) OnClose(conn gnet.Conn, _ error) (action gnet.Action) {
	ctx := conn.Context().(*connContext)
	util.Try(ctx.GetPeer(conn).Close())
	return
}
