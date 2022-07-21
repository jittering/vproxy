package vproxy

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/mattn/go-isatty"
	"github.com/mitchellh/go-homedir"
)

// Controls whether verbose messages should be printed
var VERBOSE = false

// PONG server identifier
const PONG = "hello from vproxy"

// Daemon service which hosts all the virtual reverse proxies
//
// proxy chain:
// daemon -> mux (LoggedHandler) -> /* -> VhostMux -> Vhost -> ReverseProxy -> upstream service
type Daemon struct {
	wg sync.WaitGroup

	loggedHandler *LoggedHandler

	listenHost string

	httpPort     int
	httpAddr     string
	httpListener net.Listener

	httpsPort     int
	httpsAddr     string
	httpsListener net.Listener
}

// NewDaemon
func NewDaemon(lh *LoggedHandler, listen string, httpPort int, httpsPort int) *Daemon {
	return &Daemon{loggedHandler: lh, listenHost: listen, httpPort: httpPort, httpsPort: httpsPort}
}

func rerunWithSudo(addr string) {
	// ensure sudo exists on this OS
	_, err := os.Stat("/usr/bin/sudo")
	if err != nil {
		fmt.Println("[*] error: unable to bind on ", addr)
		log.Fatal("[*] fatal: sudo not found")
		return
	}

	exe, e := os.Executable()
	if e != nil {
		log.Fatal(e)
	}

	fmt.Println("[*] rerunning with sudo")

	args := []string{exe}
	args = append(args, os.Args[1:]...)

	// pass some locations to sudo env
	home, e := homedir.Dir()
	if e != nil {
		log.Fatal(e)
	}
	env := []string{"env", "SUDO_HOME=" + home}
	env = append(env, "CERT_PATH="+CertPath())
	env = append(env, "CAROOT="+CARootPath())

	// use env hack to pass configs into child process inside sudo
	args = append(env, args...)

	e = syscall.Exec("/usr/bin/sudo", args, nil)
	if e != nil {
		log.Fatal(e)
	}
	os.Exit(0)
}

func testListener(addr string) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		if strings.Contains(err.Error(), "permission denied") {
			svc := os.Getenv("XPC_SERVICE_NAME")
			if svc == "homebrew.mxcl.vproxy" || strings.Contains(svc, "vproxy") {
				fmt.Println("[*] warning: looks like we are running via launchd; won't try rerunning with sudo")
				fmt.Println("[*]          instead, run the service as root. see docs for help.")
			} else if !isatty.IsTerminal(os.Stdin.Fd()) {
				fmt.Println("[*] warning: looks like we are running as a headless daemon; won't try rerunning with sudo")
				fmt.Println("[*]          instead, run the service as root. see docs for help.")
			} else {
				rerunWithSudo(addr)
			}
		}
		log.Fatal(err)
	}
	l.Close()
}

// Run the daemon service. Does not return until the service is killed.
func (d *Daemon) Run() {
	d.httpAddr = fmt.Sprintf("%s:%d", d.listenHost, d.httpPort)
	d.httpsAddr = fmt.Sprintf("%s:%d", d.listenHost, d.httpsPort)

	// require running as root if needed
	if d.enableHTTP() && d.httpPort < 1024 {
		testListener(d.httpAddr)
	}
	if d.enableTLS() && d.httpsPort < 1024 {
		testListener(d.httpsAddr)
	}

	d.loggedHandler.HandleFunc("/_vproxy/hello", d.hello)
	d.loggedHandler.HandleFunc("/_vproxy/clients", d.listClients)
	d.loggedHandler.HandleFunc("/_vproxy/clients/add", d.registerVhost)
	d.loggedHandler.HandleFunc("/_vproxy/clients/stream", d.streamLogs)
	d.loggedHandler.HandleFunc("/_vproxy/clients/remove", d.removeVhost)
	d.wg.Add(1) // ensure we don't exit immediately

	if d.enableHTTP() {
		fmt.Printf("[*] starting proxy: http://%s\n", d.httpAddr)
		go d.startHTTP()
	}

	if d.enableTLS() {
		fmt.Printf("[*] starting proxy: https://%s\n", d.httpsAddr)
		d.loggedHandler.DumpServers(os.Stdout)
		go d.startTLS()
	}

	d.wg.Wait()
}

func (d *Daemon) enableHTTP() bool {
	return d.httpPort > 0
}

func (d *Daemon) enableTLS() bool {
	return d.httpsPort > 0
}

func (d *Daemon) startHTTP() {
	d.wg.Add(1)
	var err error
	d.httpListener, err = net.Listen("tcp", d.httpAddr)
	if err != nil {
		log.Fatalf("failed to start listener: %s", err)
	}

	null, _ := os.Open(os.DevNull)
	nullLogger := log.New(null, "", 0)
	defer null.Close()

	server := &http.Server{Handler: d.loggedHandler, ErrorLog: nullLogger}
	server.Serve(d.httpListener)
	d.wg.Done()
}

