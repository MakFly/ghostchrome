package cmd

import (
	"fmt"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/go-rod/rod"
	"github.com/spf13/cobra"
)

var typeWaitFor string
var typeWaitTimeoutMs int
var typeSubmit bool

var typeLocator LocatorFlags

var typeCmd = &cobra.Command{
	Use:   "type <ref|text> [text] [url]",
	Short: "Type text into an input by ref or semantic locator",
	Long: `Type text into an element.

With a ref: first positional is @N, second is the text to type.
With a locator: pass --by-* flags, and a single positional (the text to type).
An optional URL argument may follow.

Examples:
  ghostchrome type @2 "hello world"
  ghostchrome type --by-label "Email" "kev@example.com"
  ghostchrome type --by-role textbox --by-name "Search" "ghostchrome"`,
	Args: cobra.RangeArgs(1, 3),
	Run: func(cmd *cobra.Command, args []string) {
		var ref, text, targetURL string
		if typeLocator.Any() {
			// With a locator: positionals are [text] [url]
			if len(args) < 1 {
				exitErr("type", fmt.Errorf("need TEXT positional when using --by-*"))
			}
			text = args[0]
			if len(args) > 1 {
				targetURL = args[1]
			}
		} else {
			if len(args) < 2 {
				exitErr("type", fmt.Errorf("need REF and TEXT (or --by-* and TEXT)"))
			}
			ref = args[0]
			text = args[1]
			if len(args) > 2 {
				targetURL = args[2]
			}
		}

		b, page := openPage()
		defer b.Close()

		snapshot := ensureSnapshot(b, page, targetURL, "load", engine.LevelSkeleton)

		waitState, waitTimeout := resolveWaitFlags(cmd, typeWaitFor, typeWaitTimeoutMs)

		// Resolve the target element through every path so --submit can
		// re-focus the exact field it typed into.
		var typedEl *rod.Element
		var rerr error
		switch {
		case typeLocator.Any():
			typedEl, rerr = engine.WaitForLocator(page, typeLocator.ToLocator(), waitState, waitTimeout)
			if rerr != nil {
				exitErr("type", rerr)
			}
		case waitTimeout > 0:
			typedEl, rerr = engine.WaitForRef(page, ref, snapshot, waitState, waitTimeout)
			if rerr != nil {
				exitIfStaleRef(rerr, "type")
				exitErr("type", rerr)
			}
		default:
			typedEl, rerr = engine.ResolveRef(page, ref, snapshot)
			if rerr != nil {
				exitIfStaleRef(rerr, "type")
				exitErr("type", rerr)
			}
		}

		if err := engine.TypeElement(page, typedEl, text); err != nil {
			exitErr("type", err)
		}

		if typeSubmit {
			if err := engine.SubmitOnElement(page, typedEl); err != nil {
				exitErr("type --submit", err)
			}
		}

		result := snapshotPage(b, page, engine.LevelSkeleton)

		type typeResult struct {
			actionResult
			Text string `json:"text"`
		}

		textOutput := engine.FormatTextProfile(result, renderProfile())
		output(&typeResult{
			actionResult: actionResult{
				Action:  "type",
				Ref:     ref,
				Locator: typeLocator.Describe(),
				Result:  result,
			},
			Text: text,
		}, textOutput)
	},
}

func init() {
	typeLocator.RegisterOn(typeCmd)
	typeCmd.Flags().StringVar(&typeWaitFor, "wait-for", "", "Wait for element state before typing: attached|visible|hidden|enabled|stable|none (default: visible)")
	typeCmd.Flags().IntVar(&typeWaitTimeoutMs, "wait-timeout-ms", 0, "Max milliseconds to wait for the element state (0 = no wait; default: 5000)")
	typeCmd.Flags().BoolVar(&typeSubmit, "submit", false, "Press Enter after typing (submit the form)")
	rootCmd.AddCommand(typeCmd)
}
