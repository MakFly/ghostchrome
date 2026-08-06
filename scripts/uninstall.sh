#!/usr/bin/env bash
# ghostchrome uninstaller — one-liner:
#   curl -fsSL https://raw.githubusercontent.com/dev-toolings/ghostchrome/main/scripts/uninstall.sh | bash
#
# Equivalent to: ghostchrome uninstall --purge --yes
# but works even if the binary is already gone.

set -euo pipefail

BIN="ghostchrome"
INSTALL_DIR="${GHOSTCHROME_INSTALL_DIR:-$HOME/.local/bin}"

info() { echo ":: $*"; }

# If the binary is still around, let it handle cleanup (sessions, skills).
if command -v "$BIN" &>/dev/null; then
  "$BIN" uninstall --purge --yes && exit 0
fi

# Manual cleanup when the binary is already missing.
info "binary not found — manual cleanup"

PATHS=(
  "$INSTALL_DIR/$BIN"
  "$HOME/.ghostchrome"
  "$HOME/.claude/skills/ghostchrome"
)
for p in "${PATHS[@]}"; do
  [ -e "$p" ] || continue
  rm -rf "$p"
  info "removed $p"
done

info "ghostchrome uninstalled."
