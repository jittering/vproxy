package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/chetan/simpleproxy"
)

var (
	bind      = flag.String("bind", "", "Bind hostnames to local ports (app.local.com:7000)")
	httpPort  = flag.Int("http", 80, "Port to listen for HTTP (0 to disable)")
	httpsPort = flag.Int("https", 443, "Port to listen for TLS (HTTPS) (0 to disable)")
)

var message = `<html>
<body>
<h1>502 Bad Gateway</h1>
<p>Can't connect to upstream server, please try again later.</p>
</body>
</html>`

func main() {
	flag.Parse()

	// if *bind == "" && len(flag.Args()) == 0 {
	// 	log.Fatal("must specify -bind")
	// }

	addr := fmt.Sprintf("127.0.0.1:%d", *httpPort)
	if IsDaemonRunning(addr) {
		startClientMode(addr)
		return
	}

	// create handlers
	bindings := strings.Split(*bind, " ")
	bindings = append(bindings, flag.Args()...)
	vhost := simpleproxy.CreateVhostMux(bindings, *httpsPort > 0)
	mux := simpleproxy.NewLoggedMux()
	mux.Handle("/", vhost)

	// start daemon
	d := &daemon{vhost: vhost, mux: mux, httpPort: *httpPort, httpsPort: *httpsPort}
	d.run()
}

func startClientMode(addr string) {
	binding := flag.Args()[0]

	uri := fmt.Sprintf("http://%s/_vproxy", addr)
	data := url.Values{}
	data.Add("binding", binding)

	res, err := http.DefaultClient.PostForm(uri, data)
	if err != nil {
		log.Fatalf("error starting client: %s\n", err)
	}

	defer res.Body.Close()
	r := bufio.NewReader(res.Body)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			fmt.Printf("error reading from daemon: %s\n", err)
			fmt.Println("exiting")
			return
		}
		fmt.Print(line)
	}
}

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

func (d *daemon) run() {
	d.mux.HandleFunc("/_vproxy/hello", d.hello)
	d.mux.Handle("/_vproxy", d)
	d.wg.Add(1) // ensure we don't exit immediately

	if d.enableHTTP() {
		d.httpAddr = fmt.Sprintf("127.0.0.1:%d", d.httpPort)
		fmt.Printf("[*] starting proxy: http://%s\n", d.httpAddr)
		go d.startHTTP()
	}

	if d.enableTLS() {
		d.httpsAddr = fmt.Sprintf("127.0.0.1:%d", d.httpsPort)
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

	null, _ := os.Open(os.DevNull)
	nullLogger := log.New(null, "", 0)
	defer null.Close()

	server := http.Server{
		Handler:   d.mux,
		TLSConfig: createTLSConfig(d.vhost),
		ErrorLog:  nullLogger,
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
		fmt.Println("flushing not supported..?")
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

// IsDaemonRunning tries to check if a vproxy daemon is already running on the given addr
func IsDaemonRunning(addr string) bool {
	res, err := http.DefaultClient.Get(fmt.Sprintf("http://%s/_vproxy/hello", addr))
	if err != nil || res.StatusCode != 200 {
		return false
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(body)) == PONG
}
