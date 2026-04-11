package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	flagConnect        string
	flagHeadless       bool
	flagTimeout        int
	flagFormat         string
	flagStealth        bool
	flagDismissCookies bool
)

var rootCmd = &cobra.Command{
	Use:   "ghostchrome",
	Short: "Ultra-light browser automation CLI for LLM agents",
	Long: `ghostchrome — a single Go binary that controls Chrome via CDP.
Designed for LLM agents: compact output, minimal tokens, zero overhead.

Commands return compact text by default or JSON with --format json.
Use 'ghostchrome serve' to start a persistent Chrome, then --connect to reuse it.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		switch flagFormat {
		case "text", "json":
			return nil
		default:
			return fmt.Errorf("invalid format %q: use text or json", flagFormat)
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagConnect, "connect", "", "WebSocket URL to connect to existing Chrome (e.g. ws://127.0.0.1:9222)")
	rootCmd.PersistentFlags().BoolVar(&flagHeadless, "headless", true, "Run Chrome in headless mode")
	rootCmd.PersistentFlags().IntVar(&flagTimeout, "timeout", 30, "Timeout in seconds for operations")
	rootCmd.PersistentFlags().StringVar(&flagFormat, "format", "text", "Output format: json or text")
	rootCmd.PersistentFlags().BoolVar(&flagStealth, "stealth", false, "Enable stealth mode (hide headless fingerprints)")
	rootCmd.PersistentFlags().BoolVar(&flagDismissCookies, "dismiss-cookies", false, "Auto-dismiss cookie consent banners")
}

func SetVersion(v string) {
	rootCmd.Version = v
}

func Execute() error {
	return rootCmd.Execute()
}

func exitErr(msg string, err error) {
	fmt.Fprintf(os.Stderr, "error: %s: %v\n", msg, err)
	os.Exit(1)
}
