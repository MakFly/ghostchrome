# Playwright CLI Parity

Status: 12-08-2026. Rebuilt from scratch — every row below was re-measured
against the current binary (`./ghostchrome <cmd> --help` resolution + live
behavior on iautos.fr/chronovet.fr where relevant) and cross-checked against the
official Playwright CLI docs. Nothing here is inherited from the previous doc.

Target: the **Playwright Agent CLI** (`@playwright/cli`, repo
`github.com/microsoft/playwright-cli`) — the token-efficient CLI for coding
agents, **not** the archived 2021 `playwright-cli` tool.

## Authoritative Source Set (fetched 06-07-2026)

- `bunx @playwright/cli@0.1.18 --help` (86 public commands)
- `bunx @playwright/cli@0.1.18 find --help`
- Introduction / all-commands: https://playwright.dev/agent-cli/introduction
- Network & routing: https://playwright.dev/agent-cli/commands/network-routing

The official public surface is **86 commands**. `config-print` and `tray` are
implemented by 0.1.18 but omitted from its top-level `--help`, so they are
tracked separately rather than inflating this baseline.

## Method & Compatibility Rules

- `compatible`: the documented name resolves AND maps to real ghostchrome
  behavior with the same intent (verified live).
- `partial`: name resolves, but Playwright CLI documents output/semantics
  ghostchrome does not yet match exactly.
- `gap`: name resolves as an explicit boundary but returns structured
  `unsupported` (needs a Playwright runtime, not CDP/Rod), or is not offered.

## Validation Gate

```bash
GOCACHE=/tmp/go-build go test ./cmd \
  -run 'TestPlaywrightCompatCommandsRegistered|TestPlaywrightCompatFlagsRegistered' -count=1
```

Passing proves every documented name is exposed. It does NOT upgrade a
`partial`/`gap` row — those require behavior-specific evidence.

## Current Public Name-Resolution Baseline (0.1.18)

| Check | Result |
|---|---:|
| Public commands in `@playwright/cli` 0.1.18 `--help` | 86 |
| Names registered by ghostchrome | 86/86 |
| Newly reconciled names | `drop`, `find` |

The registration test holds this exact 86-command list. It excludes hidden
Playwright implementation commands (`config-print`, `tray`) and ghostchrome
extras, so future CLI changes cannot be hidden by a wider local command set.

## Reconciled Behavioral Rows

| Command | Delta vs Playwright CLI |
|---|---|
| `drop` | Real `DataTransfer` drop on any strict target; repeated files and `MIME=VALUE` data are supported. It is no longer an alias for file-input upload. |
| Interaction targets | `@N`/`eN`, unique CSS, `getByRole`, `getByText`, and `getByLabel` share one strict resolver. Full Playwright locator chaining/filter syntax remains out of scope. |
| `open` | `--mobile` and named `--device` presets apply and persist emulation. Chrome/Chromium only; Firefox/WebKit/Edge remain explicit gaps. |
| `snapshot` | Emits compact Playwright-style `eN` refs; `--boxes` adds viewport-relative CSS-pixel boxes. It is intentionally not byte-identical Playwright YAML. |
| `screenshot` | `--hires` is accepted and preserves native device pixels. Native DPR was verified at 1170x2532 for a 390x844@3x profile. |
| `network` / `console` | A daemon-owned bounded NDJSON ring now captures traffic and console events between short CLI processes. Typed clear markers clear one stream without deleting the other. |
| `highlight` | Persistent pointer-transparent overlays support strict targets, `--style`, targeted `--hide`, and hide-all. |
| `show` | `--annotate` adds rectangle+note capture and writes an owner-only JSON/JPEG artifact set. The dashboard remains foreground-owned; there is no unsafe cross-process `show --kill`. |
| `video-start` / `video-stop` | A detached, daemon-attached runtime records across separate CLI processes and persists heartbeat/status. The artifact is truthfully reported as a JPEG frame sequence plus manifest; no WebM is claimed. Stale or failed runtimes recover on the next start. Action overlays remain explicit `unsupported`. |
| Output safety | `--json`, `--raw`, `--output-max-size`, `outputMaxSize`, `PLAYWRIGHT_MCP_OUTPUT_MAX_SIZE`, config/env secrets, redaction, and owner-only overflow artifacts are supported. |
| `tracing-start` / `tracing-stop` | Explicit structured `unsupported`: short-lived CDP clients cannot own a cross-command trace, and a renamed CDP JSON zip is not a Playwright Trace Viewer artifact. |
| `config-print` | Prints resolved ghostchrome config + unsupported Playwright fields; not a full Playwright schema resolver. |

