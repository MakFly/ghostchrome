---
name: ghostchrome
description: Native browser automation for LLM agents. Use whenever the user asks to scrape, navigate, click, type, screenshot, extract data from a webpage, or run a browser-based workflow. Triggers on "scrape X", "navigate to X", "extract from X", "automate X site", "fill the form on X", "screenshot X", "récupère les annonces X", "ouvre X dans un navigateur", "click on X", "test X website". Avoid for: pure HTTP fetches (use curl), reading static files (use Read), one-off URL inspection (use WebFetch).
---

# ghostchrome — native browser automation for agents

`ghostchrome` is a single Go binary that controls Chrome via CDP. It exposes
three surfaces useful to an agent: **per-call CLI commands with a managed
`-s` session** (the simplest for a shell-driven agent), a **JSONL agent loop**
(`ghostchrome agent`) for tight pipelined flows, and **site-specific recipes**
that return structured JSON in a single call.

Install (if not already on PATH): `bun install -g @ghostchrome/cli`, or
`curl -fsSL https://raw.githubusercontent.com/MakFly/ghostchrome/main/install.sh | sh`.

## When to use which surface

| Need | Use |
|---|---|
| Multi-step flow driven from the shell (one Bash call per action) | Per-call CLI + `-s <name>` (state persists across calls) |
| Scrape a known site (autoscout24, leboncoin, linkedin) | The recipe — one shell call, structured JSON out |
| Tight pipelined flow, one subprocess, no per-call spawn | The `agent` JSONL loop |
| Need to keep one Chrome alive across many actions | `-s <name>` (CLI) or the `agent` loop |
| Just check a page exists / read static HTML | NOT ghostchrome — use `WebFetch` |

## CLI with a managed session (`-s`, simplest for agents)

`-s <name>` auto-launches a persistent Chrome on the first call and **reuses
it — same tab, same cookies — on every later call**, with no `ws://` URL to
track. This lets you drive a browser with ordinary one-shot Bash commands and
keep state between them. `@ref` ids from an `extract`/`preview` survive into
the next command.

```bash
ghostchrome -s work goto https://example.com/login   # spawns Chrome on first use
ghostchrome -s work preview                          # status + errors + DOM with @refs
ghostchrome -s work type  @1 "alice@example.com" --submit
ghostchrome -s work click @3
ghostchrome -s work extract --level content          # compact a11y tree

ghostchrome sessions list            # name, port, pid, alive
ghostchrome sessions stop work       # tear the session down (or: sessions kill-all)
```

