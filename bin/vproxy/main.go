package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/jittering/vproxy"
	"github.com/urfave/cli/v2"
)

var listenDefaultAddr = "127.0.0.1"
var listenAnyIP = "0.0.0.0"

func parseFlags() {

	cli.VersionFlag = &cli.BoolFlag{
		Name:    "version",
		Aliases: []string{"V"},
		Usage:   "print the version",
	}

	app := &cli.App{
		Name:    "vproxy",
		Usage:   "zero-config virtual proxies with tls",
		Version: "0.3",

		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Load configuration from `FILE`. Overrides default file detection",
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Verbose output",
			},
		},

		Commands: []*cli.Command{
			{
				Name:    "daemon",
				Aliases: []string{"server", "d", "s"},
				Usage:   "run host daemon",
				Action:  startDaemon,
				Before:  loadDaemonConfig,
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
				Before:  loadClientConfig,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "host",
						Value: "127.0.0.1",
						Usage: "Server host IP",
					},
					&cli.IntFlag{
						Name:  "http",
						Value: 80,
						Usage: "Port to listen for HTTP (0 to disable)",
					},
					&cli.StringFlag{
						Name:  "bind",
						Usage: "Bind hostnames to local ports (e.g., app.local.com:7000)",
					},
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}

}

func verbose(c *cli.Context, a ...interface{}) {
	if c.IsSet("verbose") {
		fmt.Printf("[+] "+a[0].(string)+"\n", a[1:]...)
	}
}

func startClient(c *cli.Context) error {
	host := c.String("host")
	httpPort := c.Int("http")
	bind := c.String("bind")
	if bind == "" {
		return fmt.Errorf("missing bind")
	}

	addr := fmt.Sprintf("%s:%d", host, httpPort)
	if !vproxy.IsDaemonRunning(addr) {
		return errors.New("daemon not running on localhost")
	}

	verbose(c, "Found existing daemon, starting in client mode")
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
