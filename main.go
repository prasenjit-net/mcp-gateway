package main

import "github.com/prasenjit-net/mcp-gateway/cmd"

func main() {
	cmd.Execute(cmd.Options{
		UIHandler: uiHandler,
	})
}
