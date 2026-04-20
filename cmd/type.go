package cmd

import (
	"fmt"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

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

		if typeLocator.Any() {
			el, err := engine.ResolveByLocator(page, typeLocator.ToLocator())
			if err != nil {
				exitErr("type", err)
			}
			if err := engine.TypeElement(page, el, text); err != nil {
				exitErr("type", err)
			}
		} else {
			if err := engine.TypeRef(page, ref, text, snapshot); err != nil {
				exitIfStaleRef(err, "type")
				exitErr("type", err)
			}
		}

		result := snapshotPage(b, page, engine.LevelSkeleton)

		type typeResult struct {
			Action  string                   `json:"action"`
			Ref     string                   `json:"ref,omitempty"`
			Locator string                   `json:"locator,omitempty"`
			Text    string                   `json:"text"`
			Result  *engine.ExtractionResult `json:"result"`
		}

		textOutput := engine.FormatTextProfile(result, renderProfile())
		output(&typeResult{
			Action:  "type",
			Ref:     ref,
			Locator: typeLocator.Describe(),
			Text:    text,
			Result:  result,
		}, textOutput)
	},
}

func init() {
	typeLocator.RegisterOn(typeCmd)
	rootCmd.AddCommand(typeCmd)
}
