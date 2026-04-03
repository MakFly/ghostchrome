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

		b, err := engine.NewBrowser(flagConnect, flagHeadless, flagTimeout)
		if err != nil {
			exitErr("browser", err)
		}
		defer b.Close()

		page, err := b.Page()
		if err != nil {
			exitErr("page", err)
		}

		// If URL provided, navigate first.
		if len(args) > 1 {
			_, err := engine.Navigate(page, args[1], "load")
			if err != nil {
				exitErr("navigate", err)
			}
		}

		// Evaluate JS.
		result, err := engine.EvalJS(page, expr, flagOnRef)
		if err != nil {
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
