#!/usr/bin/env sh
# Install the plugin-kit-ai CLI from GitHub Releases.
#
# Defaults:
#   - latest published stable release from 777genius/plugin-kit-ai
#   - verifies checksums.txt
#   - installs into $HOME/.local/bin unless BIN_DIR is set
#
# Optional env:
#   VERSION                  explicit tag or version (v1.2.3 or 1.2.3)
#   BIN_DIR                  destination directory for the installed binary
#   GITHUB_TOKEN             token for API/download rate limits
#   GITHUB_API_BASE          API base URL (default https://api.github.com)
#   PLUGIN_KIT_AI_REPOSITORY owner/repo override (default 777genius/plugin-kit-ai)
#   PLUGIN_KIT_AI_RELEASE_BASE_URL override release download base (advanced/test only)
#   PLUGIN_KIT_AI_OUTPUT_FILE optional key=value output file for automation/action usage
#
# Any arguments after the script are passed to the installed plugin-kit-ai binary.
# Example:
#   curl -fsSL https://raw.githubusercontent.com/777genius/plugin-kit-ai/main/scripts/install.sh | sh -s -- add notion --dry-run

set -eu

REPOSITORY="${PLUGIN_KIT_AI_REPOSITORY:-777genius/plugin-kit-ai}"
API_BASE="${GITHUB_API_BASE:-https://api.github.com}"
RELEASE_BASE_URL="${PLUGIN_KIT_AI_RELEASE_BASE_URL:-}"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
VERSION_INPUT="${VERSION:-}"
OUTPUT_FILE="${PLUGIN_KIT_AI_OUTPUT_FILE:-}"

fail() {
  echo "plugin-kit-ai install bootstrap: $*" >&2
  exit 1
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

http_fetch() {
  url="$1"
  out="$2"
  if [ -n "${GITHUB_TOKEN:-}" ]; then
    curl -fsSL -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "Accept: application/vnd.github+json" "$url" -o "$out"
  else
    curl -fsSL -H "Accept: application/vnd.github+json" "$url" -o "$out"
  fi
}

http_fetch_stdout() {
  url="$1"
  if [ -n "${GITHUB_TOKEN:-}" ]; then
    curl -fsSL -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "Accept: application/vnd.github+json" "$url"
  else
    curl -fsSL -H "Accept: application/vnd.github+json" "$url"
  fi
}

detect_os() {
  case "$(uname -s | tr '[:upper:]' '[:lower:]')" in
    linux*) echo "linux" ;;
    darwin*) echo "darwin" ;;
    msys*|mingw*|cygwin*) echo "windows" ;;
    *) fail "unsupported OS $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *) fail "unsupported architecture $(uname -m)" ;;
  esac
}

normalize_tag() {
  raw="$1"
  case "$raw" in
    "") echo "" ;;
    latest) echo "" ;;
    v*) echo "$raw" ;;
    *) echo "v$raw" ;;
  esac
}

derive_release_base() {
  if [ -n "$RELEASE_BASE_URL" ]; then
    echo "$RELEASE_BASE_URL"
    return
  fi
  if [ "$API_BASE" = "https://api.github.com" ] || [ "$API_BASE" = "http://api.github.com" ]; then
    echo "https://github.com"
    return
  fi
  printf '%s' "$API_BASE" | sed -E 's#/api/v3/?$##; s#/api/?$##'
}

latest_tag() {
  json="$(http_fetch_stdout "$(printf '%s/repos/%s/releases/latest' "$(printf '%s' "$API_BASE" | sed 's#/$##')" "$REPOSITORY")")"
  tag="$(printf '%s' "$json" | tr -d '\n' | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
  [ -n "$tag" ] || fail "could not resolve latest release tag from ${API_BASE}"
  echo "$tag"
}

expected_asset_line() {
  checksums_file="$1"
  os_name="$2"
  arch_name="$3"
  awk -v suffix="_${os_name}_${arch_name}.tar.gz" '
    NF >= 2 && $2 ~ ("^plugin-kit-ai_.*" suffix "$") { print $1 " " $2 }
  ' "$checksums_file"
}

file_sha256() {
  file="$1"
  if command_exists sha256sum; then
    sha256sum "$file" | awk '{print $1}'
    return
  fi
  if command_exists shasum; then
    shasum -a 256 "$file" | awk '{print $1}'
    return
  fi
  if command_exists openssl; then
    openssl dgst -sha256 "$file" | awk '{print $NF}'
    return
  fi
  fail "no SHA256 tool found (need sha256sum, shasum, or openssl)"
}

