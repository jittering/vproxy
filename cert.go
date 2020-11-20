package vproxy

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/icio/mkcert"
	"github.com/mitchellh/go-homedir"
)

func CARootPath() string {
	if cp := os.Getenv("CAROOT_PATH"); cp != "" {
		// override from env
		return cp
	}

	cmd := exec.Command("mkcert", "-CAROOT")
	out, err := cmd.Output()
	if err != nil {
		log.Fatal("failed to get mkcert CA path:", err)
	}
	return strings.TrimSpace(string(out))
}

func CertPath() string {
	if cp := os.Getenv("CERT_PATH"); cp != "" {
		// override from env
		return cp
	}

	// default to user homedir
	d, err := homedir.Dir()
	if err != nil {
		log.Fatalf("failed to locate homedir: %s", err)
	}
	return filepath.Join(d, ".vproxy")
}

// MakeCert for the give hostname, if it doesn't already exist.
func MakeCert(host string) (string, string, error) {
	cp := CertPath()
	err := os.MkdirAll(cp, 0755)
	if err != nil {
		return "", "", err
	}

	certFile := filepath.Join(cp, host+".pem")
	keyFile := filepath.Join(cp, host+"-key.pem")

	if exists(certFile) && exists(keyFile) {
		// nothing to do
		return certFile, keyFile, nil
	}

	if p := os.Getenv("MKCERT_PATH"); p != "" {
		// add location of mkcert bin to PATH
		path := os.Getenv("PATH") + string(os.PathListSeparator) + filepath.Dir(p)
		os.Setenv("PATH", path)
	}

	// generate new cert using mkcert util
	cert, err := mkcert.Exec(mkcert.Domains(host), mkcert.Directory(cp))
	if err != nil {
		return "", "", err
	}

	return cert.File, cert.KeyFile, nil
}

func exists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}
