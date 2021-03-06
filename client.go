package vproxy

import (
	"bufio"
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

func StartClientMode(addr string, bind string, args []string) {
	// run command, if given
	var cmd *exec.Cmd
	if len(args) > 0 {
		cmd = runCommand(args)

		// trap signal for later cleanup
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		go func() {
			// catch ^c, cleanup
			s := <-c
			if s == nil {
				return
			}
			fmt.Println("[*] caught signal:", s)
			stopCommand(cmd)
			os.Exit(0)
		}()
	}

	uri := fmt.Sprintf("http://%s/_vproxy", addr)
	data := url.Values{}
	data.Add("binding", bind)

	fmt.Println("[*] registering vhost:", bind)
	res, err := http.DefaultClient.PostForm(uri, data)
	if err != nil {
		stopCommand(cmd)
		log.Fatalf("error starting client: %s\n", err)
	}

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
