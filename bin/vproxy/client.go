package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
)

func startClientMode(addr string) {
	fmt.Println("[*] found existing daemon, starting in client mode")
	binding := flag.Args()[0]

	uri := fmt.Sprintf("http://%s/_vproxy", addr)
	data := url.Values{}
	data.Add("binding", binding)

	fmt.Println("[*] registering vhost:", binding)
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
