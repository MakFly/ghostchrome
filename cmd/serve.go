package cmd

import (
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

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
		skipImplicitDaemon = true
		opts := buildBrowserOpts()
		wsURL, err := engine.NewLauncher(engine.LauncherOpts{
			Headless:       flagHeadless,
			Invisible:      flagInvisible,
			RemotePort:     flagPort,
			UserDataDir:    opts.UserDataDir,
			Proxy:          opts.Proxy,
			ProxyBypass:    opts.ProxyBypass,
			Extensions:     opts.Extensions,
			ExecutablePath: opts.ExecutablePath,
			Args:           opts.LaunchArgs,
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

		// Resolve the CDP port so we can detect Chrome dying and not linger as
		// an orphan serve. flagPort is set in session mode; otherwise parse it
		// from the WebSocket URL.
		port := flagPort
		if port == 0 {
			if u, perr := url.Parse(wsURL); perr == nil {
				if p, aerr := strconv.Atoi(u.Port()); aerr == nil {
					port = p
				}
			}
		}

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-sig:
				fmt.Fprintln(os.Stderr, "\nshutting down")
				return
			case <-ticker.C:
				// If Chrome is gone, exit instead of lingering with no browser.
				if port > 0 {
					if _, err := engine.DiscoverCDP([]int{port}, 1500*time.Millisecond); err != nil {
						fmt.Fprintln(os.Stderr, "chrome no longer reachable — serve exiting")
						return
					}
				}
			}
		}
	},
}

func warmUpStealth(wsURL string) error {
	b := rod.New().ControlURL(wsURL)
	if err := b.Connect(); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	// NOTE: do NOT call b.Close() — that sends `Browser.close` to CDP and kills
	// the long-lived Chrome we just launched. The Rod connection cleans itself
	// up when the process exits.

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
