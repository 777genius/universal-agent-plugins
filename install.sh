#!/usr/bin/env sh
# Install the native agentplugins CLI from verified GitHub Release assets.
#
# The CLI itself does not require Node.js. Individual plugins may declare their
# own runtime requirements, which agentplugins reports before installation.
#
# Optional environment variables:
#   AGENTPLUGINS_VERSION          version or tag (0.1.44, v0.1.44, or agentplugins-v0.1.44)
#   AGENTPLUGINS_BIN_DIR          destination directory (default: $HOME/.local/bin)
#   AGENTPLUGINS_REPOSITORY       owner/repo override for mirrors and tests
#   AGENTPLUGINS_API_BASE         GitHub API base override
#   AGENTPLUGINS_RELEASE_BASE_URL release host override
#   AGENTPLUGINS_OUTPUT_FILE      optional key=value output file for automation
#   GITHUB_TOKEN                  optional token for API and download rate limits
#
# Arguments are passed to the installed binary after installation:
#   curl -fsSL https://raw.githubusercontent.com/777genius/universal-agent-plugins/main/install.sh \
#     | sh -s -- add context7

set -eu

REPOSITORY="${AGENTPLUGINS_REPOSITORY:-777genius/universal-agent-plugins}"
API_BASE="${AGENTPLUGINS_API_BASE:-https://api.github.com}"
RELEASE_BASE_URL="${AGENTPLUGINS_RELEASE_BASE_URL:-}"
BIN_DIR="${AGENTPLUGINS_BIN_DIR:-${HOME:?HOME is required}/.local/bin}"
VERSION_INPUT="${AGENTPLUGINS_VERSION:-latest}"
OUTPUT_FILE="${AGENTPLUGINS_OUTPUT_FILE:-}"

fail() {
  echo "agentplugins installer: $*" >&2
  exit 1
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

http_fetch() {
  url="$1"
  output="$2"
  if [ -n "${GITHUB_TOKEN:-}" ]; then
    curl -fsSL --retry 3 --retry-delay 1 \
      -H "Authorization: Bearer ${GITHUB_TOKEN}" \
      -H "Accept: application/vnd.github+json" \
      "$url" -o "$output"
  else
    curl -fsSL --retry 3 --retry-delay 1 \
      -H "Accept: application/vnd.github+json" \
      "$url" -o "$output"
  fi
}

http_fetch_stdout() {
  url="$1"
  if [ -n "${GITHUB_TOKEN:-}" ]; then
    curl -fsSL --retry 3 --retry-delay 1 \
      -H "Authorization: Bearer ${GITHUB_TOKEN}" \
      -H "Accept: application/vnd.github+json" \
      "$url"
  else
    curl -fsSL --retry 3 --retry-delay 1 \
      -H "Accept: application/vnd.github+json" \
      "$url"
  fi
}

detect_os() {
  case "$(uname -s | tr '[:upper:]' '[:lower:]')" in
    linux*) echo "linux" ;;
    darwin*) echo "darwin" ;;
    msys*|mingw*|cygwin*) echo "windows" ;;
    *) fail "unsupported operating system: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m | tr '[:upper:]' '[:lower:]')" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *) fail "unsupported architecture: $(uname -m)" ;;
  esac
}

normalize_tag() {
  raw="$1"
  case "$raw" in
    ""|latest) echo "" ;;
    agentplugins-v*) echo "$raw" ;;
    v*) echo "agentplugins-$raw" ;;
    *) echo "agentplugins-v$raw" ;;
  esac
}

validate_tag() {
  tag="$1"
  printf '%s\n' "$tag" | grep -Eq '^agentplugins-v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$' \
    || fail "invalid stable version or tag: $tag"
}

latest_tag() {
  api="$(printf '%s' "$API_BASE" | sed 's#/$##')/repos/${REPOSITORY}/releases/latest"
  payload="$(http_fetch_stdout "$api")" || fail "failed to resolve the latest release"
  tag="$(printf '%s' "$payload" | tr -d '\n' | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
  [ -n "$tag" ] || fail "latest release response did not contain tag_name"
  echo "$tag"
}

derive_release_host() {
  if [ -n "$RELEASE_BASE_URL" ]; then
    printf '%s\n' "$(printf '%s' "$RELEASE_BASE_URL" | sed 's#/$##')"
    return
  fi
  case "$API_BASE" in
    https://api.github.com|https://api.github.com/) echo "https://github.com" ;;
    *) printf '%s' "$API_BASE" | sed -E 's#/api/v3/?$##; s#/api/?$##; s#/$##' ;;
  esac
}

