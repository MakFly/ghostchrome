# Playwright CLI Parity

Status: 2026-06-14.

This page tracks ghostchrome against the public Playwright CLI documentation.
It is intentionally evidence-based: a row is marked compatible only when
ghostchrome exposes the command name or a documented equivalent in the current
tree. Anything else stays a gap.

## Official Source Set

- Introduction: https://playwright.dev/agent-cli/introduction
- Capabilities: https://playwright.dev/agent-cli/capabilities
- Snapshots: https://playwright.dev/agent-cli/snapshots
- Navigation: https://playwright.dev/agent-cli/commands/navigation
- Interaction: https://playwright.dev/agent-cli/commands/interaction
- Keyboard and mouse: https://playwright.dev/agent-cli/commands/keyboard-mouse
- Tabs: https://playwright.dev/agent-cli/commands/tabs
- Dialogs: https://playwright.dev/agent-cli/commands/dialogs
- Network and mocking: https://playwright.dev/agent-cli/commands/network-routing
- Storage and authentication: https://playwright.dev/agent-cli/commands/storage
- Console and eval: https://playwright.dev/agent-cli/commands/console-eval
- Screenshots and PDF: https://playwright.dev/agent-cli/commands/screenshots-pdf
- Tracing: https://playwright.dev/agent-cli/commands/tracing
- Video recording: https://playwright.dev/agent-cli/commands/video-recording
- Sessions and dashboard: https://playwright.dev/agent-cli/sessions
- Attach: https://playwright.dev/agent-cli/commands/attach
- Configuration: https://playwright.dev/agent-cli/configuration

## Compatibility Rules

- `compatible`: command name exists and maps to an existing ghostchrome
  behavior with the same basic intent.
- `partial`: command exists, but Playwright CLI documents options or output
  semantics ghostchrome does not yet match.
- `gap`: no command or no safe equivalent yet.

## Validation Gate

Before marking a row `compatible`, verify the current command surface with:

```bash
GOCACHE=/tmp/go-build go test ./cmd \
  -run 'TestPlaywrightCompatCommandsRegistered|TestPlaywrightCompatFlagsRegistered'
```

The registration test is sourced from the official `Introduction` command list,
plus `Capabilities` for commands such as `network-state-set` and the testing
helpers. Passing that test proves the documented name is exposed; it does not
upgrade `partial` or `gap` rows without behavior-specific evidence.

## Core Commands

| Playwright CLI | ghostchrome status | Evidence / notes |
|---|---:|---|
| `open [url]` | partial | `ghostchrome open [url]` opens a browser, optionally navigates, and text output now follows Playwright CLI's `### Page` + `### Snapshot` shape with a `.playwright-cli/page-*.yml` artifact. It accepts Playwright CLI-style `--headed`, `--browser`, `--persistent`, `--profile`, and inherited `--config`; only Chrome/Chromium launch is supported today. |
| `goto <url>` | compatible | Alias for `ghostchrome navigate <url>`; when called as `goto`, text output now follows Playwright CLI's `### Page` + `### Snapshot` shape and writes a `.playwright-cli/page-*.yml` snapshot artifact. |
| `close` | compatible | `ghostchrome close` closes the active tab in the implicit daemon session (auto-spawned by default). `ghostchrome close <name>` stops a named session. |
| `go-back` | compatible | Alias for `back`; text output now follows Playwright CLI's `### Page` + `### Snapshot` shape with a `.playwright-cli/page-*.yml` artifact after navigation. |
| `go-forward` | compatible | Alias for `forward`; text output now follows Playwright CLI's `### Page` + `### Snapshot` shape with a `.playwright-cli/page-*.yml` artifact after navigation. |
| `reload` | compatible | Existing top-level command; text output now follows Playwright CLI's `### Page` + `### Snapshot` shape. |
| `click` | compatible | Existing top-level command with Playwright-compatible optional button support (`left|right|middle`) via a second positional argument; text output now follows Playwright CLI's `### Page` + `### Snapshot` shape. |
| `dblclick` | compatible | Existing top-level command with optional Playwright-compatible second positional argument for `left|right|middle` button; text output now follows Playwright CLI's `### Page` + `### Snapshot` shape. |
| `hover` | compatible | Existing top-level command; text output now follows Playwright CLI's `### Page` + `### Snapshot` shape. |
| `drag` | compatible | Existing top-level command; text output now follows Playwright CLI's `### Page` + `### Snapshot` shape. |
| `type <text>` | compatible | `type <text>` writes into the currently focused element; ref/locator forms still clear and fill a target element. Text output now follows Playwright CLI's `### Page` + `### Snapshot` shape. |
| `fill <ref> <text>` | compatible | Alias for `type`, which clears and fills an input by ref/locator and returns the same Playwright-like page-state output. |
| `select` | compatible | Existing top-level command; text output now follows Playwright CLI's `### Page` + `### Snapshot` shape. |
| `check` / `uncheck` | compatible | Existing top-level commands; text output now follows Playwright CLI's `### Page` + `### Snapshot` shape. |
| `press` | compatible | Existing top-level command; text output now follows Playwright CLI's `### Page` + `### Snapshot` shape. |
| `keydown` / `keyup` | compatible | Added top-level compatibility commands. |
| `snapshot` | partial | Alias for `extract`; supports `--filename`, `--depth`, `--raw`, CSS selector/ref scoping, Playwright-style `eN` refs, and YAML-like tree output. All page-modifying commands (navigation, interaction, viewport, tabs, dialogs) now write `.playwright-cli/page-*.yml` snapshot artifacts in text mode. Low-level mouse primitives are excluded by design. |
| `screenshot` | compatible | Existing command with `--filename`, `--full-page`, and positional `@ref` support. |
| `upload` | compatible | Existing top-level command; text output follows Playwright CLI's `### Page` + `### Snapshot` shape. |
| `dialog-accept` / `dialog-dismiss` | compatible | Added top-level aliases over the existing dialog handler; text output now follows Playwright CLI's `### Page` + `### Snapshot` shape with a post-dialog snapshot. |
| `resize` | compatible | Alias for `viewport`; text output now follows Playwright CLI's `### Page` + `### Snapshot` shape. Always snapshots after viewport change, even without a URL argument. |
| `eval` | compatible | Existing command; supports both `--on @ref` and Playwright CLI-style positional `@ref`. |
| `run-code` | gap | Command and `--filename` are registered as an explicit compatibility boundary, but they return an unsupported error because full Playwright `page/context` API execution requires a Playwright runtime. |

