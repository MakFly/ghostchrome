// Package leboncoin isolates leboncoin.fr-specific scraping recipes from
// ghostchrome's core. It mirrors the layout of packages/linkedin: cobra
// subtree under `ghostchrome leboncoin <sub>`, URL builders here, parsers
// in dedicated files, command wiring in cmd.go.
//
// Subcommands:
//   ghostchrome leboncoin search  → keyword/location/category search → CSV
//
// All subcommands reuse ghostchrome's stealth + human-input dynamics +
// optional cookie injection via `--user-profile leboncoin`.
package leboncoin

import (
	"net/url"
	"strconv"
	"strings"
)

// CategoryID maps short slugs to leboncoin's numeric `category` filter.
// Values harvested from the public category dropdown. Extend as needed.
var CategoryID = map[string]string{
	"voitures":      "2",
	"motos":         "3",
	"utilitaires":   "4",
	"caravaning":    "5",
	"nautisme":      "6",
	"immobilier":    "8",
	"ventes-immo":   "9",
	"locations":     "10",
	"colocations":   "11",
	"informatique":  "17",
	"telephonie":    "18",
	"multimedia":    "16",
	"electromenager": "24",
	"meubles":       "27",
	"electronique":  "15",
	"jeux-jouets":   "31",
	"velos":         "44",
	"emploi":        "33",
	"services":      "34",
	"animaux":       "29",
	"materiel-pro":  "32",
}

// SortToken maps friendly names to leboncoin's `sort` URL parameter.
var SortToken = map[string]string{
	"recent":     "time",
	"price-asc":  "price",
	"price-desc": "price",
	// price-desc is achieved by adding &order=desc; price-asc by &order=asc.
}

// SearchURL builds a leboncoin search URL.
//   keywords  → free text (required)
//   location  → city name OR "" for nationwide (e.g. "Paris", "Lyon")
//   category  → slug from CategoryID, or raw numeric id, or ""
//   page      → 1-indexed
//   sort      → "recent", "price-asc", "price-desc", or "" (default = relevance)
func SearchURL(keywords, location, category, sort string, page int) string {
	q := url.Values{}
	if keywords != "" {
		q.Set("text", keywords)
	}
	if location != "" {
		q.Set("locations", location)
	}
	if category != "" {
		if id, ok := CategoryID[strings.ToLower(category)]; ok {
			q.Set("category", id)
		} else {
			q.Set("category", category)
		}
	}
	switch strings.ToLower(sort) {
	case "recent":
		q.Set("sort", "time")
	case "price-asc":
		q.Set("sort", "price")
		q.Set("order", "asc")
	case "price-desc":
		q.Set("sort", "price")
		q.Set("order", "desc")
	}
	if page > 1 {
		q.Set("page", strconv.Itoa(page))
	}
	return "https://www.leboncoin.fr/recherche?" + q.Encode()
}
