package engine

import (
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// ApplyStealth applies anti-detection patches to a page via CDP.
// These hide common headless Chrome fingerprints that anti-bot systems check.
func ApplyStealth(page *rod.Page) error {
	// 1. Remove navigator.webdriver flag
	// 2. Mock chrome.runtime
	// 3. Mock permissions API
	// 4. Fix iframe contentWindow
	// 5. Set realistic navigator properties
	script := `
	// Remove webdriver flag
	Object.defineProperty(navigator, 'webdriver', { get: () => undefined });

	// Mock chrome.runtime to look like a real extension environment
	if (!window.chrome) { window.chrome = {}; }
	if (!window.chrome.runtime) {
		window.chrome.runtime = {
			connect: function() {},
			sendMessage: function() {},
			onMessage: { addListener: function() {} },
			id: undefined
		};
	}

	// Mock permissions query
	const originalQuery = window.navigator.permissions.query;
	window.navigator.permissions.query = (parameters) => (
		parameters.name === 'notifications' ?
		Promise.resolve({ state: Notification.permission }) :
		originalQuery(parameters)
	);

	// Fix plugins to look realistic
	Object.defineProperty(navigator, 'plugins', {
		get: () => {
			const plugins = [
				{ name: 'Chrome PDF Plugin', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
				{ name: 'Chrome PDF Viewer', filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai', description: '' },
				{ name: 'Native Client', filename: 'internal-nacl-plugin', description: '' }
			];
			plugins.refresh = () => {};
			return plugins;
		}
	});

	// Fix languages
	Object.defineProperty(navigator, 'languages', { get: () => ['fr-FR', 'fr', 'en-US', 'en'] });

	// Fix platform
	Object.defineProperty(navigator, 'platform', { get: () => 'MacIntel' });

	// Fix hardwareConcurrency
	Object.defineProperty(navigator, 'hardwareConcurrency', { get: () => 8 });

	// Mock WebGL vendor/renderer
	const getParameter = WebGLRenderingContext.prototype.getParameter;
	WebGLRenderingContext.prototype.getParameter = function(parameter) {
		if (parameter === 37445) return 'Google Inc. (Apple)';
		if (parameter === 37446) return 'ANGLE (Apple, Apple M1, OpenGL 4.1)';
		return getParameter.apply(this, arguments);
	};
	`

	// Inject stealth script on every new document (including navigations)
	_, err := page.EvalOnNewDocument(script)
	if err != nil {
		return err
	}

	// Set a realistic user-agent
	err = proto.NetworkSetUserAgentOverride{
		UserAgent:      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
		AcceptLanguage: "fr-FR,fr;q=0.9,en-US;q=0.8,en;q=0.7",
		Platform:       "MacIntel",
	}.Call(page)
	if err != nil {
		return err
	}

	return nil
}
