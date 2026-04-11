package cmd

import (
	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var (
	flagLevel    string
	flagSelector string
)

var extractCmd = &cobra.Command{
	Use:   "extract [url]",
	Short: "Extract the DOM as a compact accessibility tree",
	Long: `Extracts the page's accessibility tree and outputs a compact representation.
Can auto-launch Chrome if a URL is provided, or attach to a running Chrome via --connect.

Examples:
  ghostchrome extract https://example.com
  ghostchrome extract https://example.com --level skeleton
  ghostchrome extract --connect ws://... --level full

Extraction levels:
  skeleton — interactive elements + landmarks only (minimal tokens)
  content  — skeleton + text, paragraphs, images, list items (default)
  full     — everything with a non-empty name`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		level := engine.ExtractLevel(flagLevel)
		if err := engine.ValidateExtractLevel(level); err != nil {
			exitErr("extract", err)
		}

		b, page := openPage()
		defer b.Close()

		if len(args) > 0 {
			navigateIfRequested(page, args[0], "load")
		}

		result, err := engine.Extract(page, level, flagSelector)
		if err != nil {
			exitErr("extract", err)
		}
		if flagSelector == "" {
			if err := b.SaveSnapshot(page, result); err != nil {
				exitErr("snapshot", err)
			}
		}

		text := engine.FormatText(result)
		output(result, text)
	},
}

func init() {
	extractCmd.Flags().StringVar(&flagLevel, "level", "content", "Extraction level: skeleton, content, or full")
	extractCmd.Flags().StringVar(&flagSelector, "selector", "", "CSS selector to scope extraction to a subtree")
	rootCmd.AddCommand(extractCmd)
}
