package main

import (
	"github.com/prasenjit-net/mcp-gateway/buildinfo"
	"github.com/prasenjit-net/mcp-gateway/cmd"
)

// version is set at build time via -ldflags="-X main.version=X.Y.Z"
var version = "dev"

func main() {
	buildinfo.Version = version
	cmd.Execute(cmd.Options{
		UIHandler: uiHandler,
	})
}
