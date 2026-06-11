# CLI reference

Every ghostchrome command, grouped by execution path. All commands accept
the global flags listed [at the bottom](#global-flags). Run `ghostchrome
<command> --help` for the full per-command flag list.

By default every command auto-launches a temporary Chrome and shuts it
down on exit. Pass `--connect=auto` (after running `ghostchrome serve`
once) to share one long-lived Chrome across calls — recommended for
agent loops and bulk scraping.

- [HTTP fast path](#http-fast-path)
- [Page inspection](#page-inspection)
- [Interaction](#interaction)
- [Waits and assertions](#waits-and-assertions)
- [History](#history)
- [Browser and session](#browser-and-session)
- [Automation and scraping](#automation-and-scraping)
- [Network capture and replay](#network-capture-and-replay)
- [Global flags](#global-flags)

---

## HTTP fast path

No Chrome by default. Hits the URL directly with realistic browser
headers. ~10× faster than spawning a browser when the site renders
server-side or exposes JSON APIs.

### `fastfetch <url>`

GET an HTML page, detect anti-bot challenges, extract SSR data islands
(Next.js `__NEXT_DATA__`, Nuxt `__NUXT_DATA__`, hydration JSON, etc.).
Falls back to a browser when `--fallback-browser` is set and the
response looks blocked or empty.

| Flag | Default | Effect |
|---|---|---|
| `--ua "..."` | runtime-aware Chrome 135 | Override User-Agent |
| `--accept-language "..."` | `fr-FR,fr;q=0.9,...` | Override Accept-Language |
| `--header K=V` | — | Extra header (repeatable, `K:V` also accepted) |
| `--timeout-ms N` | 8000 | HTTP deadline |
| `--raw` | off | Output the HTML body instead of the JSON envelope |
| `--next-data` | off | Output only the parsed `__NEXT_DATA__` JSON |
| `--include-payloads` | off | Embed every SSR payload in the envelope |
| `--fallback-browser` | off | Spawn Chrome and re-extract on Blocked or empty SSR |
| `--pretty` | off | Pretty-print JSON output |
| `--output PATH` | stdout | Write to file |

### `fetchapi <url>`

Issue a JSON API request with browser headers. Useful for the JSON
endpoints behind SPAs once `sniff-api` has revealed them.

| Flag | Default | Effect |
|---|---|---|
| `--method METHOD` | `GET` | HTTP verb |
| `--body JSON` | — | Request body (JSON literal or `@path/to/file`) |
| `--header K=V` | — | Extra header (repeatable) |
| `--timeout-ms N` | 8000 | HTTP deadline |
| `--pretty` | off | Pretty-print JSON output |

---

## Page inspection

### `preview <url>`

All-in-one page health report: status, console + network errors, request
count, compact DOM with refs. The first call when investigating a page.

| Flag | Default | Effect |
|---|---|---|
| `--level LEVEL` | `content` | `skeleton`, `content`, `full` |
| `--wait STRATEGY` | `domcontentloaded` | `domcontentloaded`, `load`, `stable`, `idle`, `none` |
| `--selector CSS` | — | Scope DOM extract to a subtree |
| `--no-network` | off | Skip the network section |

### `navigate <url>`

Go to a URL and optionally extract the DOM. Output: status, title, final
URL, time.

| Flag | Default | Effect |
|---|---|---|
| `--wait STRATEGY` | `domcontentloaded` | Same set as `preview` |
| `--extract LEVEL` | — | If set, extract DOM at this level after navigating |

### `extract [url]`

Compact accessibility tree with refs. URL is optional when reused with
`--connect=auto` against an already-open page.

| Flag | Default | Effect |
|---|---|---|
| `--level LEVEL` | `content` | `skeleton`, `content`, `full` |
| `--selector CSS` | — | Scope to a subtree |

### `errors [url]`

Console + browser-side Log domain (CORS, CSP, mixed content, network
ERR_*) + HTTP 4xx/5xx in one report.

| Flag | Default | Effect |
|---|---|---|
| `--level all\|error\|warning` | `error` | Filter severity |
| `--with-network` | on | Include network 4xx/5xx |

### `screenshot [url]`

PNG/JPEG/WebP of the viewport, full scrollable page, or one element.

| Flag | Default | Effect |
|---|---|---|
| `--full` | off | Capture full scrollable page |
| `--element @ref` | — | Capture one element |
| `--format webp\|jpeg\|png` | `webp` | Image format |
| `--quality 1-100` | 60 | Quality for webp/jpeg |
| `--output PATH` | `screenshot.<ext>` | Output file |

### `eval [expression] [url]`

Run a JS expression in the page, await async values, return the
serialized result.

| Flag | Default | Effect |
|---|---|---|
| `--on @ref` | — | Scope `this` to this element |
| `--timeout-ms N` | 8000 | Per-call deadline |
| `--expr "..."` | — | Alternative to positional argument |

### `perf <url>`

Lighthouse-lite timing summary: navigation, paint, network breakdown.
Lower granularity than DevTools' panel, fits agent output.

### `pdf <url>`

Print the page to PDF.

| Flag | Default | Effect |
|---|---|---|
| `--output PATH` | `page.pdf` | Output file |
| `--landscape` | off | Landscape orientation |
| `--scale N` | 1.0 | Print scale |

---

## Interaction

Refs (`@1`, `@2`, ...) come from the latest snapshot. Each interactive
command navigates if you pass a URL, or operates on the current page
with `--connect=auto`.

### `click <ref|url> [url]`

Click an element. Auto-waits for attached + visible + stable + enabled.

### `dblclick <ref> [url]`

Double-click an element by `@ref`.

### `check <ref> [url]` / `uncheck <ref> [url]`

Tick / untick a checkbox or radio by `@ref`. **Idempotent**: reads the
current `checked` state first and does nothing if already in the target
state, so `check` never accidentally toggles a box that was already ticked.

### `type <ref|text> [text] [url]`

Type into an input/textarea. Field is cleared first.

| Flag | Default | Effect |
|---|---|---|
| `--submit` | off | Press Enter after typing |
| `--clear=false` | on | Don't clear the field first |

### `press <key> [url]`

Press a keyboard key. Names: `Enter`, `Tab`, `Escape`, `ArrowUp/Down`,
`PageDown`, `F1`-`F12`, single chars.

| Flag | Default | Effect |
|---|---|---|
| `--on @ref` | — | Focus element before pressing |

### `hover [ref|url] [url]`

Mouse-hover an element. Useful for menus, tooltips, on-hover content.

### `select <ref> <value> [url]`

Pick an option in a `<select>`. For multi-select, pass a comma-separated
list or repeat `--value`.

### `fill-form [json]`

Bulk fill fields from a JSON object `{"@3": "alice", "@4": "secret"}`.
Less round-trips than individual `type` calls.

### `scroll <ref>` / `scroll-by <dy>` / `scroll-to <target>`

Scroll variants. `scroll <ref>` scrolls an element into view; `scroll-by
<dy>` scrolls the page by Y pixels; `scroll-to <target>` accepts `top`,
`bottom`, or a Y pixel value.

### `upload [ref] <file> [file2 ...]`

Upload one or more files into an `<input type=file>` by ref.

---

## Waits and assertions

### `waitfor [css-selector] [url]`

Block until a CSS selector appears. Use `--text "..."` for visible-text
waiting instead. Useful in CI scripts.

| Flag | Default | Effect |
|---|---|---|
| `--timeout N` | 30 | Seconds to wait |
| `--text "..."` | — | Wait for substring instead of selector |

### `wait-port <port>`

Block until a TCP port accepts a connection. Useful in pipelines:

```bash
bun dev &
ghostchrome wait-port 3000 --timeout 30 && \
  ghostchrome preview http://localhost:3000
```

### `wait-url <url>`

Poll a URL via HTTP (no browser) until it returns the expected status.

| Flag | Default | Effect |
|---|---|---|
| `--status N` | 200 | Expected HTTP status |
| `--timeout N` | 30 | Seconds to wait |
| `--insecure` | off | Skip TLS verification |

### `assert <subcommand> <url>`

Composable page assertions for CI/smoke tests. Exit non-zero on
mismatch, useful with `&&` chaining.

| Subcommand | Checks |
|---|---|
| `selector-visible <css>` | The CSS selector is present and visible |
| `text <substring>` | Visible page text contains the substring |
| `title-matches <pattern>` | `document.title` matches a regex |
| `url-matches <pattern>` | Final URL matches a regex |
| `no-console-errors` | Zero `console.error` events captured |
| `no-network-4xx` | Zero failed (4xx/5xx) requests |
| `count <selector>` | DOM element count matches `--eq`, `--gte`, `--lte` |

---

## History

### `back` / `forward`

Navigate back/forward in browser history. Waits for the page to stabilize.

### `reload`

Refresh the current page. Waits for the `load` lifecycle event (not full
network idle), so pages with persistent analytics/chat connections still
return instead of timing out.

### `dialog`

Manage native `alert/confirm/prompt` dialogs.

| Subcommand | Effect |
|---|---|
| `accept` | Accept the next dialog |
| `dismiss` | Dismiss the next dialog |
| `set <name=value>` | Pre-set the prompt text |

---

## Browser and session

### `sessions` (and the `-s` flag)

The ergonomic, playwright-cli-style way to keep a browser alive. `-s <name>`
(or `$GHOSTCHROME_SESSION`) on **any** command auto-launches a persistent Chrome
on first use, bound to a disk profile of the same name (cookies persist under
`~/.ghostchrome/profiles/<name>`), and reuses it — including the active tab —
across calls. No `ws://` URL to manage.

```bash
ghostchrome -s work goto https://example.com   # spawn on first use
ghostchrome -s work click @3                   # reuse, state persists
GHOSTCHROME_SESSION=work ghostchrome extract    # env var = default session
```

| Subcommand | Effect |
|---|---|
| `sessions list` | Show sessions with port, PID, liveness |
| `sessions stop <name> [--purge]` | Terminate a session's Chrome; `--purge` also deletes its profile |
| `sessions prune` | Drop dead sessions (Chrome unreachable) from the registry |
| `sessions kill-all [--purge]` | Stop every session; `--purge` also deletes each profile |

### `profiles`

Persistent Chrome profiles (`~/.ghostchrome/profiles/<name>`) store cookies and
cache so logins/sessions survive. They accumulate disk — list and remove them.

| Subcommand | Effect |
|---|---|
| `profiles list` | Show profiles sorted by on-disk size (+ total) |
| `profiles rm <name> [name...]` | Delete profiles and reclaim their disk |

### `uninstall`

Stop all sessions and remove the ghostchrome binary. `--purge` also deletes the
data directories (profiles, sessions, contexts, cache). Dry-run unless `--yes`.

```bash
ghostchrome uninstall                # print what would be removed
ghostchrome uninstall --yes          # remove the binary + bundled skill
ghostchrome uninstall --purge --yes  # remove the binary, skill, AND all data
```

### `skills`

ghostchrome bundles an agent skill (teaches Claude Code how to drive it) inside
the binary. The install script installs it globally; uninstall removes it.

| Subcommand | Effect |
|---|---|
| `skills install` | Write the bundled skill to `~/.claude/skills/ghostchrome/` |
| `skills remove` | Remove the installed skill |
| `skills status` | Show whether it's installed and where |

A session resolves to `--connect` internally; `--user-profile`/`--proxy`/
`--stealth` are applied when the session's Chrome is first spawned.

### `serve`

Launch a long-lived Chrome and print its WebSocket URL. Other commands
attach with `--connect=auto` (or an explicit `--connect=ws://...`).
For most workflows `-s <name>` (above) is simpler — it spawns and reuses
the browser for you.

| Flag | Default | Effect |
|---|---|---|
| `--port N` | random | Chrome remote debugging port |

### `mcp`

Run ghostchrome as a Model Context Protocol server (stdio). Exposes 11
tools to LLM agents — see [`docs/mcp.md`](mcp.md).

### `contexts`

Manage named isolated browser contexts (incognito) inside a connected
Chrome. Letting multiple agents share one Chrome via separate contexts
avoids spawning multiple Chrome processes.

| Subcommand | Effect |
|---|---|
| `list` | Show all contexts |
| `clear <name>` | Reset cookies/storage of a context |
| `close <name>` | Close a context |
| `delete <name>` | Remove context + its profile dir |
| `save <name>` | Persist storageState to disk |
| `load <name> <state.json>` | Restore from a previously saved state |

### `tabs`

| Subcommand | Effect |
|---|---|
| `list` (default) | Show open tabs (index, URL, title) |
| `new [url]` | Open + activate a new tab (blank if no URL) |
| `switch <index>` | Make a tab active |
| `close <index>` | Close a tab |

### `viewport [width] [height] [url]`

Set the viewport dimensions. With no args, prints the current viewport.

### `cookies`

| Subcommand | Effect |
|---|---|
| `get [domain]` | Print cookies for a domain |
| `inspect` | Detailed cookie info (security flags, partition, ...) |
| `set <name=value>` | Set a cookie |

### `storage save <path>` / `storage load <path>`

Save/restore the full storage state (cookies + localStorage +
sessionStorage) as Playwright-compatible JSON.

### `import-profile`

Copy cookies/storage from a real Chrome/Edge/Brave profile on the same
machine into a ghostchrome profile. Useful to bootstrap a session that
needs to be already logged in.

### `login [site|url]`

Open a real browser window for the user to log in to a known site, then
persist cookies/storage into the active profile. Bundled flows for
common sites (Google, GitHub, LinkedIn, ...).

### `emulate`

Emulate a device preset (`iphone-15`, `pixel-8`, etc.). Sets viewport,
user agent, touch capability.

### `geolocation`

Override geolocation. Useful for testing locale-gated content.

### `extensions`

Manage bundled Chrome extensions (uBlock Lite, ICDC, Force Background
Tab) under `~/.ghostchrome/extensions/`. Auto-loaded with
`--default-extensions`.

| Subcommand | Effect |
|---|---|
| `install` | Install the curated default set |
| `list` | List installed extensions |

### `ai <goal>`

LLM-assisted action: given a natural-language goal, ghostchrome calls
the configured AI provider to plan and execute a series of CLI commands.
Requires an `ANTHROPIC_API_KEY` or compatible env var.

---

## Automation and scraping

### `agent <jsonl>`

Run an agent recipe from a JSONL file: each line is a step
(`{"op":"navigate","url":"..."}`, `{"op":"click","ref":"@3"}`, ...).
Preferred for deterministic multi-step scrapes.

### `batch [script]`

Like `agent`, but YAML/JSON script with retries, error handling, and
parallel branches.

### `collect <url> [url2 ...]`

Observer stream: NDJSON of Network + Console + Page events as the page
loads. Use for live debugging or to feed an analytics pipeline.

| Flag | Default | Effect |
|---|---|---|
| `--duration NS` | 10s | How long to listen |
| `--filter-net TYPES` | — | Restrict net events (e.g. `xhr,fetch,document`) |
| `--filter-url REGEX` | — | URL regex |

### `capture [url]`

One-shot page capture: HTML + screenshots + cookies + storage + network
log — everything bundled for offline replay/debug.

### `record <url>`

Interactive recorder: lets a human drive a real browser; ghostchrome
emits the equivalent agent JSONL script the agent can replay later.

---

## Network capture and replay

### `intercept`

Manage URL pattern rules: block, modify, or replay requests at the
network layer. Useful for testing failure modes or de-flaking flaky
endpoints.

| Subcommand | Effect |
|---|---|
| `add <pattern>` | Add a rule (block / mock / replay) |
| `list` | Show active rules |
| `clear` | Remove all rules |

### `sniff-api <url>`

Listen to all JSON/API requests a page issues and emit them in a compact
catalogue. Pairs with `fetchapi` for headless re-querying.

### `http-replay <url>`

Replay a captured HAR or NDJSON trace against a page (record once,
deflake forever).

### `trace-clear` / `trace-export` / `trace-replay`

Manage Playwright-compatible trace recordings:

- `trace-clear` — reset trace buffer
- `trace-export <path>` — write `.zip` (Playwright trace viewer reads it)
- `trace-replay <path>` — replay events against the live page

### `algolia`

Inspect Algolia search backends (almost every e-commerce SPA uses one).
Sniffs the index name and surfaces a query-ready request.

---

## Global flags

Apply to every command (where meaningful):

| Flag | Default | Effect |
|---|---|---|
| `-s, --session NAME` | `$GHOSTCHROME_SESSION` | Auto-managed persistent session: spawn a Chrome (profile `NAME`) on first use, reuse it after. |
| `--connect URL` | — | Attach to existing Chrome (`auto` to discover on 127.0.0.1:9222-9229). |
| `--context NAME` | — | Use a named isolated context in the connected Chrome (parallel sessions, no extra Chrome). |
| `--headless` | true | Headless mode. Set `--headless=false` to show a window. |
| `--invisible` | false | macOS-only: collapse the window to invisible. |
| `--user-profile NAME` | — | Persist cookies/storage at `~/.ghostchrome/profiles/NAME/`. |
| `--stealth` | false | Bundled fingerprint patches + script blocker. |
| `--block-trackers` | — | Network-layer block of DataDome, PerimeterX, etc. Auto-on with `--stealth`. |
| `--default-extensions` | false | Load bundled extensions on auto-launch. |
| `--proxy URL` | — | Route through proxy (basic auth via `http://user:pass@host:port`). |
| `--dismiss-cookies` | false | Auto-dismiss consent banners on every navigation. |
| `--timeout N` | 30 | Per-command timeout in seconds. |
| `--json` | false | Emit structured JSON instead of text. |
| `--pretty` | false | Pretty-print JSON output (paired with `--json`). |
| `--output PATH` | stdout | Write structured output to a file. |

Environment variables:

| Var | Effect |
|---|---|
| `GHOSTCHROME_MCP_LAZY=1` | MCP server defers Chrome spawn until the first tool call. |
| `GHOSTCHROME_MCP_NO_BLOCKER=1` | Disable the auto-on script blocker even with `--stealth`. |
| `GHOSTCHROME_MCP_TRACE=path` | Persist a trace of every MCP tool call to `path` (debugging). |
| `GHOSTCHROME_BROWSER_PATH=/...` | Pin the Chromium binary explicitly. |
