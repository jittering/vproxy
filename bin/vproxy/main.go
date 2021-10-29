package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jittering/truststore"
	"github.com/jittering/vproxy"
	"github.com/urfave/cli/v2"
)

var (
	version string
	commit  string
	date    string
	builtBy string
)

var listenDefaultAddr = "127.0.0.1"
var listenAnyIP = "0.0.0.0"

var reBinding = regexp.MustCompile("^.*?:[0-9]+$")

func verbose(c *cli.Context, a ...interface{}) {
	if c.IsSet("verbose") {
		fmt.Fprintf(os.Stderr, "[+] "+a[0].(string)+"\n", a[1:]...)
	}
}

func printVersion(c *cli.Context) error {
	fmt.Printf("%s version %s (commit: %s, built %s)\n", c.App.Name, c.App.Version, commit, date)
	return nil
}

func startClient(c *cli.Context) error {
	host := c.String("host")
	httpPort := c.Int("http")

	// collect and validate binds
	args := c.Args().Slice()
	binds := c.StringSlice("bind")
	if len(binds) == 0 {
		// see if one was passed as the first arg
		if c.Args().Present() {
			if b := c.Args().First(); b != "" && validateBinding(b) == nil {
				binds = append(binds, b)
			} else {
				return fmt.Errorf("must bind at least one hostname")
			}
			args = c.Args().Tail()
			if len(args) > 0 && args[0] == "--" {
				args = args[1:]
			}

		} else {
			return fmt.Errorf("must bind at least one hostname")
		}
	}
	for _, bind := range binds {
		if err := validateBinding(bind); err != nil {
			return err
		}
	}

	addr := fmt.Sprintf("%s:%d", host, httpPort)
	if !vproxy.IsDaemonRunning(addr) {
		return fmt.Errorf("daemon not running on localhost")
	}

	vproxy.StartClientMode(addr, binds, args)
	return nil
}

func validateBinding(bind string) error {
	if bind == "" || !reBinding.MatchString(bind) {
		return fmt.Errorf("invalid binding: '%s' (expected format 'host:port', e.g., 'app.local.com:7000')", bind)
	}
	return nil
}

func startDaemon(c *cli.Context) error {
	// ensure CAROOT set properly
	if os.Getenv("CAROOT_PATH") != "" {
		os.Setenv("CAROOT", os.Getenv("CAROOT_PATH"))
	}
	err := vproxy.InitTrustStore()
	if err != nil {
		return err
	}

	listen := c.String("listen")
	httpPort := c.Int("http")
	httpsPort := c.Int("https")

	vhostMux := vproxy.CreateVhostMux([]string{}, httpsPort > 0)
	loggedHandler := vproxy.NewLoggedHandler(vhostMux)

	// start daemon
	d := vproxy.NewDaemon(loggedHandler, listen, httpPort, httpsPort)
	d.Run()

	return nil
}

func listClients(c *cli.Context) error {
	host := c.String("host")
	httpPort := c.Int("http")
	addr := fmt.Sprintf("%s:%d", host, httpPort)
	uri := fmt.Sprintf("http://%s/_vproxy/clients", addr)

	res, err := http.DefaultClient.Get(uri)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			fmt.Printf("error listing vhosts: daemon not running?\n")
		} else {
			fmt.Printf("error listing vhosts: %s\n", err)
		}
		os.Exit(1)
	}

	defer res.Body.Close()
	r := bufio.NewReader(res.Body)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if err.Error() == "EOF" {
				os.Exit(0)
			}
			fmt.Printf("error reading from daemon: %s\n", err)
			os.Exit(1)
		}
		fmt.Println(strings.TrimSpace(line))
	}
}

func printCAROOT(c *cli.Context) error {
	if c.Bool("create") {
		return createCAROOT()
	}

	if c.Bool("uninstall") {
		return uninstallCAROOT()
	}

	if c.Bool("default") {
		os.Unsetenv("CAROOT_PATH")
	}
	fmt.Println(vproxy.CARootPath())
	return nil
}

func createCAROOT() error {
	// create new caroot, if needed
	path := vproxy.CARootPath()
	os.Setenv("CAROOT", path)

	if fileExists(filepath.Join(path, "rootCA.pem")) {
		fmt.Printf("CA already exists at %s\n", path)
	} else {
		fmt.Printf("creating new CA at %s\n", path)
	}

	fmt.Printf("\n >> NOTE: you may be prompted to enter your sudo password <<\n\n")

	err := os.MkdirAll(path, 0755)
	if err != nil {
		return fmt.Errorf("failed to create CA path: %v", err)
	}

	truststore.Print = true
	err = vproxy.InitTrustStore()
	if err != nil {
		return fmt.Errorf("failed to init CA: %v", err)
	}

	err = vproxy.InstallTrustStore()
	if err != nil {
		return fmt.Errorf("failed to install CA: %v", err)
	}

	return nil
}

func uninstallCAROOT() error {
	path := vproxy.CARootPath()
	os.Setenv("CAROOT", path)

	if !fileExists(filepath.Join(path, "rootCA.pem")) {
		fmt.Printf("CA not found at %s\n", path)
		return nil
	}

	fmt.Printf("\n >> NOTE: you may be prompted to enter your sudo password <<\n\n")

	truststore.Print = true
	err := vproxy.InitTrustStore()
	if err != nil {
		return fmt.Errorf("failed to load CA: %v", err)
	}

	err = vproxy.UninstallTrustStore()
	if err != nil {
		return fmt.Errorf("failed to uninstall CA: %v", err)
	}

	fmt.Println("successfully uninstalled CA")

	return nil
}

func startHello(c *cli.Context) error {
	return vproxy.StartHello(c.String("host"), c.Int("port"))
}

func printInfo(c *cli.Context) error {
	printVersion(c)
	fmt.Printf("  CAROOT=%s\n", vproxy.CARootPath())
	fmt.Printf("  CERT_PATH=%s\n", vproxy.CertPath())
	certs, _ := filepath.Glob(filepath.Join(vproxy.CertPath(), "*-key.pem"))
	fmt.Printf("\n  Nubmer of installed certs: %d\n", len(certs))
	fmt.Println("  Certs:")
	for _, cert := range certs {
		host := strings.TrimPrefix(strings.TrimSuffix(cert, "-key.pem"), vproxy.CertPath()+string(filepath.Separator))
		fmt.Printf("    %s\n", host)
	}
	return nil
}

func genBashCompletion(c *cli.Context) error {
	comp := `
#! /bin/bash

_cli_bash_autocomplete() {
	if [[ "${COMP_WORDS[0]}" != "source" ]]; then
		local cur opts base
		COMPREPLY=()
		cur="${COMP_WORDS[COMP_CWORD]}"
		if [[ "$cur" == "-"* ]]; then
			opts=$( ${COMP_WORDS[@]:0:$COMP_CWORD} ${cur} --generate-bash-completion | grep -ve '^[a-z]$' )
		else
			opts=$( ${COMP_WORDS[@]:0:$COMP_CWORD} --generate-bash-completion | grep -ve '^[a-z]$' )
		fi
		COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
		return 0
	fi
}

complete -o bashdefault -o default -o nospace -F _cli_bash_autocomplete vproxy
	`
	fmt.Println(comp)
	return nil

}

func main() {
	parseFlags()
}
