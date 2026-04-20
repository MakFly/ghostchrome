package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/spf13/cobra"
)

var flagPort int

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Launch a persistent Chrome and print the WebSocket debugger URL",
	Long: `Serve launches a long-lived Chrome process and prints the WebSocket URL.
Other ghostchrome commands connect to it via --connect, eliminating the
~4s cold start on every call.

Examples:
  ghostchrome serve --stealth
  ghostchrome serve --stealth --port 9222
  ghostchrome serve --headless=false --stealth

Then in another terminal:
  ghostchrome collect https://... --connect ws://127.0.0.1:9222/...
  ghostchrome preview https://... --connect ws://127.0.0.1:9222/...`,
	Run: func(cmd *cobra.Command, args []string) {
		wsURL, err := engine.NewLauncher(engine.LauncherOpts{
			Headless:   flagHeadless,
			RemotePort: flagPort,
		}).Launch()
		if err != nil {
			exitErr("launch chrome", err)
		}

		// Apply stealth to a warm-up page
		if flagStealth {
			if err := warmUpStealth(wsURL); err != nil {
				exitErr("stealth", err)
			}
		}

		fmt.Fprintf(os.Stderr, "Chrome ready. Connect with:\n")
		fmt.Fprintf(os.Stderr, "  ghostchrome <cmd> --connect '%s'\n\n", wsURL)
		fmt.Println(wsURL)

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig

		fmt.Fprintln(os.Stderr, "\nshutting down")
	},
}

func warmUpStealth(wsURL string) error {
	b := rod.New().ControlURL(wsURL)
	if err := b.Connect(); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer b.Close()

	page, err := b.Page(proto.TargetCreateTarget{})
	if err != nil {
		return fmt.Errorf("page: %w", err)
	}
	defer page.Close()

	return engine.ApplyStealth(page)
}

func init() {
	serveCmd.Flags().IntVar(&flagPort, "port", 0, "Chrome remote debugging port (0 = random)")
	rootCmd.AddCommand(serveCmd)
}
