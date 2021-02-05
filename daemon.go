package vproxy

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"

	"github.com/hairyhenderson/go-which"
	"github.com/mitchellh/go-homedir"
)

// PONG server identifier
const PONG = "hello from vproxy"

type Daemon struct {
	wg    sync.WaitGroup
	vhost *VhostMux
	mux   *LoggedMux

	listen string

	httpPort     int
	httpAddr     string
	httpListener net.Listener

	httpsPort     int
	httpsAddr     string
	httpsListener net.Listener
}

func NewDaemon(vhost *VhostMux, mux *LoggedMux, listen string, httpPort int, httpsPort int) *Daemon {
	return &Daemon{vhost: vhost, mux: mux, listen: listen, httpPort: httpPort, httpsPort: httpsPort}
}

func rerunWithSudo() {
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
	env = append(env, "MKCERT_PATH="+which.Which("mkcert"))
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
			rerunWithSudo()
		}
		log.Fatal(err)
	}
	l.Close()
}

func (d *Daemon) Run() {
	d.httpAddr = fmt.Sprintf("%s:%d", d.listen, d.httpPort)
	d.httpsAddr = fmt.Sprintf("%s:%d", d.listen, d.httpsPort)

	// require running as root if needed
	if d.enableHTTP() && d.httpPort < 1024 {
		testListener(d.httpAddr)
	} else if d.enableTLS() && d.httpsPort < 1024 {
		testListener(d.httpsAddr)
	}

	// ensure CAROOT set properly
	if os.Getenv("CAROOT_PATH") != "" {
		os.Setenv("CAROOT", os.Getenv("CAROOT_PATH"))
	}

	d.mux.HandleFunc("/_vproxy/hello", d.hello)
	d.mux.HandleFunc("/_vproxy/clients", d.listClients)
	d.mux.Handle("/_vproxy", d)
	d.wg.Add(1) // ensure we don't exit immediately

	if d.enableHTTP() {
		fmt.Printf("[*] starting proxy: http://%s\n", d.httpAddr)
		go d.startHTTP()
	}

	if d.enableTLS() {
		fmt.Printf("[*] starting proxy: https://%s\n", d.httpsAddr)
		if len(d.vhost.Servers) > 0 {
			fmt.Printf("    vhosts:\n")
			for _, server := range d.vhost.Servers {
				fmt.Printf("    - %s -> %d\n", server.Host, server.Port)
			}
		}

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

	server := &http.Server{Handler: d.mux, ErrorLog: nullLogger}
	err = server.Serve(d.httpListener)
	// if err != nil {
	// 	fmt.Printf("[*] error: http server exited with error: %s\n", err)
	// }
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
		Handler:   d.mux,
		TLSConfig: createTLSConfig(d.vhost),
		// ErrorLog:  nullLogger,
	}

	err = server.ServeTLS(d.httpsListener, "", "")
	// if err != nil {
	// 	fmt.Printf("[*] error: tls server exited with error: %s\n", err)
	// }
	d.wg.Done()
}

func (d *Daemon) restartTLS() {
	if d.httpsListener != nil {
		d.httpsListener.Close()
	}
	fmt.Println("[*] restarting TLS listener")
	go d.startTLS()
}

func (d *Daemon) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rw := w.(*LogRecord).ResponseWriter
	flusher, ok := rw.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	binding := r.PostFormValue("binding")
	logChan, vhost := d.addVhost(binding, w)
	if vhost == nil {
		return
	}

	defer func() {
		// Remove this client when this handler exits
		fmt.Printf("[*] removing vhost: %s -> %d\n", vhost.Host, vhost.Port)
		d.mux.RemoveLogListener(vhost.Host)
		d.restartTLS()
	}()

	// runs forever until connection closes
	d.relayLogsUntilClose(flusher, logChan, rw, w)
}

func (d *Daemon) relayLogsUntilClose(flusher http.Flusher, logChan chan string, rw http.ResponseWriter, w http.ResponseWriter) {
	// Listen to connection close and un-register logChan
	notify := rw.(http.CloseNotifier).CloseNotify()
	for {
		select {
		case <-notify:
			return
		case line := <-logChan:
			fmt.Fprintln(w, line)
			flusher.Flush()
		}
	}
}

func (d *Daemon) addVhost(binding string, w http.ResponseWriter) (chan string, *Vhost) {
	vhost, err := CreateVhost(binding, d.enableTLS())
	if err != nil {
		fmt.Printf("[*] warning: failed to register new vhost `%s`\n", binding)
		fmt.Printf("    %s\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return nil, nil
	}

	fmt.Printf("[*] registering new vhost: %s -> %d\n", vhost.Host, vhost.Port)
	d.vhost.Servers[vhost.Host] = vhost

	// Set the headers related to event streaming.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	logChan := make(chan string)
	d.mux.AddLogListener(vhost.Host, logChan)
	d.restartTLS()
	return logChan, vhost
}

func (d *Daemon) hello(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	fmt.Fprintln(w, PONG)
}

// listClients currently connected to the vproxy daemon
func (d *Daemon) listClients(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	fmt.Fprintf(w, " %d vhosts:\n", len(d.vhost.Servers))
	for _, v := range d.vhost.Servers {
		fmt.Fprintf(w, "%s -> %s:%d\n", v.Host, v.ServiceHost, v.Port)
	}
}

// Create multi-certificate TLS config from vhost config
func createTLSConfig(vhost *VhostMux) *tls.Config {
	cfg := &tls.Config{}
	for _, server := range vhost.Servers {
		cert, err := tls.LoadX509KeyPair(server.Cert, server.Key)
		if err != nil {
			log.Fatal("failed to load keypair:", err)
		}
		cfg.Certificates = append(cfg.Certificates, cert)
	}
	cfg.BuildNameToCertificate()
	return cfg
}
