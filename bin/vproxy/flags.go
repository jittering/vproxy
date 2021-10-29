package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v2"
)

func parseFlags() {

	if version == "" {
		version = "n/a"
	}
	if commit == "" {
		commit = "head"
	}
	if date == "" {
		date = "n/a"
	}

	cli.VersionFlag = &cli.BoolFlag{
		Name:    "version",
		Aliases: []string{"V"},
		Usage:   "print the version",
	}

	app := &cli.App{
		Name:    "vproxy",
		Usage:   "zero-config virtual proxies with tls",
		Version: version,

		CommandNotFound: func(c *cli.Context, cmd string) {
			fmt.Printf("error: unknown command '%s'\n\n", cmd)
			cli.ShowAppHelpAndExit(c, 1)
		},

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
						Usage: "Server HTTP port",
					},
					&cli.StringSliceFlag{
						Name:  "bind",
						Usage: "Bind hostname to local port (e.g., app.local.com:7000)",
					},
				},
			},
			{
				Name:    "list",
				Aliases: []string{"l"},
				Usage:   "list vhosts",
				Action:  listClients,
				Before:  loadClientConfig,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "host",
						Value: "127.0.0.1",
						Usage: "vproxy daemon host IP",
					},
					&cli.IntFlag{
						Name:  "http",
						Value: 80,
						Usage: "vproxy daemon http port",
					},
				},
			},
			{
				Name:   "caroot",
				Usage:  "Print CAROOT path and exit",
				Action: printCAROOT,
				Before: loadClientConfig,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "create",
						Usage: "Initialize and install the CAROOT (will not overwrite existing files)",
					},
					&cli.BoolFlag{
						Name:  "uninstall",
						Usage: "Uninstall the CAROOT (will not remove files)",
					},
					&cli.BoolFlag{
						Name:  "default",
						Usage: "Get the default CAROOT path (ignoring any config or env vars)",
					},
				},
			},
			{
				Name:   "info",
				Usage:  "Print vproxy configuration",
				Before: loadDaemonConfig,
				Action: printInfo,
			},
			{
				Name:   "hello",
				Usage:  "Start a simple Hello World http service",
				Action: startHello,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "host",
						Value: "127.0.0.1",
						Usage: "Host or IP to bind on",
					},
					&cli.IntFlag{
						Name:  "port",
						Value: 8888,
						Usage: "Port to listen on",
					},
				},
			},
			{
				Name:   "version",
				Usage:  "print the version",
				Action: printVersion,
			},
		},
	}

	cli.VersionPrinter = func(c *cli.Context) {
		printVersion(c)
	}

	err := app.Run(os.Args)
	if err != nil && !strings.Contains(err.Error(), "flag provided") {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
}
