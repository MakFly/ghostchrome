package cmd

import (
	"fmt"
	"strconv"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var flagDevice string

var devicePresets = map[string][2]int{
	"iphone-se":         {375, 667},
	"iphone-14":         {390, 844},
	"iphone-14-pro-max": {430, 932},
	"ipad":              {768, 1024},
	"ipad-pro":          {1024, 1366},
	"pixel-7":           {412, 915},
	"desktop-hd":        {1920, 1080},
	"desktop-2k":        {2560, 1440},
}

var viewportCmd = &cobra.Command{
	Use:   "viewport [width] [height] [url]",
	Short: "Set the browser viewport size",
	Long: `Set the viewport dimensions. Either provide width and height as arguments,
or use --device to pick from presets. If a URL is provided, navigates and extracts a skeleton.

Device presets: iphone-se, iphone-14, iphone-14-pro-max, ipad, ipad-pro,
pixel-7, desktop-hd, desktop-2k.

Examples:
  ghostchrome viewport 1024 768
  ghostchrome viewport --device iphone-14
  ghostchrome viewport 375 667 https://example.com
  ghostchrome viewport --device ipad https://example.com --connect ws://...`,
	Args: cobra.RangeArgs(0, 3),
	Run: func(cmd *cobra.Command, args []string) {
		var width, height int
		var url string
		var deviceLabel string

		if flagDevice != "" {
			preset, ok := devicePresets[flagDevice]
			if !ok {
				exitErr("device", fmt.Errorf("unknown device %q", flagDevice))
			}
			width = preset[0]
			height = preset[1]
			deviceLabel = flagDevice
			// If args provided with --device, first arg is URL
			if len(args) > 0 {
				url = args[0]
			}
		} else {
			if len(args) < 2 {
				exitErr("args", fmt.Errorf("width and height required (or use --device)"))
			}
			var err error
			width, err = strconv.Atoi(args[0])
			if err != nil {
				exitErr("parse width", err)
			}
			height, err = strconv.Atoi(args[1])
			if err != nil {
				exitErr("parse height", err)
			}
			if len(args) > 2 {
				url = args[2]
			}
		}

		b, page := openPage()
		defer b.Close()

		err := engine.SetViewport(page, width, height)
		if err != nil {
			exitErr("viewport", err)
		}

		type viewportResult struct {
			Width  int                      `json:"width"`
			Height int                      `json:"height"`
			Device string                   `json:"device,omitempty"`
			Result *engine.ExtractionResult `json:"result,omitempty"`
		}

		vr := &viewportResult{
			Width:  width,
			Height: height,
			Device: deviceLabel,
		}

		label := fmt.Sprintf("Viewport set to %dx%d", width, height)
		if deviceLabel != "" {
			label = fmt.Sprintf("Viewport set to %dx%d (%s)", width, height, deviceLabel)
		}

		if url != "" {
			navigateIfRequested(page, url, "load")
			result := snapshotPage(b, page, engine.LevelSkeleton)
			vr.Result = result
			label += "\n" + engine.FormatText(result)
		}

		output(vr, label)
	},
}

func init() {
	viewportCmd.Flags().StringVar(&flagDevice, "device", "", "Device preset (e.g. iphone-14, ipad, desktop-hd)")
	rootCmd.AddCommand(viewportCmd)
}
