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
		if len(args) > 2 {
			_, err := engine.Navigate(page, args[2], "load")
			if err != nil {
				exitErr("navigate", err)
			}
		}

		// Type into the ref.
		err = engine.TypeRef(page, ref, text)
		if err != nil {
			exitErr("type", err)
		}

		// Extract skeleton after type.
		result, err := engine.Extract(page, engine.LevelSkeleton, "")
		if err != nil {
			exitErr("extract after type", err)
		}

		type typeResult struct {
			Action string                  `json:"action"`
			Ref    string                  `json:"ref"`
			Text   string                  `json:"text"`
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
