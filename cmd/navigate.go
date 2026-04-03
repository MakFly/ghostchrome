package cmd

import (
	"fmt"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var (
	flagWait      string
	flagNavExtract string
)

var navigateCmd = &cobra.Command{
	Use:   "navigate <url>",
	Short: "Navigate to a URL and return page info",
	Long: `Navigate to a URL. Optionally extract the DOM in one shot with --extract.

Examples:
  ghostchrome navigate https://example.com
  ghostchrome navigate https://example.com --extract skeleton
  ghostchrome navigate https://example.com --extract content --format json`,
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

		applyStealthIfNeeded(page)

		info, err := engine.Navigate(page, targetURL, flagWait)
		if err != nil {
			exitErr("navigate", err)
		}

		dismissCookiesIfNeeded(page)

		// If --extract is set, also extract DOM
		if flagNavExtract != "" {
			level := engine.ExtractLevel(flagNavExtract)
			result, err := engine.Extract(page, level, "")
			if err != nil {
				exitErr("extract", err)
			}
			text := fmt.Sprintf("[%d] %s — %s (%dms)\n%s", info.Status, info.Title, info.URL, info.TimeMs, engine.FormatText(result))
			type navigateExtractResult struct {
				*engine.PageInfo
				DOM *engine.ExtractionResult `json:"dom"`
			}
			output(&navigateExtractResult{info, result}, text)
			return
		}

		text := fmt.Sprintf("[%d] %s — %s (%dms)", info.Status, info.Title, info.URL, info.TimeMs)
		output(info, text)
	},
}

func init() {
	navigateCmd.Flags().StringVar(&flagWait, "wait", "load", "Wait strategy: load, stable, idle, none")
	navigateCmd.Flags().StringVar(&flagNavExtract, "extract", "", "Also extract DOM: skeleton, content, or full")
	rootCmd.AddCommand(navigateCmd)
}
