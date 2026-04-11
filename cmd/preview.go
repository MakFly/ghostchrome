package cmd

import (
	"github.com/MakFly/ghostchrome/engine"
	"github.com/go-rod/rod"
	"github.com/spf13/cobra"
)

var flagPreviewLevel string

var previewCmd = &cobra.Command{
	Use:   "preview <url>",
	Short: "Full page health report: status + errors + network + DOM",
	Long: `Preview gives a complete page analysis in one command.
Combines navigate + error collection + network monitoring + DOM extraction.

Perfect for LLM agents to verify their work after code changes.

Examples:
  ghostchrome preview http://localhost:3000
  ghostchrome preview http://localhost:3000 --level skeleton
  ghostchrome preview http://localhost:8000/admin --dismiss-cookies`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		targetURL := args[0]

		b, page := openPage()
		defer b.Close()

		applyStealthIfNeeded(page)

		level := engine.ExtractLevel(flagPreviewLevel)

		result, err := engine.Preview(page, targetURL, "stable", level, func(page *rod.Page) error {
			dismissCookiesIfNeeded(page)
			return nil
		}, flagStealth)
		if err != nil {
			exitErr("preview", err)
		}

		if err := b.SaveSnapshot(page, result.DOM); err != nil {
			exitErr("snapshot", err)
		}

		text := engine.FormatPreview(result)
		output(result, text)
	},
}

func init() {
	previewCmd.Flags().StringVar(&flagPreviewLevel, "level", "skeleton", "DOM extraction level: skeleton, content, or full")
	rootCmd.AddCommand(previewCmd)
}
