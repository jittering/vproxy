package vproxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	d.updateHosts = func(string) error { return nil }

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
	d.updateHosts = func(string) error { return nil }
	var removedHost string
	d.pruneHosts = func(host string) error { removedHost = host; return nil }

	r := httptest.NewRecorder()
	d.addVhost("foo:8000", r)
	assert.Equal(t, 1, len(lh.vhostMux.Servers))
	assert.NotNil(t, lh.vhostMux.Servers["foo"], "has vhost for foo")
	assert.NotNil(t, lh.vhostMux.Servers["foo"].Handler, "has handler for foo")

	v := d.loggedHandler.GetVhost("foo")
	d.doRemoveVhost(v, r)
	assert.Equal(t, 0, len(lh.vhostMux.Servers))
	assert.Equal(t, "foo", removedHost)
}

func TestRemoveVhostHostsCleanup(t *testing.T) {
	for _, all := range []bool{false, true} {
		t.Run(fmt.Sprintf("all=%t", all), func(t *testing.T) {
			d := newMCPTestDaemon(t)
			d.updateHosts = func(string) error { return nil }
			removed := map[string]bool{}
			d.pruneHosts = func(host string) error {
				removed[host] = true
				return fmt.Errorf("read-only hosts file")
			}
			for _, host := range []string{"one.local", "two.local"} {
				if _, _, err := d.addBinding(host + ":3000"); err != nil {
					t.Fatal(err)
				}
			}
			form := url.Values{"host": {"one.local"}, "all": {fmt.Sprint(all)}}
			req := httptest.NewRequest("POST", "/_vproxy/clients/remove", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			response := httptest.NewRecorder()
			d.removeVhost(response, req)
			assert.True(t, removed["one.local"])
			assert.Equal(t, all, removed["two.local"])
			assert.Contains(t, response.Body.String(), "failed to remove one.local from system hosts file")
			assert.Nil(t, d.loggedHandler.GetVhost("one.local"))
			if all {
				assert.Empty(t, d.loggedHandler.vhostMux.snapshot())
			} else {
				assert.NotNil(t, d.loggedHandler.GetVhost("two.local"))
			}
		})
	}
}
