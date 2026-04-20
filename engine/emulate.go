package engine

import (
	"fmt"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Device describes a hardware profile used by the emulate command.
// Dimensions are CSS pixels; DPR is devicePixelRatio.
type Device struct {
	Name      string
	Width     int
	Height    int
	DPR       float64
	UserAgent string
	Mobile    bool
	Touch     bool
}

var devices = []Device{
	{
		Name: "iphone-se", Width: 375, Height: 667, DPR: 2,
		UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Mobile/15E148 Safari/604.1",
		Mobile:    true, Touch: true,
	},
	{
		Name: "iphone-14", Width: 390, Height: 844, DPR: 3,
		UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Mobile/15E148 Safari/604.1",
		Mobile:    true, Touch: true,
	},
	{
		Name: "iphone-14-pro", Width: 393, Height: 852, DPR: 3,
		UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Mobile/15E148 Safari/604.1",
		Mobile:    true, Touch: true,
	},
	{
		Name: "iphone-14-pro-max", Width: 430, Height: 932, DPR: 3,
		UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Mobile/15E148 Safari/604.1",
		Mobile:    true, Touch: true,
	},
	{
		Name: "pixel-7", Width: 412, Height: 915, DPR: 2.625,
		UserAgent: "Mozilla/5.0 (Linux; Android 14; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36",
		Mobile:    true, Touch: true,
	},
	{
		Name: "pixel-8-pro", Width: 448, Height: 998, DPR: 3,
		UserAgent: "Mozilla/5.0 (Linux; Android 14; Pixel 8 Pro) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36",
		Mobile:    true, Touch: true,
	},
	{
		Name: "ipad", Width: 768, Height: 1024, DPR: 2,
		UserAgent: "Mozilla/5.0 (iPad; CPU OS 17_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Mobile/15E148 Safari/604.1",
		Mobile:    true, Touch: true,
	},
	{
		Name: "ipad-pro", Width: 1024, Height: 1366, DPR: 2,
		UserAgent: "Mozilla/5.0 (iPad; CPU OS 17_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Mobile/15E148 Safari/604.1",
		Mobile:    true, Touch: true,
	},
	{
		Name: "desktop", Width: 1920, Height: 1080, DPR: 1,
		UserAgent: "",
		Mobile:    false, Touch: false,
	},
	{
		Name: "desktop-2k", Width: 2560, Height: 1440, DPR: 1,
		UserAgent: "",
		Mobile:    false, Touch: false,
	},
}

// DeviceByName looks up a Device preset by its canonical name.
func DeviceByName(name string) (Device, bool) {
	for _, d := range devices {
		if d.Name == name {
			return d, true
		}
	}
	return Device{}, false
}

// ListDevices returns a copy of the registered device presets.
func ListDevices() []Device {
	out := make([]Device, len(devices))
	copy(out, devices)
	return out
}

// ApplyDevice applies viewport metrics, UA, and touch emulation from a preset.
func ApplyDevice(page *rod.Page, d Device) error {
	sw, sh := d.Width, d.Height
	err := (proto.EmulationSetDeviceMetricsOverride{
		Width:             d.Width,
		Height:            d.Height,
		DeviceScaleFactor: d.DPR,
		Mobile:            d.Mobile,
		ScreenWidth:       &sw,
		ScreenHeight:      &sh,
	}).Call(page)
	if err != nil {
		return fmt.Errorf("device metrics: %w", err)
	}

	maxTouch := touchPoints(d.Touch)
	if err := (proto.EmulationSetTouchEmulationEnabled{
		Enabled:        d.Touch,
		MaxTouchPoints: &maxTouch,
	}).Call(page); err != nil {
		return fmt.Errorf("touch emulation: %w", err)
	}

	if d.UserAgent != "" {
		if err := ApplyUserAgent(page, d.UserAgent); err != nil {
			return err
		}
	}
	return nil
}

func touchPoints(enabled bool) int {
	if enabled {
		return 5
	}
	return 0
}

// ApplyUserAgent overrides navigator.userAgent and the HTTP User-Agent header.
func ApplyUserAgent(page *rod.Page, ua string) error {
	if err := (proto.NetworkSetUserAgentOverride{UserAgent: ua}).Call(page); err != nil {
		return fmt.Errorf("user-agent override: %w", err)
	}
	return nil
}

// ApplyColorScheme emulates prefers-color-scheme. Accepts "dark", "light",
// "no-preference" (case-insensitive).
func ApplyColorScheme(page *rod.Page, scheme string) error {
	normalized := strings.ToLower(strings.TrimSpace(scheme))
	switch normalized {
	case "dark", "light", "no-preference":
	default:
		return fmt.Errorf("color-scheme: expected dark|light|no-preference, got %q", scheme)
	}
	feature := proto.EmulationMediaFeature{
		Name:  "prefers-color-scheme",
		Value: normalized,
	}
	return (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{&feature}}).Call(page)
}

// ApplyTimezone overrides the JavaScript Date/Intl timezone.
func ApplyTimezone(page *rod.Page, tz string) error {
	if tz == "" {
		return fmt.Errorf("timezone: empty")
	}
	if err := (proto.EmulationSetTimezoneOverride{TimezoneID: tz}).Call(page); err != nil {
		return fmt.Errorf("timezone override: %w", err)
	}
	return nil
}
