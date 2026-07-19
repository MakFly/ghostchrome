# ghostchrome monorepo — task runner
# Install just: https://github.com/casey/just

# Single source of truth for the local build version (git tag + commit).
# Release CI stamps the exact tag; local builds report the nearest tag so
# `ghostchrome --version` is never a bare "dev".
version := `git describe --tags --always --dirty 2>/dev/null || echo dev`

# Compile-check every package (no binary emitted)
build:
    CGO_ENABLED=0 go build ./...

# Run Go tests (short mode — no integration tests)
test:
    go test -short ./...

# Regenerate contracts/commands.json from the canonical ops registry
contract:
    go generate ./internal/ops/...

# TypeScript SDK — typecheck + hermetic tests
sdk-ts:
    cd sdk/typescript && bun install && bunx tsc --noEmit && bun test

# Python SDK — hermetic tests (stdlib unittest, no install needed)
sdk-py:
    cd sdk/python && python3 -m unittest discover -s tests -q

# Run every suite: Go + both SDKs
test-all: test sdk-ts sdk-py

# Build + install locally (binary + agent skill).
# CGO_ENABLED=0 → a truly static binary (matches the "static Go binary" promise);
# -X main.version stamps the real version instead of "dev".
install:
    CGO_ENABLED=0 go build -ldflags="-s -w -X main.version={{version}}" -o ghostchrome .
    ./ghostchrome install

# Uninstall everything (binary + data + skills)
uninstall:
    ./ghostchrome uninstall --purge --yes

# End-to-end smoke against a live site (requires a running Chrome on :9222).
# Usage: just e2e            -> https://www.chronovet.fr/
#        just e2e <url>
e2e url="https://www.chronovet.fr/":
    CGO_ENABLED=0 go build -ldflags="-X main.version={{version}}" -o ghostchrome .
    GHOSTCHROME_BIN="$PWD/ghostchrome" bun run examples/typescript/chronovet.ts {{url}}
    GHOSTCHROME_BIN="$PWD/ghostchrome" python3 examples/python/chronovet.py {{url}}
