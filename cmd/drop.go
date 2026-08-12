package cmd

import (
	"fmt"
	"strings"

	"github.com/dev-toolings/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var (
	flagDropPaths []string
	flagDropData  []string
)

var dropCmd = &cobra.Command{
	Use:   "drop <target>",
	Short: "Drop files or data onto an element",
	Long: `Dispatch dragenter, dragover, and drop events on an element. Unlike upload,
drop works on arbitrary elements and populates event.dataTransfer.

TARGET accepts @ref/eN, a unique CSS selector, or getByRole/getByText/getByLabel.
Repeat --path for files and --data MIME=VALUE for DataTransfer string entries.

Examples:
  ghostchrome drop '#drop-zone' --path ./invoice.pdf
  ghostchrome drop e4 --path a.png --path b.png
  ghostchrome drop 'getByRole("region", { name: "Files" })' --data 'text/plain=hello'`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		data := make([]engine.DropData, 0, len(flagDropData))
		for _, raw := range flagDropData {
			mime, value, ok := strings.Cut(raw, "=")
			if !ok || strings.TrimSpace(mime) == "" {
				exitErr("drop", fmt.Errorf("invalid --data %q: use MIME=VALUE", raw))
			}
			data = append(data, engine.DropData{MIME: mime, Value: value})
		}

		b, page := openPage()
		defer b.Close()

		var snapshot *engine.PageSnapshot
		if isSnapshotRef(args[0]) {
			snapshot = ensureSnapshot(b, page, "", "none", engine.LevelSkeleton)
		}
		if err := engine.DropTarget(page, args[0], flagDropPaths, data, snapshot); err != nil {
			exitIfStaleRef(err, "drop")
			exitErr("drop", err)
		}

		result := snapshotPage(b, page, engine.LevelSkeleton)
		text := formatCurrentPlaywrightPageStateOutput("drop", page, result)
		output(&struct {
			Action string                   `json:"action"`
			Target string                   `json:"target"`
			Paths  []string                 `json:"paths,omitempty"`
			Data   []engine.DropData        `json:"data,omitempty"`
			Result *engine.ExtractionResult `json:"result"`
		}{Action: "drop", Target: args[0], Paths: flagDropPaths, Data: data, Result: result}, text)
	},
}

func init() {
	dropCmd.Flags().StringArrayVar(&flagDropPaths, "path", nil, "File path to add to DataTransfer (repeatable)")
	dropCmd.Flags().StringArrayVar(&flagDropData, "data", nil, "DataTransfer value as MIME=VALUE (repeatable)")
	rootCmd.AddCommand(dropCmd)
}
