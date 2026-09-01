#!/bin/sh
# Compatibility entry point for the canonical installer under scripts/.
#
# Local checkout:
#   ./install.sh --mode cli --clients claude,codex,grok
# One-liner:
#   curl -fsSL https://raw.githubusercontent.com/dev-toolings/ghostchrome/main/install.sh \
#     | sh -s -- --mode mcp

set -eu

REPO="dev-toolings/ghostchrome"
SCRIPT_DIR=""
case "$0" in
  */install.sh) SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" 2>/dev/null && pwd) ;;
esac

if [ -n "$SCRIPT_DIR" ] && [ -f "${SCRIPT_DIR}/scripts/install.sh" ]; then
  exec bash "${SCRIPT_DIR}/scripts/install.sh" "$@"
fi

command -v curl >/dev/null 2>&1 || {
  echo "error: curl is required" >&2
  exit 1
}
exec curl -fsSL "https://raw.githubusercontent.com/${REPO}/main/scripts/install.sh" \
  | bash -s -- "$@"
