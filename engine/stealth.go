package engine

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/ysmood/gson"
)

// Fallback values used if runtime detection fails.
const (
	fallbackChromeVersion = "146"
	fallbackChromeFull    = "146.0.7680.177"
)

// stealthProfile holds the values interpolated into the stealth script and CDP overrides.
type stealthProfile struct {
	chromeMajor    string // e.g. "146"
	chromeFull     string // e.g. "146.0.7680.177"
	userAgent      string
	acceptLanguage string   // e.g. "fr-FR,fr;q=0.9,en-US;q=0.8,en;q=0.7"
	navLanguages   []string // e.g. ["fr-FR", "fr", "en-US", "en"]
	primaryLang    string   // e.g. "fr-FR"
}

func newStealthProfile(page *rod.Page) stealthProfile {
	major, full := detectChromeVersion(page)
	ua := fmt.Sprintf(
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36",
		full,
	)
	primary, navLangs, acceptLang := detectLocale()
	return stealthProfile{
		chromeMajor:    major,
		chromeFull:     full,
		userAgent:      ua,
		acceptLanguage: acceptLang,
		navLanguages:   navLangs,
		primaryLang:    primary,
	}
}

// detectChromeVersion queries the connected Chrome for its version. Falls back
// to package constants if the query fails.
func detectChromeVersion(page *rod.Page) (major, full string) {
	if page == nil {
		return fallbackChromeVersion, fallbackChromeFull
	}
	v, err := page.Browser().Version()
	if err != nil || v == nil || v.Product == "" {
		return fallbackChromeVersion, fallbackChromeFull
	}
	// v.Product looks like "HeadlessChrome/146.0.7680.177" or "Chrome/146.0.7680.177".
	_, rest, ok := strings.Cut(v.Product, "/")
	if !ok || rest == "" {
		return fallbackChromeVersion, fallbackChromeFull
	}
	full = rest
	if dot := strings.IndexByte(full, '.'); dot > 0 {
		major = full[:dot]
	} else {
		major = full
	}
	return major, full
}

