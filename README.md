# ghostchrome

**Ultra-light browser automation CLI for LLM agents.** Single Go binary, native Chrome DevTools Protocol, 5× fewer tokens than Playwright CLI, faster on 19/20 operations, no Node runtime. A modern Playwright alternative built for AI agents that drive a browser in a loop.

[![Go](https://img.shields.io/github/go-mod/go-version/MakFly/ghostchrome?logo=go)](go.mod)
[![Release](https://img.shields.io/github/v/release/MakFly/ghostchrome?label=release&logo=github)](https://github.com/MakFly/ghostchrome/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![tokens vs playwright-mcp](https://img.shields.io/endpoint?url=https%3A%2F%2Fraw.githubusercontent.com%2FMakFly%2Fghostchrome%2Fmain%2Fbenchmark%2Fbadges-warm%2Ftokens.json)](benchmark/results-warm.md)
[![latency vs playwright-mcp](https://img.shields.io/endpoint?url=https%3A%2F%2Fraw.githubusercontent.com%2FMakFly%2Fghostchrome%2Fmain%2Fbenchmark%2Fbadges-warm%2Flatency.json)](benchmark/results-warm.md)
[![binary size](https://img.shields.io/endpoint?url=https%3A%2F%2Fraw.githubusercontent.com%2FMakFly%2Fghostchrome%2Fmain%2Fbenchmark%2Fbadges%2Fsize.json)](#install)

```console
$ ghostchrome preview http://localhost:3000
[200] Dashboard — http://localhost:3000 (134ms)
[errors] none
[network] 12 reqs, 0 failed
[dom]
  h1 Dashboard
  @1 b Add user
  table 5 rows
  @2 a>/settings Settings
```

One command. ~50 ms warm. ~2,000 tokens. Refs (`@1`, `@2`) you can click and type into next.

---

## Table of contents

- [Why ghostchrome](#why-ghostchrome)
- [Benchmark](#benchmark)
- [Install](#install)
- [Quickstart](#quickstart)
- [How it works](#how-it-works)
- [Comparison with playwright-cli, Playwright, Puppeteer, chromedp](#comparison)
- [Using it with LLM agents](#using-it-with-llm-agents)
- [Command reference](#command-reference)
- [Playwright CLI parity](#playwright-cli-parity)
- [Status & roadmap](#status--roadmap)
- [Contributing](#contributing)
- [License](#license)

---

## Why ghostchrome

LLM-driven browser automation has a token problem. Playwright-MCP returns a full accessibility tree on every snapshot — typically **14,000-50,000 tokens** for a real-world page — which burns the agent's context window and slows every iteration. ghostchrome was built to fix that one thing: return the smallest possible payload that an LLM still needs to act, in a single static Go binary that boots in milliseconds.

**Designed for AI agents** that drive a browser via Claude Code, the Anthropic Agent SDK, Aider, Cursor, OpenAI's Agents SDK, or any custom loop. Use it as a **Playwright alternative for headless Chrome web scraping**, as a **CDP CLI** for ops automation, or as the browsing tool behind a custom agent. No JSON-RPC overhead, no Node runtime, no `npm install`. Just `ghostchrome <command> <url>` and read the output.

What you get:
- **Filtered accessibility tree** — only interactive elements get refs (`@1`, `@2`), 3-5× fewer nodes than a full a11y dump.
- **Three extraction levels** — `skeleton` (minimal), `content` (text), `full` (everything named).
- **Transparent daemon** — every command auto-spawns a persistent background Chrome on first use (no `serve`, no `--connect`, zero config). Just run `ghostchrome goto <url>` and it works.
- **CDP-native** — built on [Rod](https://github.com/go-rod/rod), so iframe handling, stealth patches, and event capture work out of the box.
- **Single ~19 MB binary** — no Node.js, no `npm install`, no Playwright browsers download.
- **Three ways to drive it** — the CLI, an MCP server (16 tools, drop-in for `@playwright/mcp`), or typed Python / TypeScript SDKs over the persistent JSONL `agent` loop.

---

## Benchmark

Reproducible head-to-head against [@playwright/mcp](https://github.com/microsoft/playwright-mcp) and [Playwright CLI](https://playwright.dev/agent-cli/) on 5 local HTML fixtures + real public sites. Run it yourself:

```bash
./benchmark/run-bench.sh                 # cold-spawn mode (default)
BENCH_MODE=warm ./benchmark/run-bench.sh # long-lived session (real agent loop)
./benchmark/run-bench-playwright-cli.sh   # Playwright CLI: cold mode
BENCH_MODE=warm ./benchmark/run-bench-playwright-cli.sh # Playwright CLI warm mode
```

### Warm session — the real LLM-agent loop

Both tools keep one process alive across navigate+snapshot calls. This is what your agent actually does.

| Site | ghostchrome tokens | pw-mcp tokens | ghostchrome ms | pw-mcp ms |
|---|---:|---:|---:|---:|
| dashboard (CRUD table) | 549 | 2,746 | 50 | 64 |
| product page | 390 | 1,456 | 45 | 55 |
| news feed | 851 | 2,242 | 40 | 51 |
| search results | 1,224 | 2,421 | 60 | 73 |
| Hacker News (live) | 3,416 | 14,564 | 660 | 1,023 |
| **Overall** | **6,832** | **24,961** | **1,020 ms** | **1,660 ms** |

→ **3.65× fewer tokens, 1.63× faster** per snapshot. Full table: [`benchmark/results-warm.md`](benchmark/results-warm.md).

### Cold spawn — every invocation starts fresh

Apples-to-apples wall time of `process start → Chrome attach → navigate → snapshot → exit` for both tools. Chrome startup dominates and ghostchrome is ~10% slower here — which is why you should use warm session (above) for any agent workload.

→ **3.5× fewer tokens, 0.91× as fast** overall (cold). Full table: [`benchmark/results.md`](benchmark/results.md).

### ghostchrome vs playwright-cli (warm daemon)

Both tools with their daemon running, same pages. See the full 20-operation table in [Comparison](#comparison).

| Site | ghostchrome bytes | pw-cli bytes | ghostchrome ms | pw-cli ms |
|---|---:|---:|---:|---:|
| example.com (snapshot) | 202 | 411 | 19 | 61 |
| Hacker News (snapshot) | 13,845 | 57,553 | 110 | 133 |
| Wikipedia (snapshot) | 2,056 | 10,611 | 46 | 104 |
| GitHub repo (snapshot) | 15,211 | 108,828 | 273 | 198 |

→ **5.3× fewer tokens overall, faster on 19/20 operations.**

### Binary & footprint

| | ghostchrome | playwright-cli |
|---|---|---|
| Runtime | Static Go binary | Node.js |
| Install size | ~19 MB | ~330 MB (Node + Playwright + FFmpeg) |
| Cold daemon start | 315 ms | 600 ms |
| Daemon required | transparent (auto) | explicit (`open` first) |
| Dependencies | Chrome on the system *or* auto-downloaded by Rod | npm + `playwright install` |
| Protocol | CLI stdin/stdout, optional MCP server | CLI stdin/stdout |

Token estimates assume `ceil(bytes/4)`, the standard rule-of-thumb for BPE tokenizers. Numbers are medians on Linux x86_64, June 2026.

---

## Install

ghostchrome installs the same way [`@playwright/cli`](https://www.npmjs.com/package/@playwright/cli)
does — one command to get the binary, one command to wire it into your coding
agent — except there is **no Node runtime and no browser download**: it's a
single static Go binary.

| | playwright-cli | ghostchrome |
|---|---|---|
| Get the tool | `npm install -g @playwright/cli` | `curl \| bash` or `bun install -g @ghostchrome/cli` |
| Wire into the agent | `playwright-cli install --skills` | `ghostchrome install --skills` |
| Daemon | requires `open` before `goto` | transparent — just run any command |
| Uninstall | manual | `ghostchrome uninstall --purge --yes` |
| Runtime | Node.js + Playwright + FFmpeg (~330 MB) | one ~19 MB binary, system Chrome |

### 1. Install the CLI

```bash
bun install -g @ghostchrome/cli       # or: bunx @ghostchrome/cli <cmd>
# npm install -g @ghostchrome/cli     # works too
```

The package resolves the prebuilt Go binary for your platform (Linux/macOS,
amd64/arm64; Windows amd64) — no Node runtime, no postinstall, no browser
download. The bundled agent skill is installed globally to
`~/.claude/skills/ghostchrome/` (and removed on `ghostchrome uninstall`); the
curl installer below does this automatically, or run `ghostchrome skills install`.
Prefer a single binary with no package manager? Use the installer:

```bash
curl -fsSL https://raw.githubusercontent.com/MakFly/ghostchrome/main/scripts/install.sh | bash
```

Either way, verify it works:

```bash
ghostchrome --version
ghostchrome doctor          # checks Chrome, profiles, connectivity
```

### 2. Wire it into your coding agent

```bash
# Claude Code — register the MCP server (16 tools, drop-in for @playwright/mcp)
claude mcp add ghostchrome -- ghostchrome mcp

# …or attach to an already-running Chrome instead of launching one
claude mcp add ghostchrome -- ghostchrome mcp --connect=auto
```

For Codex, Cursor, Aider, or a custom loop see [Using it with LLM agents](#using-it-with-llm-agents).

### Other install methods

- **Prebuilt binaries** — macOS (Intel/ARM), Linux (amd64/arm64), Windows on the
  [Releases](https://github.com/MakFly/ghostchrome/releases) page (`ghostchrome` +
  `ghostchrome-mcp`, with `checksums.txt`).
- **From source** — `git clone https://github.com/MakFly/ghostchrome && cd ghostchrome && go build -o ghostchrome .`

> **Note:** `go install …@latest` is not supported on this repo. Versioning was
> reset to `v0.1.0`, but the earlier `v1.0.0` is pinned immutably in the Go module
> proxy, so `@latest` resolves to stale code. Use the installer, a prebuilt binary,
> or build from source.

### Requirements

- Chrome or Chromium installed. If none is found, [Rod](https://github.com/go-rod/rod) auto-downloads a compatible Chromium to `~/.cache/rod/` on first run.

---

## Quickstart

Every command auto-spawns a persistent background Chrome on first use — no `serve`, no `open`, no setup. Just run the command.

### See a page

```bash
ghostchrome preview https://example.com
```

Single command returns status code, page title, console + network errors, request count, and a compact DOM with refs. The first call auto-starts the daemon; subsequent calls reuse it (~15 ms overhead).

### Extract a clickable DOM

```bash
ghostchrome extract https://news.ycombinator.com --level content
```

Compact accessibility tree with refs (`@1`, `@2`, …). Three levels: `skeleton` (interactive only), `content` (adds text), `full` (everything named).

### Drive the page

```bash
# Each command can navigate first, then act, then return the new snapshot.
ghostchrome click @3 https://example.com/login
ghostchrome type  @1 "alice@example.com" https://example.com/login
ghostchrome press Enter https://example.com/login
```

Refs come from the previous snapshot. The browser session is preserved automatically via the implicit daemon (no `--connect` needed).

### Named sessions (`-s`, playwright-cli-style)

```bash
ghostchrome -s work goto https://example.com/login   # spawns a persistent Chrome on first use
ghostchrome -s work type  @1 "alice@example.com"      # reuses it — no ws:// to copy, state persists
ghostchrome -s work click @3
ghostchrome -s work extract --level content

ghostchrome sessions list           # work  :PORT  alive  pid …
ghostchrome sessions stop work      # tear it down
```

`-s <name>` (or `$PLAYWRIGHT_CLI_SESSION`, falling back to `$GHOSTCHROME_SESSION`) auto-launches a
persistent Chrome on first use, bound to a disk profile of the same name (cookies persist under
`~/.ghostchrome/profiles/<name>`), and reuses it — including the active tab — across calls.
Per-call latency drops to ~50 ms. No ws:// URL to manage. Manage sessions with
`ghostchrome sessions list | stop <name> | kill-all`.

> Prefer to manage Chrome yourself? `ghostchrome serve --port 9222` prints a ws:// URL and any
> command can attach with `--connect=auto` (discovers a serve on 127.0.0.1:9222-9229).

### Debug a page

```bash
ghostchrome errors https://your-site.test --level all
```

Captures `Runtime.consoleAPICalled` + `Runtime.exceptionThrown` + `Log.entryAdded` (CORS, CSP, mixed content, network ERR_*) + every HTTP 4xx/5xx — all in one snapshot.

---

## How it works

```
your agent → ghostchrome CLI → Rod (Go) → Chrome DevTools Protocol → Chrome
```

1. **CDP Accessibility tree** is fetched and **filtered**: only nodes that are interactive (or named ancestors) are kept. Everything is compressed into one indented text format with `@N` refs.
2. **Three extraction levels** let an agent ask for exactly the granularity it needs. Most agent loops stay at `content`.
3. **Refs are stable within a snapshot** and replayed on the next command via element-state cache, so `click @3` works without a new selector.
4. **Output is text first** — no JSON wrapping unless you ask for `--json`. The agent reads what a human would read in DevTools.
5. **Transparent daemon** — auto-spawns a persistent background Chrome on first use. Named sessions (`-s work`, `-s research`) run parallel isolated browsers. No `serve` needed.

Architecture deep dive: [`docs/architecture.md`](docs/architecture.md). Full CLI reference: [`docs/cli.md`](docs/cli.md). MCP server (16 tools): [`docs/mcp.md`](docs/mcp.md). Anti-bot story: [`docs/anti-bot.md`](docs/anti-bot.md). Fast HTTP path: [`docs/fast-path.md`](docs/fast-path.md).

---

## Comparison

### ghostchrome vs playwright-cli — head-to-head (20 operations)

Both tools running in daemon mode (persistent background Chrome, warm session).
Measured on real public sites, Linux x86_64, June 2026.

| # | Operation | playwright-cli | ghostchrome | Winner |
|---|---|---:|---:|---|
| 1 | goto example.com | 95 ms | 35 ms | **ghostchrome** |
| 2 | goto Hacker News | 704 ms | 655 ms | **ghostchrome** |
| 3 | goto Wikipedia | 654 ms | 505 ms | **ghostchrome** |
| 4 | goto GitHub | 1,995 ms | 1,512 ms | **ghostchrome** |
| 5 | goto httpbin | 414 ms | 361 ms | **ghostchrome** |
| 6 | snapshot example.com | 61 ms | 19 ms | **ghostchrome** |
| 7 | snapshot Hacker News | 133 ms | 110 ms | **ghostchrome** |
| 8 | snapshot Wikipedia | 104 ms | 46 ms | **ghostchrome** |
| 9 | snapshot GitHub | 198 ms | 273 ms | playwright-cli |
| 10 | snapshot httpbin | 59 ms | 20 ms | **ghostchrome** |
| 11 | click | 2,863 ms | 1,168 ms | **ghostchrome** |
| 12 | type | 88 ms | 25 ms | **ghostchrome** |
| 13 | go-back | 145 ms | 38 ms | **ghostchrome** |
| 14 | reload | 272 ms | 176 ms | **ghostchrome** |
| 15 | resize | 81 ms | 27 ms | **ghostchrome** |
| 16 | eval | 574 ms | 24 ms | **ghostchrome** |
| 17 | press Tab | 68 ms | 23 ms | **ghostchrome** |
| 18 | press Escape | 66 ms | 20 ms | **ghostchrome** |
| 19 | screenshot | 166 ms | 142 ms | **ghostchrome** |
| 20 | sessions list | 63 ms | 14 ms | **ghostchrome** |

**Score: ghostchrome 19 / 20, playwright-cli 1 / 20.**
The single playwright-cli win is snapshot on a very large page (GitHub repo, ~108K nodes) where the first CDP accessibility-tree extraction is expensive. Subsequent snapshots of the same page hit the ghostchrome cache and are instant.

### Feature comparison

| | ghostchrome | playwright-cli | Playwright (raw) | Puppeteer | chromedp |
|---|---|---|---|---|---|
| Target | LLM agents | LLM agents | Devs / QA | Devs | Devs (Go) |
| Runtime | Static Go binary | Node.js | Node.js | Node.js | Go binary |
| Install | `curl \| sh` or `bun i -g` | `npm i -g @playwright/cli` | npm + browser DL | npm + browser DL | `go install` |
| Install size | ~19 MB | ~330 MB | ~330 MB | ~280 MB | ~20 MB |
| Daemon | transparent (auto) | requires `open` first | n/a | n/a | n/a |
| Snapshot tokens | ~500–3,500 | ~2,700–57,000 | n/a (raw HTML) | n/a | n/a |
| Token ratio | **1×** | **5.3× larger** | — | — | — |
| Multi-browser | Chrome only | Chrome / FF / WebKit | Chrome / FF / WebKit | Chrome / FF | Chrome only |
| Refs for click/type | `@1`, `@2` | `e1`, `e2` | CSS / XPath | CSS / XPath | CSS / XPath |
| Stealth | built-in patches | none | external plugin | external plugin | manual |
| Snapshot caching | yes (by URL) | yes (in-process) | n/a | n/a | n/a |
| Uninstall | `ghostchrome uninstall` | manual | manual | manual | manual |

### When to pick what

- **ghostchrome** — you're piloting a browser from an LLM agent and tokens, latency, and footprint matter. Single binary, zero-config daemon, 5× fewer tokens per snapshot.
- **playwright-cli** — you need WebKit / Firefox, Playwright Trace Viewer, or `run-code` (arbitrary Playwright API execution).
- **Playwright (raw)** — you're writing E2E test suites, not driving an agent.

### Parity with playwright-cli

ghostchrome covers the **agent-relevant verb surface** of
[`@playwright/cli`](https://www.npmjs.com/package/@playwright/cli) —
`open/goto`, `click`, `dblclick`, `type`/`fill`, `check`/`uncheck`,
`select`, `hover`, `drag`, `press`, `upload`, `snapshot`/`extract`, `eval`,
`reload`, `back`/`forward`, `tabs`, cookies & storage, `screenshot`, `pdf`,
`route`, `console`, `network`, `dialog-*`, `attach`, sessions, config — plus
things playwright-cli has no equivalent for: `preview` (one-shot page health),
`collect` (auto-listing extraction), `perf` (Web Vitals), `assert` (CI exit
codes), built-in stealth, and transparent daemon (no `open` needed).

Explicit non-goals: WebKit/Firefox, `run-code` (Playwright runtime),
`pause-at`/`resume`/`step-over` (Playwright debug protocol), Playwright Trace
Viewer-compatible `trace.zip`.

Full parity matrix: [`docs/playwright-cli-parity.md`](docs/playwright-cli-parity.md).

---

## Using it with LLM agents

One binary, **three surfaces**, same engine:

1. **MCP stdio server** (`ghostchrome mcp`) — 16 tools, the drop-in replacement for `@playwright/mcp`.
2. **Regular CLI** — allowlist `ghostchrome` for shell-tool agents.
3. **Typed SDKs** (`sdk/python`, `sdk/typescript`) — drive the persistent JSONL `agent` loop from code.

### Claude Code (Anthropic)

```bash
claude mcp add ghostchrome -- ghostchrome mcp --stealth
```

That's it. Claude Code will spawn `ghostchrome mcp` in stdio mode on demand and route the 16 tools to the model. Add `--connect=auto` to attach to an already-running Chrome instead of launching one.

### Codex (OpenAI)

```bash
codex mcp add ghostchrome -- ghostchrome mcp --stealth
```

### MCP tool surface (v2.0)

Deliberately small — 16 tools, no fat. Each one is on the hot path of a browser-driving loop. (Earlier versions exposed 38; see [`docs/mcp.md`](docs/mcp.md) for why it was trimmed.)

| Tool | Purpose |
|---|---|
| `snapshot` | Status + errors + network + DOM with refs — canonical first call |
| `navigate` | Go to URL without snapshot |
| `click` | Click `@ref` |
| `type` | Type into `@ref` (`submit:true` to press Enter after) |
| `select` | Pick option in `<select>` by `@ref` |
| `press` | Send key (Enter, Tab, Escape, ArrowDown, ...) |
| `hover` | Hover an element by `@ref` (reveal dropdowns, tooltips) |
| `drag` | Drag from one `@ref` to another |
| `fill_form` | Bulk-fill form fields from `{ref: value}` JSON |
| `upload` | Attach files to an `<input type=file>` by `@ref` |
| `tabs` | List / switch / open / close browser tabs |
| `wait_for` | Wait for selector / text / timeout |
| `eval` | Run JS — escape hatch for anything else |
| `screenshot` | WebP/JPEG/PNG of viewport, full page, or element |
| `back` / `forward` | Browser history |

Niche workflows (cookies, storage, viewport, network sniff/replay, tracing) live in the CLI only. Reach them via `eval` or shell out when needed.

### Typed SDKs — Python & TypeScript

In-repo at [`sdk/python/`](sdk/python) and [`sdk/typescript/`](sdk/typescript). Each is a thin, typed client that spawns a persistent `ghostchrome agent` subprocess and speaks its JSONL protocol over stdio, so refs (`@1`, `@2`) and session state persist across calls. Result types are matched to what the binary actually emits (re-measured with [`scripts/measure-agent-ops.sh`](scripts/measure-agent-ops.sh), never guessed).

> **Not published to any package registry yet.** The SDK source lives in this repo
> (and in the `v0.1.0` source tarball), but the packages are **not** on npm or PyPI —
> so `npm install @ghostchrome/sdk` / `pip install ghostchrome` do **not** work yet.

| Channel | Status | How to install |
|---|---|---|
| GitHub repo — `sdk/python`, `sdk/typescript` | ✅ available | clone, or `pip install "git+…#subdirectory=sdk/python"` (below) |
| npm — `@ghostchrome/sdk` | ❌ not published | — |
| PyPI — `ghostchrome` | ❌ not published | — |

Both SDKs require the `ghostchrome` binary on `PATH`.

```python
# pip install "git+https://github.com/MakFly/ghostchrome.git#subdirectory=sdk/python"
from ghostchrome import Ghostchrome

with Ghostchrome(extra_flags=["--connect=auto"]) as gc:
    nav, _ = gc.navigate("https://example.com")
    print(nav.status, nav.title)            # 200, "Example Domain"
    tree, _ = gc.extract(level="skeleton")
    print(tree.stats.interactive_count)     # @ref count
    gc.click("@1")
```

```ts
// build + local install: cd sdk/typescript && bun run build && bun add /path/to/sdk/typescript
import { createGhostchrome } from "@ghostchrome/sdk";

const gc = createGhostchrome({ flags: ["--connect=auto"] });
const { result } = await gc.navigate("https://example.com");
console.log(result.status, result.title);
const dom = await gc.extract({ level: "skeleton" });
await gc.close();
```

Runnable end-to-end examples (both languages) live in [`examples/`](examples).

### Custom loop — shell-out, zero SDK

```python
import subprocess, json
def snapshot(url):
    r = subprocess.run(
        ["ghostchrome", "preview", url, "--connect=auto", "--json"],
        capture_output=True, text=True, check=True,
    )
    return json.loads(r.stdout)
```

### Aider / Cursor / any agent with shell access

Use `ghostchrome` as a regular shell command. The daemon starts automatically — no `serve` step.

Recipes: [`docs/recipes/`](docs/recipes/) — Algolia, AutoScout24, bulk scrape, registry sweep, agent JSONL mode.

---

## Command reference

<details>
<summary>Click to expand the full command surface</summary>

```text
Page inspection
  preview <url>                 Page health: status, errors, network, DOM
  navigate <url>                Navigate; optionally extract
  extract  <url>                Compact accessibility tree with refs
  screenshot <url>              PNG of viewport, full page, or element
  eval "<expr>" <url>           Run JS, await async, return value
  errors <url>                  Console + Log + network 4xx/5xx
  perf <url>                    Lighthouse-lite timing summary

Interaction (refs from the last snapshot)
  click @N <url>
  dblclick @N <url>             Double-click an element
  type @N "text" [--submit]     Type; --submit presses Enter after
  fill-form <json>              Bulk fill {@ref: value}
  check @N / uncheck @N         Idempotent checkbox / radio toggle
  select @N "option" <url>
  hover @N <url>
  drag @from @to                Drag-and-drop between refs
  press <key> [--on @N] <url>
  upload @N <file...>           Attach files to a file input

Browser & session
  serve [--port N]              Long-lived Chrome; prints ws:// URL
  tabs                          List tabs
  tabs new [url]                Open + activate a new tab
  tabs switch <i> / close <i>   Switch / close a tab by index
  reload                        Refresh the current page
  back / forward
  waitfor "selector" <url>
  import-profile                Clone an existing Chrome profile (cookies)
  doctor                        Diagnose setup (Chrome, profiles, connectivity)

Scraping & bulk
  batch <jsonl>                 Run agent ops from a JSONL file
  fastfetch <url>               HTML-only fast path, no JS render
  collect <url>                 Observer stream (NDJSON of net+console+page events)

Agents
  agent                         Drive the browser from JSONL ops on stdin
  mcp                           Run as an MCP server (stdio, 16 tools)
```

Full details: [`docs/cli.md`](docs/cli.md).

</details>

---

## Playwright CLI parity

ghostchrome exposes Playwright CLI-compatible command names for the core
browser loop where the behavior maps cleanly to existing CDP/Rod primitives:
`open`, `snapshot`, `fill`, `resize`, `go-back`, `go-forward`, `state-save`,
`state-load`, `attach --cdp=<channel|url>`, `cookie-*`, `localstorage-*`,
`sessionstorage-*`, `dialog-*`, `tab-*`, session management aliases, and raw
mouse/key aliases.

The tracked source-of-truth matrix is [`docs/playwright-cli-parity.md`](docs/playwright-cli-parity.md).
It separates compatible commands from partial matches and explicit gaps so the
project does not claim parity that is not implemented.

---

## Status & roadmap

**Stable** — preview, navigate, extract, click/type/select/hover/press, errors, screenshot, eval, serve, `--connect=auto`, MCP server (16 tools), JSONL `agent` loop, typed Python & TypeScript SDKs.

**Experimental** — stealth patches, AI extractors, opt-in content-boundary fencing. Tracked behind flags; APIs may change.

**Not in scope (yet)** — Firefox/WebKit support (would arrive via a `playwright-core` subprocess fallback, not native), GUI test runner, visual regression diff.

Versioning follows SemVer; see [`.claude/rules/versioning.md`](.claude/rules/versioning.md).

---

## Contributing

PRs welcome. The codebase is small and laid out in [`engine/`](engine/) (CDP logic) and [`cmd/`](cmd/) (one Cobra command per file). Run tests with `go test ./...`. Bench changes should include a re-run of `./benchmark/run-bench.sh` so reviewers can verify the numbers don't regress.

When the agent surface changes, **re-measure the live binary** with [`scripts/measure-agent-ops.sh`](scripts/measure-agent-ops.sh) and update the in-repo SDKs at [`sdk/typescript/`](sdk/typescript) and [`sdk/python/`](sdk/python) so their result types match what the binary emits — never guess. See [`CLAUDE.md`](CLAUDE.md).

---

## License

[MIT](LICENSE) © 2026 MakFly.
