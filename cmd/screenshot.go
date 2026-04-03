package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var (
	flagFull       bool
	flagElement    string
	flagQuality    int
	flagOutputPath string
)

var screenshotCmd = &cobra.Command{
	Use:   "screenshot [url]",
	Short: "Capture a screenshot of the page or an element",
	Long: `Take a screenshot of the current page, full page, or a specific element.
If a URL is provided, navigates first then captures.

Examples:
  ghostchrome screenshot https://example.com
  ghostchrome screenshot https://example.com --full
  ghostchrome screenshot --element @3 --connect ws://...
  ghostchrome screenshot https://example.com --output page.png --quality 90`,
	Args: cobra.MaximumNArgs(1),
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

		// If URL provided, navigate first.
		if len(args) > 0 {
			_, err := engine.Navigate(page, args[0], "load")
			if err != nil {
				exitErr("navigate", err)
			}
		}

		// Take screenshot.
		data, err := engine.TakeScreenshot(page, flagFull, flagElement, flagQuality)
		if err != nil {
			exitErr("screenshot", err)
		}

		// Determine output path.
		outPath := flagOutputPath
		if outPath == "" {
			outPath = fmt.Sprintf("/tmp/ghostchrome-screenshot-%d.png", time.Now().UnixMilli())
		}

		// Write file.
		err = os.WriteFile(outPath, data, 0644)
		if err != nil {
			exitErr("write screenshot", err)
		}

		sizeBytes := len(data)

		type screenshotResult struct {
			Path      string `json:"path"`
			SizeBytes int    `json:"size_bytes"`
		}

		text := fmt.Sprintf("Screenshot saved to %s (%d bytes)", outPath, sizeBytes)
		output(&screenshotResult{
			Path:      outPath,
			SizeBytes: sizeBytes,
		}, text)
	},
}

func init() {
	screenshotCmd.Flags().BoolVar(&flagFull, "full", false, "Capture full scrollable page")
	screenshotCmd.Flags().StringVar(&flagElement, "element", "", "Capture specific element by @ref")
	screenshotCmd.Flags().IntVar(&flagQuality, "quality", 80, "JPEG quality 1-100 (PNG if <= 0)")
	screenshotCmd.Flags().StringVar(&flagOutputPath, "output", "", "Output file path (default: /tmp/ghostchrome-screenshot-<timestamp>.png)")
	rootCmd.AddCommand(screenshotCmd)
}
