package cmd

import (
	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var flagOnRef string

var evalCmd = &cobra.Command{
	Use:   "eval <expression> [url]",
	Short: "Evaluate JavaScript on the page",
	Long: `Evaluate a JavaScript expression on the page and return the result.
If a URL is provided, navigates first then evaluates.
Use --on @ref to evaluate in the context of a specific element.

Examples:
  ghostchrome eval "document.title" https://example.com
  ghostchrome eval "this.textContent" --on @1 --connect ws://...
  ghostchrome eval "window.location.href"`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		expr := args[0]
		targetURL := ""
		if len(args) > 1 {
			targetURL = args[1]
		}

		b, page := openPage()
		defer b.Close()

		var snapshot *engine.PageSnapshot
		if flagOnRef != "" || targetURL != "" {
			snapshot = ensureSnapshot(b, page, targetURL, "load", engine.LevelSkeleton)
		}

		result, err := engine.EvalJS(page, expr, flagOnRef, snapshot)
		if err != nil {
			exitIfStaleRef(err, "eval")
			exitErr("eval", err)
		}

		type evalResult struct {
			Expression string `json:"expression"`
			Result     string `json:"result"`
		}

		output(&evalResult{
			Expression: expr,
			Result:     result,
		}, result)
	},
}

func init() {
	evalCmd.Flags().StringVar(&flagOnRef, "on", "", "Evaluate in context of element @ref")
	rootCmd.AddCommand(evalCmd)
}