file_sha256() {
  file="$1"
  if command_exists sha256sum; then
    sha256sum "$file" | awk '{print $1}'
  elif command_exists shasum; then
    shasum -a 256 "$file" | awk '{print $1}'
  elif command_exists openssl; then
    openssl dgst -sha256 "$file" | awk '{print $NF}'
  else
    fail "no SHA-256 utility found; install sha256sum, shasum, or openssl"
  fi
}

write_output() {
  key="$1"
  value="$2"
  if [ -n "$OUTPUT_FILE" ]; then
    printf '%s=%s\n' "$key" "$value" >>"$OUTPUT_FILE"
  fi
}

command_exists curl || fail "curl is required"

OS_NAME="$(detect_os)"
ARCH_NAME="$(detect_arch)"
TAG="$(normalize_tag "$VERSION_INPUT")"
if [ -z "$TAG" ]; then
  TAG="$(latest_tag)"
fi
validate_tag "$TAG"
VERSION="${TAG#agentplugins-v}"

SUFFIX=""
if [ "$OS_NAME" = "windows" ]; then
  SUFFIX=".exe"
fi
ASSET_NAME="agentplugins_${VERSION}_${OS_NAME}_${ARCH_NAME}${SUFFIX}"
DOWNLOAD_BASE="$(derive_release_host)/${REPOSITORY}/releases/download/${TAG}"

TEMP_ROOT="$(mktemp -d)"
INSTALL_TEMP=""
cleanup() {
  if [ -n "$INSTALL_TEMP" ] && [ -e "$INSTALL_TEMP" ]; then
    rm -f "$INSTALL_TEMP"
  fi
  rm -rf "$TEMP_ROOT"
}
trap cleanup EXIT HUP INT TERM

CHECKSUMS_PATH="$TEMP_ROOT/checksums.txt"
BINARY_PATH="$TEMP_ROOT/$ASSET_NAME"
http_fetch "$DOWNLOAD_BASE/checksums.txt" "$CHECKSUMS_PATH" \
  || fail "failed to download checksums.txt for ${REPOSITORY}@${TAG}"

MATCHES="$(awk -v asset="$ASSET_NAME" '$2 == asset { print $1 }' "$CHECKSUMS_PATH")"
MATCH_COUNT="$(printf '%s\n' "$MATCHES" | sed '/^$/d' | wc -l | tr -d ' ')"
[ "$MATCH_COUNT" -eq 1 ] || fail "checksums.txt must contain exactly one entry for $ASSET_NAME"
EXPECTED_SUM="$(printf '%s\n' "$MATCHES" | sed -n '1p')"
printf '%s\n' "$EXPECTED_SUM" | grep -Eq '^[0-9a-f]{64}$' \
  || fail "checksums.txt contains an invalid SHA-256 for $ASSET_NAME"

http_fetch "$DOWNLOAD_BASE/$ASSET_NAME" "$BINARY_PATH" \
  || fail "failed to download $ASSET_NAME"
[ -s "$BINARY_PATH" ] || fail "downloaded binary is empty: $ASSET_NAME"
ACTUAL_SUM="$(file_sha256 "$BINARY_PATH")"
[ "$ACTUAL_SUM" = "$EXPECTED_SUM" ] || fail "checksum mismatch for $ASSET_NAME"
chmod 0755 "$BINARY_PATH" 2>/dev/null || true

OBSERVED_VERSION="$($BINARY_PATH version 2>&1)" \
  || fail "downloaded binary failed its version check"
[ "$OBSERVED_VERSION" = "agentplugins $VERSION" ] \
  || fail "downloaded binary reported unexpected version: $OBSERVED_VERSION"

mkdir -p "$BIN_DIR"
DEST_NAME="agentplugins${SUFFIX}"
DEST_PATH="$BIN_DIR/$DEST_NAME"
[ ! -d "$DEST_PATH" ] || fail "install destination is a directory: $DEST_PATH"
INSTALL_TEMP="$(mktemp "$BIN_DIR/.agentplugins.XXXXXX")"
cp "$BINARY_PATH" "$INSTALL_TEMP"
chmod 0755 "$INSTALL_TEMP" 2>/dev/null || true
mv -f "$INSTALL_TEMP" "$DEST_PATH"
INSTALL_TEMP=""

echo "Installed agentplugins $VERSION"
echo "Path: $DEST_PATH"
echo "SHA-256: $ACTUAL_SUM"
case ":${PATH:-}:" in
  *":${BIN_DIR}:"*) ;;
  *) echo "PATH hint: add $BIN_DIR to PATH" ;;
esac

write_output "version" "$VERSION"
write_output "tag" "$TAG"
write_output "path" "$DEST_PATH"
write_output "asset" "$ASSET_NAME"
write_output "sha256" "$ACTUAL_SUM"

if [ "$#" -gt 0 ]; then
  "$DEST_PATH" "$@"
fi
