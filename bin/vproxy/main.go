package main

import (
	"bufio"
	"errors"
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
	binds := c.StringSlice("bind")
	if len(binds) == 0 {
		return fmt.Errorf("must bind at least one hostname")
	}

	addr := fmt.Sprintf("%s:%d", host, httpPort)
	if !vproxy.IsDaemonRunning(addr) {
		return errors.New("daemon not running on localhost")
	}

	// check all binds
	reBind := regexp.MustCompile("^.*?:[0-9]+$")
	for _, bind := range binds {
		if bind == "" || !reBind.MatchString(bind) {
			return fmt.Errorf("error: invalid binding: '%s' (expected format 'app.local.com:7000')", bind)
		}
	}

	verbose(c, "Found existing daemon, starting in client mode")
	vproxy.StartClientMode(addr, binds, c.Args().Slice())
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

func main() {
	parseFlags()
}
