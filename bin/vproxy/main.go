package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/chetan/simpleproxy"
)

var (
	listen    = flag.String("listen", "127.0.0.1", "IP to listen on (0.0.0.0 for all IPs)")
	bind      = flag.String("bind", "", "Bind hostnames to local ports (app.local.com:7000)")
	httpPort  = flag.Int("http", 80, "Port to listen for HTTP (0 to disable)")
	httpsPort = flag.Int("https", 443, "Port to listen for TLS (HTTPS) (0 to disable)")
)

var defaultListenAddr = "127.0.0.1"
var anyIP = "0.0.0.0"

func main() {
	flag.Parse()

	// if *bind == "" && len(flag.Args()) == 0 {
	// 	log.Fatal("must specify -bind")
	// }

	if *listen == "" {
		listen = &defaultListenAddr
	} else if *listen == "0" {
		listen = &anyIP
	}

	addr := fmt.Sprintf("%s:%d", *listen, *httpPort)
	if IsDaemonRunning(addr) {
		startClientMode(addr)
		return
	}

	// create handlers
	bindings := strings.Split(*bind, " ")
	if len(bindings) == 0 {
		// add bindings from remaining args
		bindings = append(bindings, flag.Args()[0])
	}
	vhost := simpleproxy.CreateVhostMux(bindings, *httpsPort > 0)
	mux := simpleproxy.NewLoggedMux()
	mux.Handle("/", vhost)

	// start daemon
	d := &daemon{vhost: vhost, mux: mux, listen: *listen, httpPort: *httpPort, httpsPort: *httpsPort}
	d.run()
}
