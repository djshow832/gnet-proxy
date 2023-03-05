package main

import (
	"flag"
	"fmt"
	"github.com/djshow832/gnet-proxy/dcli"
	"github.com/djshow832/gnet-proxy/poolcli"
	"github.com/djshow832/gnet-proxy/srvcli"
	"github.com/djshow832/gnet-proxy/util"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"strings"
)

func main() {
	runtime.GOMAXPROCS(1)

	var port int
	var statusPort int
	var mode int
	var backends string
	flag.IntVar(&port, "port", 6000, "server port")
	flag.IntVar(&statusPort, "statusPort", 6061, "status port")
	flag.IntVar(&mode, "mode", 0, "run mode")
	flag.StringVar(&backends, "backend", ":4000", "backend addrs")
	flag.Parse()

	bs := strings.Split(backends, ",")

	go func() {
		util.Try(http.ListenAndServe(fmt.Sprintf(":%d", statusPort), nil))
	}()
	switch mode {
	case 0:
		srvcli.StartSrvCliMode(port, bs)
	case 1:
		dcli.StartDoubleCliMode(port, bs)
	default:
		poolcli.StartPoolCliMode(port, bs)
	}
}
