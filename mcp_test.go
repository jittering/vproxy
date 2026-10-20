package vproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newMCPTestDaemon(t *testing.T) *Daemon {
	t.Helper()
	t.Setenv("CERT_PATH", t.TempDir())
	lh := &LoggedHandler{ServeMux: http.NewServeMux(), vhostMux: CreateVhostMux(nil, false)}
	d := NewDaemon(lh, "127.0.0.1", 0, 0)
	d.updateHosts = func(string) error { return fmt.Errorf("test hosts file is read-only") }
	t.Cleanup(func() {
		for _, v := range lh.vhostMux.snapshot() {
			lh.RemoveVhost(v.Host)
		}
	})
	return d
}

func TestMCPTools(t *testing.T) {
	d := newMCPTestDaemon(t)
	d.loggedHandler.Handle("/_vproxy/mcp", d.MCPHandler(""))
	server := httptest.NewServer(d.loggedHandler)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/_vproxy/mcp"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tools, err := session.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 3 {
		t.Fatalf("tools: %v, %v", tools, err)
	}
	call := func(name string, args map[string]any, wantError bool) string {
		t.Helper()
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError != wantError {
			t.Fatalf("%s: unexpected result: %+v", name, result)
		}
		data, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	call("list", nil, false)
	message := call("add", map[string]any{"binding": "app.local:3000"}, false)
	if !strings.Contains(message, "read-only") {
		t.Fatal("hosts-file warning missing from result")
	}
	if result := call("list", nil, false); !strings.Contains(result, "app.local") || !strings.Contains(result, "3000") {
		t.Fatalf("missing registered proxy: %s", result)
	}
	call("add", map[string]any{"binding": "app.local:4000"}, false)
	if v := d.loggedHandler.GetVhost("app.local"); v == nil || v.ServicePort != 4000 {
		t.Fatal("add did not replace existing proxy")
	}
	for _, binding := range []string{"", ":3000", "bad host:3000", "foo:0", "foo:65536", "foo:80:90", "foo..local:80"} {
		call("add", map[string]any{"binding": binding}, true)
	}
	call("add", map[string]any{}, true)
	call("remove", map[string]any{"host": "missing.local"}, true)
	call("remove", map[string]any{"host": "app.local"}, false)
	if len(d.loggedHandler.vhostMux.snapshot()) != 0 {
		t.Fatal("remove left a proxy registered")
	}
	data, err := os.ReadFile(filepath.Join(CertPath(), "vhosts.json"))
	if err != nil || string(data) != "{}" {
		t.Fatalf("removal not persisted: %s, %v", data, err)
	}
}

func TestMCPAuthentication(t *testing.T) {
	d := newMCPTestDaemon(t)
	for _, test := range []struct {
		name, token, auth, origin string
		status                    int
	}{
		{"disabled", "", "", "", 200},
		{"missing", "secret", "", "", 401},
		{"wrong", "secret", "Bearer wrong", "", 401},
		{"wrong scheme", "secret", "Basic secret", "", 401},
		{"valid", "secret", "Bearer secret", "", 200},
		{"case insensitive", "secret", "bearer secret", "", 200},
		{"cross origin", "", "", "https://other.example", 403},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "http://localhost/_vproxy/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			req.Header.Set("Authorization", test.auth)
			req.Header.Set("Origin", test.origin)
			w := httptest.NewRecorder()
			d.MCPHandler(test.token).ServeHTTP(w, req)
			if w.Code != test.status {
				t.Fatalf("status = %d, want %d: %s", w.Code, test.status, w.Body.String())
			}
			if w.Code == 401 && w.Header().Get("WWW-Authenticate") == "" {
				t.Fatal("missing Bearer challenge")
			}
		})
	}
	// Authentication applies to every transport method, not just initialization.
	for _, method := range []string{"GET", "POST", "DELETE"} {
		w := httptest.NewRecorder()
		d.MCPHandler("secret").ServeHTTP(w, httptest.NewRequest(method, "/_vproxy/mcp", nil))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated %s returned %d", method, w.Code)
		}
	}
}
