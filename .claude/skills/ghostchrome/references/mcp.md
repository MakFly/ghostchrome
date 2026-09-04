# MCP reference

Read this reference only after the installation manifest selects `mcp`. Use the
registered Ghostchrome MCP connection; do not invoke the CLI or spawn a second
MCP process from a shell. Setup renders the client configuration with the stable
installed executable, normally `~/.ghostchrome/bin/ghostchrome-mcp`.

## Tool flow

Start with `snapshot` when entering or revisiting a page. It returns page
metadata, console/network observations, and a compact accessibility tree with
refs. Use a fresh snapshot after navigation, a route transition, a modal, or a
DOM-changing interaction.

The 17-tool surface is:

```text
snapshot, navigate, click, type, select, press, hover, drag,
fill_form, upload, tabs, dialog, wait_for, eval, screenshot, back, forward
```

Use refs from the current snapshot for interaction. Validate each mutating tool
call through a URL/title change, a control state, visible text, or a new
snapshot. Use `wait_for` with a selector or state condition for dynamic pages;
prefer a bounded condition over arbitrary sleeps. Use `screenshot` for visual
evidence, not as a replacement for semantic extraction.

There are no recipes, CLI session commands, `perf`, `capture`, or JSONL batch
operations on this MCP surface. Request an explicit CLI-mode setup switch when
one of those capabilities is required; never call an unregistered checkout as a
fallback.

## Configuration shape

The setup command writes the equivalent of this configuration for MCP-capable
clients. Replace the placeholder with the absolute path emitted by
`ghostchrome setup status`; never point at a repository build.

```toml
[mcp_servers.ghostchrome]
command = "/home/user/.ghostchrome/bin/ghostchrome-mcp"
args = ["--stealth"]
```

Claude Code and other clients may store the same command in a JSON or their own
configuration format. Let setup render those formats and preserve unmanaged
fields. A mode switch removes only the Ghostchrome entry that setup owns.

## Runtime controls

Configure the standalone server through environment variables when setup or the
client supports them:

| Variable | Purpose |
| --- | --- |
| `GHOSTCHROME_CONNECT` | Attach to a local or explicit CDP endpoint instead of spawning Chrome. |
| `GHOSTCHROME_HEADLESS` | Select headless or visible Chrome. |
| `GHOSTCHROME_STEALTH` | Enable anti-fingerprint patches when policy permits. |
| `GHOSTCHROME_PROFILE` | Select persistent profile storage. |
| `GHOSTCHROME_TIMEOUT` | Bound individual tool operations. |
| `GHOSTCHROME_MCP_LAZY` | Defer browser launch until the first tool call. |
| `GHOSTCHROME_IDLE_TIMEOUT` | Release idle Chrome while retaining the stdio server. |

The default idle timeout is 15 minutes. The MCP process remains available while
the browser is released; the next browser tool call recreates the context. This
keeps idle RSS bounded without requiring a client re-registration.

## Lifecycle and protocol hygiene

Keep JSON-RPC protocol traffic on stdin/stdout. Treat stderr as diagnostics and
never parse it as tool output. Allow the client to close stdin when the flow is
complete. On graceful shutdown, the server closes its page, browser, and profile
handles. If a server appears stuck, inspect `ghostchrome setup doctor
--strict` and the client process tree before stopping it; do not kill unrelated
Chrome instances.

Each MCP client should hold one Ghostchrome connection. Duplicate registrations
can create one browser per server and inflate memory. Remove stale managed
entries through `ghostchrome setup switch --to mcp --yes`, not by deleting
arbitrary client configuration sections.

## MCP failures

For a missing tool connection, report the client and mode mismatch and run the
setup doctor. For a tool timeout, retain the last snapshot, check `errors`, and
retry only an idempotent operation once when the page is still reachable. Avoid
repeating form submissions, uploads, purchases, or destructive actions without
fresh confirmation. For a browser launch failure, report the missing executable
or native dependency and stop; do not silently switch transports.
