package vproxy

import (
	"fmt"
	"net/http"
)

// StartHello world service on the given host/port
//
// This is a simple service which always responds with 'Hello World'.
// It's mainly here to serve as a simple demo of vproxy's abilities (see readme).
func StartHello(host string, port int) error {
	fmt.Printf("~> starting vproxy hello service at http://%s:%d\n", host, port)
	return http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), http.HandlerFunc(helloHandler))
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	fmt.Fprintf(w, "<h1>Hello World!</h1>")
}
