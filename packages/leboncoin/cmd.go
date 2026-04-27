package leboncoin

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"time"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/spf13/cobra"
)

// CommandFactory builds the `ghostchrome leboncoin ...` cobra subtree.
// Same shape as packages/linkedin so callers stay symmetric.
type CommandFactory struct {
	BuildBrowserOpts func() engine.BrowserOpts
}

// Register attaches the leboncoin subcommand tree to the given parent.
func (f *CommandFactory) Register(parent *cobra.Command) {
	root := &cobra.Command{
		Use:   "leboncoin",
		Short: "leboncoin.fr scraping recipes (search, ...)",
		Long: `leboncoin.fr-specific commands.

Subcommands:
  search   Keyword/location/category search → CSV`,
	}
	root.AddCommand(f.searchCmd())
	root.AddCommand(f.importCookiesCmd())
	parent.AddCommand(root)
}

// ─────────────────────────── search ───────────────────────────

func (f *CommandFactory) searchCmd() *cobra.Command {
	var (
		keywords  string
		location  string
		category  string
		sort      string
		pages     int
		output    string
		sleepMin  int
		sleepMax  int
		statePath string
		appendCSV bool
	)
	cmd := &cobra.Command{
		Use:   "search",
		Short: "leboncoin search → CSV (ads listing)",
		Example: `  ghostchrome --user-profile leboncoin leboncoin search \
    --keywords "Macbook Pro M1" --location "Paris" --pages 3 --output mbp.csv

  ghostchrome --user-profile leboncoin leboncoin search \
    --keywords "Clio 4" --category voitures --sort price-asc --pages 5`,
		Run: func(cmd *cobra.Command, args []string) {
			if keywords == "" && location == "" && category == "" {
				exitErr("leboncoin search", fmt.Errorf("at least one of --keywords / --location / --category is required"))
			}
			opts := f.BuildBrowserOpts()

			page, b := openSession(opts)
			defer b.Close()
			defer snapshotCookies(page, opts.UserDataDir)

			seen := loadState(statePath)
			seenInit := len(seen)

			out, closeFn := openCSV(output, appendCSV)
			defer closeFn()
			w := csv.NewWriter(out)
			defer w.Flush()
			if !appendCSV || isEmpty(output) {
				_ = w.Write([]string{"ad_id", "ad_url", "category_slug", "title", "price", "price_drop", "location", "category", "seller", "delivery", "sponsored", "first_seen"})
			}

			now := time.Now().Format(time.RFC3339)
			total := 0
			for p := 1; p <= pages; p++ {
				pagedURL := SearchURL(keywords, location, category, sort, p)
				fmt.Fprintf(os.Stderr, "[page %d/%d] %s\n", p, pages, pagedURL)
				if _, err := engine.Navigate(page, pagedURL, "load"); err != nil {
					fmt.Fprintf(os.Stderr, "  navigate error: %v\n", err)
					continue
				}
				time.Sleep(4 * time.Second)
				autoScroll(page)
				time.Sleep(2 * time.Second)

				result, err := engine.Extract(page, engine.LevelContent, "")
				if err != nil {
					fmt.Fprintf(os.Stderr, "  extract error: %v\n", err)
					continue
				}
				text := engine.FormatTextProfile(result, engine.ProfileAgent("text"))
				if dbg := os.Getenv("LBC_DEBUG_DUMP"); dbg != "" {
					_ = os.WriteFile(dbg, []byte(text), 0o644)
					fmt.Fprintf(os.Stderr, "  [debug] dumped %d bytes → %s\n", len(text), dbg)
				}
				added := 0
				for _, ad := range ParseSearchResults(text) {
					if seen[ad.AdID] {
						continue
					}
					seen[ad.AdID] = true
					_ = w.Write([]string{
						ad.AdID, ad.AdURL, ad.CategorySlug, ad.Title, ad.Price,
						boolStr(ad.PriceDrop), ad.Location, ad.Category, ad.Seller,
						boolStr(ad.Delivery), boolStr(ad.Sponsored), now,
					})
					added++
					total++
				}
				w.Flush()
				fmt.Fprintf(os.Stderr, "  +%d new (%d total this run, %d in state)\n", added, total, len(seen))
				if p < pages {
					sleepHumanized(sleepMin, sleepMax)
				}
			}
			if statePath != "" {
				if err := saveState(statePath, seen); err != nil {
					fmt.Fprintf(os.Stderr, "warning: state save failed: %v\n", err)
				} else {
					fmt.Fprintf(os.Stderr, "state: %d known keys (was %d)\n", len(seen), seenInit)
				}
			}
			fmt.Fprintf(os.Stderr, "\nDone. %d new ads%s\n", total, dest(output))
		},
	}
	cmd.Flags().StringVar(&keywords, "keywords", "", "Free-text search")
	cmd.Flags().StringVar(&location, "location", "", "City name (e.g. Paris) — empty for nationwide")
	cmd.Flags().StringVar(&category, "category", "", "Category slug (voitures, immobilier, locations, multimedia, ...) or numeric id")
	cmd.Flags().StringVar(&sort, "sort", "", "Sort order: recent, price-asc, price-desc (default: relevance)")
	cmd.Flags().IntVar(&pages, "pages", 3, "Number of result pages")
	cmd.Flags().StringVar(&output, "output", "", "CSV output path (default: stdout)")
	cmd.Flags().IntVar(&sleepMin, "sleep-min", 8, "Min seconds between pages")
	cmd.Flags().IntVar(&sleepMax, "sleep-max", 20, "Max seconds between pages")
	cmd.Flags().StringVar(&statePath, "state", "", "Path to JSON state file for cross-run dedup (skips ad ids seen previously)")
	cmd.Flags().BoolVar(&appendCSV, "append", false, "Append to --output instead of overwriting")
	return cmd
}

