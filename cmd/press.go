package cmd

import (
	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var flagPressOn string

var pressCmd = &cobra.Command{
	Use:   "press <key> [url]",
	Short: "Press a keyboard key",
	Long: `Press a keyboard key (Enter, Tab, Escape, Backspace, ArrowDown, etc.).
If a URL is provided, navigates first then presses.
Use --on @ref to focus an element before pressing.
After pressing, extracts a skeleton of the resulting page.

Examples:
  ghostchrome press Enter https://example.com --on @2
  ghostchrome press Tab
  ghostchrome press Escape --connect ws://...`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		targetURL := ""
		if len(args) > 1 {
			targetURL = args[1]
		}

		b, page := openPage()
		defer b.Close()

		snapshot := ensureSnapshot(b, page, targetURL, "load", engine.LevelSkeleton)

		err := engine.PressKey(page, key, flagPressOn, snapshot)
		if err != nil {
			exitIfStaleRef(err, "press")
			exitErr("press", err)
		}

		result := snapshotPage(b, page, engine.LevelSkeleton)

		type pressResult struct {
			Action string                   `json:"action"`
			Key    string                   `json:"key"`
			On     string                   `json:"on,omitempty"`
			Result *engine.ExtractionResult `json:"result"`
		}

		text := engine.FormatText(result)
		output(&pressResult{
			Action: "press",
			Key:    key,
			On:     flagPressOn,
			Result: result,
		}, text)
	},
}

func init() {
	pressCmd.Flags().StringVar(&flagPressOn, "on", "", "Focus element by @ref before pressing (e.g. @2)")
	rootCmd.AddCommand(pressCmd)
}
