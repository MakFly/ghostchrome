package cmd

import (
	"fmt"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var (
	flagWait       string
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

		b, page := openPage()
		defer b.Close()

		info := navigateIfRequested(page, targetURL, flagWait)

		// If --extract is set, also extract DOM
		if flagNavExtract != "" {
			level := engine.ExtractLevel(flagNavExtract)
			result := snapshotPage(b, page, level)
			text := fmt.Sprintf("[%d] %s — %s (%dms)\n%s", info.Status, info.Title, info.URL, info.TimeMs, engine.FormatTextProfile(result, renderProfile()))
			type navigateExtractResult struct {
				*engine.PageInfo
				DOM *engine.ExtractionResult `json:"dom"`
			}
			output(&navigateExtractResult{info, result}, text)
			return
		}

		_ = snapshotPage(b, page, engine.LevelSkeleton)

		text := fmt.Sprintf("[%d] %s — %s (%dms)", info.Status, info.Title, info.URL, info.TimeMs)
		output(info, text)
	},
}

func init() {
	navigateCmd.Flags().StringVar(&flagWait, "wait", "load", "Wait strategy: load, stable, idle, none")
	navigateCmd.Flags().StringVar(&flagNavExtract, "extract", "", "Also extract DOM: skeleton, content, or full")
	rootCmd.AddCommand(navigateCmd)
}
