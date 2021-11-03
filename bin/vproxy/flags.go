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
		Usage:   "Print the version",
	}
	cli.HelpFlag = &cli.BoolFlag{
		Name:    "help",
		Aliases: []string{"h"},
		Usage:   "Show help (add -v to show all)",
	}

	hideFlags := shouldHideFlags()

	app := &cli.App{
		Name:    "vproxy",
		Usage:   "Zero-config virtual host reverse proxies with TLS, for local development",
		Version: version,

		Description: `# In terminal one (or via service launcher):
vproxy daemon

# In terminal two:
vproxy connect hello.local:8888 -- vproxy hello

		See docs for more https://github.com/jittering/vproxy
		`,

		EnableBashCompletion: true,
		HideHelpCommand:      hideFlags,

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
				Usage:   "Run host daemon",
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
				Name:    "connect",
				Aliases: []string{"client", "c", "add"},
				Usage:   "Add a new vhost",
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
					&cli.BoolFlag{
						Name:  "detach",
						Usage: "Do not stream logs after binding",
					},
				},
			},
			{
				Name:      "tail",
				Aliases:   []string{"stream", "attach"},
				Usage:     "Stream logs for given vhost",
				Action:    tailLogs,
				Before:    loadClientConfig,
				UsageText: `vproxy tail [command options] <hostname>`,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "host",
						Value: "127.0.0.1",
						Usage: "Daemon host IP",
					},
					&cli.IntFlag{
						Name:  "http",
						Value: 80,
						Usage: "Daemon HTTP port",
					},
					&cli.BoolFlag{
						Name:  "no-follow",
						Usage: "Get the most recent logs and exit",
					},
				},
			},
			{
				Name:    "list",
				Aliases: []string{"l"},
				Usage:   "List current vhosts",
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
				Hidden: hideFlags,
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
				Name:   "bash_completion",
				Usage:  "Generate bash completion script",
				Action: genBashCompletion,
				Description: `To use bash completion, add the following to your .bashrc:

	command -v vproxy >/dev/null 2>&1 && eval "$(vproxy bash_completion)"

or add a file to your bash_completion.d:

	vproxy bash_completion > /etc/bash_completion.d/vproxy
				`,
			},
			{
				Name:   "version",
				Usage:  "Print the version",
				Action: printVersion,
				Hidden: hideFlags,
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

// Hide flags unless verbose flag set
func shouldHideFlags() bool {
	for _, f := range os.Args {
		if f == "--verbose" || f == "-v" {
			return false
		}
	}
	return true
}
