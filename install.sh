#!/bin/sh
# ghostchrome installer — works on macOS and Linux
# Usage: curl -fsSL https://raw.githubusercontent.com/MakFly/ghostchrome/main/install.sh | sh

set -e

REPO="MakFly/ghostchrome"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS and arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

case "$OS" in
  darwin|linux) ;;
  *) echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

BINARY="ghostchrome-${OS}-${ARCH}"

# Get latest version
VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$VERSION" ]; then
  echo "Failed to fetch latest version" >&2
  exit 1
fi

URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"

echo "Installing ghostchrome ${VERSION} (${OS}/${ARCH})..."

# Download
TMP=$(mktemp)
SUMS=$(mktemp)
trap 'rm -f "$TMP" "$SUMS"' EXIT
curl -fsSL "$URL" -o "$TMP"
curl -fsSL "$CHECKSUMS_URL" -o "$SUMS"
chmod +x "$TMP"

if command -v sha256sum >/dev/null 2>&1; then
  EXPECTED=$(grep " ${BINARY}\$" "$SUMS" | awk '{print $1}')
  [ -n "$EXPECTED" ] || { echo "Missing checksum for ${BINARY}" >&2; exit 1; }
  printf '%s  %s\n' "$EXPECTED" "$TMP" | sha256sum -c -
elif command -v shasum >/dev/null 2>&1; then
  EXPECTED=$(grep " ${BINARY}\$" "$SUMS" | awk '{print $1}')
  [ -n "$EXPECTED" ] || { echo "Missing checksum for ${BINARY}" >&2; exit 1; }
  ACTUAL=$(shasum -a 256 "$TMP" | awk '{print $1}')
  [ "$EXPECTED" = "$ACTUAL" ] || { echo "Checksum mismatch for ${BINARY}" >&2; exit 1; }
else
  echo "Warning: no SHA-256 verifier found, skipping checksum validation" >&2
fi

# Install
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP" "${INSTALL_DIR}/ghostchrome"
else
  echo "Need sudo to install to ${INSTALL_DIR}"
  sudo mv "$TMP" "${INSTALL_DIR}/ghostchrome"
fi

echo "ghostchrome ${VERSION} installed to ${INSTALL_DIR}/ghostchrome"
"${INSTALL_DIR}/ghostchrome" --version
