package cmd

import (
	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var clickCmd = &cobra.Command{
	Use:   "click <ref> [url]",
	Short: "Click an interactive element by ref",
	Long: `Click an element identified by its @ref (e.g. @1, @3).
If a URL is provided, navigates first then clicks.
After clicking, extracts a skeleton of the resulting page.

Examples:
  ghostchrome click @1 https://example.com
  ghostchrome click @3 --connect ws://...`,
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
			_, err := engine.Navigate(page, args[1], "load")
			if err != nil {
				exitErr("navigate", err)
			}
		}

		// Click the ref.
		err = engine.ClickRef(page, ref)
		if err != nil {
			exitErr("click", err)
		}

		// Extract skeleton after click.
		result, err := engine.Extract(page, engine.LevelSkeleton, "")
		if err != nil {
			exitErr("extract after click", err)
		}

		type clickResult struct {
			Action string                  `json:"action"`
			Ref    string                  `json:"ref"`
			Result *engine.ExtractionResult `json:"result"`
		}

		text := engine.FormatText(result)
		output(&clickResult{
			Action: "click",
			Ref:    ref,
			Result: result,
		}, text)
	},
}

func init() {
	rootCmd.AddCommand(clickCmd)
}
