#!/usr/bin/env bash
# ghostchrome installer. Exactly one runtime mode is installed at a time.
#
#   curl -fsSL https://raw.githubusercontent.com/dev-toolings/ghostchrome/main/scripts/install.sh \
#     | bash -s -- --mode cli --clients claude,codex,grok
#
# Environment overrides:
#   GHOSTCHROME_INSTALL_ROOT  state and user-binary root (default ~/.ghostchrome)
#   GHOSTCHROME_INSTALL_DIR   binary directory override (legacy)
#   GHOSTCHROME_VERSION       release tag (default latest)

set -euo pipefail

REPO="dev-toolings/ghostchrome"
MODE=""
CLIENTS="claude,codex,grok"
VERSION="${GHOSTCHROME_VERSION:-}"
INSTALL_ROOT="${GHOSTCHROME_INSTALL_ROOT:-$HOME/.ghostchrome}"
INSTALL_DIR="${GHOSTCHROME_INSTALL_DIR:-}"
SYSTEM=0

die()  { echo "error: $*" >&2; exit 1; }
info() { echo ":: $*"; }
warn() { echo "warning: $*" >&2; }

usage() {
  cat <<'EOF'
Install exactly one Ghostchrome runtime.

Usage:
  install.sh --mode cli|mcp [--clients claude,codex,grok] [options]

Options:
  --mode MODE          Required: cli or mcp
  --clients LIST       Global clients to configure (default: claude,codex,grok)
  --version TAG        Release tag (default: latest)
  --install-root DIR   State and user-binary root (default: ~/.ghostchrome)
  --system             Install the selected binary in /usr/local/bin
  --help               Show this help

The installer refuses an existing opposite-mode installation. Switching modes
requires the explicit `ghostchrome setup switch --to MODE --yes` command.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --mode)
      [ "$#" -ge 2 ] || die "--mode requires cli or mcp"
      MODE="$2"; shift 2 ;;
    --clients)
      [ "$#" -ge 2 ] || die "--clients requires a comma-separated list"
      CLIENTS="$2"; shift 2 ;;
    --version)
      [ "$#" -ge 2 ] || die "--version requires a release tag"
      VERSION="$2"; shift 2 ;;
    --install-root)
      [ "$#" -ge 2 ] || die "--install-root requires a path"
      INSTALL_ROOT="$2"; shift 2 ;;
    --system) SYSTEM=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option: $1 (use --help)" ;;
  esac
done

[ "$MODE" = "cli" ] || [ "$MODE" = "mcp" ] || die "--mode is required and must be cli or mcp"
[ -n "$CLIENTS" ] || die "--clients cannot be empty"

for client in ${CLIENTS//,/ }; do
  case "$client" in
    claude|codex|grok) ;;
    *) die "unsupported client '$client' (choose claude, codex, or grok)" ;;
  esac
done

detect_os() {
  case "$(uname -s)" in
    Linux*)  echo "linux" ;;
    Darwin*) echo "darwin" ;;
    *)       die "unsupported OS: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) die "unsupported architecture: $(uname -m)" ;;
  esac
}

latest_version() {
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
    | head -1
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    die "sha256sum or shasum is required; refusing an unverified download"
  fi
}

verify_checksum() {
  file="$1"
  asset="$2"
  expected=$(awk -v name="$asset" '$2 == name { print $1; exit }' "$SUMS")
  printf '%s' "$expected" | grep -Eq '^[[:xdigit:]]{64}$' \
    || die "missing or invalid SHA-256 checksum for ${asset}"
  actual=$(sha256_file "$file")
  [ "$expected" = "$actual" ] || die "checksum mismatch for ${asset}"
}

manifest_mode() {
  [ -f "$MANIFEST" ] || return 0
  sed -n 's/^[[:space:]]*"mode"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$MANIFEST" | head -1
}

