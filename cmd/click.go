package cmd

import (
	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var clickLocator LocatorFlags

var clickCmd = &cobra.Command{
	Use:   "click [ref|url]",
	Short: "Click an interactive element by ref or semantic locator",
	Long: `Click an element identified by:
  - its @ref (from the last snapshot), OR
  - a semantic locator via --by-role / --by-name / --by-label / --by-text.

If a URL is provided (without a ref), it is navigated first.
After clicking, extracts a skeleton of the resulting page.

Examples:
  ghostchrome click @3 --connect ws://...
  ghostchrome click --by-role button --by-name "Sign in"
  ghostchrome click --by-text "Learn more"
  ghostchrome click @1 https://example.com`,
	Args: cobra.RangeArgs(0, 2),
	Run: func(cmd *cobra.Command, args []string) {
		ref := ""
		targetURL := ""
		if !clickLocator.Any() {
			if len(args) == 0 {
				exitErr("click", errNeedRefOrLocator())
			}
			ref = args[0]
			if len(args) > 1 {
				targetURL = args[1]
			}
		} else if len(args) > 0 {
			targetURL = args[0]
		}

		b, page := openPage()
		defer b.Close()

		snapshot := ensureSnapshot(b, page, targetURL, "load", engine.LevelSkeleton)

		if clickLocator.Any() {
			el, err := engine.ResolveByLocator(page, clickLocator.ToLocator())
			if err != nil {
				exitErr("click", err)
			}
			if err := engine.ClickElement(page, el); err != nil {
				exitErr("click", err)
			}
		} else {
			if err := engine.ClickRef(page, ref, snapshot); err != nil {
				exitIfStaleRef(err, "click")
				exitErr("click", err)
			}
		}

		result := snapshotPage(b, page, engine.LevelSkeleton)

		type clickResult struct {
			Action  string                   `json:"action"`
			Ref     string                   `json:"ref,omitempty"`
			Locator string                   `json:"locator,omitempty"`
			Result  *engine.ExtractionResult `json:"result"`
		}

		text := engine.FormatTextProfile(result, renderProfile())
		output(&clickResult{
			Action:  "click",
			Ref:     ref,
			Locator: clickLocator.Describe(),
			Result:  result,
		}, text)
	},
}

func init() {
	clickLocator.RegisterOn(clickCmd)
	rootCmd.AddCommand(clickCmd)
}
