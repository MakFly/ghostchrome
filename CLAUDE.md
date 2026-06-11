# CLAUDE.md — ghostchrome

## Project overview

ghostchrome is an ultra-light CLI browser automation tool written in Go, designed for LLM agents. It uses Chrome DevTools Protocol (CDP) via Rod to control Chrome headless and returns compact output optimized for minimal token usage.

## Architecture

```
main.go → cmd/ (cobra commands) → engine/ (Rod/CDP logic) → Chrome
```

- `engine/browser.go` — Browser lifecycle (connect/launch/close)
- `engine/navigator.go` — Page navigation with wait strategies
- `engine/extractor.go` — CDP Accessibility tree → compact DOM with refs (@1, @2)
- `engine/interactor.go` — Click, type, hover, select, press, viewport, tabs, dialog
- `engine/errors.go` — Console + network error collection
- `engine/preview.go` — All-in-one page health report
- `engine/stealth.go` — Anti-detection patches
- `engine/cookies.go` — Cookie banner auto-dismiss
- `cmd/*.go` — One file per cobra command
- `internal/ops/` — Canonical op catalog (single source of truth); `go generate` emits `contracts/commands.json`; a parity test guards JSONL/MCP/AI surface drift
- `contracts/commands.json` — Generated op contract the SDKs are typed against
- `sdk/typescript/`, `sdk/python/` — In-repo typed SDKs; thin clients that spawn a persistent `ghostchrome agent` subprocess and speak the JSONL protocol over stdio
- `examples/` — Runnable end-to-end examples (TS + Python) attaching via `--connect=auto`
- `packages/<site>/` + `cmd/<site>.go` — Site scrapers, **gitignored** (kept on disk, never committed), compiled in only via `go build -tags recipes .`

## Build & test

```bash
go build -o ghostchrome .
go test ./engine/...
./ghostchrome preview https://example.com
```

## Key design decisions

- **CLI over MCP**: CLI is the 2026 trend for browser-LLM integration. Simpler, no JSON-RPC overhead.
- **Rod over chromedp**: Decode-on-demand, no zombie processes, native iframe support.
- **Filtered accessibility tree**: Only interactive elements get refs. 7-25x fewer tokens than full a11y tree.
- **Three extraction levels**: skeleton (minimal) / content (text) / full (everything named).
- **Auto-launch Chrome**: No need for `serve` — each command can launch a temporary Chrome. Use `--connect` for sessions.

## Runtime policy (preferred mode)

**Always run against an already-running Chrome — never spawn.** Default to `--connect=auto`
(zero-spawn attach, commit `83afd9a`) or an explicit `--connect=ws://...`. Cold spawn is a
fallback, not the happy path. Rationale:

- avoids Chrome startup cost per command (~hundreds of ms)
- preserves session state (cookies, storage, open tabs) across ops
- reduces fingerprint variance vs. fresh-profile spawns
- plays well with `serve` and persistent profiles under `~/.ghostchrome/profiles/<name>`

When designing new commands, flags, or SDK call paths, assume `--connect=auto` is the
default execution context. Spawn paths must remain a documented escape hatch only.

## Conventions

- Language: English for code, comments, and commits
- Commits: Conventional Commits (`feat:`, `fix:`, `chore:`, etc.)
- Package manager: Go modules only
- No external runtime dependencies (single static binary)

## Versioning

Follow SemVer (vMAJOR.MINOR.PATCH):
- MAJOR: Breaking CLI interface changes (renamed commands, changed output format)
- MINOR: New commands or flags (backward compatible)
- PATCH: Bug fixes, performance improvements

## SDK synchronization

The SDKs live **in this repo** under `sdk/typescript/` and `sdk/python/` (TypeScript
and Python only — no external `../ghostchrome-sdk` repo, no PHP). They are typed
against the generated contract and driven by the JSONL `agent` loop.

Status: both SDKs are at **v0.2.0**, cover **100% of the JSONL-surface ops**, are
publish-ready (TS `bun run build` → `dist/`; Python ships `py.typed`), and their
result types are matched to what the binary actually emits.

### Stay in the truth — ALWAYS re-measure, never guess

The op *names / args / surfaces* have a single source of truth: `internal/ops/`
(the canonical catalog, which generates `contracts/commands.json`). But the op
**result shapes** are owned by the running binary, not by any hand-written type.
Past SDK drift (e.g. `extract.stats` had `{total,interactive}` while the binary
emits `{total_nodes,filtered_nodes,interactive_count}`; `errors` returns a top-level
array, not `{errors:[]}`; empty ops OMIT `result`) all came from *guessing*.

Rule: before changing the SDK or the contract, **measure the live binary**:

```bash
# 1. start a Chrome the agent can attach to
google-chrome --headless=new --remote-debugging-port=9222 --user-data-dir=/tmp/gc-measure about:blank &
# 2. print the REAL result shape of every JSONL op
scripts/measure-agent-ops.sh
```

The shapes it prints are ground truth (read from the binary, cannot drift). Make
the SDK types match that output, not your assumptions.

### Change workflow

1. Edit `internal/ops/ops.go` (the catalog), then `go generate ./internal/ops/...`
   to regenerate `contracts/commands.json`.
2. **Re-measure** with `scripts/measure-agent-ops.sh` and reconcile result shapes.
3. Update the typed wrappers in `sdk/typescript/src/` and `sdk/python/ghostchrome/`
   plus their hermetic tests (`bun test`, `python -m unittest discover -s tests`).
   Each SDK has a contract-coverage test asserting every JSONL op has a method.
4. Update or add an `examples/` script if usage changed; re-run the e2e
   (`just e2e` against a live Chrome) so real shapes flow through both SDKs.
5. The parity test in `internal/ops/` fails loudly if JSONL / MCP / AI surfaces
   drift from the catalog — keep all three in sync.
6. Run `ig index .` after file changes.

Build/test everything via the root `justfile` (`build`, `test`, `test-all`,
`contract`, `e2e`) or directly: `go test ./...`, then the two SDK suites.
