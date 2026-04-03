package cmd

import (
	"fmt"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var backCmd = &cobra.Command{
	Use:   "back",
	Short: "Navigate back in browser history",
	Long: `Go back one step in the current page's navigation history.
Returns the resulting page URL and title after navigating back.

Examples:
  ghostchrome back --connect ws://127.0.0.1:9222`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		b, err := engine.NewBrowser(flagConnect, flagHeadless, flagTimeout)
		if err != nil {
			exitErr("browser", err)
		}
		defer b.Close()

		page, err := b.Page()
		if err != nil {
			exitErr("page", err)
		}

		err = page.NavigateBack()
		if err != nil {
			exitErr("back", err)
		}

		page.MustWaitStable()

		info, err := page.Info()
		if err != nil {
			exitErr("page info", err)
		}

		type navResult struct {
			Action string `json:"action"`
			URL    string `json:"url"`
			Title  string `json:"title"`
		}

		text := fmt.Sprintf("[back] %s — %s", info.Title, info.URL)
		output(&navResult{
			Action: "back",
			URL:    info.URL,
			Title:  info.Title,
		}, text)
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
		b, err := engine.NewBrowser(flagConnect, flagHeadless, flagTimeout)
		if err != nil {
			exitErr("browser", err)
		}
		defer b.Close()

		page, err := b.Page()
		if err != nil {
			exitErr("page", err)
		}

		err = page.NavigateForward()
		if err != nil {
			exitErr("forward", err)
		}

		page.MustWaitStable()

		info, err := page.Info()
		if err != nil {
			exitErr("page info", err)
		}

		type navResult struct {
			Action string `json:"action"`
			URL    string `json:"url"`
			Title  string `json:"title"`
		}

		text := fmt.Sprintf("[forward] %s — %s", info.Title, info.URL)
		output(&navResult{
			Action: "forward",
			URL:    info.URL,
			Title:  info.Title,
		}, text)
	},
}

func init() {
	rootCmd.AddCommand(backCmd)
	rootCmd.AddCommand(forwardCmd)
}
