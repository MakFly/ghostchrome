# Ghostchrome Benchmark Results

Date: 2026-04-03

## Token Efficiency (chars/4 ≈ tokens)

| Site | skeleton | content | full |
|---|---|---|---|
| example.com | ~19 tokens | ~61 tokens | ~78 tokens |
| github.com | ~1,968 tokens | ~4,361 tokens | ~4,607 tokens |
| news.ycombinator.com | ~4,712 tokens | ~7,071 tokens | ~9,207 tokens |

## Comparison vs Playwright MCP

| Metric | Playwright MCP | ghostchrome (skeleton) | ghostchrome (content) |
|---|---|---|---|
| Tokens/snapshot (GitHub) | 14,000-19,000 | **1,968** | **4,361** |
| Tokens/snapshot (HN) | 15,000-30,000 | **4,712** | **7,071** |
| Reduction | baseline | **7-10x fewer** | **3-4x fewer** |
| Tool definitions overhead | ~13,700 tokens | **0** (CLI) | **0** (CLI) |

## Performance

| Metric | Value |
|---|---|
| Binary size | 15 MB |
| Navigate + page load | ~800ms (example.com) |
| Peak RSS (tool only) | ~191 MB (includes Chrome) |
| Go binary RSS alone | ~15 MB |

## vs Alternatives

| Tool | Runtime | Binary/deps | Tokens/snapshot |
|---|---|---|---|
| Playwright MCP | Node.js | ~200 MB | 14,000-50,000 |
| Playwright CLI | Node.js | ~200 MB | ~6,750 |
| agent-browser | Rust+Node daemon | ~50 MB | 200-400 |
| chrome-agent | Rust | ~10 MB | ~50 (inspect) |
| **ghostchrome** | **Go** | **15 MB** | **1,968-4,712 (skeleton)** |