## Network

| Playwright CLI | ghostchrome status | Evidence / notes |
|---|---:|---|
| `network` | partial | Added top-level observer with `--filter`, `--static`, `--request-body`, `--request-headers`, `--clear`, and a bounded persistent session buffer. It still only appends events captured while `network` runs, not all traffic from every command. |
| `route` | compatible | Added persistent top-level route command with `--status`, `--body`, `--content-type`, `--header`, and `--remove-header`. Header removal continues the matched request instead of fulfilling it. |
| `route-list` | compatible | Added top-level route registry listing. |
| `unroute` | compatible | Added top-level route removal by pattern/id, or all routes when no argument is given. |
| `network-state-set` | compatible | Added `network-state-set online` and `network-state-set offline` via CDP network condition emulation. |

## Storage

| Playwright CLI | ghostchrome status | Evidence / notes |
|---|---:|---|
| `state-save [filename]` | compatible | Added top-level alias over `storage save`; optional positional filename is supported. |
| `state-load <filename>` | compatible | Added top-level alias over `storage load`. |
| `cookie-list` | compatible | Added top-level command with `--domain` and `--path`. |
| `cookie-get` | compatible | Added top-level command. |
| `cookie-set <name> <value>` | compatible | Added Playwright CLI-compatible argument form. |
| `cookie-delete` | compatible | Added top-level alias. |
| `cookie-clear` | compatible | Added top-level alias. |
| `localstorage-list/get/set/delete/clear` | compatible | Added top-level commands for current-page `localStorage`. |
| `sessionstorage-list/get/set/delete/clear` | compatible | Added top-level commands for current-page `sessionStorage`. |

## Vision, Tabs, And DevTools

