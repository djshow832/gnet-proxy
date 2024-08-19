package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"strings"

	"github.com/djshow832/gnet-proxy/dcli"
	"github.com/djshow832/gnet-proxy/netpoll"
	"github.com/djshow832/gnet-proxy/poolcli"
	"github.com/djshow832/gnet-proxy/srvcli"
	"github.com/djshow832/gnet-proxy/util"
)

func main() {
	runtime.GOMAXPROCS(1)

	var port int
	var statusPort int
	var mode int
	var backends string
	flag.IntVar(&port, "port", 6000, "server port")
	flag.IntVar(&statusPort, "statusPort", 3080, "status port")
	flag.IntVar(&mode, "mode", 0, "run mode")
	flag.StringVar(&backends, "backends", ":4000", "backend addrs")
	flag.Parse()

	bs := strings.Split(backends, ",")

	// /debug/pprof
	go func() {
		util.Try(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", statusPort), nil))
	}()

	switch mode {
	case 0:
		srvcli.StartSrvCliMode(port, bs)
	case 1:
		dcli.StartDoubleCliMode(port, bs)
	case 2:
		poolcli.StartPoolCliMode(port, bs)
	case 3:
		netpoll.StartNetpollMode(port, bs)
	}
}
