package cmd

import (
	"os"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var (
	flagLevel    string
	flagSelector string
	flagDiff     string // "auto", "on", "off"
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
  ghostchrome extract --connect ws://... --diff on        # diff vs last snapshot

Extraction levels:
  skeleton — interactive elements + landmarks only (minimal tokens)
  content  — skeleton + text, paragraphs, images, list items (default)
  full     — everything with a non-empty name

--diff:
  auto — diff in agent profile when a previous snapshot exists (default)
  on   — always diff (error if no previous snapshot)
  off  — always return the full extraction tree`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		level := engine.ExtractLevel(flagLevel)
		if err := engine.ValidateExtractLevel(level); err != nil {
			exitErr("extract", err)
		}

		b, page := openPage()
		defer b.Close()

		// Snapshot the previous refs *before* re-extracting, so we can diff.
		var prev *engine.PageSnapshot
		if flagSelector == "" && flagDiff != "off" {
			prev = b.Snapshot(page)
		}

		if len(args) > 0 {
			navigateIfRequested(page, args[0], "load")
		}

		result, err := engine.Extract(page, level, flagSelector)
		if err != nil {
			exitErr("extract", err)
		}

		var fresh *engine.PageSnapshot
		if flagSelector == "" {
			fresh, err = engine.BuildSnapshot(page, result)
			if err != nil {
				exitErr("snapshot", err)
			}
			if err := b.SaveSnapshot(page, result); err != nil {
				exitErr("snapshot", err)
			}
		}

		profile := renderProfile()
		if shouldDiff(flagDiff, profile, prev, fresh) {
			diff := engine.DiffRefs(prev.Refs, fresh.Refs)
			output(&diff, engine.FormatDiff(diff))
			return
		}
		if flagDiff == "on" && prev == nil {
			os.Stderr.WriteString("extract: --diff on requested but no previous snapshot; returning full tree\n")
		}

		text := engine.FormatTextProfile(result, profile)
		output(result, text)
	},
}

func shouldDiff(mode string, profile engine.RenderProfile, prev, curr *engine.PageSnapshot) bool {
	if prev == nil || curr == nil {
		return false
	}
	switch mode {
	case "on":
		return true
	case "off":
		return false
	default:
		// auto: diff when in agent profile and we actually have a prior snapshot
		return profile.Agent
	}
}

func init() {
	extractCmd.Flags().StringVar(&flagLevel, "level", "content", "Extraction level: skeleton, content, or full")
	extractCmd.Flags().StringVar(&flagSelector, "selector", "", "CSS selector to scope extraction to a subtree")
	extractCmd.Flags().StringVar(&flagDiff, "diff", "auto", "Return diff vs last snapshot: auto, on, off")
	rootCmd.AddCommand(extractCmd)
}
