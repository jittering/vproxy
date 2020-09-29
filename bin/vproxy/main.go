package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/chetan/simpleproxy"
)

var (
	bind      = flag.String("bind", "", "Bind hostnames to local ports (app.local.com:7000)")
	httpPort  = flag.Int("http", 80, "Port to listen for HTTP (0 to disable)")
	httpsPort = flag.Int("https", 443, "Port to listen for TLS (HTTPS) (0 to disable)")
)

var message = `<html>
<body>
<h1>502 Bad Gateway</h1>
<p>Can't connect to upstream server, please try again later.</p>
</body>
</html>`

func main() {
	flag.Parse()

	if *bind == "" && len(flag.Args()) == 0 {
		log.Fatal("must specify -bind")
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

type daemon struct {
	wg    sync.WaitGroup
	vhost *simpleproxy.VhostMux
	mux   *simpleproxy.LoggedMux

	httpPort     int
	httpAddr     string
	httpListener net.Listener

	httpsPort     int
	httpsAddr     string
	httpsListener net.Listener
}

func (d *daemon) run() {
	d.wg.Add(1) // ensure we don't exit immediately
	if d.httpPort > 0 {
		d.httpAddr = fmt.Sprintf("127.0.0.1:%d", d.httpPort)
		fmt.Printf("[*] starting proxy: http://%s\n", d.httpAddr)
		go d.startHTTP()
	}
	if d.httpsPort > 0 {
		d.httpsAddr = fmt.Sprintf("127.0.0.1:%d", d.httpsPort)
		fmt.Printf("[*] starting proxy: https://%s\n", d.httpsAddr)
		fmt.Printf("    vhosts:\n")
		go d.startTLS()
	}
	d.wg.Wait()
}

func (d *daemon) startHTTP() {
	d.wg.Add(1)
	var err error
	d.httpListener, err = net.Listen("tcp", d.httpAddr)
	if err != nil {
		log.Fatalf("failed to start listener: %s", err)
	}

	server := &http.Server{Handler: d.mux}
	server.Serve(d.httpListener)
	d.wg.Done()
}

func (d *daemon) startTLS() {
	d.wg.Add(1)
	var err error
	d.httpsListener, err = net.Listen("tcp", d.httpsAddr)
	if err != nil {
		log.Fatalf("failed to start listener: %s", err)
	}

	server := http.Server{
		Handler:   d.mux,
		TLSConfig: createTLSConfig(d.vhost),
	}

	server.Serve(d.httpsListener)
	d.wg.Done()
}

// Create multi-certificate TLS config from vhost config
func createTLSConfig(vhost *simpleproxy.VhostMux) *tls.Config {
	cfg := &tls.Config{}
	for _, server := range vhost.Servers {
		fmt.Printf("    - %s -> %d\n", server.Host, server.Port)
		cert, err := tls.LoadX509KeyPair(server.Cert, server.Key)
		if err != nil {
			log.Fatal("failed to load keypair:", err)
		}
		cfg.Certificates = append(cfg.Certificates, cert)
	}
	cfg.BuildNameToCertificate()
	return cfg
}
