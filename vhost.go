package vproxy

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/cbednarski/hostess/hostess"
)

type Vhost struct {
	Host    string
	Port    int
	Handler http.Handler
	Cert    string
	Key     string
}

// VhostMux is an http.Handler whose ServeHTTP forwards the request to
// backend Servers according to the incoming request URL
type VhostMux struct {
	Servers map[string]*Vhost
}

func (v *VhostMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	originalURL := r.Host + r.URL.Path

	host := getHostName(r.Host)
	vhost := v.Servers[host]
	if vhost == nil {
		log.Printf("Host Not Found: `%s`", host)
		w.WriteHeader(404)
		fmt.Fprintln(w, "host not found:", host)
		return
	}

	defer func() {
		if val := recover(); val != nil {
			log.Printf("Error proxying request `%s` to `%s`: %v", originalURL, r.URL, val)
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "Error proxying request `%s` to `%s`: %v", originalURL, r.URL, val)
		}
	}()

	// handle it
	vhost.Handler.ServeHTTP(w, r)
}

// CreateVhostMux config from list of bindings
func CreateVhostMux(bindings []string, useTLS bool) *VhostMux {
	servers := make(map[string]*Vhost)
	for _, binding := range bindings {
		if binding != "" {
			vhost, err := CreateVhost(binding, useTLS)
			if err != nil {
				// on startup, bail immediately
				log.Fatal(err)
			}
			servers[vhost.Host] = vhost
		}
	}

	return &VhostMux{Servers: servers}
}

// CreateVhost for the host:port pair, optionally with a TLS cert
func CreateVhost(input string, useTLS bool) (*Vhost, error) {
	s := strings.Split(input, ":")
	if len(s) < 2 {
		// invalid binding
		return nil, fmt.Errorf("error: invalid binding '%s'", input)
	}

	hostname := s[0]
	targetPort, err := strconv.Atoi(s[1])
	if err != nil {
		return nil, fmt.Errorf("failed to parse target port: %s", err)
	}

	proxy := CreateProxy(url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", targetPort)}, hostname)

	// Add IP to hosts
	addToHosts(hostname)

	vhost := &Vhost{
		Host: hostname, Port: targetPort, Handler: proxy,
	}

	if useTLS {
		vhost.Cert, vhost.Key, err = MakeCert(hostname)
		if err != nil {
			return nil, fmt.Errorf("failed to generate cert for host %s: %s", hostname, err)
		}
	}

	return vhost, nil
}

func addToHosts(host string) error {
	hosts, errs := hostess.LoadHostfile()
	if len(errs) > 0 {
		return errs[0]
	}

	hostname, err := hostess.NewHostname(host, "127.0.0.1", true)
	if err != nil {
		return err
	}

	err = hosts.Hosts.Add(hostname)
	if err != nil {
		if !strings.Contains(err.Error(), "duplicate") {
			// ignore duplicate hostname errors (already added previously)
			return err
		}
	}

	return hosts.Save()
}
