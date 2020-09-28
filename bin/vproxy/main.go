package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/chetan/simpleproxy"
)

var (
	bind   = flag.String("bind", "", "Bind hostnames to local ports (app.local.com:7000)")
	useTLS = flag.Bool("tls", false, "Enable TLS")
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
		servers[hostname] = &simpleproxy.Vhost{
			Host: hostname, Port: targetPort, Handler: proxy,
		}

	}

	vhost := &simpleproxy.VhostMux{servers}
	listenAddr := fmt.Sprintf("127.0.0.1:%d", 9999)

	mux := simpleproxy.NewLoggedMux()
	mux.Handle("/", vhost)

	fmt.Printf("[*] starting proxy: http://%s\n\n", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, mux))

}
