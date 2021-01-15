package main

import (
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/pelletier/go-toml"
	"github.com/urfave/cli/v2"
)

// Config file fields for vproxy
type Config struct {
	Verbose bool

	Server struct {
		Verbose bool

		Listen string
		HTTP   int
		HTTPS  int

		CaRootPath string `toml:"caroot_path"`
		CertPath   string `toml:"cert_path"`
		MkcertPath string `toml:"mkcert_path"`
	}

	Client struct {
		Verbose bool

		Host string
		HTTP int
		Bind string
	}
}

func fileExists(name string) bool {
	_, err := os.Stat(name)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}

func findConfig(files ...string) string {
	for _, config := range files {
		if config != "" {
			if fileExists(config) {
				return config
			}
			if strings.Contains(config, ".conf") {
				// look for .toml also
				conf := strings.Replace(config, ".conf", ".toml", 1)
				if fileExists(conf) {
					return conf
				}
			}
		}
	}
	return ""
}

func homeConfPath() string {
	d, err := homedir.Dir()
	if err == nil {
		return path.Join(d, ".vproxy.conf")
	}
	return ""
}

func findClientConfig(path string) string {
	return findConfig(path, ".vproxy.conf", homeConfPath(), "/usr/local/etc/vproxy.conf", "/usr/etc/vproxy.conf")
}

func findDaemonConfig(path string) string {
	return findConfig(path, homeConfPath(), "/usr/local/etc/vproxy.conf", "/usr/etc/vproxy.conf")
}

func loadConfigFile(path string) (*Config, error) {
	t, err := toml.LoadFile(path)
	if err != nil {
		return nil, err
	}
	var conf Config
	err = t.Unmarshal(&conf)
	if err != nil {
		return nil, err
	}
	return &conf, nil
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

func loadClientConfig(c *cli.Context) error {
	conf := findClientConfig(c.String("config"))
	if cf := c.String("config"); c.IsSet("config") && conf != cf {
		log.Fatalf("error: config file not found: %s\n", cf)
	}
	if conf == "" {
		return nil
	}
	verbose(c, "Loading config file %s", conf)
	config, err := loadConfigFile(conf)
	if err != nil {
		return err
	}
	if config != nil {
		if v := (config.Client.Verbose || config.Verbose); v && !c.IsSet("verbose") {
			c.Lineage()[1].Set("verbose", "true")
			verbose(c, "Loading config file %s", conf)
			verbose(c, "via conf: verbose=true")
		}
		if v := config.Client.Host; v != "" && !c.IsSet("host") {
			verbose(c, "via conf: host=%s", v)
			c.Set("host", v)
		}
		if v := config.Client.HTTP; v > 0 && !c.IsSet("http") {
			verbose(c, "via conf: http=%d", v)
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
	conf := findClientConfig(c.String("config"))
	if cf := c.String("config"); c.IsSet("config") && conf != cf {
		log.Fatalf("error: config file not found: %s\n", cf)
	}
	if conf == "" {
		return nil
	}

	verbose(c, "Loading config file %s", conf)
	config, err := loadConfigFile(conf)
	if err != nil {
		return err
	}

	if config != nil {
		if v := (config.Server.Verbose || config.Verbose); v && !c.IsSet("verbose") {
			c.Lineage()[1].Set("verbose", "true")
			verbose(c, "Loading config file %s", conf)
			verbose(c, "via conf: verbose=true")
		}
		if v := config.Server.Listen; v != "" && !c.IsSet("listen") {
			verbose(c, "via conf: listen=%s", v)
			c.Set("listen", v)
		}
		if v := config.Server.HTTP; v > 0 && !c.IsSet("http") {
			verbose(c, "via conf: http=%d", v)
			c.Set("http", strconv.Itoa(v))
		}
		if v := config.Server.HTTPS; v > 0 && !c.IsSet("https") {
			verbose(c, "via conf: https=%d", v)
			c.Set("https", strconv.Itoa(v))
		}
		if v := config.Server.CaRootPath; v != "" {
			os.Setenv("CAROOT_PATH", v)
		}
		if v := config.Server.CertPath; v != "" {
			os.Setenv("CERT_PATH", v)
		}
		if v := config.Server.MkcertPath; v != "" {
			os.Setenv("MKCERT_PATH", v)
		} else {
			// try adding /usr/local/bin to the path as mkcert may commonly be installed there
			path := os.Getenv("PATH")
			local := "/usr/local/bin"
			if !strings.Contains(path, local) {
				verbose(c, "adding "+local+" to PATH")
				os.Setenv("PATH", local+string(filepath.ListSeparator)+path)
			}
		}
	}
	cleanListenAddr(c)
	return nil
}
