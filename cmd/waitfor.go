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

		b, err := engine.NewBrowser(flagConnect, flagHeadless, flagTimeout)
		if err != nil {
			exitErr("browser", err)
		}
		defer b.Close()

		page, err := b.Page()
		if err != nil {
			exitErr("page", err)
		}

		// If URL provided, navigate first.
		if len(args) > 1 {
			applyStealthIfNeeded(page)
			_, err := engine.Navigate(page, args[1], "load")
			if err != nil {
				exitErr("navigate", err)
			}
			dismissCookiesIfNeeded(page)
		}

		// Wait for the selector.
		err = engine.WaitForSelector(page, selector, flagWaitforTimeout)
		if err != nil {
			exitErr("waitfor", err)
		}

		// Extract skeleton after element found.
		result, err := engine.Extract(page, engine.LevelSkeleton, "")
		if err != nil {
			exitErr("extract after waitfor", err)
		}

		type waitforResult struct {
			Action   string                  `json:"action"`
			Selector string                  `json:"selector"`
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
