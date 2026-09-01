#!/usr/bin/env bash
# Minimal CLI-mode flow. Pass a URL explicitly; the example performs no submit.
set -euo pipefail

if [[ $# -ne 1 ]]; then
  printf 'usage: %s URL\n' "$0" >&2
  exit 64
fi

gc_bin=${GHOSTCHROME_BIN:-ghostchrome}
session=${GHOSTCHROME_SESSION:-skill-demo}

cleanup() {
  if [[ ${GHOSTCHROME_KEEP_SESSION:-0} == 1 ]]; then
    return
  fi
  if ! "$gc_bin" sessions stop "$session" >/dev/null 2>&1; then
    printf 'warning: could not stop session %s\n' "$session" >&2
  fi
}
trap cleanup EXIT

"$gc_bin" -s "$session" goto "$1" --wait stable --format json
"$gc_bin" -s "$session" extract --level skeleton --format json
