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
		targetURL := ""
		if len(args) > 1 {
			targetURL = args[1]
		}

		b, page := openPage()
		defer b.Close()

		snapshot := ensureSnapshot(b, page, targetURL, "load", engine.LevelSkeleton)

		err := engine.ClickRef(page, ref, snapshot)
		if err != nil {
			exitIfStaleRef(err, "click")
			exitErr("click", err)
		}

		result := snapshotPage(b, page, engine.LevelSkeleton)

		type clickResult struct {
			Action string                   `json:"action"`
			Ref    string                   `json:"ref"`
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
