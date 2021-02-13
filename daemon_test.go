package vproxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListClients(t *testing.T) {
	request, _ := http.NewRequest("GET", "/events/next/", nil)
	response := httptest.NewRecorder()

	vhostMux := CreateVhostMux([]string{}, true)
	rootMux := NewLoggedHandler(vhostMux)

	// start daemon
	d := NewDaemon(vhostMux, rootMux, "127.0.0.1", 80, 443)

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
}
