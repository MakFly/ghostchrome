package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-rod/rod/lib/launcher"
	"github.com/spf13/cobra"
)

var flagPort int

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Launch a persistent Chrome and print the WebSocket debugger URL",
	Run: func(cmd *cobra.Command, args []string) {
		l, err := launcher.New().
			Headless(flagHeadless).
			Set("remote-debugging-port", fmt.Sprintf("%d", flagPort)).
			Launch()
		if err != nil {
			exitErr("launch chrome", err)
		}

		fmt.Println(l)

		// Block until SIGINT or SIGTERM
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig

		fmt.Fprintln(os.Stderr, "\nshutting down")
	},
}

func init() {
	serveCmd.Flags().IntVar(&flagPort, "port", 9222, "Chrome remote debugging port")
	rootCmd.AddCommand(serveCmd)
}
