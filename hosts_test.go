package vproxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveFromHostsFile(t *testing.T) {
	for _, test := range []struct {
		name, input, host, want string
	}{
		{"standalone", "# local hosts\n127.0.0.1 localhost\n127.0.0.1 app.local\n", "app.local", "# local hosts\n127.0.0.1 localhost\n"},
		{"shared aliases", "127.0.0.1 localhost app.local other.local\n", "app.local", "127.0.0.1 localhost other.local\n"},
		{"duplicate entries", "127.0.0.1 app.local\n::1 app.local localhost\n", "app.local", "::1 localhost\n"},
		{"missing", "# keep this\n127.0.0.1 localhost\n", "app.local", "# keep this\n127.0.0.1 localhost\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			filename := filepath.Join(t.TempDir(), "hosts")
			if err := os.WriteFile(filename, []byte(test.input), 0600); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				if err := removeFromHostsFile(test.host, filename); err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(filename)
				if err != nil {
					t.Fatal(err)
				}
				// txeh normalizes whitespace when rendering address lines.
				if strings.Join(strings.Fields(string(data)), " ") != strings.Join(strings.Fields(test.want), " ") {
					t.Fatalf("hosts = %q, want %q", data, test.want)
				}
			}
		})
	}
}

func TestRemoveFromHostsFileErrors(t *testing.T) {
	if err := removeFromHostsFile("app.local", filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected missing-file error")
	}
}
