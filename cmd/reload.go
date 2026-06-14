package cmd

import (
	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var reloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Reload the current page",
	Long: `Reload (refresh) the current page and return the resulting URL and title.

Waits for the "load" lifecycle event rather than full network idle, so pages
with persistent analytics/chat connections (which never go idle) still return.

Examples:
  ghostchrome reload --connect ws://127.0.0.1:9222`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		b, page := openPage()
		defer b.Close()

		if err := page.Reload(); err != nil {
			exitErr("reload", err)
		}
		// Best-effort: a page that never reaches "load" (heavy SPA) must not
		// fail the command — we still report whatever info we can read.
		_ = engine.WaitForPage(page, "load")

		info, err := page.Info()
		if err != nil {
			exitErr("page info", err)
		}

		result := snapshotPage(b, page, engine.LevelSkeleton)

		text := formatPlaywrightPageStateOutput(&engine.PageInfo{
			URL:   info.URL,
			Title: info.Title,
		}, result)
		output(&navResult{
			Action: "reload",
			URL:    info.URL,
			Title:  info.Title,
		}, text)
	},
}

func init() {
	rootCmd.AddCommand(reloadCmd)
}
