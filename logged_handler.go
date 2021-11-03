package vproxy

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

var defaultTLSHost = "vproxy.local"

// LoggedHandler is an extension of http.ServeMux which multiplexes requests to
// the vhost backends (via a handler) and logs each request.
// TODO: replace ServeMux with a proper router (chi?)
type LoggedHandler struct {
	*http.ServeMux
	VhostLogListeners map[string]chan string
	vhostMux          *VhostMux

	defaultHost string
	defaultCert string
	defaultKey  string
}

// NewLoggedHandler wraps the given handler with a request/response logger
func NewLoggedHandler(vm *VhostMux) *LoggedHandler {
	lh := &LoggedHandler{
		ServeMux:          http.NewServeMux(),
		VhostLogListeners: make(map[string]chan string),
		vhostMux:          vm,
	}

	lh.defaultHost = defaultTLSHost
	lh.createDefaultCert()

	// Map all requests, by default, to the appropriate vhost
	lh.Handle("/", vm)
	return lh
}

func (lh *LoggedHandler) createDefaultCert() {
	var err error
	lh.defaultCert, lh.defaultKey, err = MakeCert(lh.defaultHost)
	if err != nil {
		log.Fatalf("failed to create default cert for vproxy.local: %s", err)
	}
}

func (lh *LoggedHandler) AddVhost(vhost *Vhost, listener chan string) {
	lh.VhostLogListeners[vhost.Host] = listener
	lh.vhostMux.Servers[vhost.Host] = vhost
}

func (lh *LoggedHandler) RemoveVhost(host string) {
	delete(lh.VhostLogListeners, host)
	delete(lh.vhostMux.Servers, host)
}

// DumpServers to the given writer
func (lh *LoggedHandler) DumpServers(w io.Writer) {
	lh.vhostMux.DumpServers(w)
}

// Create multi-certificate TLS config from vhost config
func (lh *LoggedHandler) CreateTLSConfig() *tls.Config {
	cfg := &tls.Config{}

	// Add default internal cert
	cert, err := tls.LoadX509KeyPair(lh.defaultCert, lh.defaultKey)
	if err != nil {
		log.Fatal("failed to load keypair:", err)
	}
	cfg.Certificates = append(cfg.Certificates, cert)

	// add cert for each vhost
	for _, server := range lh.vhostMux.Servers {
		cert, err := tls.LoadX509KeyPair(server.Cert, server.Key)
		if err != nil {
			log.Fatal("failed to load keypair:", err)
		}
		cfg.Certificates = append(cfg.Certificates, cert)
	}

	// build cn and return
	cfg.BuildNameToCertificate()
	return cfg
}

func (lh *LoggedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	record := &LogRecord{
		ResponseWriter: w,
	}

	// serve request and capture timings
	startTime := time.Now()
	lh.ServeMux.ServeHTTP(record, r)
	finishTime := time.Now()
	elapsedTime := finishTime.Sub(startTime)
	host := getHostName(r.Host)

	l := fmt.Sprintf("%s [%s] %s [ %d ] %s %d %s", r.RemoteAddr, host, r.Method, record.status, r.URL, r.ContentLength, elapsedTime)
	log.Println(l)

	if listener, ok := lh.VhostLogListeners[host]; ok {
		listener <- l
	}
}

// ignore port num, if any
func getHostName(input string) string {
	s := strings.Split(input, ":")
	return s[0]
}

// LogRecord is a thin wrapper around http.ResponseWriter which allows us to
// capture the number of response bytes written and the http status code.
type LogRecord struct {
	http.ResponseWriter
	status        int
	responseBytes int64
}

// Write wrapper that counts bytes
func (r *LogRecord) Write(p []byte) (int, error) {
	written, err := r.ResponseWriter.Write(p)
	r.responseBytes += int64(written)
	return written, err
}

// WriteHeader wrapper that captures status code
func (r *LogRecord) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Hijack wrapper
func (r *LogRecord) Hijack() (rwc net.Conn, buf *bufio.ReadWriter, err error) {
	hj, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		log.Fatal("error: expected a hijacker here")
	}
	return hj.Hijack()
}