`$GHOSTCHROME_SESSION=work` sets the default so you can drop the flag. Add
`--stealth` on the **first** call only (it's baked into the spawned Chrome);
add `--timeout 60` for slow sites. Per-call latency after the first spawn is
~50 ms.

**Clean up when done** — a session is a persistent background Chrome plus an
on-disk profile, so tear it down so neither piles up:

```bash
ghostchrome sessions kill-all --purge   # stop every session AND delete its profile
ghostchrome sessions prune              # drop dead sessions from the registry
ghostchrome profiles list               # see profiles and their disk size
ghostchrome profiles rm <name>          # reclaim a profile's disk
```

Keep `--purge` for throwaway sessions; omit it when you want cookies to persist
for a later `-s <name>` call.

## Recipe surface (one-shot, structured JSON)

```bash
# autoscout24 — listings (JSONL, one car per line)
ghostchrome --stealth --dismiss-cookies autoscout24 search \
  --make renault --model clio --price-max 10000 --pages 3 \
  --output clios.jsonl

# autoscout24 — single listing detail (JSON)
ghostchrome --stealth --dismiss-cookies autoscout24 detail <url-or-slug> \
  --output detail.json

# Other recipes follow the same shape
ghostchrome --user-profile leboncoin leboncoin search --keywords "MacBook" --pages 2
ghostchrome --user-profile linkedin  linkedin people --keywords "DevOps" --country FR
```

Read the JSONL output line-by-line — each line is a fully-typed record.
Recipes already handle stealth, cookie banners, pagination, and dedup.

## Agent surface (JSONL loop, persistent browser)

Spawn `ghostchrome agent` as a subprocess. Send one JSON request per line on
stdin, read one JSON response per line on stdout. The browser stays alive
across requests — refs from a prior `extract` are valid for the next
`click`/`type`.

### Request / response shape

```jsonc
// Request
{"id":"r1","op":"navigate","args":{"url":"https://example.com"}}
// Response
{"id":"r1","ok":true,"result":{"url":"...","title":"...","status":200}}
```

### Available ops

| op | args | result |
|---|---|---|
| `init` | — | open browser eagerly |
| `navigate` | `{url, wait?}` (`load`/`stable`/`networkidle`) | `{url,title,status,time_ms}` |
| `back` / `forward` / `reload` | — | `{url,title}` |
| `extract` | `{level?, selector?}` (`skeleton`/`content`/`full`) | a11y tree + `refs: {@1: {...}}` |
| `click` / `dblclick` | `{ref}` | — |
| `check` / `uncheck` | `{ref}` (idempotent checkbox/radio) | — |
| `type` | `{ref, text, submit?}` (`submit` presses Enter) | — |
| `press` | `{key, ref?}` | — |
| `hover` | `{ref}` | — |
| `select` | `{ref, values[]}` | — |
| `fill` | `{fields: {ref: value}}` | `{filled: N}` |
| `scroll_by` | `{dy}` | `{y}` |
| `scroll_to` | `{y?, bottom?}` | `{y}` |
| `eval` | `{expr, ref?}` | `{value}` |
| `screenshot` | `{full_page?, ref?, quality?}` | `{mime, base64}` |
| `wait` | `{selector?, ms?}` | — |
| `errors` | — | `[{type,text,...}]` |
| `url` | — | `{url, title}` |
| `close` | — | — |

`@ref` ids come from a prior `extract`. If a ref goes stale (DOM changed),
the next op auto-resnaps and retries once — you don't have to handle it.

### Persistent flags

Set them once on the `agent` invocation; they apply to every op in the loop:

- `--stealth` — hide headless fingerprints (use for any anti-bot site)
- `--dismiss-cookies` — auto-dismiss cookie banners after navigation
- `-s, --session <name>` — managed persistent session (spawn once, reuse across calls); also a global flag on every CLI command
- `--user-profile <name>` — persistent Chrome profile under `~/.ghostchrome/profiles/<name>`
- `--connect=auto` — attach to a running Chrome on `:9222-9229` instead of spawning
- `--connect ws://...` — attach to a specific Chrome (e.g. one launched by `ghostchrome serve`)
- `--human` — humanized input dynamics (Bezier mouse paths, jittered typing)

### Example: search a site, click first result, extract its title

```bash
printf '%s\n' \
  '{"id":"r1","op":"navigate","args":{"url":"https://duckduckgo.com/?q=anthropic"}}' \
  '{"id":"r2","op":"extract","args":{"level":"skeleton"}}' \
  '{"id":"r3","op":"click","args":{"ref":"@1"}}' \
  '{"id":"r4","op":"wait","args":{"ms":1500}}' \
  '{"id":"r5","op":"eval","args":{"expr":"document.title"}}' \
  '{"id":"r6","op":"close"}' \
| ghostchrome --stealth agent
```

### Driving from code

When you write a script that drives the loop, **always read responses
line-by-line and match by `id`** — responses can interleave if you pipeline.
Keep stdin open until you've emitted `close`, otherwise the browser exits
mid-flow.

## Choosing flags for tough sites (DataDome, Cloudflare, PerimeterX)

Default to: `--stealth --dismiss-cookies --human`. If the site still blocks:

1. Add `--user-profile <name>` and run `ghostchrome --user-profile <name> login <url>` once interactively to seed cookies.
2. If the user has Chrome already running with `--remote-debugging-port=9222`, prefer `--connect=auto` — work in a real user session, anti-bot rarely bites.
3. Add `--wait-ms 3000` on navigate to let DataDome's challenge resolve.

## Common mistakes

- **Don't call `ghostchrome` per action *without* `-s`/`--connect`** — a bare call cold-spawns Chrome each time (~4 s) and leaves nothing to reuse. With `-s <name>` (or the `agent` loop) per-call is fine and fast (~50 ms) — the browser is reused.
- **Don't leave sessions running** — a forgotten `-s` session is a background Chrome + a growing profile. End with `sessions kill-all --purge` (or `--purge` on `stop`).
- **Don't parse the a11y tree as text when a recipe exists**. If the site has a recipe, use it.
- **Don't use `eval` to scrape big data sets** — use `extract` with `selector` if you need a subtree, or write a recipe (see `packages/autoscout24/` for a template; the cleanest pattern is reading `window.__NEXT_DATA__` from Next.js sites).
- **Don't ignore stderr** — recipes log progress and warnings there; only stdout is the structured payload.

## MCP server

`ghostchrome mcp` runs a Model Context Protocol server over stdio (16 tools, a
drop-in alternative to `@playwright/mcp`). Register it with Claude Code:

```bash
claude mcp add ghostchrome -- ghostchrome mcp            # spawns its own Chrome
claude mcp add ghostchrome -- ghostchrome mcp --connect=auto   # attach to a running one
```

## Where to read more

- `docs/cli.md` — full CLI reference (every command + global flags).
- `cmd/agent.go` — the JSONL dispatcher and the full op list.
- `internal/ops/ops.go` + `contracts/commands.json` — canonical op catalog.
- `packages/autoscout24/autoscout24.go` — template for adding a new site recipe.