refuse_opposite_artifacts() {
  opposite="$1"
  selected_target="$2"
  installed=$(manifest_mode)
  if [ -n "$installed" ] && [ "$installed" != "$MODE" ]; then
    die "managed ${installed} mode is already installed; run 'ghostchrome setup switch --to ${MODE} --yes'"
  fi

  # Check managed and conventional legacy locations. Files outside these
  # locations are intentionally not guessed at or removed.
  for candidate in "$BIN_DIR/$opposite" "$HOME/.local/bin/$opposite" "/usr/local/bin/$opposite"; do
    [ "$candidate" = "$selected_target" ] && continue
    [ -e "$candidate" ] || continue
    die "found opposite-mode binary at ${candidate}; remove it or use the explicit setup switch"
  done

  if [ -e "$selected_target" ] && [ -z "$installed" ]; then
    die "refusing to overwrite unmanaged binary ${selected_target}; move it or record it with setup"
  fi
}

refuse_mcp_registrations_in_cli_mode() {
  [ "$MODE" = "cli" ] || return 0
  for config in "$HOME/.claude.json" "$HOME/.codex/config.toml" "$HOME/.grok/config.toml"; do
    [ -f "$config" ] || continue
    if grep -q 'ghostchrome' "$config"; then
      die "existing Ghostchrome MCP registration in ${config}; use 'ghostchrome setup switch --to cli --yes' to migrate it"
    fi
  done
}

install_binary() {
  target="$1"
  source="$2"
  if ! mkdir -p "$BIN_DIR" 2>/dev/null; then
    command -v sudo >/dev/null 2>&1 || die "cannot create ${BIN_DIR}; rerun with --install-root or install sudo"
    sudo mkdir -p "$BIN_DIR"
  fi
  chmod 0755 "$source"
  if [ ! -w "$BIN_DIR" ]; then
    command -v sudo >/dev/null 2>&1 || die "cannot write ${BIN_DIR}; rerun with --install-root or install sudo"
    sudo install -m 0755 "$source" "$target"
    return 0
  fi
  tmp_target=$(mktemp "${BIN_DIR}/.${BINARY}.XXXXXX")
  chmod 0755 "$tmp_target"
  mv "$source" "$tmp_target"
  # The temporary file is in the destination directory, so the final rename is atomic.
  mv -f "$tmp_target" "$target"
}

write_toml_server() {
  config="$1"
  command_path="$2"
  mkdir -p "$(dirname "$config")"
  chmod 0700 "$(dirname "$config")" 2>/dev/null || true

  if [ -f "$config" ] && grep -q '^\[mcp_servers\.ghostchrome\][[:space:]]*$' "$config"; then
    if grep -A8 '^\[mcp_servers\.ghostchrome\][[:space:]]*$' "$config" | grep -Fq "command = \"${command_path}\""; then
      return 0
    fi
    die "existing unmanaged ghostchrome MCP entry in ${config}; refusing to overwrite it"
  fi

  tmp=$(mktemp "${config}.XXXXXX")
  if [ -f "$config" ]; then
    cat "$config" > "$tmp"
    [ ! -s "$tmp" ] || printf '\n' >> "$tmp"
  fi
  printf '[mcp_servers.ghostchrome]\ncommand = "%s"\nargs = []\nenabled = true\n\n[mcp_servers.ghostchrome.env]\nGHOSTCHROME_STEALTH = "true"\n' "$command_path" >> "$tmp"
  chmod 0600 "$tmp"
  mv "$tmp" "$config"
}

