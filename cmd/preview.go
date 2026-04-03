package cmd

import (
	"github.com/MakFly/ghostchrome/engine"
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

		b, err := engine.NewBrowser(flagConnect, flagHeadless, flagTimeout)
		if err != nil {
			exitErr("browser", err)
		}
		defer b.Close()

		page, err := b.Page()
		if err != nil {
			exitErr("page", err)
		}

		applyStealthIfNeeded(page)

		level := engine.ExtractLevel(flagPreviewLevel)

		result, err := engine.Preview(page, targetURL, "stable", level)
		if err != nil {
			exitErr("preview", err)
		}

		dismissCookiesIfNeeded(page)

		text := engine.FormatPreview(result)
		output(result, text)
	},
}

func init() {
	previewCmd.Flags().StringVar(&flagPreviewLevel, "level", "skeleton", "DOM extraction level: skeleton, content, or full")
	rootCmd.AddCommand(previewCmd)
}
