package cmd

import (
	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var typeCmd = &cobra.Command{
	Use:   "type <ref> <text> [url]",
	Short: "Type text into an interactive element by ref",
	Long: `Type text into an element identified by its @ref (e.g. @2).
If a URL is provided, navigates first then types.
After typing, extracts a skeleton of the resulting page.

Examples:
  ghostchrome type @2 "hello world" https://example.com
  ghostchrome type @1 "search query" --connect ws://...`,
	Args: cobra.RangeArgs(2, 3),
	Run: func(cmd *cobra.Command, args []string) {
		ref := args[0]
		text := args[1]
		targetURL := ""
		if len(args) > 2 {
			targetURL = args[2]
		}

		b, page := openPage()
		defer b.Close()

		snapshot := ensureSnapshot(b, page, targetURL, "load", engine.LevelSkeleton)

		err := engine.TypeRef(page, ref, text, snapshot)
		if err != nil {
			exitIfStaleRef(err, "type")
			exitErr("type", err)
		}

		result := snapshotPage(b, page, engine.LevelSkeleton)

		type typeResult struct {
			Action string                   `json:"action"`
			Ref    string                   `json:"ref"`
			Text   string                   `json:"text"`
			Result *engine.ExtractionResult `json:"result"`
		}

		textOutput := engine.FormatText(result)
		output(&typeResult{
			Action: "type",
			Ref:    ref,
			Text:   text,
			Result: result,
		}, textOutput)
	},
}

func init() {
	rootCmd.AddCommand(typeCmd)
}
