package main

import (
	"flag"
	_ "net/http/pprof"
	"runtime"
	"strings"

	"github.com/djshow832/gnet-proxy/dcli"
	"github.com/djshow832/gnet-proxy/netpoll"
	"github.com/djshow832/gnet-proxy/poolcli"
	"github.com/djshow832/gnet-proxy/srvcli"
)

func main() {
	runtime.GOMAXPROCS(1)

	var port int
	var mode int
	var backends string
	flag.IntVar(&port, "port", 6000, "server port")
	flag.IntVar(&mode, "mode", 0, "run mode")
	flag.StringVar(&backends, "backends", ":4000", "backend addrs")
	flag.Parse()

	bs := strings.Split(backends, ",")

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