## Gap Rows — Structural (need the Playwright runtime, not CDP/Rod)

| Command | Why | Behavior |
|---|---|---|
| `run-code` | Executes the Playwright `page`/`context` API | resolves → `unsupported` |
| `pause-at` / `resume` / `step-over` | Playwright test debug protocol | resolves → `unsupported` (verified: `pause-at` returns a structured reason) |
| `tracing-start` / `tracing-stop` | Playwright trace schema plus one long-lived recorder client | resolves → `unsupported`; no false `trace.zip` claim |

## `find`

`find [text]` is compatible. It searches the current Playwright-style snapshot
with a case-insensitive substring match; `find --regex <pattern>` uses a Go
regular expression. Both forms return each matching line plus three snapshot
lines of surrounding context. The command first reuses the persisted extraction
for the unchanged page, and only re-extracts when that cache is absent or stale.

## Flag / Attach-Level Rows

| Item | Status | Notes |
|---|---:|---|
| `-s=<name>` sessions | compatible | Global `--session`/`-s`; `PLAYWRIGHT_CLI_SESSION` maps to it, `GHOSTCHROME_SESSION` as fallback. |
| Implicit persistent daemon | compatible | Auto-spawns one background Chrome (session `default`) on first use and **reuses it** across commands — matching Playwright CLI's persistent-server model. (The recursive-serve fork bomb that broke this was fixed on 06-07-2026, commit `677795a`.) |
| `install --skills` | compatible | Installs the bundled agent skill. |
| `attach --cdp=<url\|channel>` | compatible | ws/http endpoints + local `chrome*`/`msedge*` channel discovery. |
| `attach --extension` / `--endpoint` | gap | Registered boundary → `unsupported` (needs Playwright extension/server bridge). |
| `open --browser=firefox/webkit/msedge` | gap | Registered → rejected; ghostchrome is CDP-native (Chromium only). |

## ghostchrome Extras (superset — NOT Playwright CLI parity)

These commands exist in ghostchrome but are **not** part of the Playwright CLI
surface. They must never be counted as parity; they are a bonus over it.

- Native observer/capture commands remain a superset of the Playwright names.
- `detach` and `install-browser` expose ghostchrome-specific lifecycle boundaries.
- The broader ghostchrome-native surface: `preview`, `extract`, `errors`,
  `perf`, `capture`, `intercept`, `doctor`, `dashboard`, `mcp`, `agent`, `ai`,
  cookie-banner auto-dismiss, stealth + native DataDome recovery, site recipes.

## Non-Goals (intentional, CDP/Rod-native tool)

1. `run-code` Playwright runtime execution.
2. `pause-at` / `resume` / `step-over` test-debug protocol.
3. Firefox / WebKit / Edge launch (requires the Playwright server binary).
4. Trace-Viewer-compatible `trace.zip` until a Playwright trace-schema writer and daemon-owned lifecycle exist.
5. Playwright `--endpoint` / `--extension` attach bridges.

## Next Parity Work

1. Decide whether `snapshot` must be byte-identical Playwright YAML or whether
   the compact form remains the intended compatibility layer.
2. Extend the strict locator subset only from measured, high-frequency failures;
   do not import the Playwright runtime grammar wholesale.
3. Wire the CLI-only network/visual commands into `internal/ops/` + the SDKs
   (today they are CLI-only, not in the catalog/contract/parity test).
