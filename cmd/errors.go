package cmd

import (
	"time"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var (
	flagErrLevel       string
	flagErrWithNetwork bool
)

var errorsCmd = &cobra.Command{
	Use:   "errors [url]",
	Short: "Collect console and network errors from a page",
	Long: `Navigate to a URL and collect JavaScript console errors, uncaught exceptions,
and HTTP 4xx/5xx network errors.

Examples:
  ghostchrome errors https://example.com
  ghostchrome errors https://example.com --level all
  ghostchrome errors https://example.com --level warning --with-network=false`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		targetURL := args[0]

		b, err := engine.NewBrowser(flagConnect, flagHeadless, flagTimeout)
		if err != nil {
			exitErr("browser launch", err)
		}
		defer b.Close()

		page, err := b.Page()
		if err != nil {
			exitErr("create page", err)
		}

		// Start error collector before navigation so it catches everything
		collector := engine.NewErrorCollector(page)

		_, err = engine.Navigate(page, targetURL, "load")
		if err != nil {
			exitErr("navigate", err)
		}

		// Give a brief moment for late-arriving events
		time.Sleep(500 * time.Millisecond)

		allErrors := collector.Errors()

		// Filter errors based on flags
		var filtered []engine.ErrorEntry
		for _, e := range allErrors {
			// Skip network errors if --with-network=false
			if e.Type == "network" && !flagErrWithNetwork {
				continue
			}

			switch flagErrLevel {
			case "error":
				// Only console errors, uncaught exceptions, 5xx
				if e.Type == "console" && e.Level != "error" {
					continue
				}
				if e.Type == "network" && e.Level != "5xx" {
					continue
				}
			case "warning":
				// errors + warnings + 4xx
				// everything passes except nothing to exclude at this level
			case "all":
				// everything
			}

			filtered = append(filtered, e)
		}

		type errorsResult struct {
			Errors []engine.ErrorEntry `json:"errors"`
			Count  int                 `json:"count"`
		}

		if filtered == nil {
			filtered = []engine.ErrorEntry{}
		}

		result := errorsResult{
			Errors: filtered,
			Count:  len(filtered),
		}

		output(result, engine.FormatErrors(filtered))
	},
}

func init() {
	errorsCmd.Flags().StringVar(&flagErrLevel, "level", "error", "Filter level: error, warning, all")
	errorsCmd.Flags().BoolVar(&flagErrWithNetwork, "with-network", true, "Include network errors (4xx/5xx)")
	rootCmd.AddCommand(errorsCmd)
}
