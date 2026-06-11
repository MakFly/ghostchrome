package cmd

import (
	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var dblclickCmd = &cobra.Command{
	Use:   "dblclick [ref] [url]",
	Short: "Double-click an interactive element by ref",
	Long: `Double-click the element identified by its @ref (from the last snapshot).
If a URL is provided after the ref, it is navigated first.
After double-clicking, extracts a skeleton of the resulting page.

Examples:
  ghostchrome dblclick @3 --connect ws://127.0.0.1:9222
  ghostchrome dblclick @1 https://example.com`,
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

		if err := engine.DblClickRef(page, ref, snapshot); err != nil {
			exitIfStaleRef(err, "dblclick")
			exitErr("dblclick", err)
		}

		result := snapshotPage(b, page, engine.LevelSkeleton)
		text := engine.FormatTextProfile(result, renderProfile())
		output(&actionResult{
			Action: "dblclick",
			Ref:    ref,
			Result: result,
		}, text)
	},
}

func init() {
	rootCmd.AddCommand(dblclickCmd)
}
