package vproxy

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gammazero/deque"
	"github.com/txn2/txeh"
)

// Vhost represents a single backend service
type Vhost struct {
	Host string // virtual host name

	ServiceHost string // service host or IP
	Port        int    // service port

	Handler http.Handler
	Cert    string // TLS Certificate
	Key     string // TLS Private Key

	logRing   *deque.Deque
	logChan   LogListener
	listeners []LogListener
}

type LogListener chan string

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

// DumpServers to the given writer
func (v *VhostMux) DumpServers(w io.Writer) {
	switch c := len(v.Servers); c {
	case 0:
		fmt.Fprintln(w, "0 vhosts")
	case 1:
		fmt.Fprintln(w, "1 vhost:")
	default:
		fmt.Fprintf(w, "%d vhosts:", c)
	}
	for _, v := range v.Servers {
		fmt.Fprintf(w, "%s -> %s:%d\n", v.Host, v.ServiceHost, v.Port)
	}
}

// CreateVhostMux config, optionally initialized with a list of bindings
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
	targetHost := "127.0.0.1"
	targetURL := url.URL{Scheme: "http", Host: fmt.Sprintf("%s:%d", targetHost, targetPort)}

	proxy := CreateProxy(targetURL, hostname)

	vhost := &Vhost{
		Host: hostname, ServiceHost: targetHost, Port: targetPort, Handler: proxy,
		logRing: deque.New(10, 16),
		logChan: make(LogListener, 10),
	}

	go vhost.populateLogBuffer()

	if useTLS {
		vhost.Cert, vhost.Key, err = MakeCert(hostname)
		if err != nil {
			return nil, fmt.Errorf("failed to generate cert for host %s: %s", hostname, err)
		}
	}

	return vhost, nil
}

func (v *Vhost) NewLogListener() LogListener {
	logChan := make(LogListener, 100)
	v.listeners = append(v.listeners, logChan)
	return logChan
}

func (v *Vhost) RemoveLogListener(logChan LogListener) {
	index := 0
	for _, i := range v.listeners {
		if i != logChan {
			v.listeners[index] = i
			index++
		}
	}
	v.listeners = v.listeners[:index]
}

func (v *Vhost) Close() {

}

func (v *Vhost) populateLogBuffer() {
	for {
		line := <-v.logChan
		if v.logRing.Len() < 10 {
			v.logRing.PushBack(line)
		} else {
			v.logRing.Rotate(1)
			v.logRing.Set(9, line)
		}
	}
}

// Map given host to 127.0.0.1 in system hosts file (usually /etc/hosts)
func addToHosts(host string) error {
	hosts, err := txeh.NewHostsDefault()
	if err != nil {
		return err
	}

	hosts.AddHost("127.0.0.1", host)
	return hosts.Save()
}
