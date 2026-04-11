package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
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
		l := launcher.New().
			Headless(false).
			HeadlessNew(flagHeadless).
			Set("disable-blink-features", "AutomationControlled").
			Set("window-size", "1920,1080").
			Delete("enable-automation")

		if flagPort > 0 {
			l = l.RemoteDebuggingPort(flagPort)
		}

		wsURL, err := l.Launch()
		if err != nil {
			exitErr("launch chrome", err)
		}

		// Apply stealth to a warm-up page
		if flagStealth {
			b := rod.New().ControlURL(wsURL)
			if err := b.Connect(); err != nil {
				exitErr("connect", err)
			}
			page, err := b.Page(proto.TargetCreateTarget{})
			if err != nil {
				exitErr("page", err)
			}
			if err := engine.ApplyStealth(page); err != nil {
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

func init() {
	serveCmd.Flags().IntVar(&flagPort, "port", 0, "Chrome remote debugging port (0 = random)")
	rootCmd.AddCommand(serveCmd)
}
