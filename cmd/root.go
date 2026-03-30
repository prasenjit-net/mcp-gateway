package cmd

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/prasenjit-net/mcp-gateway/buildinfo"
)

const usageText = `MCP Gateway — wrap REST APIs as MCP tools

Usage:
  mcp-gateway <command> [options]

Commands:
  init    Create a default config.toml and data directory
  serve   Start the MCP Gateway server

Run 'mcp-gateway <command> --help' for command-specific options.
`

// Options holds dependencies that must be supplied by the main package
// (e.g. the embedded UI handler whose embed.FS lives in package main).
type Options struct {
	// UIHandler returns the http.Handler that serves the embedded admin UI.
	// If nil, serve will skip mounting the UI.
	UIHandler func() http.Handler
}

// Execute parses os.Args and dispatches to the appropriate subcommand.
func Execute(opts Options) {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usageText)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		runInit(os.Args[2:])
	case "serve":
		runServe(os.Args[2:], opts)
	case "version":
		fmt.Printf("mcp-gateway %s\n", buildinfo.Version)
	case "-h", "--help", "help":
		fmt.Print(usageText)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %q\n\n%s", os.Args[1], usageText)
		os.Exit(1)
	}
}

// newFlagSet creates a FlagSet that exits on error.
func newFlagSet(name string) *flag.FlagSet {
	return flag.NewFlagSet(name, flag.ExitOnError)
}
