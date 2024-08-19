package netpoll

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/netpoll"
	"github.com/djshow832/gnet-proxy/util"
	"github.com/tiancaiamao/gp"
)

func StartNetpollMode(port int, backends []string) {
	p = newProxy(fmt.Sprintf(":%d", port), backends)
	p.Start()
}

var p *Proxy

type Proxy struct {
	sync.RWMutex
	listenAddr string
	curIndex   int
	backends   []string
	gopool     *gp.Pool
	evl        netpoll.EventLoop
}

func newProxy(listenAddr string, backends []string) *Proxy {
	return &Proxy{
		listenAddr: listenAddr,
		backends:   backends,
		gopool:     gp.New(100, time.Minute),
	}
}

func (p *Proxy) Start() {
	listener := util.Try(netpoll.CreateListener("tcp", p.listenAddr)).(netpoll.Listener)
	netpoll.Configure(netpoll.Config{
		PollerNum: 1,
		Runner: func(ctx context.Context, f func()) {
			p.gopool.Go(f)
		},
	})
	p.evl = util.Try(netpoll.NewEventLoop(onRequest, netpoll.WithOnConnect(onConnect))).(netpoll.EventLoop)
	p.evl.Serve(listener)
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
	p.evl.Shutdown(context.Background())
}

type connContext struct {
	client netpoll.Connection
	server netpoll.Connection
}

func onConnect(ctx context.Context, client netpoll.Connection) context.Context {
	backend := p.GetBackend()
	server := util.Try(netpoll.DialConnection("tcp", backend, time.Second)).(netpoll.Connection)
	cc := connContext{
		client: client,
		server: server,
	}
	server.SetOnRequest(cc.onResponse)
	ctx = context.WithValue(ctx, "client", client)
	ctx = context.WithValue(ctx, "server", server)
	return ctx
}

func onRequest(ctx context.Context, conn netpoll.Connection) (err error) {
	req := util.Try(conn.Reader().ReadBinary(conn.Reader().Len())).([]byte)
	server := ctx.Value("server").(netpoll.Connection)
	_, _ = server.Write([]byte(req))
	return nil
}

func (cc connContext) onResponse(ctx context.Context, conn netpoll.Connection) (err error) {
	rep := util.Try(conn.Reader().ReadBinary(conn.Reader().Len())).([]byte)
	client := cc.client
	_, _ = client.Write([]byte(rep))
	return nil
}
