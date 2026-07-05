package engine

import (
	"os"
	"testing"
)

// lacentraleFixtureHTML loads the real DataDome interstitial captured from
// https://www.lacentrale.fr/listing (headful, stealth, --user-profile
// ddspike). The IP had already accumulated several hits that day, so
// DataDome escalated straight to its visual/interactive CAPTCHA ('t':'bv')
// rather than the silent JS-only check — this is the actual signature
// ghostchrome must recognize, not a "Just a moment"-style text interstitial.
func lacentraleFixtureHTML(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("testdata/lacentrale_listing.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(data)
}

func TestClassifyBotChallengeHTML_DataDomeCaptcha(t *testing.T) {
	html := lacentraleFixtureHTML(t)
	matched, ambient := classifyBotChallengeHTML("https://www.lacentrale.fr/listing", "lacentrale.fr", html)
	if !matched {
		t.Fatalf("expected the lacentrale DataDome CAPTCHA fixture to match as a bot challenge (ambient=%v)", ambient)
	}
}

func TestIsInteractiveDataDomeCaptcha_LaCentraleFixture(t *testing.T) {
	html := lacentraleFixtureHTML(t)
	if !isInteractiveDataDomeCaptcha(html) {
		t.Fatal("expected the lacentrale fixture (geo.captcha-delivery.com/captcha iframe) to be classified as an interactive CAPTCHA")
	}
}

// TestClassifyBotChallengeHTML_NoRegressionOnCloudflare guards the existing
// Cloudflare detection paths against regressions introduced by the
// classifyBotChallengeHTML refactor (isBotChallenge previously inlined this
// logic directly).
func TestClassifyBotChallengeHTML_NoRegressionOnCloudflare(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		title   string
		html    string
		matched bool
		ambient bool
	}{
		{
			name:    "just a moment title",
			url:     "https://example.com/",
			title:   "Just a moment...",
			html:    "<html></html>",
			matched: true,
		},
		{
			name:    "attention required title",
			url:     "https://example.com/",
			title:   "Attention Required! | Cloudflare",
			html:    "<html></html>",
			matched: true,
		},
		{
			name:    "cf challenge running marker",
			url:     "https://example.com/",
			title:   "example",
			html:    "<html><script>cf-challenge-running</script></html>",
			matched: true,
		},
		{
			name:    "ambient turnstile marker alone is not decisive",
			url:     "https://example.com/",
			title:   "example",
			html:    "<html><script src=\"https://challenges.cloudflare.com/turnstile.js\"></script></html>",
			matched: false,
			ambient: true,
		},
		{
			name:    "ordinary page",
			url:     "https://example.com/",
			title:   "example",
			html:    "<html><body><h1>hello</h1></body></html>",
			matched: false,
			ambient: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			matched, ambient := classifyBotChallengeHTML(tc.url, tc.title, tc.html)
			if matched != tc.matched || ambient != tc.ambient {
				t.Fatalf("classifyBotChallengeHTML(%q, %q) = (%v, %v), want (%v, %v)",
					tc.url, tc.title, matched, ambient, tc.matched, tc.ambient)
			}
		})
	}
}

// TestIsInteractiveDataDomeCaptcha_NotAmbientTag guards against a false
// positive on the plain DataDome SDK tag (present on virtually every
// DataDome-protected page, including ones that pass through silently) — only
// the actual captcha iframe must be classified as unsolvable-by-waiting.
func TestIsInteractiveDataDomeCaptcha_NotAmbientTag(t *testing.T) {
	html := `<html><head><script src="https://ct.captcha-delivery.com/c.js"></script></head><body>real content</body></html>`
	if isInteractiveDataDomeCaptcha(html) {
		t.Fatal("plain DataDome SDK tag must not be classified as an interactive captcha")
	}
}
