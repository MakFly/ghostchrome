package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dev-toolings/ghostchrome/engine/dashboard"
	"github.com/spf13/cobra"
)

var flagDashboardPort int

var dashboardCmd = &cobra.Command{
	Use:     "dashboard [url]",
	Aliases: []string{"show"},
	Short:   "Open a live browser viewport dashboard",
	Long: `Start a local web dashboard that streams the browser viewport
via WebSocket screencast. Navigate to the dashboard URL in any browser
to see real-time page activity.

Examples:
  ghostchrome dashboard https://example.com --port 8080
  ghostchrome dashboard --connect auto --port 3000`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		b, page := openPage()
		defer b.Close()

		if len(args) > 0 {
			navigateIfRequested(page, args[0], "load")
		}

		dash, addr, err := dashboard.Start(page, flagDashboardPort)
		if err != nil {
			exitErr("dashboard", err)
		}
		defer dash.Stop()

		fmt.Fprintf(os.Stderr, "[dashboard] live at %s\n", addr)

		type dashResult struct {
			URL string `json:"url"`
		}
		output(&dashResult{URL: addr}, fmt.Sprintf("[dashboard] %s", addr))

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		fmt.Fprintln(os.Stderr, "\n[dashboard] shutting down")
	},
}

func init() {
	dashboardCmd.Flags().IntVar(&flagDashboardPort, "port", 0, "Dashboard HTTP port (0 = random)")
	rootCmd.AddCommand(dashboardCmd)
}
