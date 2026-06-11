package cmd

import (
	"fmt"
	"os"

	enginemcp "github.com/MakFly/ghostchrome/engine/mcp"
	mcpsrv "github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run ghostchrome as a Model Context Protocol server (stdio)",
	Long: `Start an MCP server that exposes the ghostchrome browser to LLM agents
(Claude Code, Codex, Cursor, ...) over JSON-RPC on stdin/stdout.

The server holds one long-lived Chrome + page across tool calls so refs
(@1, @2, ...) extracted by one tool stay valid for the next. Honours the
same global flags as the regular CLI: --connect, --headless, --user-profile,
--stealth, --dismiss-cookies, --proxy, --timeout.

Wire-up (one-time):

  # Claude Code
  claude mcp add ghostchrome -- ghostchrome mcp --stealth

  # Codex
  codex mcp add ghostchrome -- ghostchrome mcp --stealth

The server speaks MCP 2025-11-25 and exposes 16 tools:

  snapshot       page status + errors + network + DOM with refs (canonical first call)
  navigate       go to URL without snapshot
  click          click @ref
  type           type text into @ref (with submit=true to press Enter)
  select         pick option in <select> by @ref
  press          send a key (Enter, Tab, Escape, ...)
  hover          hover an element by @ref
  drag           drag from one @ref to another
  fill_form      bulk-fill form fields from JSON {ref: value}
  upload         attach files to <input type=file> by @ref
  tabs           list / switch / close browser tabs
  wait_for       wait for selector / text / timeout
  eval           run JS, escape hatch for anything else
  screenshot     WebP/JPEG/PNG of viewport, full page, or element (with annotate)
  back / forward browser history`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		opts := enginemcp.Options{
			Connect:        flagConnect,
			Headless:       flagHeadless,
			Invisible:      flagInvisible,
			UserProfile:    flagUserProfile,
			Stealth:        flagStealth,
			DismissCookies: flagDismissCookies,
			Proxy:          flagProxy,
			TimeoutSec:     flagTimeout,
			BlockTrackers:  flagMCPBlockTrackers,
		}
		s := enginemcp.New(opts)
		defer s.Close()

		// Pre-spawn Chrome in the background while the MCP client is still
		// negotiating capabilities. The 1-2s cold start is hidden from the
		// first user-facing tool call. Lazy mode disabled via GHOSTCHROME_MCP_LAZY=1.
		if os.Getenv("GHOSTCHROME_MCP_LAZY") != "1" {
			s.PrewarmAsync()
		}

		fmt.Fprintln(os.Stderr, "[ghostchrome mcp] ready on stdio (prewarm in background)")
		if err := mcpsrv.ServeStdio(s.Build("ghostchrome", rootCmd.Version)); err != nil {
			fmt.Fprintf(os.Stderr, "mcp server: %v\n", err)
			os.Exit(1)
		}
	},
}

var flagMCPBlockTrackers bool

func init() {
	mcpCmd.Flags().BoolVar(&flagMCPBlockTrackers, "block-trackers", false,
		"Block known anti-bot/fingerprint scripts (DataDome, PerimeterX, ...) at the network layer. Auto-on with --stealth.")
	rootCmd.AddCommand(mcpCmd)
}
