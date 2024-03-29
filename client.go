package vproxy

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
)

type Client struct {
	Addr string
	cmd  *exec.Cmd
	wg   *sync.WaitGroup
}

func (c *Client) uri(path string) string {
	return fmt.Sprintf("http://%s/_vproxy%s", c.Addr, path)
}

func (c *Client) AddBindings(binds []string, detach bool, args []string) {
	if len(binds) == 0 {
		fmt.Println("error: must bind at least one hostname")
		os.Exit(1)
	}

	c.RunCommand(args)

	c.wg = &sync.WaitGroup{}
	for _, bind := range binds {
		c.wg.Add(1)
		go c.AddBinding(bind, detach)
		c.wg.Wait()
	}

	// c.wg.Add(1)
	c.wg.Wait()
}

func (c *Client) RunCommand(args []string) {
	// run command, if given
	if len(args) == 0 {
		return
	}
	c.cmd = runCommand(args)

	// trap signal for later cleanup
	cs := make(chan os.Signal, 1)
	signal.Notify(cs, os.Interrupt)
	go func() {
		// catch ^c, cleanup
		s := <-cs
		if s == nil {
			return
		}
		fmt.Println("[*] caught signal:", s)
		stopCommand(c.cmd)
		os.Exit(0)
	}()
}

// Add single binding. Blocks when detach=false
func (c *Client) AddBinding(bind string, detach bool) {
	data := url.Values{}
	data.Add("binding", bind)

	s := strings.Split(bind, ":")
	if len(s) >= 2 {
		fmt.Printf("[*] registering vhost: https://%s -> %s\n", s[0], bind)
	} else {
		fmt.Println("[*] registering vhost:", bind)
	}
	res, err := http.DefaultClient.PostForm(c.uri("/clients/add"), data)
	if err != nil {
		stopCommand(c.cmd)
		log.Fatalf("error registering client: %s\n", err)
	}
	b, err := io.ReadAll(res.Body)
	if err == nil {
		fmt.Println(string(b))
	}
	res.Body.Close()

	if detach {
		c.wg.Done()
	} else {
		c.Tail(s[0], true)
	}
}

func (c *Client) Tail(hostname string, follow bool) {
	data := url.Values{}
	data.Add("host", hostname)
	res, err := http.DefaultClient.PostForm(c.uri("/clients/stream"), data)
	if err != nil {
		log.Fatalf("error: %s\n", err)
	}
	fmt.Printf("[*] streaming logs for %s\n", hostname)
	streamLogs(res, follow)
}

func streamLogs(res *http.Response, follow bool) {
	defer res.Body.Close()
	r := bufio.NewReader(res.Body)
	for {
		line, err := r.ReadString('\n')
		if line == "---\n" && !follow {
			os.Exit(0)
		}
		if err != nil {
			if line != "" && strings.Contains(line, "error") {
				fmt.Println(line)
				os.Exit(1)
			} else if err.Error() == "EOF" {
				fmt.Println("[*] daemon connection closed")
			} else {
				fmt.Printf("error reading from daemon: %s\n", err)
				fmt.Println("exiting")
			}
			os.Exit(0)
		}

		fmt.Print(line)
	}
}

func (c *Client) RemoveVhost(hostname string, all bool) {
	data := url.Values{}
	data.Add("host", hostname)
	data.Add("all", strconv.FormatBool(all))
	res, err := http.DefaultClient.PostForm(c.uri("/clients/remove"), data)
	if err != nil {
		log.Fatalf("error: %s\n", err)
	}
	defer res.Body.Close()
	r := bufio.NewReader(res.Body)
	b, _ := ioutil.ReadAll(r)
	s := strings.TrimSpace(string(b))
	if s != "" {
		fmt.Println(s)
	}
}

// IsDaemonRunning tries to check if a vproxy daemon is already running on the given addr
func (c *Client) IsDaemonRunning() bool {
	res, err := http.DefaultClient.Get(c.uri("/hello"))
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
