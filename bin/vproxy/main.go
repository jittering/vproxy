package main

import (
	"fmt"
	"log"
	"os"

	"github.com/jittering/vproxy"
	"github.com/urfave/cli/v2"
)

var listenDefaultAddr = "127.0.0.1"
var listenAnyIP = "0.0.0.0"

func parseFlags() {
	app := &cli.App{
		Name:    "vproxy",
		Usage:   "zero-config virtual proxies with tls",
		Version: "0.3",

		Commands: []*cli.Command{
			{
				Name:    "daemon",
				Aliases: []string{"server", "d", "s"},
				Usage:   "run host daemon",
				Action:  startDaemon,
				Before:  cleanListenAddr,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "listen",
						Aliases: []string{"l"},
						Value:   "127.0.0.1",
						Usage:   "IP to listen on (0 or 0.0.0.0 for all IPs)",
					},
					&cli.IntFlag{
						Name:  "http",
						Value: 80,
						Usage: "Port to listen for HTTP (0 to disable)",
					},
					&cli.IntFlag{
						Name:  "https",
						Value: 443,
						Usage: "Port to listen for HTTP (0 to disable)",
					},
				},
			},
			{
				Name:    "client",
				Aliases: []string{"c"},
				Usage:   "run in client mode",
				Action:  startClient,
				Before:  cleanListenAddr,
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:  "http",
						Value: 80,
						Usage: "Port to listen for HTTP (0 to disable)",
					},
					&cli.StringFlag{
						Name:     "bind",
						Required: true,
						Usage:    "Bind hostnames to local ports (app.local.com:7000)",
					},
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

}

// transform listen addr arg
func cleanListenAddr(c *cli.Context) error {
	listen := c.String("listen")
	if listen == "" {
		c.Set("listen", listenDefaultAddr)
	} else if listen == "0" {
		c.Set("listen", listenAnyIP)
	}
	return nil
}

func startClient(c *cli.Context) error {
	listen := c.String("listen")
	httpPort := c.Int("http")
	addr := fmt.Sprintf("%s:%d", listen, httpPort)

	if !vproxy.IsDaemonRunning(addr) {
		log.Fatal("daemon not running on localhost")
	}

	bind := c.String("bind")

	vproxy.StartClientMode(addr, bind)
	return nil
}

func startDaemon(c *cli.Context) error {
	listen := c.String("listen")
	httpPort := c.Int("http")
	httpsPort := c.Int("https")

	vhostMux := vproxy.CreateVhostMux([]string{}, httpsPort > 0)
	rootMux := vproxy.NewLoggedMux()
	rootMux.Handle("/", vhostMux)

	// start daemon
	d := vproxy.NewDaemon(vhostMux, rootMux, listen, httpPort, httpsPort)
	d.Run()

	return nil
}

func main() {
	parseFlags()
}
