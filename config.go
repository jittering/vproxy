package vproxy

import (
	"os"
	"path"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/mitchellh/go-homedir"
)

type Config struct {
	Server struct {
	}
	Client struct {
		Listen string
		Http   int
		Bind   string
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

func FindClientConfig(path string) string {
	return findConfig(path, ".vproxy.conf", homeConfPath(), "/usr/local/etc/vproxy.conf", "/usr/etc/vproxy.conf")
}

func FindDaemonConfig(path string) string {
	return findConfig(path, homeConfPath(), "/usr/local/etc/vproxy.conf", "/usr/etc/vproxy.conf")
}

func LoadConfigFile(path string) (*Config, error) {
	var conf Config
	if _, err := toml.DecodeFile(path, &conf); err != nil {
		return nil, err
	}
	return &conf, nil
}
