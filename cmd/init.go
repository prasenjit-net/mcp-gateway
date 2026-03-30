package cmd

import (
	"fmt"
	"os"

	"github.com/prasenjit-net/mcp-gateway/config"
	"github.com/prasenjit-net/mcp-gateway/tlsutil"
)

func runInit(args []string) {
	fs := newFlagSet("init")
	configFile := fs.String("config", config.DefaultConfigFile, "path to config file to create")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: mcp-gateway init [options]

Initialize MCP Gateway by writing a default config file and creating the
data directory. Existing files are not overwritten.

Options:
`)
		fs.PrintDefaults()
	}
	fs.Parse(args) //nolint:errcheck

	// --- config file -------------------------------------------------------
	if _, err := os.Stat(*configFile); err == nil {
		fmt.Printf("config file already exists: %s (skipping)\n", *configFile)
	} else {
		if err := os.WriteFile(*configFile, []byte(config.DefaultConfigContent), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", *configFile, err)
			os.Exit(1)
		}
		fmt.Printf("created config file: %s\n", *configFile)
	}

	// --- data directory ----------------------------------------------------
	// Read data_dir from the newly written (or existing) config so the
	// directory we create matches what the server will actually use.
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not read config to determine data_dir: %v\n", err)
		cfg = config.DefaultConfig()
	}

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating data dir %s: %v\n", cfg.DataDir, err)
		os.Exit(1)
	}
	fmt.Printf("data directory ready: %s\n", cfg.DataDir)

	// --- TLS certificate ---------------------------------------------------
	if err := tlsutil.GenerateSelfSigned(cfg.TLS.CertFile, cfg.TLS.KeyFile); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not generate TLS certificate: %v\n", err)
	} else {
		// Check if they already existed (GenerateSelfSigned is idempotent)
		fmt.Printf("TLS certificate: %s\n", cfg.TLS.CertFile)
		fmt.Printf("TLS key:         %s\n", cfg.TLS.KeyFile)
	}

	fmt.Println()
	fmt.Println("MCP Gateway initialised. Next steps:")
	fmt.Printf("  1. Edit %s to set your listen address, OpenAI key, etc.\n", *configFile)
	fmt.Println("  2. Run:  mcp-gateway serve")
	fmt.Printf("         or: mcp-gateway serve --config %s --port 8080\n", *configFile)
}