| Playwright CLI | ghostchrome status | Evidence / notes |
|---|---:|---|
| `mousemove` | compatible | Added top-level alias over raw mouse move. Low-level primitive: no page-state snapshot (used in sequences where per-op snapshots would be prohibitively expensive). |
| `mousedown` / `mouseup` | compatible | Added top-level aliases with optional button argument. Low-level primitives: no page-state snapshot. |
| `mousewheel` | compatible | Added top-level command using Playwright CLI positional `dx dy`. Low-level primitive: no page-state snapshot. |
| `tab-list` | compatible | Added top-level alias over `tabs`. |
| `tab-new` | compatible | Added top-level alias over `tabs new`; text output now follows Playwright CLI's `### Page` + `### Snapshot` shape. |
| `tab-select` | compatible | Added top-level alias over `tabs switch`; text output now follows Playwright CLI's `### Page` + `### Snapshot` shape. |
| `tab-close [index]` | compatible | Added top-level command; no argument closes the active tab. |
| `console` | partial | Added top-level observer with `console error`, `console warning`, `console debug`, `--level`, `--wait`, `--clear`, and a bounded persistent session buffer. It still only appends events captured while `console` runs, not all console output from every command. |
| `tracing-start` / `tracing-stop` | partial | Added CDP browser tracing for persistent sessions. Default output is now `.playwright-cli/trace.zip`, but it is a ghostchrome CDP trace bundle marked `playwright_compatible=false`, not a Playwright Trace Viewer-compatible trace yet. Explicit `--output trace.json` still writes raw CDP JSON. |
| `video-start` / `video-stop` / `video-chapter` | partial | Added persistent session video metadata and chapter manifest commands with `--size`, `--description`, and `--duration`. `saveVideo` config and `PLAYWRIGHT_MCP_SAVE_VIDEO=WxH` now auto-start video metadata for persistent sessions. WebM frame recording is not implemented yet. |
| `show` | compatible | Alias for `dashboard`. |
| `pause-at` / `resume` / `step-over` | gap | Command names are registered as explicit compatibility boundaries and return structured unsupported output. Real support requires the Playwright test debugging protocol, not CDP/Rod alone. |
| `pdf` | compatible | Existing command with `--filename`; URL is optional and omitted calls print the current page. |

## Sessions And Attach

| Playwright CLI | ghostchrome status | Evidence / notes |
|---|---:|---|
| `-s=<name>` sessions | compatible | Existing global `--session` / `-s` starts and reuses named Chrome sessions. `PLAYWRIGHT_CLI_SESSION` now maps to the same default session name, with `GHOSTCHROME_SESSION` kept as a fallback. |
| `list` | compatible | Added top-level alias over `sessions list`. |
| `close` | compatible | Closes the active tab in the implicit daemon session, or stops a named session by name. |
| `close-all` | compatible | Added top-level alias over the session stop-all implementation. |
| `kill-all` | compatible | Added top-level alias over the session stop-all implementation. |
| `delete-data` | compatible | Added top-level command that stops the named session if present and deletes its profile data. |
| `attach --cdp=<url>` | compatible | Added `attach --cdp=http(s)://...` and `attach --cdp=ws(s)://...`; it registers a named session, using `default` when `-s` is omitted. |
| `attach --cdp=<channel>` | partial | Added local CDP channel discovery for `chrome*` and `msedge*` names by scanning known remote-debugging ports and matching Chrome/Edge product data. Exact beta/dev/canary distinction depends on what `/json/version` exposes. |
| `attach <session-name>` | partial | Added positional attach for an existing live ghostchrome session. Playwright `--debug=cli` sessions are not supported unless they expose a CDP endpoint. |
| `attach --endpoint=<url>` | gap | Registered as an explicit compatibility boundary with structured unsupported output. Real support requires the Playwright protocol, not CDP/Rod alone. |
| `attach --extension[=<channel>]` | gap | Registered as an explicit compatibility boundary with structured unsupported output. Real support requires the Playwright browser extension bridge. |

## Configuration

