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

// LoggedHandler is an http.Server implementation which multiplexes requests to the
// vhost backends (via a handler) and logs each request.
type LoggedHandler struct {
	*http.ServeMux
	VhostLogListeners map[string]chan string
	vhostMux          *VhostMux
}

// NewLoggedHandler wraps the given handler with a request/response logger
func NewLoggedHandler(vm *VhostMux) *LoggedHandler {
	lh := &LoggedHandler{
		ServeMux:          http.NewServeMux(),
		VhostLogListeners: make(map[string]chan string),
		vhostMux:          vm,
	}
	lh.Handle("/", vm)
	return lh
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
	fmt.Fprintf(w, "%d vhosts:\n", len(lh.vhostMux.Servers))
	for _, v := range lh.vhostMux.Servers {
		fmt.Fprintf(w, "%s -> %s:%d\n", v.Host, v.ServiceHost, v.Port)
	}
}

// Create multi-certificate TLS config from vhost config
func (lh *LoggedHandler) CreateTLSConfig() *tls.Config {
	cfg := &tls.Config{}
	for _, server := range lh.vhostMux.Servers {
		cert, err := tls.LoadX509KeyPair(server.Cert, server.Key)
		if err != nil {
			log.Fatal("failed to load keypair:", err)
		}
		cfg.Certificates = append(cfg.Certificates, cert)
	}
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
