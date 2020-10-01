package main

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

	"github.com/chetan/simpleproxy"
	"github.com/hairyhenderson/go-which"
	"github.com/mitchellh/go-homedir"
)

// PONG server identifier
const PONG = "hello from vproxy"

type daemon struct {
	wg    sync.WaitGroup
	vhost *simpleproxy.VhostMux
	mux   *simpleproxy.LoggedMux

	httpPort     int
	httpAddr     string
	httpListener net.Listener

	httpsPort     int
	httpsAddr     string
	httpsListener net.Listener
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
	env = append(env, "CERT_PATH="+simpleproxy.CertPath())
	env = append(env, "CAROOT="+simpleproxy.CARootPath())

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

func (d *daemon) run() {
	d.httpAddr = fmt.Sprintf("127.0.0.1:%d", d.httpPort)
	d.httpsAddr = fmt.Sprintf("127.0.0.1:%d", d.httpsPort)

	// require running as root if needed
	if d.enableHTTP() && d.httpPort < 1024 {
		testListener(d.httpAddr)
	} else if d.enableTLS() && d.httpsPort < 1024 {
		testListener(d.httpsAddr)
	}

	d.mux.HandleFunc("/_vproxy/hello", d.hello)
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

func (d *daemon) enableHTTP() bool {
	return d.httpPort > 0
}

func (d *daemon) enableTLS() bool {
	return d.httpsPort > 0
}

func (d *daemon) startHTTP() {
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

func (d *daemon) startTLS() {
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

func (d *daemon) restartTLS() {
	if d.httpsListener != nil {
		d.httpsListener.Close()
	}
	fmt.Println("[*] restarting TLS listener")
	go d.startTLS()
}

func (d *daemon) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rw := w.(*simpleproxy.LogRecord).ResponseWriter
	flusher, ok := rw.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	binding := r.PostFormValue("binding")
	vhost, err := simpleproxy.CreateVhost(binding, d.enableTLS())
	if err != nil {
		fmt.Printf("[*] warning: failed to register new vhost `%s`", binding)
		w.WriteHeader(http.StatusBadRequest)
		return
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

	// Remove this client when this handler exits.
	defer func() {
		fmt.Printf("[*] removing vhost: %s -> %d\n", vhost.Host, vhost.Port)
		d.mux.RemoveLogListener(vhost.Host)
		d.restartTLS()
	}()

	// Listen to connection close and un-register logChan
	notify := rw.(http.CloseNotifier).CloseNotify()

	for {
		select {
		case <-notify:
			return
		default:
			fmt.Fprintln(w, <-logChan)
			flusher.Flush()
		}
	}
}

func (d *daemon) hello(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	fmt.Fprintln(w, PONG)
}

// Create multi-certificate TLS config from vhost config
func createTLSConfig(vhost *simpleproxy.VhostMux) *tls.Config {
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
