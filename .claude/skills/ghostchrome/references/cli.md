# CLI and JSONL reference

Read this reference only after the installation manifest selects `cli`. The
installed command is the only browser transport in this mode:

```bash
ghostchrome setup status
ghostchrome -s work goto https://example.com --wait stable
```

## Named sessions

Pass `-s <name>` to every per-call command. The first command creates or
reconnects the persistent Chrome for that name; subsequent calls reuse the tab,
cookies, local storage, and latest snapshot refs. Set
`GHOSTCHROME_SESSION=<name>` when a shell flow cannot conveniently repeat the
flag. Use a separate name for unrelated accounts or concurrent tasks.

```bash
ghostchrome sessions list
ghostchrome sessions stop work
ghostchrome sessions stop work --purge  # disposable profile only
ghostchrome sessions prune
ghostchrome profiles list
```

Stopping a session closes its browser. Profile data remains by default so a
later run can reuse authentication state. Restrict `--purge` to profiles that
are explicitly disposable; never use it as routine stale-ref recovery.

## Small, observable flows

Use `goto` or `navigate` for page movement. Use `extract` for the compact
accessibility tree and stable `@ref` handles. Use `preview` when a single health
report must include status, DOM, console errors, and network errors.

```bash
ghostchrome -s work goto https://example.com --wait stable --format json
ghostchrome -s work extract --level content --format json
ghostchrome -s work click @2 --format json
ghostchrome -s work type @4 'query' --submit --format json
ghostchrome -s work preview --level skeleton --wait stable --format json
```

Re-extract after navigation or a DOM-changing interaction. A ref is a hint into
the latest snapshot, not a permanent selector. Prefer `--selector` to scope a
large extraction. Send machine-readable output to stdout and retain stderr for
diagnostics; do not parse progress warnings as records.

## Recipes

Use a registered recipe when it produces the requested site data. Recipes emit
structured output, handle pagination and deduplication, and can apply cookie or
stealth policy consistently. Confirm the recipe's schema before returning data.
Keep one record per output line when the recipe uses JSONL.

```bash
ghostchrome --stealth --dismiss-cookies cars-listings list autoscout24 \
  --make renault --model clio --price-max 10000 --pages 3 \
  --output clios.jsonl
```

Do not use `eval` to dump a full document or large application state. Use a
scoped extraction, a recipe, or a narrowly justified expression instead.

## JSONL agent loop

Use `ghostchrome agent` when a flow has many operations and one subprocess is
more efficient than repeated CLI invocations. Send exactly one JSON object per
line on stdin and read exactly one response per line on stdout. Responses may
arrive out of order when requests are pipelined; match every response by `id`.

```json
{"id":"r1","op":"navigate","args":{"url":"https://example.com","wait":"stable"}}
{"id":"r2","op":"extract","args":{"level":"skeleton"}}
{"id":"r3","op":"close"}
```

A successful response has `ok: true`; an error response has `ok: false` and an
actionable error payload. Keep stdin open until `close` is sent and its response
is received. Capture stderr separately. Use persistent flags on the invocation:

```bash
ghostchrome --stealth --dismiss-cookies -s work agent
```

The available operation families include `init`, `navigate`, `back`, `forward`,
`reload`, `extract`, `click`, `dblclick`, `check`, `uncheck`, `type`, `press`,
`hover`, `select`, `fill`, `scroll_by`, `scroll_to`, `eval`, `screenshot`,
`wait`, `errors`, `url`, and `close`. Confirm exact argument and result shapes
against the generated contract when writing an SDK or a reusable integration.

## Output and failure handling

Use `--format json` or `--json` for automation. Treat a non-zero exit code,
`ok: false`, timeout, or missing browser as a failed operation. Preserve the
last useful snapshot and report the URL, operation, and concise error. Run
`ghostchrome setup doctor --strict` for missing binaries, Chrome libraries, or
CDP connectivity; do not switch to MCP to hide a CLI installation failure.
