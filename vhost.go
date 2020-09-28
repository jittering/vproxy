package simpleproxy

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

type Vhost struct {
	Host    string
	Port    int
	Handler http.Handler
}

// VhostMux is an http.Handler whose ServeHTTP forwards the request to
// backend Servers according to the incoming request URL
type VhostMux struct {
	Servers map[string]*Vhost
}

func (v *VhostMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	originalUrl := r.Host + r.URL.Path

	// ignore port num, if any
	s := strings.Split(r.Host, ":")
	host := s[0]

	vhost := v.Servers[host]
	if vhost == nil {
		log.Printf("Host Not Found: `%s`", host)
		w.WriteHeader(404)
		fmt.Fprintln(w, "host not found:", host)
		return
	}

	defer func() {
		if val := recover(); val != nil {
			log.Printf("Error proxying request `%s` to `%s`: %v", originalUrl, r.URL, val)
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "Error proxying request `%s` to `%s`: %v", originalUrl, r.URL, val)
		}
	}()

	// handle it
	vhost.Handler.ServeHTTP(w, r)
}
