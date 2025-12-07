package vproxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var temp = ""

func setup() error {
	var err error
	temp, err = os.MkdirTemp("", "vproxy")
	if err != nil {
		return err
	}
	fmt.Println("using temp dir:", temp)
	os.Setenv("CERT_PATH", temp)
	os.Setenv("CAROOT_PATH", temp)
	os.Setenv("CAROOT", temp)
	err = InitTrustStore()
	if err != nil {
		return err
	}
	return nil
}

func reset() {
	os.Remove(path.Join(temp, "vhosts.json"))
}

func listTempDir() {
	files, err := os.ReadDir(temp)
	if err != nil {
		fmt.Println("error reading temp dir:", err)
		return
	}
	fmt.Println("temp dir contents:")
	for _, f := range files {
		fmt.Println(" -", f.Name())
	}
}

func teardown() {
	os.RemoveAll(temp)
}

func TestMain(m *testing.M) {
	err := setup()
	if err != nil {
		fmt.Println("setup error:", err)
		os.Exit(1)
	}

	code := m.Run()

	teardown()

	os.Exit(code)
}

func TestListClients(t *testing.T) {
	reset()
	request, _ := http.NewRequest("GET", "/events/next/", nil)
	response := httptest.NewRecorder()

	vhostMux := CreateVhostMux([]string{}, true)
	lh := NewLoggedHandler(vhostMux)

	// start daemon
	d := NewDaemon(lh, "127.0.0.1", 0, 0)

	d.listClients(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("Non-expected status code%v:\n\tbody: %v", "200", response.Code)
	}

	response = httptest.NewRecorder()
	d.addVhost("test.local.com:8888", httptest.NewRecorder())
	d.listClients(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("Non-expected status code%v:\n\tbody: %v", "200", response.Code)
	}
	res := response.Body.String()
	if len(strings.Split(res, "\n")) < 3 {
		t.Fatalf("response too short:\n%s", res)
	}
	if !strings.Contains(res, "test.local.com") {
		t.Fatalf("list does not contain test.local.com:\n%s", response.Body.String())
	}
	d.Shutdown()
}

func TestAddRemoveVhost(t *testing.T) {
	reset()
	vhostMux := CreateVhostMux([]string{}, true)
	lh := NewLoggedHandler(vhostMux)
	assert.Equal(t, 0, len(lh.vhostMux.Servers))

	d := NewDaemon(lh, "", 0, 0)

	r := httptest.NewRecorder()
	d.addVhost("foo:8000", r)
	assert.Equal(t, 1, len(lh.vhostMux.Servers))
	assert.NotNil(t, lh.vhostMux.Servers["foo"], "has vhost for foo")
	assert.NotNil(t, lh.vhostMux.Servers["foo"].Handler, "has handler for foo")

	v := d.loggedHandler.GetVhost("foo")
	d.doRemoveVhost(v, r)
	assert.Equal(t, 0, len(lh.vhostMux.Servers))
}
