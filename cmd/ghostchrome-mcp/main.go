// ghostchrome-mcp is a standalone MCP server binary that exposes the
// ghostchrome browser automation engine over JSON-RPC stdio.
//
// Configuration is via environment variables:
//
//	GHOSTCHROME_CONNECT     — ws:// URL or "auto"
//	GHOSTCHROME_HEADLESS    — "false" to run headful (default true)
//	GHOSTCHROME_STEALTH     — "true" to enable stealth
//	GHOSTCHROME_PROXY       — upstream proxy URL
//	GHOSTCHROME_PROFILE     — persistent profile name
//	GHOSTCHROME_TIMEOUT     — timeout in seconds (default 30)
//	GHOSTCHROME_VAULT_KEY   — encryption key for state vault
//	GHOSTCHROME_MCP_LAZY    — "1" to skip Chrome prewarm
package main

import (
	"fmt"
	"os"
	"strconv"

	enginemcp "github.com/MakFly/ghostchrome/engine/mcp"
	mcpsrv "github.com/mark3labs/mcp-go/server"
)

const version = "0.1.0"

func main() {
	opts := enginemcp.Options{
		Connect:     os.Getenv("GHOSTCHROME_CONNECT"),
		Headless:    envBool("GHOSTCHROME_HEADLESS", true),
		Stealth:     envBool("GHOSTCHROME_STEALTH", false),
		Proxy:       os.Getenv("GHOSTCHROME_PROXY"),
		UserProfile: os.Getenv("GHOSTCHROME_PROFILE"),
		TimeoutSec:  envInt("GHOSTCHROME_TIMEOUT", 30),
	}

	s := enginemcp.New(opts)
	defer s.Close()

	if os.Getenv("GHOSTCHROME_MCP_LAZY") != "1" {
		s.PrewarmAsync()
	}

	fmt.Fprintln(os.Stderr, "[ghostchrome-mcp] ready on stdio")
	if err := mcpsrv.ServeStdio(s.Build("ghostchrome", version)); err != nil {
		fmt.Fprintf(os.Stderr, "mcp server: %v\n", err)
		os.Exit(1)
	}
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
