# Changelog

All notable changes to ghostchrome are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/); the project follows SemVer.

## [0.1.0] — 2026-06-11

First release of the project. Single static Go binary that drives Chrome over
CDP for LLM agents.

### Added
- **Managed sessions (`-s` / `--session`)** — playwright-cli `-s` parity. A named
  session auto-spawns a persistent Chrome on first use (bound to a disk profile of
  the same name, cookies persist), reuses it — including the active tab — across
  calls, and needs no `ws://` URL. `$GHOSTCHROME_SESSION` sets the default; manage
  with `ghostchrome sessions list | stop <name> | kill-all`. `goto` added as an
  alias of `navigate`.
- **bun/npm install** — `bun install -g @ghostchrome/cli` (or npm). A meta package
  resolves the prebuilt Go binary for the host platform via os/cpu-gated
  optionalDependencies (no postinstall, bun-compatible). Published by CI on tag.
- **playwright-cli parity verbs**: `reload`, `dblclick @ref`, `check`/`uncheck @ref`
  (idempotent checkbox/radio), `tabs new [url]`, and `type --submit` (press Enter
  after typing). Wired across the CLI, the JSONL `agent` op surface, and both SDKs.
- Install docs aligned with `@playwright/cli`: one-line install + one-line
  `claude mcp add` agent wiring.

### Fixed
- engine: release `ProviderFunc` cleanup in `Browser.Close` (and on connect
  failure) — provider-provisioned Chrome was leaked.
- engine: write session state atomically (temp file + rename) to survive a
  crash mid-write.
- engine: strip credentials from the Chrome `--proxy-server` flag (auth via CDP).
- engine/mcp: `wait_for` is now cancellable via context instead of blocking
  sleeps that ignored client disconnects.
- cmd/agent: surface `extract` arg-parse errors; log stdout write failures;
  fill form fields in deterministic `@ref` order (was Go-map-random).
- ops: catalog the 5 MCP tools (`drag`, `fill_form`, `hover`, `tabs`, `upload`)
  that had drifted out of the parity guard, so `contracts/commands.json` matches
  the 16 tools the MCP server actually registers.
- sdk(ts): escalate `SIGTERM` → `SIGKILL` on dispose; default per-op timeout 30s.
- sdk(py): read agent stdout line-buffered instead of byte-by-byte.

### Core
- Single static Go binary that drives Chrome over CDP (via Rod) and returns
  compact, token-efficient output for LLM agents.
- Filtered accessibility extraction with stable `@ref` ids; three levels
  (`skeleton`, `content`, `full`).
- `preview` page-health report, `extract`, `errors`, `eval`, `screenshot`,
  interaction ops (`click`/`type`/`select`/`hover`/`press`), `serve`, and
  `--connect=auto` zero-spawn attach.
- Stealth patches, cookie-banner auto-dismiss, persistent profiles, encrypted
  state vault, domain policy, and a live monitoring dashboard.

### Agent surfaces
- **MCP server** (`ghostchrome mcp`) — 16 tools, a drop-in replacement for
  `@playwright/mcp`; standalone `ghostchrome-mcp` binary also shipped.
- **JSONL `agent` loop** — persistent stdin/stdout op protocol.
- **Canonical ops catalog** (`internal/ops`) as the single source of truth,
  generating `contracts/commands.json`, guarded by a parity test.

### SDKs (in-repo)
- **Python** (`sdk/python`) and **TypeScript** (`sdk/typescript`) typed clients
  that drive the persistent JSONL agent loop. Result types are measured against
  the live binary (`scripts/measure-agent-ops.sh`), not guessed.

### Other
- Opt-in content-boundary fencing for untrusted page text (prompt-injection
  defense) on the `extract` CLI.
- Site scrapers live out of the committed tree and compile in only via
  `go build -tags recipes .`.
- Runnable end-to-end examples under `examples/`.

[0.1.0]: https://github.com/MakFly/ghostchrome/releases/tag/v0.1.0
