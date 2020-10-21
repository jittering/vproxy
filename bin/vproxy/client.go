package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
)

func startClientMode(addr string) {
	fmt.Println("[*] found existing daemon, starting in client mode")
	args := flag.Args()

	if len(args) == 0 && *bind == "" {
		log.Fatal("missing vhost binding")
	}

	var cmd *exec.Cmd
	if len(args) > 1 {
		cmd = runCommand(args[1:])
	}

	var binding string
	if len(args) > 0 {
		binding = args[0]
	} else {
		binding = *bind
	}

	uri := fmt.Sprintf("http://%s/_vproxy", addr)
	data := url.Values{}
	data.Add("binding", binding)

	fmt.Println("[*] registering vhost:", binding)
	res, err := http.DefaultClient.PostForm(uri, data)
	if err != nil {
		stopCommand(cmd)
		log.Fatalf("error starting client: %s\n", err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		// catch ^c, cleanup
		<-c
		stopCommand(cmd)
		os.Exit(0)
	}()

	defer res.Body.Close()
	r := bufio.NewReader(res.Body)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			fmt.Printf("error reading from daemon: %s\n", err)
			stopCommand(cmd)
			fmt.Println("exiting")
			os.Exit(0)
		}
		log.Print(line)
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
