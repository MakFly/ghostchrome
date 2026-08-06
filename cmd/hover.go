package cmd

import (
	"github.com/dev-toolings/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var hoverLocator LocatorFlags
var hoverWaitFor string
var hoverWaitTimeoutMs int

var hoverCmd = &cobra.Command{
	Use:   "hover [ref|url]",
	Short: "Hover over an element by ref or semantic locator",
	Long: `Hover over an element identified by @ref or --by-role / --by-name /
--by-label / --by-text. If a URL is provided, navigates first.

Examples:
  ghostchrome hover @2 --connect ws://...
  ghostchrome hover --by-role link --by-name "Docs"`,
	Args: cobra.RangeArgs(0, 2),
	Run: func(cmd *cobra.Command, args []string) {
		ref := ""
		targetURL := ""
		if !hoverLocator.Any() {
			if len(args) == 0 {
				exitErr("hover", errNeedRefOrLocator())
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

		waitState, waitTimeout := resolveWaitFlags(cmd, hoverWaitFor, hoverWaitTimeoutMs)

		if hoverLocator.Any() {
			el, err := engine.WaitForLocator(page, hoverLocator.ToLocator(), waitState, waitTimeout)
			if err != nil {
				exitErr("hover", err)
			}
			if err := engine.HoverElement(page, el); err != nil {
				exitErr("hover", err)
			}
		} else {
			if waitTimeout > 0 {
				el, err := engine.WaitForRef(page, ref, snapshot, waitState, waitTimeout)
				if err != nil {
					exitIfStaleRef(err, "hover")
					exitErr("hover", err)
				}
				if err := engine.HoverElement(page, el); err != nil {
					exitErr("hover", err)
				}
			} else {
				if err := engine.HoverRef(page, ref, snapshot); err != nil {
					exitIfStaleRef(err, "hover")
					exitErr("hover", err)
				}
			}
		}

		result := snapshotPage(b, page, engine.LevelSkeleton)

		text := formatCurrentPlaywrightPageStateOutput("hover", page, result)
		output(&actionResult{
			Action:  "hover",
			Ref:     ref,
			Locator: hoverLocator.Describe(),
			Result:  result,
		}, text)
	},
}

func init() {
	hoverLocator.RegisterOn(hoverCmd)
	hoverCmd.Flags().StringVar(&hoverWaitFor, "wait-for", "", "Wait for element state before hovering: attached|visible|hidden|enabled|stable|none (default: visible)")
	hoverCmd.Flags().IntVar(&hoverWaitTimeoutMs, "wait-timeout-ms", 0, "Max milliseconds to wait for the element state (0 = no wait; default: 5000)")
	rootCmd.AddCommand(hoverCmd)
}
