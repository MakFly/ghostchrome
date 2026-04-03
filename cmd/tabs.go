package cmd

import (
	"fmt"
	"strconv"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var tabsCmd = &cobra.Command{
	Use:   "tabs",
	Short: "List open browser tabs",
	Long: `List all open tabs in the browser with their index, title, and URL.
The active tab is marked with *active*.

Examples:
  ghostchrome tabs --connect ws://127.0.0.1:9222`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		b, err := engine.NewBrowser(flagConnect, flagHeadless, flagTimeout)
		if err != nil {
			exitErr("browser", err)
		}
		defer b.Close()

		browser := b.RawBrowser()
		tabs, err := engine.ListTabs(browser)
		if err != nil {
			exitErr("list tabs", err)
		}

		type tabsResult struct {
			Tabs []engine.TabInfo `json:"tabs"`
		}

		var text string
		for i, t := range tabs {
			text += fmt.Sprintf("[%d] %s — %s\n", i, t.Title, t.URL)
		}

		output(&tabsResult{Tabs: tabs}, text)
	},
}

var tabsSwitchCmd = &cobra.Command{
	Use:   "switch <index>",
	Short: "Switch to a tab by index",
	Long: `Activate the tab at the given index.

Examples:
  ghostchrome tabs switch 2 --connect ws://127.0.0.1:9222`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		idx, err := strconv.Atoi(args[0])
		if err != nil {
			exitErr("parse index", err)
		}

		b, err := engine.NewBrowser(flagConnect, flagHeadless, flagTimeout)
		if err != nil {
			exitErr("browser", err)
		}
		defer b.Close()

		browser := b.RawBrowser()
		_, err = engine.SwitchTab(browser, idx)
		if err != nil {
			exitErr("switch tab", err)
		}

		type switchResult struct {
			Action string `json:"action"`
			Index  int    `json:"index"`
		}

		text := fmt.Sprintf("Switched to tab %d", idx)
		output(&switchResult{Action: "switch", Index: idx}, text)
	},
}

var tabsCloseCmd = &cobra.Command{
	Use:   "close <index>",
	Short: "Close a tab by index",
	Long: `Close the tab at the given index.

Examples:
  ghostchrome tabs close 1 --connect ws://127.0.0.1:9222`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		idx, err := strconv.Atoi(args[0])
		if err != nil {
			exitErr("parse index", err)
		}

		b, err := engine.NewBrowser(flagConnect, flagHeadless, flagTimeout)
		if err != nil {
			exitErr("browser", err)
		}
		defer b.Close()

		browser := b.RawBrowser()
		err = engine.CloseTab(browser, idx)
		if err != nil {
			exitErr("close tab", err)
		}

		type closeResult struct {
			Action string `json:"action"`
			Index  int    `json:"index"`
		}

		text := fmt.Sprintf("Closed tab %d", idx)
		output(&closeResult{Action: "close", Index: idx}, text)
	},
}

func init() {
	tabsCmd.AddCommand(tabsSwitchCmd)
	tabsCmd.AddCommand(tabsCloseCmd)
	rootCmd.AddCommand(tabsCmd)
}
