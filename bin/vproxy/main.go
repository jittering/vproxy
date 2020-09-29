package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/chetan/simpleproxy"
)

var (
	bind      = flag.String("bind", "", "Bind hostnames to local ports (app.local.com:7000)")
	httpPort  = flag.Int("http", 80, "Port to listen for HTTP (0 to disable)")
	httpsPort = flag.Int("https", 443, "Port to listen for TLS (HTTPS) (0 to disable)")
)

func main() {
	flag.Parse()

	// if *bind == "" && len(flag.Args()) == 0 {
	// 	log.Fatal("must specify -bind")
	// }

	addr := fmt.Sprintf("127.0.0.1:%d", *httpPort)
	if IsDaemonRunning(addr) {
		startClientMode(addr)
		return
	}

	// create handlers
	bindings := strings.Split(*bind, " ")
	bindings = append(bindings, flag.Args()...)
	vhost := simpleproxy.CreateVhostMux(bindings, *httpsPort > 0)
	mux := simpleproxy.NewLoggedMux()
	mux.Handle("/", vhost)

	// start daemon
	d := &daemon{vhost: vhost, mux: mux, httpPort: *httpPort, httpsPort: *httpsPort}
	d.run()
}
