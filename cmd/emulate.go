package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var (
	flagEmulateDevice      string
	flagEmulateUserAgent   string
	flagEmulateColorScheme string
	flagEmulateTimezone    string
	flagEmulateList        bool
)

var emulateCmd = &cobra.Command{
	Use:   "emulate",
	Short: "Emulate a device or override UA / color-scheme / timezone",
	Long: `Switch the browser to a device profile (viewport + UA + DPR + touch) or
override individual emulation axes.

IMPORTANT: CDP emulation overrides (user-agent, timezone, color-scheme) are
session-scoped — they apply only while the current CDP websocket is alive.
When ghostchrome runs as a one-shot CLI (no --connect), that's the full
process lifetime and everything works. When using --connect to a persistent
Chrome, each CLI invocation is a new short-lived session, so overrides reset
between calls. To carry emulation across multiple steps, use "ghostchrome
batch" with an "emulate ..." verb as the first line — the whole batch runs
in one session.

Device presets (--device):
  iphone-se, iphone-14, iphone-14-pro, iphone-14-pro-max,
  pixel-7, pixel-8-pro, ipad, ipad-pro, desktop, desktop-2k

Per-axis overrides:
  --user-agent "<ua string>"
  --color-scheme dark|light|no-preference
  --timezone Europe/Paris      (IANA tz database name)

Examples:
  ghostchrome emulate --device iphone-14-pro
  ghostchrome emulate --user-agent "Mozilla/5.0 ..." --color-scheme dark
  ghostchrome emulate --list                         # print available presets

  # Multi-step flow with emulation persisting:
  printf 'emulate device=iphone-14\nnavigate https://m.example.com\nextract\n' | \
    ghostchrome batch -`,
	Run: func(cmd *cobra.Command, args []string) {
		if flagEmulateList {
			output(listDevices(), formatDeviceList())
			return
		}
		if flagEmulateDevice == "" && flagEmulateUserAgent == "" && flagEmulateColorScheme == "" && flagEmulateTimezone == "" {
			exitErr("emulate", fmt.Errorf("need --device, --user-agent, --color-scheme, --timezone, or --list"))
		}

		b, page := openPage()
		defer b.Close()

		applied := map[string]string{}

		if flagEmulateDevice != "" {
			d, ok := engine.DeviceByName(flagEmulateDevice)
			if !ok {
				exitErr("emulate", fmt.Errorf("unknown --device %q (use --list to see presets)", flagEmulateDevice))
			}
			if err := engine.ApplyDevice(page, d); err != nil {
				exitErr("emulate", err)
			}
			applied["device"] = d.Name
			applied["viewport"] = fmt.Sprintf("%dx%d@%.1fx", d.Width, d.Height, d.DPR)
		}
		if flagEmulateUserAgent != "" {
			if err := engine.ApplyUserAgent(page, flagEmulateUserAgent); err != nil {
				exitErr("emulate", err)
			}
			applied["user-agent"] = flagEmulateUserAgent
		}
		if flagEmulateColorScheme != "" {
			if err := engine.ApplyColorScheme(page, flagEmulateColorScheme); err != nil {
				exitErr("emulate", err)
			}
			applied["color-scheme"] = flagEmulateColorScheme
		}
		if flagEmulateTimezone != "" {
			if err := engine.ApplyTimezone(page, flagEmulateTimezone); err != nil {
				exitErr("emulate", err)
			}
			applied["timezone"] = flagEmulateTimezone
		}

		type emulateResult struct {
			Action  string            `json:"action"`
			Applied map[string]string `json:"applied"`
		}

		var sb strings.Builder
		sb.WriteString("[emulate] applied:")
		keys := make([]string, 0, len(applied))
		for k := range applied {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&sb, " %s=%s", k, applied[k])
		}
		output(&emulateResult{Action: "emulate", Applied: applied}, sb.String())
	},
}

func listDevices() []engine.Device {
	devices := engine.ListDevices()
	sort.Slice(devices, func(i, j int) bool { return devices[i].Name < devices[j].Name })
	return devices
}

func formatDeviceList() string {
	var sb strings.Builder
	sb.WriteString("[devices]\n")
	for _, d := range listDevices() {
		fmt.Fprintf(&sb, "  %-20s %dx%d @%.1fx %s %s\n",
			d.Name, d.Width, d.Height, d.DPR,
			tag(d.Mobile, "mobile"), tag(d.Touch, "touch"))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func tag(b bool, label string) string {
	if b {
		return label
	}
	return strings.Repeat(" ", len(label))
}

func init() {
	emulateCmd.Flags().StringVar(&flagEmulateDevice, "device", "", "Device preset (see --list)")
	emulateCmd.Flags().StringVar(&flagEmulateUserAgent, "user-agent", "", "Override the user-agent header and navigator.userAgent")
	emulateCmd.Flags().StringVar(&flagEmulateColorScheme, "color-scheme", "", "Emulate prefers-color-scheme: dark, light, no-preference")
	emulateCmd.Flags().StringVar(&flagEmulateTimezone, "timezone", "", "Override the timezone (IANA tz name)")
	emulateCmd.Flags().BoolVar(&flagEmulateList, "list", false, "List available device presets")
	rootCmd.AddCommand(emulateCmd)
}
