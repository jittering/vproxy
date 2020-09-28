package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/chetan/simpleproxy"
)

var (
	bind      = flag.String("bind", "", "Bind hostnames to local ports (app.local.com:7000)")
	httpPort  = flag.Int("http", 80, "Port to listen for HTTP on")
	httpsPort = flag.Int("https", 443, "Port to listen for TLS (HTTPS)")
	useTLS    = flag.Bool("tls", false, "Enable TLS")
)

var message = `<html>
<body>
<h1>502 Bad Gateway</h1>
<p>Can't connect to upstream server, please try again later.</p>
</body>
</html>`

func main() {
	flag.Parse()

	if *bind == "" {
		log.Fatal("must specify -bind")
	}

	vhost := createVhostMux(bind)

	mux := simpleproxy.NewLoggedMux()
	mux.Handle("/", vhost)

	var wg sync.WaitGroup
	wg.Add(1)

	listenAddr := fmt.Sprintf("127.0.0.1:%d", 80)
	fmt.Printf("[*] starting proxy: http://%s\n", listenAddr)
	go func() {
		log.Fatal(http.ListenAndServe(listenAddr, mux))
		wg.Done()
	}()

	if *useTLS {
		wg.Add(1)
		go func() {
			listenAddrTLS := fmt.Sprintf("127.0.0.1:%d", 443)
			fmt.Printf("[*] starting proxy: https://%s\n", listenAddrTLS)
			fmt.Printf("    vhosts:\n")
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

			server := http.Server{
				Addr:      listenAddrTLS,
				Handler:   mux,
				TLSConfig: cfg,
			}
			server.ListenAndServeTLS("", "")

			wg.Done()
		}()
	}

	wg.Wait()
}

// Create vhost config for each binding
func createVhostMux(bind *string) *simpleproxy.VhostMux {
	servers := make(map[string]*simpleproxy.Vhost)
	bindings := strings.Split(*bind, " ")
	for _, binding := range bindings {
		s := strings.Split(binding, ":")
		hostname := s[0]
		targetPort, err := strconv.Atoi(s[1])
		if err != nil {
			log.Fatal("failed to parse target port:", err)
			os.Exit(1)
		}

		proxy := simpleproxy.CreateProxy(url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", targetPort)})

		vhost := &simpleproxy.Vhost{
			Host: hostname, Port: targetPort, Handler: proxy,
		}

		if *useTLS {
			vhost.Cert, vhost.Key, err = simpleproxy.MakeCert(hostname)
			if err != nil {
				log.Fatalf("failed to generate cert for host %s", hostname)
			}
		}

		servers[hostname] = vhost
	}

	return &simpleproxy.VhostMux{Servers: servers}
}
