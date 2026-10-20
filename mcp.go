package vproxy

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type proxyInfo struct {
	Host        string `json:"host"`
	ServiceHost string `json:"service_host"`
	ServicePort int    `json:"service_port"`
}

type listOutput struct {
	Proxies []proxyInfo `json:"proxies"`
}

type addInput struct {
	Binding string `json:"binding" jsonschema:"Hostname and local upstream port, e.g. app.local:3000"`
}

type removeInput struct {
	Host string `json:"host" jsonschema:"Exact proxy hostname to remove, e.g. app.local"`
}

type changeOutput struct {
	Message string `json:"message"`
}

// MCPHandler exposes proxy management over stateless MCP Streamable HTTP.
// An empty token disables authentication; otherwise every request requires it.
func (d *Daemon) MCPHandler(token string) http.Handler {
	s := mcp.NewServer(&mcp.Implementation{Name: "vproxy", Version: "1.0.0"}, nil)
	mcp.AddTool(s, &mcp.Tool{
		Name: "list", Description: "List registered vproxy reverse proxies.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, listOutput, error) {
		out := listOutput{Proxies: []proxyInfo{}}
		for _, v := range d.loggedHandler.vhostMux.snapshot() {
			out.Proxies = append(out.Proxies, proxyInfo{v.Host, v.ServiceHost, v.ServicePort})
		}
		sort.Slice(out.Proxies, func(i, j int) bool { return out.Proxies[i].Host < out.Proxies[j].Host })
		return nil, out, nil
	})
	mcp.AddTool(s, &mcp.Tool{
		Name: "add", Description: "Add or replace a reverse proxy from a hostname to a local port. Persists the binding, updates the hosts file, and generates a certificate when TLS is enabled.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input addInput) (*mcp.CallToolResult, changeOutput, error) {
		if err := validateMCPBinding(input.Binding); err != nil {
			return nil, changeOutput{}, err
		}
		_, message, err := d.addBinding(input.Binding)
		return nil, changeOutput{Message: message}, err
	})
	mcp.AddTool(s, &mcp.Tool{
		Name: "remove", Description: "Remove a registered reverse proxy by hostname and persist the change. Existing hosts-file entries and certificates are retained.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input removeInput) (*mcp.CallToolResult, changeOutput, error) {
		d.managementMu.Lock()
		defer d.managementMu.Unlock()
		v := d.loggedHandler.GetVhost(input.Host)
		if v == nil {
			return nil, changeOutput{}, fmt.Errorf("host %q not found", input.Host)
		}
		d.doRemoveVhost(v, io.Discard)
		return nil, changeOutput{Message: "Removed " + input.Host}, nil
	})
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return s }, &mcp.StreamableHTTPOptions{
		Stateless: true, JSONResponse: true,
	})
	expected := sha256.Sum256([]byte(token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token != "" {
			scheme, value, ok := strings.Cut(r.Header.Get("Authorization"), " ")
			actual := sha256.Sum256([]byte(value))
			if !ok || !strings.EqualFold(scheme, "Bearer") || subtle.ConstantTimeCompare(actual[:], expected[:]) != 1 {
				w.Header().Set("WWW-Authenticate", `Bearer realm="vproxy"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}
		// Native MCP clients omit Origin. Reject cross-origin browser requests.
		if origin := r.Header.Get("Origin"); origin != "" {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			if origin != scheme+"://"+r.Host {
				http.Error(w, "Forbidden origin", http.StatusForbidden)
				return
			}
		}
		handler.ServeHTTP(w, r)
	})
}

func validateMCPBinding(binding string) error {
	host, port, ok := strings.Cut(binding, ":")
	n, err := strconv.Atoi(port)
	if !ok || err != nil || n < 1 || n > 65535 || len(host) > 253 {
		return fmt.Errorf("invalid binding: expected hostname:port with port between 1 and 65535")
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return fmt.Errorf("invalid hostname %q", host)
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return fmt.Errorf("invalid hostname %q", host)
			}
		}
	}
	return nil
}
