package vproxy

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

// StartHello world service on the given host/port
//
// This is a simple service which always responds with 'Hello World'.
// It's mainly here to serve as a simple demo of vproxy's abilities (see readme).
func StartHello(host string, port int) error {
	fmt.Printf("~> starting vproxy hello service at http://%s:%d\n", host, port)

	// trap signals so we can print before exiting
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		// catch ^c, cleanup
		s := <-c
		if s == nil {
			return
		}
		fmt.Println("~> caught signal:", s)
		os.Exit(0)
	}()

	return http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), http.HandlerFunc(helloHandler))
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	fmt.Fprintf(w, "<h1>Hello World!</h1>")
}
