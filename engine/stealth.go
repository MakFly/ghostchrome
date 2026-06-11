package engine

import (
	"fmt"
	"os"
	"runtime"
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

// osFingerprint is a self-consistent set of OS-identifying values. The whole
// point: every signal (UA string, navigator.platform, Client Hints, WebGL
// renderer) must agree with each other AND with the host the browser is
// really running on. A macOS UA on a Linux box is the single biggest
// inconsistency anti-bot challenges (Cloudflare) flag — so we derive the
// fingerprint from runtime.GOOS instead of hard-coding macOS.
type osFingerprint struct {
	uaOS          string // UA "(...)" segment, e.g. "X11; Linux x86_64"
	navPlatform   string // navigator.platform, e.g. "Linux x86_64"
	chPlatform    string // Client-Hint platform, e.g. "Linux"
	chPlatformVer string // Client-Hint platformVersion
	chArch        string // Client-Hint architecture, e.g. "x86"
	chBitness     string // Client-Hint bitness, e.g. "64"
	webglVendor   string // UNMASKED_VENDOR_WEBGL (param 37445)
	webglRenderer string // UNMASKED_RENDERER_WEBGL (param 37446) — must NOT say "SwiftShader"
}

var (
	linuxFingerprint = osFingerprint{
		uaOS:          "X11; Linux x86_64",
		navPlatform:   "Linux x86_64",
		chPlatform:    "Linux",
		chPlatformVer: "6.8.0",
		chArch:        "x86",
		chBitness:     "64",
		// A plausible Intel integrated-graphics box. The default headless
		// renderer is "SwiftShader", which is a dead automation giveaway —
		// this string makes WebGL look like an ordinary Linux laptop.
		webglVendor:   "Google Inc. (Intel)",
		webglRenderer: "ANGLE (Intel, Mesa Intel(R) UHD Graphics 620 (KBL GT2), OpenGL 4.6 (Core Profile) Mesa 23.2.1-1ubuntu3.1)",
	}
	macFingerprint = osFingerprint{
		uaOS:          "Macintosh; Intel Mac OS X 10_15_7",
		navPlatform:   "MacIntel",
		chPlatform:    "macOS",
		chPlatformVer: "15.3.0",
		chArch:        "arm",
		chBitness:     "64",
		webglVendor:   "Google Inc. (Apple)",
		webglRenderer: "ANGLE (Apple, ANGLE Metal Renderer: Apple M1, Unspecified Version)",
	}
	windowsFingerprint = osFingerprint{
		uaOS:          "Windows NT 10.0; Win64; x64",
		navPlatform:   "Win32",
		chPlatform:    "Windows",
		chPlatformVer: "15.0.0",
		chArch:        "x86",
		chBitness:     "64",
		webglVendor:   "Google Inc. (Intel)",
		webglRenderer: "ANGLE (Intel, Intel(R) UHD Graphics 620 (0x00005917) Direct3D11 vs_5_0 ps_5_0, D3D11)",
	}
)

// detectOSFingerprint picks the fingerprint matching the real host OS so the
// browser never claims to be an OS it is not running on.
func detectOSFingerprint() osFingerprint {
	switch runtime.GOOS {
	case "darwin":
		return macFingerprint
	case "windows":
		return windowsFingerprint
	default:
		return linuxFingerprint
	}
}

// stealthProfile holds the values interpolated into the stealth script and CDP overrides.
type stealthProfile struct {
	chromeMajor    string // e.g. "146"
	chromeFull     string // e.g. "146.0.7680.177"
	userAgent      string
	acceptLanguage string   // e.g. "fr-FR,fr;q=0.9,en-US;q=0.8,en;q=0.7"
	navLanguages   []string // e.g. ["fr-FR", "fr", "en-US", "en"]
	primaryLang    string   // e.g. "fr-FR"
	os             osFingerprint
}

func newStealthProfile(page *rod.Page) stealthProfile {
	major, full := detectChromeVersion(page)
	osFp := detectOSFingerprint()
	// Modern Chrome ships a "reduced" UA: the version is always <major>.0.0.0.
	ua := fmt.Sprintf(
		"Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s.0.0.0 Safari/537.36",
		osFp.uaOS, major,
	)
	primary, navLangs, acceptLang := detectLocale()
	return stealthProfile{
		chromeMajor:    major,
		chromeFull:     full,
		userAgent:      ua,
		acceptLanguage: acceptLang,
		navLanguages:   navLangs,
		primaryLang:    primary,
		os:             osFp,
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

		Object.defineProperty = function defineProperty(obj, prop, desc) {
			if (typeof prop === 'string' && cdcPattern.test(prop)) {
				return obj;
			}
			return origDefine.call(this, obj, prop, desc);
		};

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

	// --- defineNative bootstrap ---
	// Wraps Function.prototype.toString in a Proxy so that any getter/setter/method
	// we install reports "function NAME() { [native code] }" instead of leaking its
	// real source. This closes the main fingerprinting hole vs DataDome / Akamai /
	// fingerprintjs which call .toString() on overridden getters to detect spoofing.
	(function() {
		const nativeFns = new WeakSet();
		const origToString = Function.prototype.toString;
		const proxyToString = new Proxy(origToString, {
			apply(target, thisArg, args) {
				if (thisArg && nativeFns.has(thisArg)) {
					const name = (thisArg.name || '').replace(/^(get |set )/, '');
					return 'function ' + name + '() { [native code] }';
				}
				return Reflect.apply(target, thisArg, args);
			},
		});
		Function.prototype.toString = proxyToString;
		nativeFns.add(proxyToString);
		nativeFns.add(Function.prototype.toString);

		window.__defineNative = function(obj, prop, getter) {
			try {
				Object.defineProperty(getter, 'name', { value: 'get ' + prop, configurable: true });
			} catch(e) {}
			nativeFns.add(getter);
			Object.defineProperty(obj, prop, { get: getter, configurable: true, enumerable: true });
		};
		window.__markNative = function(fn) { if (typeof fn === 'function') nativeFns.add(fn); return fn; };
		nativeFns.add(window.__defineNative);
		nativeFns.add(window.__markNative);
		// Mark our hijacked Object.defineProperty as native (keeps the cdc-block hook hidden).
		try {
			Object.defineProperty(Object.defineProperty, 'name', { value: 'defineProperty', configurable: true });
		} catch(e) {}
		nativeFns.add(Object.defineProperty);
	})();

	// --- prototype handles ---
	// CRITICAL: every navigator/screen getter we install MUST go on the
	// *prototype*, never the instance. On a real Chrome,
	// Object.getOwnPropertyNames(navigator) and (screen) return []. Defining
	// getters on the instance pollutes that list — which is exactly the
	// "navigatorWebdriver" red flag Cloudflare's challenge keys on.
	const navProto = Object.getPrototypeOf(navigator);
	const screenProto = Object.getPrototypeOf(screen);

	// --- webdriver ---
	// Override on the prototype to match native Chrome behavior.
	// EmulationSetAutomationOverride handles the C++ level flag,
	// but we reinforce here for defense in depth.
	try { delete navProto.webdriver; } catch(e) {}
	__defineNative(navProto, 'webdriver', () => false);

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
	const permQuery = function query(parameters) {
		return parameters.name === 'notifications'
			? Promise.resolve({ state: Notification.permission })
			: originalQuery(parameters);
	};
	window.__markNative(permQuery);
	window.navigator.permissions.query = permQuery;

	// --- plugins ---
	// Must pass instanceof PluginArray check
	__defineNative(navProto, 'plugins', () => {
		const p = [0, 1, 2];
		p[0] = { name: 'Chrome PDF Plugin', filename: 'internal-pdf-viewer', description: 'Portable Document Format', length: 1 };
		p[1] = { name: 'Chrome PDF Viewer', filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai', description: '', length: 1 };
		p[2] = { name: 'Native Client', filename: 'internal-nacl-plugin', description: '', length: 2 };
		p.item = function(i) { return this[i] || null; };
		p.namedItem = function(n) { return this.find(x => x.name === n) || null; };
		p.refresh = function() {};
		return p;
	});

	// --- mimeTypes ---
	__defineNative(navProto, 'mimeTypes', () => {
		const mt = [
			{ type: 'application/pdf', suffixes: 'pdf', description: 'Portable Document Format' },
			{ type: 'application/x-nacl', suffixes: '', description: 'Native Client Executable' },
		];
		mt.item = function(i) { return this[i] || null; };
		mt.namedItem = function(n) { return this.find(x => x.type === n) || null; };
		mt.refresh = function() {};
		return mt;
	});

	// --- languages ---
	__defineNative(navProto, 'languages', () => __NAV_LANGUAGES__);
	__defineNative(navProto, 'language', () => '__PRIMARY_LANG__');

	// --- platform ---
	__defineNative(navProto, 'platform', () => '__NAV_PLATFORM__');

	// --- hardware ---
	__defineNative(navProto, 'hardwareConcurrency', () => 8);
	__defineNative(navProto, 'deviceMemory', () => 8);
	__defineNative(navProto, 'maxTouchPoints', () => 0);

	// --- navigator.connection ---
	if (!navigator.connection) {
		__defineNative(navProto, 'connection', () => ({
			effectiveType: '4g',
			rtt: 50,
			downlink: 10,
			saveData: false,
		}));
	}

	// --- screen dimensions (match window-size 1920x1080) ---
	__defineNative(screenProto, 'width', () => 1920);
	__defineNative(screenProto, 'height', () => 1080);
	__defineNative(screenProto, 'availWidth', () => 1920);
	__defineNative(screenProto, 'availHeight', () => 1040);
	__defineNative(screenProto, 'colorDepth', () => 24);
	__defineNative(screenProto, 'pixelDepth', () => 24);

	// --- WebGL ---
	// The headless renderer reports "SwiftShader" — a hard automation tell.
	// Override UNMASKED_VENDOR_WEBGL (37445) / UNMASKED_RENDERER_WEBGL (37446)
	// with values consistent with the spoofed OS.
	const getParam = WebGLRenderingContext.prototype.getParameter;
	const wrappedGetParam = function getParameter(p) {
		if (p === 37445) return '__WEBGL_VENDOR__';
		if (p === 37446) return '__WEBGL_RENDERER__';
		return getParam.apply(this, arguments);
	};
	window.__markNative(wrappedGetParam);
	WebGLRenderingContext.prototype.getParameter = wrappedGetParam;
	if (typeof WebGL2RenderingContext !== 'undefined') {
		const getParam2 = WebGL2RenderingContext.prototype.getParameter;
		const wrappedGetParam2 = function getParameter(p) {
			if (p === 37445) return '__WEBGL_VENDOR__';
			if (p === 37446) return '__WEBGL_RENDERER__';
			return getParam2.apply(this, arguments);
		};
		window.__markNative(wrappedGetParam2);
		WebGL2RenderingContext.prototype.getParameter = wrappedGetParam2;
	}

	// --- Brave detection ---
	__defineNative(navProto, 'brave', () => undefined);

	// --- iframe contentWindow fix ---
	// Capture __defineNative into a closure-local reference so the getter still works
	// after we delete window.__defineNative at the end of this script.
	try {
		const _defineNative = window.__defineNative;
		const origGetter = HTMLIFrameElement.prototype.__lookupGetter__('contentWindow');
		if (origGetter) {
			_defineNative(HTMLIFrameElement.prototype, 'contentWindow', function() {
				const w = origGetter.call(this);
				if (w) {
					try { _defineNative(w.navigator, 'webdriver', () => false); } catch(e) {}
				}
				return w;
			});
		}
	} catch(e) {}

	// --- Notification permission ---
	if (typeof Notification !== 'undefined' && Notification.permission === 'default') {
		__defineNative(Notification, 'permission', () => 'default');
	}

	// --- window dimensions consistency ---
	__defineNative(window, 'outerWidth', () => 1920);
	__defineNative(window, 'outerHeight', () => 1080);
	__defineNative(window, 'innerWidth', () => 1920);
	__defineNative(window, 'innerHeight', () => 937);
	__defineNative(window, 'screenX', () => 0);
	__defineNative(window, 'screenY', () => 0);

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
		try { __defineNative(window, prop, () => undefined); } catch(e) {}
	}

	// --- Stack trace sanitization ---
	const origPrepare = Error.prepareStackTrace;
	const newPrepare = function prepareStackTrace(err, stack) {
		if (origPrepare) {
			const result = origPrepare(err, stack);
			if (typeof result === 'string') {
				return result.replace(/pptr:|puppeteer:|playwright:|__puppeteer_evaluation_script__|__playwright_evaluation_script__/g, '');
			}
			return result;
		}
		return err.stack;
	};
	window.__markNative(newPrepare);
	Error.prepareStackTrace = newPrepare;

	// --- Cleanup: remove our bootstrap helpers from window so detectors can't see them ---
	try { delete window.__defineNative; } catch(e) {}
	try { delete window.__markNative; } catch(e) {}
	`

	// Template runtime values into the script.
	script = strings.ReplaceAll(script, "__NAV_LANGUAGES__", jsStringList(profile.navLanguages))
	script = strings.ReplaceAll(script, "__PRIMARY_LANG__", profile.primaryLang)
	script = strings.ReplaceAll(script, "__NAV_PLATFORM__", profile.os.navPlatform)
	script = strings.ReplaceAll(script, "__WEBGL_VENDOR__", profile.os.webglVendor)
	script = strings.ReplaceAll(script, "__WEBGL_RENDERER__", profile.os.webglRenderer)

	_, err := page.EvalOnNewDocument(script)
	if err != nil {
		return err
	}

	// Set realistic user-agent + Client Hints (critical for DataDome / Cloudflare).
	// Every field below is derived from the host OS fingerprint so the UA
	// string, navigator.platform, and Client Hints all agree with each other.
	err = proto.NetworkSetUserAgentOverride{
		UserAgent:      profile.userAgent,
		AcceptLanguage: profile.acceptLanguage,
		Platform:       profile.os.navPlatform,
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
			Platform:        profile.os.chPlatform,
			PlatformVersion: profile.os.chPlatformVer,
			Architecture:    profile.os.chArch,
			Model:           "",
			Mobile:          false,
			Bitness:         profile.os.chBitness,
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
			"Sec-CH-UA-Platform":        gson.New(fmt.Sprintf("%q", profile.os.chPlatform)),
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
// and waits for the challenge to clear.
//
// Two behaviours make it materially more reliable than a plain poll loop:
//
//   - Reload of a wedged challenge. Cloudflare hands out an independent
//     challenge token per navigation; an instance that hasn't cleared in
//     ~20s almost never un-sticks on its own, but a fresh reload usually
//     gets a solvable one. We reload up to 3 times within the budget.
//   - A one-off pointer nudge. A few Cloudflare variants only progress once
//     the page has observed a genuine input event.
//
// Returns true if a challenge was detected and ultimately cleared.
func WaitForBotChallenge(page *rod.Page, timeout time.Duration) bool {
	if !isBotChallenge(page) {
		return false
	}

	deadline := time.Now().Add(timeout)
	const stuckAfter = 20 * time.Second
	const maxReloads = 3
	nextReload := time.Now().Add(stuckAfter)
	reloads := 0
	nudged := false

	for time.Now().Before(deadline) {
		time.Sleep(1 * time.Second)

		if !isBotChallenge(page) {
			// Challenge cleared — let the real page settle before returning.
			_ = page.WaitStable(time.Second)
			return true
		}

		if !nudged {
			nudged = true
			_ = page.Mouse.MoveTo(proto.NewPoint(420, 360))
			_ = page.Mouse.MoveTo(proto.NewPoint(680, 500))
		}

		if reloads < maxReloads && time.Now().After(nextReload) {
			// A wedged challenge is almost always caused by stale Cloudflare
			// challenge-state cookies (cf_chl_*, and a cf_clearance that is
			// no longer accepted). Dropping every cf_* / __cf* cookie before
			// reloading gives the next attempt the same clean slate a fresh
			// profile gets — empirically the state that solves reliably.
			clearCloudflareCookies(page)
			if err := page.Reload(); err == nil {
				reloads++
			}
			nextReload = time.Now().Add(stuckAfter)
		}
	}
	return false
}

// clearCloudflareCookies removes Cloudflare cookies (cf_*, __cf*) from the
// browser so the next navigation faces a fresh challenge rather than being
// wedged by a stale, half-completed one.
func clearCloudflareCookies(page *rod.Page) {
	cookies, err := page.Browser().GetCookies()
	if err != nil {
		return
	}
	for _, c := range cookies {
		n := strings.ToLower(c.Name)
		if strings.HasPrefix(n, "cf_") || strings.HasPrefix(n, "__cf") {
			_ = proto.NetworkDeleteCookies{Name: c.Name, Domain: c.Domain}.Call(page)
		}
	}
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
