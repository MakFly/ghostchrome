package engine

import (
	"strings"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// TestStealthToStringHidesGetters verifies that overridden getters and methods
// installed by ApplyStealth report "[native code]" via .toString(), so that
// fingerprinting scripts (DataDome, Akamai, fingerprintjs) can't detect the
// spoofing by inspecting the getter source.
func TestStealthToStringHidesGetters(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Chrome")
	}
	l := launcher.New().Headless(true).Leakless(false)
	if needsNoSandbox() {
		l = l.NoSandbox(true)
	}
	controlURL, err := l.Launch()
	if err != nil {
		t.Skipf("launch chrome: %v (skipping: chrome not available)", err)
	}
	defer l.Kill()

	b := rod.New().ControlURL(controlURL)
	if err := b.Connect(); err != nil {
		t.Skipf("connect: %v (skipping)", err)
	}
	defer b.Close()

	page, err := b.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	if err := ApplyStealth(page); err != nil {
		t.Fatalf("apply stealth: %v", err)
	}

	if err := page.Navigate("about:blank"); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}

	res, err := page.Eval(`() => {
		// Walk the prototype chain for a property's accessor descriptor. Chromium
		// moves some of these between the instance and the prototype across
		// versions (e.g. navigator.plugins), so a fixed lookup target is brittle.
		// Returns the getter source, or "" when the property is not an accessor
		// getter on this build (nothing for stealth to hide via a getter).
		const getterSrc = (start, prop) => {
			for (let o = start; o; o = Object.getPrototypeOf(o)) {
				const d = Object.getOwnPropertyDescriptor(o, prop);
				if (d) return d.get ? d.get.toString() : '';
			}
			return '';
		};
		return {
			webdriver: getterSrc(navigator, 'webdriver'),
			plugins: getterSrc(navigator, 'plugins'),
			languages: getterSrc(navigator, 'languages'),
			platform: getterSrc(navigator, 'platform'),
			hardwareConcurrency: getterSrc(navigator, 'hardwareConcurrency'),
			screenWidth: getterSrc(screen, 'width'),
			outerWidth: getterSrc(window, 'outerWidth'),
			fnToString: Function.prototype.toString.toString(),
			webglGetParam: WebGLRenderingContext.prototype.getParameter.toString(),
			permQuery: navigator.permissions.query.toString(),
		};
	}`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	checks := map[string]string{
		"webdriver":           res.Value.Get("webdriver").Str(),
		"plugins":             res.Value.Get("plugins").Str(),
		"languages":           res.Value.Get("languages").Str(),
		"platform":            res.Value.Get("platform").Str(),
		"hardwareConcurrency": res.Value.Get("hardwareConcurrency").Str(),
		"screenWidth":         res.Value.Get("screenWidth").Str(),
		"outerWidth":          res.Value.Get("outerWidth").Str(),
		"fnToString":          res.Value.Get("fnToString").Str(),
		"webglGetParam":       res.Value.Get("webglGetParam").Str(),
		"permQuery":           res.Value.Get("permQuery").Str(),
	}

	for name, got := range checks {
		if got == "" {
			// Property is not an accessor getter on this Chromium build —
			// nothing for stealth to hide via a getter, so skip it.
			t.Logf("%s: no accessor getter on this Chromium build — skipped", name)
			continue
		}
		if !strings.Contains(got, "[native code]") {
			t.Errorf("%s.toString() leaks source — got %q, want substring %q", name, got, "[native code]")
		}
	}

	// Sanity: __defineNative bootstrap helper must be removed from window.
	leak, err := page.Eval(`() => typeof window.__defineNative + ',' + typeof window.__markNative`)
	if err != nil {
		t.Fatalf("eval leak check: %v", err)
	}
	if got := leak.Value.Str(); got != "undefined,undefined" {
		t.Errorf("bootstrap helpers leaked on window: %q", got)
	}
}