func (d *Daemon) startTLS() {
	d.wg.Add(1)
	var err error
	d.httpsListener, err = net.Listen("tcp", d.httpsAddr)
	if err != nil {
		log.Fatalf("failed to start listener: %s", err)
	}

	// null, _ := os.Open(os.DevNull)
	// nullLogger := log.New(null, "", 0)
	// defer null.Close()

	server := http.Server{
		Handler:   d.loggedHandler,
		TLSConfig: d.loggedHandler.CreateTLSConfig(),
		// ErrorLog:  nullLogger,
	}

	server.ServeTLS(d.httpsListener, "", "")
	d.wg.Done()
}

func (d *Daemon) restartTLS() {
	if d.httpsListener != nil {
		d.httpsListener.Close()
	}
	fmt.Println("[*] restarting TLS listener")
	go d.startTLS()
}

// registerVhost handler creates and starts a new vhost reverse proxy
func (d *Daemon) registerVhost(w http.ResponseWriter, r *http.Request) {
	binding := r.PostFormValue("binding")
	d.addVhost(binding, w)
}

// streamLogs for a given hostname back to the caller. Runs forever until client
// disconnects.
func (d *Daemon) streamLogs(w http.ResponseWriter, r *http.Request) {
	hostname := r.PostFormValue("host")
	vhost := d.loggedHandler.GetVhost(hostname)
	if vhost == nil {
		fmt.Fprintf(w, "[*] error: host '%s' not found", hostname)
		return
	}

	// runs forever until connection closes
	d.relayLogsUntilClose(vhost, w, r.Context())
}

func (d *Daemon) relayLogsUntilClose(vhost *Vhost, w http.ResponseWriter, reqCtx context.Context) {
	flusher, ok := w.(*LogRecord).ResponseWriter.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	// initial flush to open the stream
	fmt.Fprint(w, "")
	flusher.Flush()

	logChan := vhost.NewLogListener()

	// read existing logs first
	buff := vhost.BufferAsString()
	if buff != "" {
		fmt.Fprint(w, buff)
		fmt.Fprintln(w, "---")
	}

	// Listen to connection close and un-register logChan
	for {
		select {
		case <-reqCtx.Done():
			vhost.RemoveLogListener(logChan)
			return
		case line := <-logChan:
			fmt.Fprintln(w, line)
			flusher.Flush()
		}
	}
}

func (d *Daemon) removeVhost(w http.ResponseWriter, r *http.Request) {
	hostname := r.PostFormValue("host")
	all, _ := strconv.ParseBool(r.PostFormValue("all"))

	if all {
		for _, vhost := range d.loggedHandler.vhostMux.Servers {
			d.doRemoveVhost(vhost, w)
		}

	} else if hostname != "" {
		vhost := d.loggedHandler.GetVhost(hostname)
		if vhost == nil {
			fmt.Fprintf(w, "error: host '%s' not found", hostname)
			return
		}
		d.doRemoveVhost(vhost, w)

	} else {
		fmt.Fprint(w, "error: missing hostname")
		return
	}

}

func (d *Daemon) doRemoveVhost(vhost *Vhost, w http.ResponseWriter) {
	fmt.Printf("[*] removing vhost: %s -> %d\n", vhost.Host, vhost.Port)
	fmt.Fprintf(w, "removing vhost: %s -> %d\n", vhost.Host, vhost.Port)
	vhost.Close()
	d.loggedHandler.RemoveVhost(vhost.Host)
}

// addVhost for the given binding to the LoggedHandler
func (d *Daemon) addVhost(binding string, w http.ResponseWriter) *Vhost {
	vhost, err := CreateVhost(binding, d.enableTLS())
	if err != nil {
		fmt.Printf("[*] warning: failed to register new vhost `%s`\n", binding)
		fmt.Printf("    %s\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}

	// remove any existing vhost
	if v := d.loggedHandler.GetVhost(vhost.Host); v != nil {
		fmt.Printf("[*] removing existing vhost: %s -> %d\n", v.Host, v.Port)
		v.Close()
		d.loggedHandler.RemoveVhost(vhost.Host)
	}

	fmt.Printf("[*] registering new vhost: %s -> %d\n", vhost.Host, vhost.Port)

	// Set the headers related to event streaming.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	d.loggedHandler.AddVhost(vhost)
	if d.enableTLS() {
		d.restartTLS()
	}

	err = addToHosts(vhost.Host)
	if err != nil {
		msg := fmt.Sprintf("[*] warning: failed to add %s to system hosts file: %s\n", vhost.Host, err)
		fmt.Println(msg)
		fmt.Fprintln(w, msg)
	}

	fmt.Fprintf(w, "[*] added vhost: %s", binding)

	return vhost
}

func (d *Daemon) hello(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	fmt.Fprintln(w, PONG)
}

// listClients currently connected to the vproxy daemon
func (d *Daemon) listClients(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	d.loggedHandler.DumpServers(w)
}