write_output() {
  key="$1"
  value="$2"
  if [ -n "$OUTPUT_FILE" ]; then
    printf '%s=%s\n' "$key" "$value" >>"$OUTPUT_FILE"
  fi
}

display_path() {
  path="$1"
  if [ "$OS_NAME" = "windows" ]; then
    printf '%s' "$path" | sed 's#/#\\#g'
    return
  fi
  printf '%s' "$path"
}

OS_NAME="$(detect_os)"
ARCH_NAME="$(detect_arch)"
TAG="$(normalize_tag "$VERSION_INPUT")"
if [ -z "$TAG" ]; then
  TAG="$(latest_tag)"
fi
DOWNLOAD_BASE="$(printf '%s/%s/releases/download/%s' "$(derive_release_base | sed 's#/$##')" "$REPOSITORY" "$TAG")"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

checksums_path="$tmp/checksums.txt"
http_fetch "${DOWNLOAD_BASE}/checksums.txt" "$checksums_path" || fail "failed to download checksums.txt for ${REPOSITORY}@${TAG}"

matches="$(expected_asset_line "$checksums_path" "$OS_NAME" "$ARCH_NAME")"
match_count="$(printf '%s\n' "$matches" | sed '/^$/d' | wc -l | tr -d ' ')"
if [ "$match_count" -eq 0 ]; then
  fail "no release asset in checksums.txt for ${OS_NAME}/${ARCH_NAME}"
fi
if [ "$match_count" -ne 1 ]; then
  fail "ambiguous release assets in checksums.txt for ${OS_NAME}/${ARCH_NAME}"
fi

EXPECTED_SUM="$(printf '%s\n' "$matches" | awk '{print $1}')"
ASSET_NAME="$(printf '%s\n' "$matches" | awk '{print $2}')"
ARCHIVE_PATH="$tmp/$ASSET_NAME"
http_fetch "${DOWNLOAD_BASE}/${ASSET_NAME}" "$ARCHIVE_PATH" || fail "failed to download ${ASSET_NAME}"

ACTUAL_SUM="$(file_sha256 "$ARCHIVE_PATH")"
if [ "$ACTUAL_SUM" != "$EXPECTED_SUM" ]; then
  fail "checksum mismatch for ${ASSET_NAME}"
fi

extract_dir="$tmp/extract"
mkdir -p "$extract_dir"
tar -xzf "$ARCHIVE_PATH" -C "$extract_dir"

BINARY_PATH=""
if [ "$OS_NAME" = "windows" ]; then
  candidate="$extract_dir/plugin-kit-ai.exe"
  [ -f "$candidate" ] && BINARY_PATH="$candidate"
else
  candidate="$extract_dir/plugin-kit-ai"
  [ -f "$candidate" ] && BINARY_PATH="$candidate"
fi
if [ -z "$BINARY_PATH" ]; then
  candidate="$(find "$extract_dir" -maxdepth 1 -type f \( -name 'plugin-kit-ai' -o -name 'plugin-kit-ai.exe' \) | head -n 1)"
  [ -n "$candidate" ] || fail "archive ${ASSET_NAME} does not contain plugin-kit-ai binary at archive root"
  BINARY_PATH="$candidate"
fi

mkdir -p "$BIN_DIR"
DEST_PATH="$BIN_DIR/$(basename "$BINARY_PATH")"
DISPLAY_DEST_PATH="$(display_path "$DEST_PATH")"
cp "$BINARY_PATH" "$DEST_PATH"
chmod 0755 "$DEST_PATH" 2>/dev/null || true

echo "Installed plugin-kit-ai"
echo "Version: ${TAG}"
echo "Repository: ${REPOSITORY}"
echo "Asset: ${ASSET_NAME}"
echo "Installed path: ${DISPLAY_DEST_PATH}"
echo "Checksum: verified via checksums.txt"

case ":${PATH}:" in
  *":${BIN_DIR}:"*) ;;
  *)
    echo "PATH hint: add ${BIN_DIR} to PATH"
    ;;
esac

write_output "version" "$TAG"
write_output "path" "$DISPLAY_DEST_PATH"
write_output "bin_dir" "$BIN_DIR"
write_output "asset" "$ASSET_NAME"

if [ "$#" -gt 0 ]; then
  echo "Running plugin-kit-ai $*"
  "$DEST_PATH" "$@"
fi
