// microbench aggregates per-trial JSON files into a head-to-head report
// (ghostchrome vs playwright-mcp) and writes:
//   - benchmark/results.md   (human-readable table)
//   - benchmark/results.json (machine, for tooling)
//   - benchmark/badges/{tokens,latency,size}.json (shields.io endpoint)
//
// Input JSON shape (one trial per file in --input dir):
//
//	{"tool":"ghostchrome|playwright-mcp", "url":"...", "site":"<label>",
//	 "bytes":1234, "durationMs":567, "peakRssKb":89000}
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type trial struct {
	Tool       string `json:"tool"`
	URL        string `json:"url"`
	Site       string `json:"site"`
	Bytes      int    `json:"bytes"`
	DurationMs int    `json:"durationMs"`
	PeakRSSKB  int    `json:"peakRssKb"`
}

type stat struct {
	Bytes      int
	DurationMs int
	PeakRSSKB  int
	Tokens     int // ceil(bytes/4), matches the existing benchmark/report convention
	N          int
}

type siteRow struct {
	Site string
	URL  string
	GC   stat // ghostchrome
	PW   stat // playwright-mcp
}

func median(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	sort.Ints(xs)
	n := len(xs)
	if n%2 == 1 {
		return xs[n/2]
	}
	return (xs[n/2-1] + xs[n/2]) / 2
}

func aggregate(trials []trial) []siteRow {
	bySite := make(map[string]map[string][]trial)
	siteOrder := []string{}
	urlBySite := make(map[string]string)
	for _, t := range trials {
		if _, ok := bySite[t.Site]; !ok {
			bySite[t.Site] = make(map[string][]trial)
			siteOrder = append(siteOrder, t.Site)
		}
		bySite[t.Site][t.Tool] = append(bySite[t.Site][t.Tool], t)
		urlBySite[t.Site] = t.URL
	}
	rows := make([]siteRow, 0, len(siteOrder))
	for _, site := range siteOrder {
		row := siteRow{Site: site, URL: urlBySite[site]}
		row.GC = medianStat(bySite[site]["ghostchrome"])
		row.PW = medianStat(bySite[site]["playwright-mcp"])
		rows = append(rows, row)
	}
	return rows
}

func medianStat(ts []trial) stat {
	if len(ts) == 0 {
		return stat{}
	}
	bytes := make([]int, len(ts))
	durs := make([]int, len(ts))
	rss := make([]int, len(ts))
	for i, t := range ts {
		bytes[i] = t.Bytes
		durs[i] = t.DurationMs
		rss[i] = t.PeakRSSKB
	}
	b := median(bytes)
	return stat{
		Bytes:      b,
		DurationMs: median(durs),
		PeakRSSKB:  median(rss),
		Tokens:     int(math.Ceil(float64(b) / 4.0)),
		N:          len(ts),
	}
}

func ratio(a, b int) float64 {
	if a == 0 {
		return 0
	}
	return float64(b) / float64(a)
}

