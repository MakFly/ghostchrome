package leboncoin

import (
	"regexp"
	"strings"
)

// AdRecord is a row in the search-results CSV output.
type AdRecord struct {
	AdURL        string
	AdID         string
	CategorySlug string
	Title        string
	Price        string // raw, e.g. "175 €", "2 590 €"
	Location     string // "Paris 75019 Bassin de la Villette"
	Seller       string // "professionnel" if pro, "" otherwise
	Delivery     bool   // true if "Livraison possible" present
	PriceDrop    bool   // true if ".. Baisse de prix" annotation present
	Category     string // human-readable, e.g. "Ordinateurs"
}

// adAnchorRe matches the line that opens an ad block:
//   @29 a>leboncoin.fr/ad/ordinateurs/3180540132 l'annonce
// or the www-prefixed variant. We capture the slug + id and use the
// position of this line as the block start.
var adAnchorRe = regexp.MustCompile(`^\s*@\d+\s+a>(?:www\.)?leboncoin\.fr/ad/([^/\s]+)/(\d+)\b`)

// txtLineRe matches the renderer's text-content lines (`   txt <body>`).
var txtLineRe = regexp.MustCompile(`^\s*txt\s+(.+?)\s*$`)

// priceLineRe captures the numeric price out of a "Prix: 1 200 €." line
// (with optional ".. Baisse de prix" suffix). Spaces in the number can
// be ASCII or U+00A0.
// `\s` in Go's RE2 is ASCII-only, and leboncoin uses two distinct
// non-ASCII spaces in price strings: U+202F (NARROW NO-BREAK SPACE) as
// the thousands separator inside the number ("4 800") and U+00A0
// (NO-BREAK SPACE) before the euro sign ("4 800 €"). The class
// below accepts both, plus regular ASCII space, so we cover all
// observed render variants.
var priceLineRe = regexp.MustCompile(`^Prix\s*:[\s\x{00A0}\x{202F}]*((?:\d{1,3}(?:[\s\x{00A0}\x{202F}]\d{3})*|\d+)[\s\x{00A0}\x{202F}]*€)`)

// ParseSearchResults walks the rendered text and extracts one AdRecord
// per `/ad/<slug>/<id>` anchor. The card's metadata sits in the lines
// that follow the anchor, until either the next anchor or a level
// change. We use a permissive forward-scan with early stop on the next
// `@N a>...` line so a malformed card doesn't poison its neighbours.
func ParseSearchResults(text string) []AdRecord {
	lines := strings.Split(text, "\n")
	var out []AdRecord
	seen := map[string]bool{}

	for i := 0; i < len(lines); i++ {
		m := adAnchorRe.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		slug, id := m[1], m[2]
		if seen[id] {
			continue
		}
		seen[id] = true

		rec := AdRecord{
			AdURL:        "https://www.leboncoin.fr/ad/" + slug + "/" + id,
			AdID:         id,
			CategorySlug: slug,
		}

		// Forward scan until next anchor or up to 30 lines (safety).
		end := len(lines)
		if cap := i + 30; cap < end {
			end = cap
		}
		for j := i + 1; j < end; j++ {
			if adAnchorRe.MatchString(lines[j]) {
				break
			}
			tm := txtLineRe.FindStringSubmatch(lines[j])
			if tm == nil {
				continue
			}
			body := tm[1]

			switch {
			case strings.HasPrefix(body, "Prix:"):
				if pm := priceLineRe.FindStringSubmatch(body); pm != nil {
					rec.Price = strings.TrimSpace(pm[1])
				}
				if strings.Contains(body, "Baisse de prix") {
					rec.PriceDrop = true
				}
			case strings.HasPrefix(body, "Située à "):
				rec.Location = strings.TrimSuffix(strings.TrimPrefix(body, "Située à "), ".")
			case strings.HasPrefix(body, "Catégorie :"):
				rec.Category = strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(body, "Catégorie :")), ".")
			case strings.HasPrefix(body, "Vendeur "):
				rec.Seller = strings.TrimSuffix(strings.TrimPrefix(body, "Vendeur "), ".")
			case body == "Livraison possible":
				rec.Delivery = true
			case body == "Ajouter l’annonce aux favoris" || body == "l’annonce" || body == "l'annonce":
				// noise from anchor labels, skip
			case rec.Title == "":
				// First non-noise text line after the anchor is the title.
				rec.Title = body
			}
		}
		out = append(out, rec)
	}
	return out
}
