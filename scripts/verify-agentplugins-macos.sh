#!/usr/bin/env bash
# Run only on a separate, secret-free runner after signing and notarization.
set -euo pipefail

[[ $# == 2 && "$1" =~ ^agentplugins-v[0-9]+\.[0-9]+\.[0-9]+$ ]] || exit 1
version="${1#agentplugins-v}"
directory="$2"
for target in amd64 arm64; do
  case "$target" in
    amd64) expected="${SIGNED_DARWIN_AMD64_SHA256:-}" ;;
    arm64) expected="${SIGNED_DARWIN_ARM64_SHA256:-}" ;;
  esac
  file="${directory}/agentplugins_${version}_darwin_${target}"
  [[ "$expected" =~ ^[0-9a-f]{64}$ && -f "$file" && ! -L "$file" ]] || exit 1
  [[ "$(shasum -a 256 "$file" | cut -d ' ' -f 1)" == "$expected" ]] || exit 1
done
[[ "$(uname -m)" == arm64 ]] || { echo 'Expected Apple silicon runner' >&2; exit 1; }
work="$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/agentplugins-verify.XXXXXX")"
work="$(cd "$work" && pwd -P)"
trap 'rm -rf "$work"' EXIT
mkdir "$work/home"
cp "$directory/agentplugins_${version}_darwin_arm64" "$work/agentplugins"
chmod 0755 "$work/agentplugins"
xattr -w com.apple.quarantine "0081;$(printf '%x' "$(date +%s)");agentplugins-signing;" "$work/agentplugins"
xattr -p com.apple.quarantine "$work/agentplugins" > /dev/null
spctl --status || true
for attempt in 1 2 3 4 5; do
  if observed="$(env -i HOME="$work/home" PATH=/usr/bin:/bin "$work/agentplugins" version 2>&1)" &&
    [[ "$observed" == "agentplugins $version" ]]; then
    echo "Quarantined signed binary ran: $observed"
    exit 0
  fi
  printf '%s\n' "$observed" >&2
  [[ "$attempt" -lt 5 ]] || exit 1
  sleep $((attempt * 30))
done
