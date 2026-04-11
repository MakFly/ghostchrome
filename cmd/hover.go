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
		targetURL := ""
		if len(args) > 1 {
			targetURL = args[1]
		}

		b, page := openPage()
		defer b.Close()

		snapshot := ensureSnapshot(b, page, targetURL, "load", engine.LevelSkeleton)

		err := engine.HoverRef(page, ref, snapshot)
		if err != nil {
			exitIfStaleRef(err, "hover")
			exitErr("hover", err)
		}

		result := snapshotPage(b, page, engine.LevelSkeleton)

		type hoverResult struct {
			Action string                   `json:"action"`
			Ref    string                   `json:"ref"`
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
