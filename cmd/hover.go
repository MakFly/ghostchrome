package cmd

import (
	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var hoverCmd = &cobra.Command{
	Use:   "hover <ref> [url]",
	Short: "Hover over an element by ref",
	Long: `Hover over an element identified by its @ref (e.g. @1, @3).
If a URL is provided, navigates first then hovers.
After hovering, extracts a skeleton of the resulting page.

Examples:
  ghostchrome hover @2 https://example.com
  ghostchrome hover @4 --connect ws://...`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		ref := args[0]

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

		// Hover the ref.
		err = engine.HoverRef(page, ref)
		if err != nil {
			exitErr("hover", err)
		}

		// Extract skeleton after hover.
		result, err := engine.Extract(page, engine.LevelSkeleton, "")
		if err != nil {
			exitErr("extract after hover", err)
		}

		type hoverResult struct {
			Action string                  `json:"action"`
			Ref    string                  `json:"ref"`
			Result *engine.ExtractionResult `json:"result"`
		}

		text := engine.FormatText(result)
		output(&hoverResult{
			Action: "hover",
			Ref:    ref,
			Result: result,
		}, text)
	},
}

func init() {
	rootCmd.AddCommand(hoverCmd)
}