func loadTrials(dir string) ([]trial, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []trial
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		var t trial
		if err := json.Unmarshal(data, &t); err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		if t.Tool == "" || t.Site == "" {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

func writeMarkdown(path, mode string, rows []siteRow, gcBinarySize int64) error {
	var b strings.Builder
	title := "Benchmark — ghostchrome vs Playwright-MCP"
	if mode == "warm" {
		title += " (warm session)"
	} else if mode == "cold" {
		title += " (cold spawn)"
	}
	fmt.Fprintf(&b, "# %s\n\n", title)
	if mode == "warm" {
		fmt.Fprintf(&b, "Per-site median over N trials, **warm session**: ghostchrome runs against a long-lived `serve` instance, Playwright-MCP keeps one MCP server alive for all URLs. Each timing measures only `navigate + snapshot`, not process startup. This is the real LLM-agent loop.\n\n")
	} else {
		fmt.Fprintf(&b, "Per-site median over N trials, **cold spawn**: every command starts a fresh process and Chrome session. Snapshot payload = the text content an LLM agent receives from the tool call.\n\n")
	}
	fmt.Fprintf(&b, "| Site | ghostchrome bytes | pw-mcp bytes | tokens ratio | ghostchrome ms | pw-mcp ms | latency ratio |\n")
	fmt.Fprintf(&b, "|---|---:|---:|---:|---:|---:|---:|\n")
	totalGCBytes, totalPWBytes := 0, 0
	totalGCMs, totalPWMs := 0, 0
	for _, r := range rows {
		totalGCBytes += r.GC.Bytes
		totalPWBytes += r.PW.Bytes
		totalGCMs += r.GC.DurationMs
		totalPWMs += r.PW.DurationMs
		fmt.Fprintf(&b, "| %s | %d | %d | **%.2f×** | %d | %d | **%.2f×** |\n",
			r.Site, r.GC.Bytes, r.PW.Bytes, ratio(r.GC.Bytes, r.PW.Bytes),
			r.GC.DurationMs, r.PW.DurationMs, ratio(r.GC.DurationMs, r.PW.DurationMs))
	}
	overallTokens := ratio(totalGCBytes, totalPWBytes)
	overallLat := ratio(totalGCMs, totalPWMs)
	fmt.Fprintf(&b, "| **Overall** | %d | %d | **%.2f×** | %d | %d | **%.2f×** |\n",
		totalGCBytes, totalPWBytes, overallTokens, totalGCMs, totalPWMs, overallLat)
	fmt.Fprintf(&b, "\n_Lower is better for ghostchrome columns. Ratio = pw-mcp / ghostchrome (so 3.0× means ghostchrome returns 3× less / runs 3× faster)._\n\n")
	if overallLat < 1 && mode == "cold" {
		fmt.Fprintf(&b, "> **Latency caveat:** cold-spawn mode times the full Chrome+process startup on every call. For the real LLM-agent loop (long-lived `serve` instance), see `benchmark/results-warm.md` — run `BENCH_MODE=warm ./benchmark/run-bench.sh` to regenerate.\n\n")
	}
	fmt.Fprintf(&b, "## Binary size\n\n")
	fmt.Fprintf(&b, "- ghostchrome: %.1f MB (single Go binary)\n", float64(gcBinarySize)/(1024*1024))
	fmt.Fprintf(&b, "- Playwright-MCP: requires Node.js (~80 MB) + Playwright (~250 MB w/ browsers)\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

type badge struct {
	SchemaVersion int    `json:"schemaVersion"`
	Label         string `json:"label"`
	Message       string `json:"message"`
	Color         string `json:"color"`
}

func writeBadge(path, label, message, color string) error {
	body, _ := json.MarshalIndent(badge{1, label, message, color}, "", "  ")
	return os.WriteFile(path, append(body, '\n'), 0o644)
}

func badgeColor(x float64) string {
	switch {
	case x >= 3:
		return "brightgreen"
	case x >= 2:
		return "green"
	case x >= 1.5:
		return "yellowgreen"
	case x >= 1:
		return "yellow"
	default:
		return "orange"
	}
}

func main() {
	in := flag.String("input", "benchmark/trials", "dir of per-trial JSON files")
	mdOut := flag.String("md", "benchmark/results.md", "markdown output path")
	jsonOut := flag.String("json", "benchmark/results.json", "json output path")
	badgesDir := flag.String("badges", "benchmark/badges", "shields.io endpoint JSON dir")
	binPath := flag.String("binary", "bin/ghostchrome", "path to compiled ghostchrome binary (for size badge)")
	mode := flag.String("mode", "cold", "cold | warm — annotates the report")
	flag.Parse()

	trials, err := loadTrials(*in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load trials:", err)
		os.Exit(1)
	}
	if len(trials) == 0 {
		fmt.Fprintln(os.Stderr, "no trials found in", *in)
		os.Exit(1)
	}
	rows := aggregate(trials)

	var binarySize int64
	if fi, err := os.Stat(*binPath); err == nil {
		binarySize = fi.Size()
	}

	if err := writeMarkdown(*mdOut, *mode, rows, binarySize); err != nil {
		fmt.Fprintln(os.Stderr, "write md:", err)
		os.Exit(1)
	}

	totalGCBytes, totalPWBytes := 0, 0
	totalGCMs, totalPWMs := 0, 0
	for _, r := range rows {
		totalGCBytes += r.GC.Bytes
		totalPWBytes += r.PW.Bytes
		totalGCMs += r.GC.DurationMs
		totalPWMs += r.PW.DurationMs
	}
	overallTokens := ratio(totalGCBytes, totalPWBytes)
	overallLat := ratio(totalGCMs, totalPWMs)

	summary := struct {
		Rows         []siteRow `json:"rows"`
		TokensRatio  float64   `json:"tokensRatioOverall"`
		LatencyRatio float64   `json:"latencyRatioOverall"`
		BinarySize   int64     `json:"ghostchromeBinaryBytes"`
	}{rows, overallTokens, overallLat, binarySize}
	body, _ := json.MarshalIndent(summary, "", "  ")
	if err := os.WriteFile(*jsonOut, append(body, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write json:", err)
		os.Exit(1)
	}

	_ = os.MkdirAll(*badgesDir, 0o755)
	tokensMsg := fmt.Sprintf("%.1f× fewer", overallTokens)
	tokensColor := badgeColor(overallTokens)
	if overallTokens < 1 {
		tokensMsg = fmt.Sprintf("%.1f× more", 1/overallTokens)
		tokensColor = "red"
	}
	_ = writeBadge(filepath.Join(*badgesDir, "tokens.json"),
		"tokens vs playwright-mcp",
		tokensMsg,
		tokensColor)

	latMsg := fmt.Sprintf("%.1f× faster", overallLat)
	latColor := badgeColor(overallLat)
	if overallLat < 1 {
		latMsg = fmt.Sprintf("%.1f× slower (cold spawn)", 1/overallLat)
		latColor = "orange"
	}
	_ = writeBadge(filepath.Join(*badgesDir, "latency.json"),
		"latency vs playwright-mcp",
		latMsg,
		latColor)
	if binarySize > 0 {
		_ = writeBadge(filepath.Join(*badgesDir, "size.json"),
			"binary size",
			fmt.Sprintf("%.1f MB", float64(binarySize)/(1024*1024)),
			"blue")
	}

	fmt.Printf("microbench: %d trials, %d sites → %s | tokens %.2f× | latency %.2f×\n",
		len(trials), len(rows), *mdOut, overallTokens, overallLat)
}
