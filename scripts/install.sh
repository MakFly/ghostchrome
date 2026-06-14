#!/usr/bin/env bash
# ghostchrome installer — one-liner:
#   curl -fsSL https://raw.githubusercontent.com/MakFly/ghostchrome/main/scripts/install.sh | bash
#
# Environment overrides:
#   GHOSTCHROME_INSTALL_DIR  — binary location (default: ~/.local/bin)
#   GHOSTCHROME_VERSION      — tag to install  (default: latest)

set -euo pipefail

REPO="MakFly/ghostchrome"
INSTALL_DIR="${GHOSTCHROME_INSTALL_DIR:-$HOME/.local/bin}"
BIN="ghostchrome"

# --- helpers ----------------------------------------------------------------

die()  { echo "error: $*" >&2; exit 1; }
info() { echo ":: $*"; }

detect_os() {
  case "$(uname -s)" in
    Linux*)  echo "linux"  ;;
    Darwin*) echo "darwin" ;;
    *)       die "unsupported OS: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)   echo "amd64"  ;;
    aarch64|arm64)   echo "arm64"  ;;
    *)               die "unsupported arch: $(uname -m)" ;;
  esac
}

latest_version() {
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | head -1 | sed 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/'
}

# --- main -------------------------------------------------------------------

OS="$(detect_os)"
ARCH="$(detect_arch)"
VERSION="${GHOSTCHROME_VERSION:-$(latest_version)}"

[ -z "$VERSION" ] && die "could not determine latest version — set GHOSTCHROME_VERSION"

ASSET="${BIN}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"

info "installing ghostchrome ${VERSION} (${OS}/${ARCH})"
info "downloading ${URL}"

mkdir -p "$INSTALL_DIR"
curl -fsSL -o "${INSTALL_DIR}/${BIN}" "$URL"
chmod +x "${INSTALL_DIR}/${BIN}"

info "installed → ${INSTALL_DIR}/${BIN}"

# Ensure INSTALL_DIR is on PATH
if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
  info ""
  info "add to your shell profile:"
  info "  export PATH=\"${INSTALL_DIR}:\$PATH\""
  info ""
fi

# Install the bundled agent skill for Claude Code
"${INSTALL_DIR}/${BIN}" skills install 2>/dev/null || true

info "done. run: ghostchrome doctor"
