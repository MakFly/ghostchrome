package cmd

import (
	"fmt"

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
		switch level {
		case engine.LevelSkeleton, engine.LevelContent, engine.LevelFull:
			// valid
		default:
			exitErr("extract", fmt.Errorf("invalid level %q — use skeleton, content, or full", flagLevel))
		}

		b, err := engine.NewBrowser(flagConnect, flagHeadless, flagTimeout)
		if err != nil {
			exitErr("browser", err)
		}
		defer b.Close()

		page, err := b.Page()
		if err != nil {
			exitErr("page", err)
		}

		// If URL provided, apply stealth and navigate first
		if len(args) > 0 {
			applyStealthIfNeeded(page)
			_, err := engine.Navigate(page, args[0], "load")
			if err != nil {
				exitErr("navigate", err)
			}
			dismissCookiesIfNeeded(page)
		}

		result, err := engine.Extract(page, level, flagSelector)
		if err != nil {
			exitErr("extract", err)
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
