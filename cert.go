package simpleproxy

import (
	"log"
	"os"
	"path/filepath"

	"github.com/icio/mkcert"
	"github.com/mitchellh/go-homedir"
)

func CertPath() string {
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
