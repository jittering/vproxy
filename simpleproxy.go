package main

import (
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"io/ioutil"
	"bytes"
	"strings"
	"time"
)

var (
	target   = flag.String("target", "", "Target URL")
	httpAddr = flag.String("listen", ":9001", "HTTP Listen Address")
	staticFilePath = flag.String("static", "", "Static files path example: /path/:/staticdirectory")
)

var message = `<html>
<body>
<h1>502 Bad Gateway</h1>
<p>Can't connect to upstream server, please try again later.</p>
</body>
</html>`


type ProxyTransport struct {

}


type LoggedMux struct {
	*http.ServeMux
}

func NewLoggedMux() *LoggedMux {
	var mux = &LoggedMux{}
	mux.ServeMux = http.NewServeMux()
	return mux
}


type LogRecord struct {
	http.ResponseWriter
	status int
	responseBytes int64
}

func (r *LogRecord) Write(p []byte) (int, error) {
	written, err := r.ResponseWriter.Write(p)
	r.responseBytes += int64(written)
	return written, err
}

func (r *LogRecord) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (mux *LoggedMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	record := &LogRecord{
		ResponseWriter: w,
	}

	startTime := time.Now()
	mux.ServeMux.ServeHTTP(record, r)
	finishTime := time.Now()
	elapsedTime := finishTime.Sub(startTime)
	log.Println(r.RemoteAddr, " ", r.Method, "[", record.status, "]", r.URL, r.ContentLength, elapsedTime)
}


func (t *ProxyTransport) RoundTrip(request *http.Request) (*http.Response, error){
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


func main() {
	flag.Parse()
	if *target == "" {
		log.Fatal("must specify -target")
	}

	u, err := url.Parse(*target)

	if err != nil {
		log.Fatal(err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		log.Println(u.Scheme, u.Scheme == "http")
		log.Fatal("target should have protocol, eg: -target http://localhost:8000 ")
	}

	p := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			p, q := r.URL.Path, r.URL.RawQuery
			*r.URL = *u
			r.URL.Path, r.URL.RawQuery = p, q
			r.Host = u.Host
		},
		Transport: &ProxyTransport {

		},

	}

	mux := NewLoggedMux()
	mux.Handle("/", p)

	if *staticFilePath != "" {
		paths := strings.Split(*staticFilePath, ":")
		fs := http.FileServer(http.Dir(paths[1]))
		mux.Handle(paths[0], http.StripPrefix(paths[0], fs))
	}

	log.Fatal(http.ListenAndServe(*httpAddr, mux))
}