// detectLocale derives navigator.languages + Accept-Language from LANG / LC_ALL.
// Falls back to en-US/en when nothing is set (safer default than fr-FR for CI).
func detectLocale() (primary string, navLangs []string, acceptLang string) {
	raw := firstNonEmpty(os.Getenv("LC_ALL"), os.Getenv("LANG"), "en_US.UTF-8")
	// Trim encoding suffix, e.g. "fr_FR.UTF-8" → "fr_FR"
	if idx := strings.IndexByte(raw, '.'); idx > 0 {
		raw = raw[:idx]
	}
	raw = strings.ReplaceAll(raw, "_", "-")
	if raw == "" || raw == "C" || raw == "POSIX" {
		raw = "en-US"
	}
	primary = raw
	base := raw
	if idx := strings.IndexByte(raw, '-'); idx > 0 {
		base = raw[:idx]
	}
	navLangs = []string{primary, base, "en-US", "en"}
	navLangs = dedupeStrings(navLangs)
	acceptLang = fmt.Sprintf("%s,%s;q=0.9,en-US;q=0.8,en;q=0.7", primary, base)
	return primary, navLangs, acceptLang
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// jsStringList formats a Go []string as a JavaScript array literal of single-quoted strings.
func jsStringList(items []string) string {
	parts := make([]string, 0, len(items))
	for _, s := range items {
		parts = append(parts, "'"+strings.ReplaceAll(s, "'", "\\'")+"'")
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// ApplyStealth applies anti-detection patches to a page via CDP.
// Targets DataDome, Akamai, and similar bot-detection systems.
func ApplyStealth(page *rod.Page) error {
	profile := newStealthProfile(page)
	return applyStealthWithProfile(page, profile)
}

func applyStealthWithProfile(page *rod.Page, profile stealthProfile) error {
	// 1. Disable automation flag at the C++ level (prevents navigator.webdriver = true)
	_ = proto.EmulationSetAutomationOverride{Enabled: true}.Call(page)

	script := `
	// --- Remove CDP/DevTools fingerprints ($cdc_, $chrome_asyncScriptInfo, etc.) ---
	// Chrome injects $cdc_ variables when controlled via CDP. DataDome scans for them.
	// We intercept Object.defineProperty to block future injections and clean existing ones.
	(function() {
		const origDefine = Object.defineProperty;
		const cdcPattern = /^\$(?:cdc_|chrome_|wdc_)/;

		Object.defineProperty = function(obj, prop, desc) {
			if (typeof prop === 'string' && cdcPattern.test(prop)) {
				return obj;
			}
			return origDefine.call(this, obj, prop, desc);
		};
		// Preserve native toString to avoid detection
		origDefine.call(Object, Object.defineProperty, 'toString', {
			value: function() { return 'function defineProperty() { [native code] }'; },
			writable: false, configurable: true,
		});

		// Clean existing $cdc_ properties
		const cleanObj = (obj) => {
			if (!obj) return;
			try {
				for (const key of Object.getOwnPropertyNames(obj)) {
					if (cdcPattern.test(key)) { try { delete obj[key]; } catch(e) {} }
				}
			} catch(e) {}
		};
		cleanObj(document);
		cleanObj(window);

		// Reactive cleanup via MutationObserver — no leaked timers on SPA navigations.
		try {
			const observer = new MutationObserver(() => { cleanObj(document); cleanObj(window); });
			observer.observe(document, { childList: true, subtree: true, attributes: false });
			// Safety net: stop observing after 10s, by then any late CDP injection has fired.
			setTimeout(() => { try { observer.disconnect(); } catch(e) {} }, 10000);
		} catch(e) {}
	})();

	// --- webdriver ---
	// Override on the prototype to match native Chrome behavior.
	// EmulationSetAutomationOverride handles the C++ level flag,
	// but we reinforce here for defense in depth.
	const navProto = Object.getPrototypeOf(navigator);
	try { delete navProto.webdriver; } catch(e) {}
	Object.defineProperty(navProto, 'webdriver', {
		get: () => false,
		enumerable: true,
		configurable: true,
	});

	// --- chrome object ---
	if (!window.chrome) { window.chrome = {}; }
	if (!window.chrome.runtime) {
		window.chrome.runtime = {
			connect: function() {},
			sendMessage: function() {},
			onMessage: { addListener: function() {} },
			id: undefined,
		};
	}
	if (!window.chrome.csi) {
		window.chrome.csi = function() {
			return {
				startE: Date.now(),
				onloadT: Date.now(),
				pageT: performance.now(),
				tran: 15,
			};
		};
	}
	if (!window.chrome.loadTimes) {
		window.chrome.loadTimes = function() {
			return {
				commitLoadTime: Date.now() / 1000,
				connectionInfo: 'h2',
				finishDocumentLoadTime: Date.now() / 1000,
				finishLoadTime: Date.now() / 1000,
				firstPaintAfterLoadTime: 0,
				firstPaintTime: Date.now() / 1000,
				navigationType: 'Other',
				npnNegotiatedProtocol: 'h2',
				requestTime: Date.now() / 1000,
				startLoadTime: Date.now() / 1000,
				wasAlternateProtocolAvailable: false,
				wasFetchedViaSpdy: true,
				wasNpnNegotiated: true,
			};
		};
	}
	if (!window.chrome.app) {
		window.chrome.app = {
			isInstalled: false,
			InstallState: { DISABLED: 'disabled', INSTALLED: 'installed', NOT_INSTALLED: 'not_installed' },
			RunningState: { CANNOT_RUN: 'cannot_run', READY_TO_RUN: 'ready_to_run', RUNNING: 'running' },
			getDetails: function() { return null; },
			getIsInstalled: function() { return false; },
			installState: function() { return 'not_installed'; },
		};
	}

	// --- permissions ---
	const originalQuery = window.navigator.permissions.query.bind(window.navigator.permissions);
	window.navigator.permissions.query = (parameters) => (
		parameters.name === 'notifications'
			? Promise.resolve({ state: Notification.permission })
			: originalQuery(parameters)
	);

	// --- plugins ---
	// Must pass instanceof PluginArray check
	Object.defineProperty(navigator, 'plugins', {
		get: () => {
			const p = [0, 1, 2];
			p[0] = { name: 'Chrome PDF Plugin', filename: 'internal-pdf-viewer', description: 'Portable Document Format', length: 1 };
			p[1] = { name: 'Chrome PDF Viewer', filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai', description: '', length: 1 };
			p[2] = { name: 'Native Client', filename: 'internal-nacl-plugin', description: '', length: 2 };
			p.item = function(i) { return this[i] || null; };
			p.namedItem = function(n) { return this.find(x => x.name === n) || null; };
			p.refresh = function() {};
			return p;
		},
		configurable: true,
	});

	// --- mimeTypes ---
	Object.defineProperty(navigator, 'mimeTypes', {
		get: () => {
			const mt = [
				{ type: 'application/pdf', suffixes: 'pdf', description: 'Portable Document Format' },
				{ type: 'application/x-nacl', suffixes: '', description: 'Native Client Executable' },
			];
			mt.item = function(i) { return this[i] || null; };
			mt.namedItem = function(n) { return this.find(x => x.type === n) || null; };
			mt.refresh = function() {};
			return mt;
		},
		configurable: true,
	});

	// --- languages ---
	Object.defineProperty(navigator, 'languages', { get: () => __NAV_LANGUAGES__ });
	Object.defineProperty(navigator, 'language', { get: () => '__PRIMARY_LANG__' });

	// --- platform ---
	Object.defineProperty(navigator, 'platform', { get: () => 'MacIntel' });

	// --- hardware ---
	Object.defineProperty(navigator, 'hardwareConcurrency', { get: () => 8 });
	Object.defineProperty(navigator, 'deviceMemory', { get: () => 8 });
	Object.defineProperty(navigator, 'maxTouchPoints', { get: () => 0 });

	// --- navigator.connection ---
	if (!navigator.connection) {
		Object.defineProperty(navigator, 'connection', {
			get: () => ({
				effectiveType: '4g',
				rtt: 50,
				downlink: 10,
				saveData: false,
			}),
		});
	}

	// --- screen dimensions (match window-size 1920x1080) ---
	Object.defineProperty(screen, 'width', { get: () => 1920 });
	Object.defineProperty(screen, 'height', { get: () => 1080 });
	Object.defineProperty(screen, 'availWidth', { get: () => 1920 });
	Object.defineProperty(screen, 'availHeight', { get: () => 1040 });
	Object.defineProperty(screen, 'colorDepth', { get: () => 24 });
	Object.defineProperty(screen, 'pixelDepth', { get: () => 24 });

	// --- WebGL ---
	const getParam = WebGLRenderingContext.prototype.getParameter;
	WebGLRenderingContext.prototype.getParameter = function(p) {
		if (p === 37445) return 'Google Inc. (Apple)';
		if (p === 37446) return 'ANGLE (Apple, ANGLE Metal Renderer: Apple M1, Unspecified Version)';
		return getParam.apply(this, arguments);
	};
	if (typeof WebGL2RenderingContext !== 'undefined') {
		const getParam2 = WebGL2RenderingContext.prototype.getParameter;
		WebGL2RenderingContext.prototype.getParameter = function(p) {
			if (p === 37445) return 'Google Inc. (Apple)';
			if (p === 37446) return 'ANGLE (Apple, ANGLE Metal Renderer: Apple M1, Unspecified Version)';
			return getParam2.apply(this, arguments);
		};
	}

	// --- Brave detection ---
	Object.defineProperty(navigator, 'brave', { get: () => undefined });

	// --- iframe contentWindow fix ---
	try {
		const origGetter = HTMLIFrameElement.prototype.__lookupGetter__('contentWindow');
		if (origGetter) {
			Object.defineProperty(HTMLIFrameElement.prototype, 'contentWindow', {
				get: function() {
					const w = origGetter.call(this);
					if (w) {
						try { Object.defineProperty(w.navigator, 'webdriver', { get: () => false }); } catch(e) {}
					}
					return w;
				},
			});
		}
	} catch(e) {}

	// --- Notification permission ---
	if (typeof Notification !== 'undefined' && Notification.permission === 'default') {
		Object.defineProperty(Notification, 'permission', { get: () => 'default' });
	}

	// --- window dimensions consistency ---
	Object.defineProperty(window, 'outerWidth', { get: () => 1920 });
	Object.defineProperty(window, 'outerHeight', { get: () => 1080 });
	Object.defineProperty(window, 'innerWidth', { get: () => 1920 });
	Object.defineProperty(window, 'innerHeight', { get: () => 937 });
	Object.defineProperty(window, 'screenX', { get: () => 0 });
	Object.defineProperty(window, 'screenY', { get: () => 0 });

	// --- Remove automation indicators ---
	const automationProps = [
		'callPhantom', '_phantom', '__nightmare', 'domAutomation',
		'domAutomationController', '_Selenium_IDE_Recorder',
		'__webdriver_script_fn', '__driver_evaluate', '__webdriver_evaluate',
		'__fxdriver_evaluate', '__driver_unwrap', '__webdriver_unwrap',
		'__selenium_unwrap', '__lastWatirAlert', '__lastWatirConfirm',
		'__lastWatirPrompt',
	];
	for (const prop of automationProps) {
		delete window[prop];
		try { Object.defineProperty(window, prop, { get: () => undefined }); } catch(e) {}
	}

	// --- Stack trace sanitization ---
	const origPrepare = Error.prepareStackTrace;
	Error.prepareStackTrace = function(err, stack) {
		if (origPrepare) {
			const result = origPrepare(err, stack);
			if (typeof result === 'string') {
				return result.replace(/pptr:|puppeteer:|playwright:|__puppeteer_evaluation_script__|__playwright_evaluation_script__/g, '');
			}
			return result;
		}
		return err.stack;
	};
	`

	// Template runtime values into the script.
	script = strings.ReplaceAll(script, "__NAV_LANGUAGES__", jsStringList(profile.navLanguages))
	script = strings.ReplaceAll(script, "__PRIMARY_LANG__", profile.primaryLang)

	_, err := page.EvalOnNewDocument(script)
	if err != nil {
		return err
	}

	// Set realistic user-agent + Client Hints (critical for DataDome)
	err = proto.NetworkSetUserAgentOverride{
		UserAgent:      profile.userAgent,
		AcceptLanguage: profile.acceptLanguage,
		Platform:       "MacIntel",
		UserAgentMetadata: &proto.EmulationUserAgentMetadata{
			Brands: []*proto.EmulationUserAgentBrandVersion{
				{Brand: "Chromium", Version: profile.chromeMajor},
				{Brand: "Google Chrome", Version: profile.chromeMajor},
				{Brand: "Not?A_Brand", Version: "99"},
			},
			FullVersionList: []*proto.EmulationUserAgentBrandVersion{
				{Brand: "Chromium", Version: profile.chromeFull},
				{Brand: "Google Chrome", Version: profile.chromeFull},
				{Brand: "Not?A_Brand", Version: "99.0.0.0"},
			},
			FullVersion:     profile.chromeFull,
			Platform:        "macOS",
			PlatformVersion: "15.3.0",
			Architecture:    "arm",
			Model:           "",
			Mobile:          false,
			Bitness:         "64",
			Wow64:           false,
		},
	}.Call(page)
	if err != nil {
		return err
	}

	// Set extra HTTP headers to match real Chrome
	secChUA := fmt.Sprintf(`"Chromium";v="%s", "Google Chrome";v="%s", "Not?A_Brand";v="99"`, profile.chromeMajor, profile.chromeMajor)
	err = proto.NetworkSetExtraHTTPHeaders{
		Headers: proto.NetworkHeaders{
			"Sec-CH-UA":                 gson.New(secChUA),
			"Sec-CH-UA-Mobile":          gson.New("?0"),
			"Sec-CH-UA-Platform":        gson.New(`"macOS"`),
			"Upgrade-Insecure-Requests": gson.New("1"),
		},
	}.Call(page)
	if err != nil {
		return err
	}

	// Set viewport to match screen dimensions
	sw, sh := 1920, 1080
	err = proto.EmulationSetDeviceMetricsOverride{
		Width:             1920,
		Height:            1080,
		DeviceScaleFactor: 1,
		Mobile:            false,
		ScreenWidth:       &sw,
		ScreenHeight:      &sh,
	}.Call(page)
	if err != nil {
		return err
	}

	return nil
}

// WaitForBotChallenge detects bot-challenge pages (DataDome, Cloudflare, etc.)
// and waits for the challenge JS to resolve and the page to reload.
// Returns true if a challenge was detected and resolved.
func WaitForBotChallenge(page *rod.Page, timeout time.Duration) bool {
	if !isBotChallenge(page) {
		return false
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// Wait a bit for challenge JS to execute and set cookie
		time.Sleep(1 * time.Second)

		// Check if we're still on a challenge page
		if !isBotChallenge(page) {
			_ = page.WaitStable(500 * time.Millisecond)
			return true
		}
	}
	return false
}

// isBotChallenge checks if the current page is a known bot-challenge interstitial.
func isBotChallenge(page *rod.Page) bool {
	info, err := page.Info()
	if err != nil {
		return false
	}

	if strings.Contains(info.URL, "captcha-delivery.com") ||
		strings.Contains(info.URL, "geo.captcha-delivery.com") {
		return true
	}

	// "Just a moment..." / "Attention Required! | Cloudflare" → CF interstitial.
	if strings.Contains(info.Title, "Just a moment") ||
		strings.Contains(info.Title, "Attention Required") {
		return true
	}

	html, err := page.HTML()
	if err != nil {
		return false
	}

	challengeMarkers := []string{
		"captcha-delivery.com",
		"ct.captcha-delivery.com/c.js",
		"geo.captcha-delivery.com",
		"cdn-cgi/challenge-platform",
		"challenges.cloudflare.com",
		"cf-turnstile",
		"_cf-chl-opt",
		"window._cf_chl_opt",
	}

	for _, marker := range challengeMarkers {
		if strings.Contains(html, marker) {
			return true
		}
	}

	return false
}
