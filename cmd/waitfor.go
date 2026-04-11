package cmd

import (
	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var flagWaitforTimeout int

var waitforCmd = &cobra.Command{
	Use:   "waitfor <selector> [url]",
	Short: "Wait for a CSS selector to appear on the page",
	Long: `Wait for an element matching a CSS selector to appear on the page.
If a URL is provided, navigates first then waits.
If found, extracts a skeleton of the resulting page.
If timeout is reached, exits with error.

Examples:
  ghostchrome waitfor "#results" https://example.com
  ghostchrome waitfor ".loaded" --timeout 20
  ghostchrome waitfor "div.content" --connect ws://...`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		selector := args[0]
		targetURL := ""
		if len(args) > 1 {
			targetURL = args[1]
		}

		b, page := openPage()
		defer b.Close()

		navigateIfRequested(page, targetURL, "load")

		err := engine.WaitForSelector(page, selector, flagWaitforTimeout)
		if err != nil {
			exitErr("waitfor", err)
		}

		result := snapshotPage(b, page, engine.LevelSkeleton)

		type waitforResult struct {
			Action   string                   `json:"action"`
			Selector string                   `json:"selector"`
			Result   *engine.ExtractionResult `json:"result"`
		}

		text := engine.FormatText(result)
		output(&waitforResult{
			Action:   "waitfor",
			Selector: selector,
			Result:   result,
		}, text)
	},
}

func init() {
	waitforCmd.Flags().IntVar(&flagWaitforTimeout, "timeout", 10, "Timeout in seconds to wait for selector")
	rootCmd.AddCommand(waitforCmd)
}