write_json_server() {
  config="$1"
  command_path="$2"
  command -v python3 >/dev/null 2>&1 || die "python3 is required to update ${config} safely"
  mkdir -p "$(dirname "$config")"
  chmod 0700 "$(dirname "$config")" 2>/dev/null || true
  python3 - "$config" "$command_path" <<'PY'
import json
import os
import sys
import tempfile

path, command = sys.argv[1:]
try:
    with open(path, encoding="utf-8") as stream:
        root = json.load(stream)
except FileNotFoundError:
    root = {}
except json.JSONDecodeError as exc:
    raise SystemExit(f"cannot parse {path}: {exc}")

servers = root.setdefault("mcpServers", {})
if not isinstance(servers, dict):
    raise SystemExit(f"{path} has an invalid mcpServers object")
existing = servers.get("ghostchrome")
if existing is not None and (not isinstance(existing, dict) or existing.get("command") != command):
    raise SystemExit(f"existing unmanaged ghostchrome MCP entry in {path}")
if existing is None:
    servers["ghostchrome"] = {
        "type": "stdio",
        "command": command,
        "args": [],
        "env": {"GHOSTCHROME_STEALTH": "true"},
    }

directory = os.path.dirname(path) or "."
fd, temporary = tempfile.mkstemp(prefix=".ghostchrome-", dir=directory, text=True)
try:
    with os.fdopen(fd, "w", encoding="utf-8") as stream:
        json.dump(root, stream, indent=2)
        stream.write("\n")
        stream.flush()
        os.fsync(stream.fileno())
    os.chmod(temporary, 0o600)
    os.replace(temporary, path)
except BaseException:
    try:
        os.unlink(temporary)
    except FileNotFoundError:
        pass
    raise
PY
}

configure_claude() {
  target="$1"
  write_json_server "$HOME/.claude.json" "$target"
}

configure_mcp_clients() {
  target="$1"
  for client in ${CLIENTS//,/ }; do
    case "$client" in
      claude) configure_claude "$target" ;;
      codex)  write_toml_server "$HOME/.codex/config.toml" "$target" ;;
      grok)   write_toml_server "$HOME/.grok/config.toml" "$target" ;;
    esac
  done
}

install_skill_for_client() {
  client="$1"
  source_dir="$SKILL_EXTRACT/ghostchrome"
  case "$client" in
    claude) destination="$HOME/.claude/skills/ghostchrome" ;;
    codex)  destination="$HOME/.codex/skills/ghostchrome" ;;
    grok)   destination="$HOME/.grok/skills/ghostchrome" ;;
  esac

  if [ -e "$destination" ]; then
    [ -f "$destination/SKILL.md" ] || die "existing unmanaged skill directory ${destination}"
    existing_hash=$(sha256_file "$destination/SKILL.md")
    [ "$existing_hash" = "$SKILL_HASH" ] || die "existing unmanaged skill ${destination}; refusing to overwrite it"
    return 0
  fi
  mkdir -p "$(dirname "$destination")"
  chmod 0700 "$(dirname "$destination")" 2>/dev/null || true
  cp -R "$source_dir" "$destination"
  chmod 0700 "$destination"
  find "$destination" -type f -exec chmod 0600 {} \;
  # Skills may ship validation or runnable example scripts. Keep the skill
  # private by default, but preserve execution for those explicitly executable
  # shell assets so a fresh install passes its own validator.
  find "$destination" -type f -name '*.sh' -exec chmod 0755 {} \;
}

write_manifest() {
  manifest_tmp=$(mktemp "${INSTALL_ROOT}/.install.json.XXXXXX")
  managed="    \"${TARGET}\""
  for client in ${CLIENTS//,/ }; do
    case "$client" in
      claude) skill_root="$HOME/.claude/skills/ghostchrome" ;;
      codex)  skill_root="$HOME/.codex/skills/ghostchrome" ;;
      grok)   skill_root="$HOME/.grok/skills/ghostchrome" ;;
    esac
    # Keep every packaged skill file in the manifest so setup uninstall and
    # upgrades can distinguish managed references from user-authored files.
    for relative in SKILL.md references/cli.md references/mcp.md references/troubleshooting.md references/packaging.md examples/cli-flow.sh examples/mcp-config.toml scripts/validate-skill.sh; do
      managed="${managed},"$'\n'"    \"${skill_root}/${relative}\""
    done
    if [ "$MODE" = "mcp" ]; then
      case "$client" in
        claude) managed="${managed},"$'\n'"    \"$HOME/.claude.json\"" ;;
        codex)  managed="${managed},"$'\n'"    \"$HOME/.codex/config.toml\"" ;;
        grok)   managed="${managed},"$'\n'"    \"$HOME/.grok/config.toml\"" ;;
      esac
    fi
  done
  now=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
  cat > "$manifest_tmp" <<EOF
{
  "schema_version": 1,
  "mode": "${MODE}",
  "version": "${VERSION}",
  "install_root": "${INSTALL_ROOT}",
  "binary": "${TARGET}",
  "clients": ["${CLIENTS//,/\",\"}"],
  "skill_sha256": "${SKILL_HASH}",
  "managed_files": [
${managed}
  ],
  "installed_at": "${now}"
}
EOF
  chmod 0600 "$manifest_tmp"
  mv "$manifest_tmp" "$MANIFEST"
}

