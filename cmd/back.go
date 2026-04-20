package cmd

import (
	"fmt"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/go-rod/rod"
	"github.com/spf13/cobra"
)

type navResult struct {
	Action string `json:"action"`
	URL    string `json:"url"`
	Title  string `json:"title"`
}

func runHistoryStep(action string, step func(*rod.Page) error) {
	b, page := openPage()
	defer b.Close()

	if err := step(page); err != nil {
		exitErr(action, err)
	}

	if err := engine.WaitForPage(page, "stable"); err != nil {
		exitErr(action, err)
	}

	info, err := page.Info()
	if err != nil {
		exitErr("page info", err)
	}

	_ = snapshotPage(b, page, engine.LevelSkeleton)

	text := fmt.Sprintf("[%s] %s — %s", action, info.Title, info.URL)
	output(&navResult{
		Action: action,
		URL:    info.URL,
		Title:  info.Title,
	}, text)
}

var backCmd = &cobra.Command{
	Use:   "back",
	Short: "Navigate back in browser history",
	Long: `Go back one step in the current page's navigation history.
Returns the resulting page URL and title after navigating back.

Examples:
  ghostchrome back --connect ws://127.0.0.1:9222`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runHistoryStep("back", (*rod.Page).NavigateBack)
	},
}

var forwardCmd = &cobra.Command{
	Use:   "forward",
	Short: "Navigate forward in browser history",
	Long: `Go forward one step in the current page's navigation history.
Returns the resulting page URL and title after navigating forward.

Examples:
  ghostchrome forward --connect ws://127.0.0.1:9222`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runHistoryStep("forward", (*rod.Page).NavigateForward)
	},
}

func init() {
	rootCmd.AddCommand(backCmd)
	rootCmd.AddCommand(forwardCmd)
}