| Playwright CLI | ghostchrome status | Evidence / notes |
|---|---:|---|
| `open --headed` | compatible | Added global `--headed` inverse for ghostchrome's existing `--headless` flag. |
| `open --browser=chrome/chromium` | compatible | Added `open --browser`; Chrome/Chromium map to ghostchrome's CDP launcher. |
| `open --browser=firefox/webkit/msedge` | gap | Registered but rejected explicitly. ghostchrome does not launch Firefox/WebKit/Edge through Playwright today. |
| `open --persistent` | compatible | Added `open --persistent`; it maps to ghostchrome profile `default` when no profile is already selected. |
| `open --profile=<path>` | compatible | Added an `open`-local `--profile` flag for a browser user data directory, separate from ghostchrome's global render-profile flag on other commands. |
| `--config <file>` / auto `.playwright/cli.config.json` | partial | Added JSON config loading for mappable fields: `browser.isolated=true`, CDP endpoint, CDP headers/timeout, headless, executable path, Chromium launch args, proxy server/bypass/credentials, user data dir, navigation timeout, `outputDir` for compatible artifacts, `console.level`, `saveVideo`, `browser.initScript`, `browser.contextOptions.viewport`, `browser.contextOptions.userAgent`, `browser.contextOptions.locale`, `browser.contextOptions.permissions`, `browser.contextOptions.serviceWorkers`, and `browser.contextOptions.storageState`. `browser.initPage` and other unsupported schema fields are reported by `config-print`. |
| `PLAYWRIGHT_CLI_SESSION`, `PLAYWRIGHT_MCP_CONFIG`, `PLAYWRIGHT_MCP_HEADLESS`, `PLAYWRIGHT_MCP_ISOLATED=true`, `PLAYWRIGHT_MCP_CDP_ENDPOINT`, `PLAYWRIGHT_MCP_USER_DATA_DIR`, `PLAYWRIGHT_MCP_EXECUTABLE_PATH`, `PLAYWRIGHT_MCP_DEVICE`, `PLAYWRIGHT_MCP_STORAGE_STATE`, `PLAYWRIGHT_MCP_VIEWPORT_SIZE`, `PLAYWRIGHT_MCP_USER_AGENT`, `PLAYWRIGHT_MCP_IGNORE_HTTPS_ERRORS`, `PLAYWRIGHT_MCP_TIMEOUT_NAVIGATION`, `PLAYWRIGHT_MCP_CONSOLE_LEVEL`, `PLAYWRIGHT_MCP_PROXY_SERVER`, `PLAYWRIGHT_MCP_PROXY_BYPASS`, `PLAYWRIGHT_MCP_OUTPUT_DIR`, `PLAYWRIGHT_MCP_NO_SANDBOX`, `PLAYWRIGHT_MCP_INIT_SCRIPT` | compatible | Env values map onto the same config/flag fields with CLI flags taking precedence. Unknown devices / unsupported browsers are reported instead of silently ignored. |
| `config-print` | partial | Added command that prints resolved ghostchrome config plus unsupported Playwright config fields. It is not a full Playwright schema resolver yet. |
| `install --skills` | compatible | Added top-level command that reuses `ghostchrome skills install`. |

## Testing

| Playwright CLI | ghostchrome status | Evidence / notes |
|---|---:|---|
| `verify-element-visible` | compatible | Added top-level assertion by role/name or shared `--by-*` semantic locator flags. The public Playwright page describes the behavior but does not publish a full signature, so ghostchrome documents its accepted forms. |
| `verify-text-visible` | compatible | Added top-level visible text assertion. Supports `--url` to navigate first, or current session/page by default. |
| `verify-list-visible` | compatible | Added top-level assertion that a visible `ul`, `ol`, or `[role=list]` contains all provided items. |
| `verify-value` | compatible | Added top-level form-field value assertion for @ref/eN, CSS selector, or `--by-*` locator. |
| `generate-locator` | compatible | Added top-level Playwright locator suggestion output. Uses `getByRole`, `getByText`, or `getByLabel` forms from refs/text/semantic flags. |

## Output Format Alignment Summary

All page-modifying commands now emit the Playwright CLI `### Page` + `### Snapshot`
text shape (with `.playwright-cli/page-*.yml` artifact) when `--format` is not
`json`. Commands excluded by design:

- **Mouse primitives** (`mousemove`, `mousedown`, `mouseup`, `mousewheel`):
  low-level input used in sequences; per-op snapshot is prohibitively expensive.
- **screenshot / pdf**: artifact-oriented output; Playwright CLI returns only
  `"Screenshot saved to ..."` / `"PDF saved to ..."`, no page-state.
- **eval**: returns raw JS result only, matching Playwright CLI.

JSON output (`--format json`) is unchanged for all commands.

## Structural Gaps (Not Closable Without External Dependencies)

These remain explicit non-goals or long-term targets:

1. **`run-code` Playwright runtime** — requires Playwright `page`/`context` API
   execution, not achievable with CDP/Rod alone.
2. **`pause-at` / `resume` / `step-over`** — requires Playwright test debugging
   protocol, not CDP/Rod.
3. **Cross-browser Firefox/WebKit/Edge** — ghostchrome is CDP-native; Playwright
   browser launch for non-Chromium requires the Playwright server binary.
4. **`trace.zip` Playwright Trace Viewer compatible** — current trace is a CDP
   bundle; converting to Playwright's trace format requires the Playwright trace
   event schema.
5. ~~**Daemon/session implicit Playwright CLI equivalence**~~ — **Closed.**
   ghostchrome now auto-spawns a persistent background Chrome on first use
   (session "default"), matching Playwright CLI behavior. Opt-out with
   `GHOSTCHROME_NO_DAEMON=1`.

## Next Parity Work

1. Decide whether snapshot files must exactly match Playwright's YAML format or
   whether ghostchrome's compact text/JSON snapshot file is the intended
   compatibility layer.
2. Decide intentionally whether to implement Playwright-only surfaces
   (`run-code`, Playwright test debugging, Playwright server endpoint attach)
   or document them as non-goals for a CDP/Rod-native tool.
