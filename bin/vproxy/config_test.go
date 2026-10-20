package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/urfave/cli/v2"
)

func TestLoadMCPToken(t *testing.T) {
	for _, token := range []string{"", "my-secret-token"} {
		t.Run(token, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "vproxy.toml")
			if err := os.WriteFile(file, []byte("[mcp]\ntoken = \""+token+"\"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			flags := flag.NewFlagSet("test", flag.ContinueOnError)
			flags.String("listen", "127.0.0.1", "")
			app := cli.NewApp()
			ctx := cli.NewContext(app, flags, nil)
			if err := loadConfig(ctx, file); err != nil {
				t.Fatal(err)
			}
			if got, ok := app.Metadata["mcp-token"].(string); !ok || got != token {
				t.Fatalf("MCP token was not loaded correctly")
			}
		})
	}
}
