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
	Host string `json:"host"` // virtual host name

	ServiceHost string // service host or IP
	ServicePort int    // service port

	Handler http.Handler `json:"-"`
	Cert    string       // TLS Certificate
	Key     string       // TLS Private Key

	logRing   *deque.Deque[string] `json:"-"`
	logChan   LogListener          `json:"-"`
	listeners []LogListener        `json:"-"`
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
		fmt.Fprintf(w, "%d vhosts:\n", c)
	}
	for _, v := range v.Servers {
		fmt.Fprintf(w, "%s -> %s:%d\n", v.Host, v.ServiceHost, v.ServicePort)
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

	vhost := &Vhost{
		Host:        hostname,
		ServiceHost: targetHost,
		ServicePort: targetPort,
	}

	if useTLS {
		vhost.Cert, vhost.Key, err = MakeCert(hostname)
		if err != nil {
			return nil, fmt.Errorf("failed to generate cert for host %s: %s", hostname, err)
		}
	}

	return vhost, nil
}

func (v *Vhost) Init() {
	targetURL := url.URL{Scheme: "http", Host: fmt.Sprintf("%s:%d", v.ServiceHost, v.ServicePort)}
	v.Handler = CreateProxy(targetURL, v.Host)
	v.logChan = make(LogListener, 10)
	// set fixed capacity at 16
	v.logRing = &deque.Deque[string]{}
	v.logRing.Grow(16)
	v.logRing.SetBaseCap(16)
	go v.populateLogBuffer()
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

func (v *Vhost) BufferAsString() string {
	if v.logRing.Len() == 0 {
		return ""
	}
	buff := ""
	for i := 0; i < v.logRing.Len(); i++ {
		s := v.logRing.At(i)
		if s != "" {
			buff += s + "\n"
		}
	}
	return buff
}

func (v *Vhost) Close() {
	if v.logChan != nil {
		close(v.logChan)
	}
	if v.logRing != nil {
		v.logRing.Clear()
	}
}

func (v *Vhost) populateLogBuffer() {
	for line := range v.logChan {
		if v.logRing.Len() < 10 {
			v.logRing.PushBack(line)
		} else {
			v.logRing.Rotate(1)
			v.logRing.Set(9, line)
		}
	}
}

func (v Vhost) String() string {
	return fmt.Sprintf("%s -> %s:%d", v.Host, v.ServiceHost, v.ServicePort)
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
