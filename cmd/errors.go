package cmd

import (
	"github.com/go-rod/rod"

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
		switch flagErrLevel {
		case "error", "warning", "all":
		default:
			exitErr("errors", errInvalidArg("level", flagErrLevel, "error, warning, all"))
		}

		b, page := openPage()
		defer b.Close()

		applyStealthIfNeeded(page)
		allErrors, err := engine.CollectErrors(page, targetURL, "load", func(page *rod.Page) error {
			dismissCookiesIfNeeded(page)
			return nil
		})
		if err != nil {
			exitErr("errors", err)
		}

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
