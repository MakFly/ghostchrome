package cmd

import (
	"strings"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var flagSelectValues string

var selectCmd = &cobra.Command{
	Use:   "select <ref> <value> [url]",
	Short: "Select option(s) in a <select> element by ref",
	Long: `Select one or more options in a <select> element identified by its @ref.
If a URL is provided, navigates first then selects.
Use --values "a,b,c" for multi-select.
After selecting, extracts a skeleton of the resulting page.

Examples:
  ghostchrome select @5 "option1" https://example.com
  ghostchrome select @5 "" --values "a,b,c" https://example.com`,
	Args: cobra.RangeArgs(2, 3),
	Run: func(cmd *cobra.Command, args []string) {
		ref := args[0]

		var values []string
		if flagSelectValues != "" {
			values = strings.Split(flagSelectValues, ",")
		} else {
			values = []string{args[1]}
		}
		targetURL := ""
		if len(args) > 2 {
			targetURL = args[2]
		}

		b, page := openPage()
		defer b.Close()

		snapshot := ensureSnapshot(b, page, targetURL, "load", engine.LevelSkeleton)

		err := engine.SelectOption(page, ref, values, snapshot)
		if err != nil {
			exitIfStaleRef(err, "select")
			exitErr("select", err)
		}

		result := snapshotPage(b, page, engine.LevelSkeleton)

		type selectResult struct {
			Action string                   `json:"action"`
			Ref    string                   `json:"ref"`
			Values []string                 `json:"values"`
			Result *engine.ExtractionResult `json:"result"`
		}

		text := engine.FormatText(result)
		output(&selectResult{
			Action: "select",
			Ref:    ref,
			Values: values,
			Result: result,
		}, text)
	},
}

func init() {
	selectCmd.Flags().StringVar(&flagSelectValues, "values", "", "Comma-separated values for multi-select")
	rootCmd.AddCommand(selectCmd)
}