// ─────────────────────────── helpers (mirror of linkedin) ───────────────────────────

func openSession(opts engine.BrowserOpts) (*rod.Page, *engine.Browser) {
	br, err := engine.NewBrowserWith(opts)
	if err != nil {
		exitErr("leboncoin", err)
	}
	pg, err := br.Page()
	if err != nil {
		br.Close()
		exitErr("leboncoin", err)
	}
	if opts.UserDataDir != "" && opts.ConnectURL == "" {
		if cookies, _ := engine.LoadCookiesJSON(opts.UserDataDir); len(cookies) > 0 {
			_, _ = engine.InjectCookies(pg, cookies)
		}
	}
	return pg, br
}

// snapshotCookies pulls the current cookie jar via CDP and rewrites the
// profile snapshot. Datadome rotates its `datadome` cookie ~every 30
// minutes; persisting the latest value lets the next run re-inject a
// still-fresh token instead of replaying a stale one (which Datadome
// rejects with a 403). Best-effort: any failure is logged and ignored
// so a parse hiccup never breaks the scrape itself.
func snapshotCookies(page *rod.Page, profileDir string) {
	if profileDir == "" {
		return
	}
	got, err := proto.NetworkGetAllCookies{}.Call(page)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  [cookies] snapshot skipped: %v\n", err)
		return
	}
	out := make([]engine.CookieRecord, 0, len(got.Cookies))
	for _, c := range got.Cookies {
		// Keep only cookies that belong to the leboncoin domain — we
		// don't want third-party trackers (doubleclick, criteo, ...)
		// to bloat the snapshot.
		if !strings.Contains(c.Domain, "leboncoin.fr") {
			continue
		}
		out = append(out, engine.CookieRecord{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  float64(c.Expires),
			Secure:   c.Secure,
			HTTPOnly: c.HTTPOnly,
			SameSite: string(c.SameSite),
		})
	}
	if err := engine.SaveCookiesJSON(profileDir, out); err != nil {
		fmt.Fprintf(os.Stderr, "  [cookies] save failed: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "  [cookies] refreshed snapshot (%d entries) → %s\n", len(out), profileDir)
}

func autoScroll(page *rod.Page) {
	for i := 0; i < 4; i++ {
		_, _ = page.Eval(`() => window.scrollBy(0, window.innerHeight * 1.5)`)
		time.Sleep(700 * time.Millisecond)
	}
}

func sleepHumanized(minSec, maxSec int) {
	if maxSec <= minSec {
		maxSec = minSec + 5
	}
	d := minSec + rand.IntN(maxSec-minSec+1)
	fmt.Fprintf(os.Stderr, "  …sleeping %ds (humanized)\n", d)
	time.Sleep(time.Duration(d) * time.Second)
}

func openCSV(path string, appendMode bool) (*os.File, func()) {
	if path == "" {
		return os.Stdout, func() {}
	}
	flag := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if appendMode {
		flag = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	}
	f, err := os.OpenFile(path, flag, 0o644)
	if err != nil {
		exitErr("leboncoin", err)
	}
	return f, func() { _ = f.Close() }
}

func isEmpty(path string) bool {
	if path == "" {
		return true
	}
	st, err := os.Stat(path)
	return err != nil || st.Size() == 0
}

func loadState(path string) map[string]bool {
	out := map[string]bool{}
	if path == "" {
		return out
	}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	var keys []string
	if err := json.NewDecoder(f).Decode(&keys); err != nil {
		return out
	}
	for _, k := range keys {
		out[k] = true
	}
	return out
}

func saveState(path string, seen map[string]bool) error {
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(f).Encode(keys); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func dest(path string) string {
	if path == "" {
		return " (stdout)"
	}
	return " → " + path
}

func exitErr(action string, err error) {
	fmt.Fprintf(os.Stderr, "error: %s: %v\n", action, err)
	os.Exit(1)
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return ""
}

// keep imports tidy if some helpers get pruned later
var _ = strings.ToLower
