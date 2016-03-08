package main

import (
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

var (
	target   = flag.String("target", "", "Target URL")
	httpAddr = flag.String("listen", ":9001", "HTTP Listen Address")
)

func main() {
	flag.Parse()
	if *target == "" {
		log.Fatal("must specify -target")
	}

	u, err := url.Parse(*target)

	if err != nil {
		log.Fatal(err)
	}

	p := &httputil.ReverseProxy{Director: func(r *http.Request) {
		log.Println(r.URL)
		p, q := r.URL.Path, r.URL.RawQuery
		*r.URL = *u
		r.URL.Path, r.URL.RawQuery = p, q
		r.Host = u.Host
	}}

	log.Fatal(http.ListenAndServe(*httpAddr, p))
}
