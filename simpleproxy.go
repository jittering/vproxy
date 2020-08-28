package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

var (
	bind           = flag.String("bind", "", "Bind local and remote ports (8000:7000)")
	targetAddr     = flag.String("target", "", "Target URL")
	listenAddr     = flag.String("listen", ":9001", "HTTP Listen Address")
	staticFilePath = flag.String("static", "", "Static files path example: /path/:/staticdirectory")
	targetURL      *url.URL
	cmd            *exec.Cmd
)

var message = `<html>
<body>
<h1>502 Bad Gateway</h1>
<p>Can't connect to upstream server, please try again later.</p>
</body>
</html>`

type ProxyTransport struct {
}

func (t *ProxyTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := http.DefaultTransport.RoundTrip(request)

	if err != nil {
		resp := &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       ioutil.NopCloser(bytes.NewBufferString(message)),
		}

		resp.StatusCode = 503
		resp.Status = "Can't connect to upstream server"
		log.Println("error ", err)
		return resp, nil
	}

	return response, err
}

func parseOpts() {
	flag.Parse()

	if *targetAddr == "" && *bind == "" {
		log.Fatal("must specify -target OR -bind")
	}

	if *targetAddr != "" {
		targetURL, err := url.Parse(*targetAddr)
		if err != nil {
			log.Fatal(err)
		}

		if targetURL.Scheme != "http" && targetURL.Scheme != "https" {
			log.Println(targetURL.Scheme, targetURL.Scheme == "http")
			log.Fatal("target should have protocol, eg: -target http://localhost:8000 ")
		}
	}

	// use bind shorthand
	if *bind != "" {
		if strings.Contains(*bind, ":") {
			s := strings.Split(*bind, ":")
			localPort, err := strconv.Atoi(s[0])
			if err != nil {
				log.Fatal("failed to parse local port:", err)
				os.Exit(1)
			}
			listenAddr = ptr(fmt.Sprintf(":%d", localPort))

			remotePort, err := strconv.Atoi(s[1])
			if err != nil {
				log.Fatal("failed to parse remote port:", err)
				os.Exit(1)
			}
			targetURL = &url.URL{Scheme: "http", Host: fmt.Sprintf("localhost:%d", remotePort)}

		} else {
			remotePort, err := strconv.Atoi(*bind)
			if err != nil {
				log.Fatal("failed to parse remote port:", err)
				os.Exit(1)
			}
			listenAddr = ptr(fmt.Sprintf(":%d", remotePort))
		}
	}
}

func runCommand() {
	args := flag.Args()
	cmd = exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("[*] running command:", cmd)
	err := cmd.Start()
	if err != nil {
		log.Fatal("error starting command: ", err)
	}
}

func main() {
	parseOpts()

	p := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			p, q := r.URL.Path, r.URL.RawQuery
			*r.URL = *targetURL
			r.URL.Path, r.URL.RawQuery = p, q
			r.Host = targetURL.Host
		},
		Transport: &ProxyTransport{},
	}

	mux := NewLoggedMux()
	mux.Handle("/", p)

	if *staticFilePath != "" {
		paths := strings.Split(*staticFilePath, ":")
		fs := http.FileServer(http.Dir(paths[1]))
		mux.Handle(paths[0], http.StripPrefix(paths[0], fs))
	}

	if len(flag.Args()) > 0 {
		runCommand()
	}

	fmt.Printf("[*] starting proxy: http://localhost%s -> %s\n\n", *listenAddr, targetURL)
	log.Fatal(http.ListenAndServe(*listenAddr, mux))
}

func ptr(str string) *string {
	return &str
}
