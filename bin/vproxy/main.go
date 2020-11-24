package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

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

// transform listen addr arg
func cleanListenAddr(c *cli.Context) {
	listen := c.String("listen")
	if listen == "" {
		c.Set("listen", listenDefaultAddr)
	} else if listen == "0" {
		c.Set("listen", listenAnyIP)
	}
}

func verbose(c *cli.Context, a ...interface{}) {
	if c.IsSet("verbose") {
		fmt.Printf("[+] "+a[0].(string)+"\n", a[1:]...)
	}
}

func loadClientConfig(c *cli.Context) error {
	conf := FindClientConfig(c.String("config"))
	if cf := c.String("config"); c.IsSet("config") && conf != cf {
		log.Fatalf("error: config file not found: %s\n", cf)
	}
	if conf == "" {
		return nil
	}
	verbose(c, "Loading config file %s", conf)
	config, err := LoadConfigFile(conf)
	if err != nil {
		return err
	}
	if config != nil {
		if v := config.Client.Host; v != "" && !c.IsSet("host") {
			verbose(c, "via conf: host=%s", v)
			c.Set("host", v)
		}
		if v := config.Client.Http; v > 0 && !c.IsSet("http") {
			verbose(c, "via conf: http=%s", v)
			c.Set("http", strconv.Itoa(v))
		}
		if v := config.Client.Bind; v != "" && !c.IsSet("bind") {
			verbose(c, "via conf: bind=%s", v)
			c.Set("bind", v)
		}
	}
	return nil
}

func loadDaemonConfig(c *cli.Context) error {
	conf := FindClientConfig(c.String("config"))
	if cf := c.String("config"); c.IsSet("config") && conf != cf {
		log.Fatalf("error: config file not found: %s\n", cf)
	}
	if conf == "" {
		return nil
	}
	verbose(c, "Loading config file %s", conf)
	config, err := LoadConfigFile(conf)
	if err != nil {
		return err
	}
	if config != nil {
		if v := config.Server.Listen; v != "" && !c.IsSet("listen") {
			verbose(c, "via conf: listen=%s", v)
			c.Set("listen", v)
		}
		if v := config.Server.Http; v > 0 && !c.IsSet("http") {
			verbose(c, "via conf: http=%s", v)
			c.Set("http", strconv.Itoa(v))
		}
		if v := config.Server.Https; v > 0 && !c.IsSet("https") {
			verbose(c, "via conf: https=%s", v)
			c.Set("https", strconv.Itoa(v))
		}
	}
	cleanListenAddr(c)
	return nil
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
