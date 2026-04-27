# AGENTS.md — ghostchrome

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