OS="$(detect_os)"
ARCH="$(detect_arch)"
[ -n "$VERSION" ] || VERSION="$(latest_version)"
[ -n "$VERSION" ] || die "could not determine release version"

if [ "$SYSTEM" -eq 1 ]; then
  BIN_DIR="${GHOSTCHROME_SYSTEM_BIN_DIR:-/usr/local/bin}"
else
  BIN_DIR="${INSTALL_DIR:-${INSTALL_ROOT}/bin}"
fi

BINARY="ghostchrome"
[ "$MODE" = "mcp" ] && BINARY="ghostchrome-mcp"
TARGET="${BIN_DIR}/${BINARY}"
MANIFEST="${INSTALL_ROOT}/install.json"
opposite="ghostchrome"
[ "$MODE" = "cli" ] && opposite="ghostchrome-mcp"

mkdir -p "$INSTALL_ROOT"
chmod 0700 "$INSTALL_ROOT" 2>/dev/null || true
refuse_opposite_artifacts "$opposite" "$TARGET"
refuse_mcp_registrations_in_cli_mode

TMP_DIR=$(mktemp -d)
SUMS="${TMP_DIR}/checksums.txt"
SKILL_ARCHIVE="${TMP_DIR}/ghostchrome-skill.tar.gz"
SKILL_EXTRACT="${TMP_DIR}/skill"
trap 'rm -rf "$TMP_DIR"' EXIT

ASSET="${BINARY}-${OS}-${ARCH}"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
info "installing Ghostchrome ${VERSION} in ${MODE} mode (${OS}/${ARCH})"
curl -fsSL "${BASE_URL}/${ASSET}" -o "${TMP_DIR}/${ASSET}"
curl -fsSL "${BASE_URL}/checksums.txt" -o "$SUMS"
verify_checksum "${TMP_DIR}/${ASSET}" "$ASSET"

# The standalone MCP binary does not carry CLI-only skill management commands;
# both installation modes consume this shared release asset.
curl -fsSL "${BASE_URL}/ghostchrome-skill.tar.gz" -o "$SKILL_ARCHIVE"
verify_checksum "$SKILL_ARCHIVE" "ghostchrome-skill.tar.gz"
mkdir -p "$SKILL_EXTRACT"
tar -xzf "$SKILL_ARCHIVE" -C "$SKILL_EXTRACT"
[ -f "$SKILL_EXTRACT/ghostchrome/SKILL.md" ] || die "skill archive has no ghostchrome/SKILL.md"
SKILL_HASH=$(sha256_file "$SKILL_EXTRACT/ghostchrome/SKILL.md")

install_binary "$TARGET" "${TMP_DIR}/${ASSET}"
for client in ${CLIENTS//,/ }; do
  install_skill_for_client "$client"
done

if [ "$MODE" = "mcp" ]; then
  configure_mcp_clients "$TARGET"
fi

write_manifest
info "installed ${TARGET}"
info "manifest: ${MANIFEST}"
if [ "$SYSTEM" -eq 0 ] && ! printf '%s\n' "$PATH" | tr ':' '\n' | grep -Fqx "$BIN_DIR"; then
  info "add to your shell profile: export PATH=\"${BIN_DIR}:\$PATH\""
fi
"$TARGET" --version 2>/dev/null || true
info "done. run 'ghostchrome setup doctor --strict' from a CLI installation for diagnostics"
