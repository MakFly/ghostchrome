# ghostchrome

Ultra-light browser automation CLI for LLM agents. Single Go binary, zero Node.js, 7-25x fewer tokens than Playwright MCP.

```
ghostchrome preview http://localhost:3000
```

```
[200] Dashboard — http://localhost:3000 (134ms)
[errors] none
[network] 12 reqs, 0 failed
[dom]
  [h1] Dashboard
  [btn @1] Add User
  [table] 5 rows
  [link @2 href=/settings] Settings
```

## Why

Playwright MCP burns **14,000-50,000 tokens per snapshot**. ghostchrome returns the same information in **1,000-5,000 tokens** using a filtered accessibility tree with compact refs.

| | Playwright MCP | ghostchrome |
|---|---|---|
| Tokens/snapshot (GitHub.com) | ~14,000-19,000 | **~2,000** |
| Runtime | Node.js (~200MB) | **Go binary (15MB)** |
| Startup | 2-5s | **<1s** |
| Dependencies | npm install | **Single binary** |
| Protocol | MCP (JSON-RPC) | **CLI (stdin/stdout)** |

## Install

### Quick install (macOS & Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/MakFly/ghostchrome/main/install.sh | sh
```

### Go install

```bash
go install github.com/MakFly/ghostchrome@latest
```

### Manual download

Prebuilt binaries on [Releases](https://github.com/MakFly/ghostchrome/releases) — macOS (Intel/ARM), Linux (amd64/arm64), Windows.

### Requirements

- Chrome or Chromium installed on your system

## Commands

### Page inspection

```bash
# Full health report (status + errors + network + DOM)
ghostchrome preview <url> [--level skeleton|content|full]

# Navigate and get page info
ghostchrome navigate <url> [--wait load|stable|idle|none] [--extract skeleton|content|full]

# Extract DOM as compact accessibility tree
ghostchrome extract <url> [--level skeleton|content|full] [--selector "main"]

# Capture screenshot
ghostchrome screenshot <url> [--full] [--element @ref] [--output path]

# Evaluate JavaScript (supports async/await)
ghostchrome eval "<expression>" <url> [--on @ref]

# Collect console + network errors
ghostchrome errors <url>
```

### Interaction (via refs)

Refs (`@1`, `@2`, etc.) come from the latest page snapshot produced by `preview`, `extract`, `navigate --extract`, or an interaction command that first navigates to a URL.

```bash
ghostchrome click @3 <url>              # Click a button/link
ghostchrome type @2 "hello" <url>       # Type into an input
ghostchrome select @5 "option" <url>    # Select dropdown option
ghostchrome hover @1 <url>             # Hover (menus, tooltips)
ghostchrome press Enter <url>          # Keyboard key
ghostchrome press Tab --on @2 <url>    # Key on specific element
```

### Navigation

```bash
ghostchrome back                        # Browser history back
ghostchrome forward                     # Browser history forward
ghostchrome waitfor "selector" <url>   # Wait for element to appear
```

### Browser management

```bash
ghostchrome tabs                        # List open tabs
ghostchrome tabs switch 2              # Switch to tab
ghostchrome tabs close 1               # Close tab
ghostchrome viewport 375 667           # Set viewport size
ghostchrome viewport --device iphone-14 <url>  # Device preset
ghostchrome dialog accept              # Wait for and handle next JS alert/confirm
ghostchrome serve [--port 9222]        # Persistent Chrome session
```

### Device presets

`iphone-se`, `iphone-14`, `iphone-14-pro-max`, `ipad`, `ipad-pro`, `pixel-7`, `desktop-hd`, `desktop-2k`

## Extraction levels

| Level | Content | Tokens (GitHub.com) |
|---|---|---|
| `skeleton` | Headings, buttons, links, forms, landmarks | ~2,000 |
| `content` | + visible text, paragraphs, images | ~4,300 |
| `full` | + all named ARIA nodes | ~4,600 |

## Global flags

```
--connect ws://...   Connect to existing Chrome
--headless=false     Show browser window
--timeout 30         Operation timeout (seconds)
--format json        Output as JSON (default: text)
--stealth            Hide headless fingerprints
--dismiss-cookies    Auto-dismiss cookie banners
```

## Session mode

For multi-step workflows, use a persistent Chrome:

```bash
# Terminal 1: start Chrome
ghostchrome serve --port 9222

# Terminal 2: interact
ghostchrome navigate --connect ws://... https://app.com/login
ghostchrome type @1 "user@test.com" --connect ws://...
ghostchrome type @2 "password" --connect ws://...
ghostchrome click @3 --connect ws://...
ghostchrome preview --connect ws://...
```

## Use with LLM agents

ghostchrome is designed for AI coding agents (Claude Code, Cursor, etc.) to verify their work:

```bash
# After implementing a feature, the LLM runs:
ghostchrome preview http://localhost:3000/dashboard
# → sees status, errors, network, DOM in ~1,000 tokens
# → if errors: fixes them automatically
```

## Built with

- [Rod](https://github.com/go-rod/rod) — Go Chrome DevTools Protocol driver
- [Cobra](https://github.com/spf13/cobra) — CLI framework
- Chrome DevTools Protocol — direct CDP, no intermediary

## License

MIT
