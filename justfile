# ghostchrome monorepo — task runner
# Install just: https://github.com/casey/just

# Build the Go binary
build:
    go build ./...

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

# Build + install locally (binary + agent skill)
install:
    go build -o ghostchrome .
    ./ghostchrome install

# Uninstall everything (binary + data + skills)
uninstall:
    ./ghostchrome uninstall --purge --yes

# End-to-end smoke against a live site (requires a running Chrome on :9222).
# Usage: just e2e            -> https://www.chronovet.fr/
#        just e2e <url>
e2e url="https://www.chronovet.fr/":
    go build -o ghostchrome .
    GHOSTCHROME_BIN="$PWD/ghostchrome" bun run examples/typescript/chronovet.ts {{url}}
    GHOSTCHROME_BIN="$PWD/ghostchrome" python3 examples/python/chronovet.py {{url}}
