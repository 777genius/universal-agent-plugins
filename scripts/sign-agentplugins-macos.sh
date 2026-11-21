#!/usr/bin/env bash
# Developer ID signing and notarization of the two native macOS agentplugins
# release binaries. The sign-macos job of agentplugins-release.yml runs it; the
# same script is the one-time local identity check in docs/agentplugins-release.md.
#
# Usage: sign-agentplugins-macos.sh <agentplugins-vX.Y.Z> <binary-dir> <evidence-dir>
#
# <binary-dir> must contain exactly agentplugins_X.Y.Z_darwin_amd64 and
# agentplugins_X.Y.Z_darwin_arm64; both are signed in place. Required
# environment: APPLE_CERTIFICATE (base64 .p12), APPLE_CERTIFICATE_PASSWORD,
# APPLE_TEAM_ID, APPLE_API_KEY_BASE64 (base64 App Store Connect .p8 key),
# APPLE_API_KEY_ID and APPLE_API_ISSUER. Secrets are never printed. The
# certificate is imported only into a throwaway keychain with a random
# password; that keychain and every decoded key file are removed on exit.
set +x
set -euo pipefail
umask 077

# Public, not secret: the release team and the code-signing identifier.
readonly EXPECTED_TEAM_ID=86399583GS
readonly SIGNING_IDENTIFIER=com.777genius.agentplugins
readonly NOTARY_WAIT_TIMEOUT=30m
readonly TARGETS="darwin_amd64 darwin_arm64"

fail() {
  echo "macOS signing: $*" >&2
  exit 1
}

[[ $# -eq 3 ]] || fail "usage: sign-agentplugins-macos.sh <agentplugins-vX.Y.Z> <binary-dir> <evidence-dir>"
tag="$1"
binary_dir="$2"
evidence_dir="$3"
[[ "${tag}" =~ ^agentplugins-v[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "invalid release tag: ${tag}"
version="${tag#agentplugins-v}"
for name in APPLE_CERTIFICATE APPLE_CERTIFICATE_PASSWORD APPLE_TEAM_ID APPLE_API_KEY_BASE64 APPLE_API_KEY_ID APPLE_API_ISSUER; do
  [[ -n "${!name:-}" ]] || fail "missing ${name}"
done
[[ "${APPLE_TEAM_ID}" == "${EXPECTED_TEAM_ID}" ]] || fail "APPLE_TEAM_ID is not the release team ${EXPECTED_TEAM_ID}"

[[ -d "${binary_dir}" && ! -L "${binary_dir}" ]] || fail "binary directory is missing: ${binary_dir}"
[[ "$(find "${binary_dir}" -mindepth 1 -maxdepth 1 | wc -l | tr -d ' ')" == 2 ]] ||
  fail "expected exactly the two macOS binaries in ${binary_dir}"
for target in ${TARGETS}; do
  file="${binary_dir}/agentplugins_${version}_${target}"
  [[ -f "${file}" && ! -L "${file}" ]] || fail "missing regular file agentplugins_${version}_${target}"
  case "${target}" in
    darwin_amd64) expected_arch=x86_64 ;;
    darwin_arm64) expected_arch=arm64 ;;
  esac
  [[ "$(lipo -archs "${file}")" == "${expected_arch}" ]] ||
    fail "agentplugins_${version}_${target} is not a thin ${expected_arch} Mach-O"
  # Artifact transfer drops the mode; the content is what gets signed and released.
  chmod 0755 "${file}"
done
mkdir -p "${evidence_dir}"

work="$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/agentplugins-macos-signing.XXXXXX")"
# The CLI refuses a HOME with symlinks or doubled slashes (macOS TMPDIR ends in a slash
# and /var is a symlink), so use the canonical physical path from here on.
work="$(cd "${work}" && pwd -P)"
keychain="${work}/signing.keychain-db"
keychain_created=false
original_keychains=()
keychain_listing="$(security list-keychains -d user)"
while IFS= read -r line; do
  line="$(sed -e 's/^[[:space:]]*"//' -e 's/"[[:space:]]*$//' <<< "${line}")"
  if [[ -n "${line}" ]]; then
    original_keychains+=("${line}")
  fi
done <<< "${keychain_listing}"

cleanup() {
  local status=$?
  set +e
  trap - EXIT
  if [[ "${keychain_created}" == true ]]; then
    security list-keychains -d user -s ${original_keychains[@]+"${original_keychains[@]}"}
    security delete-keychain "${keychain}" > /dev/null 2>&1 ||
      echo "macOS signing: could not delete throwaway keychain ${keychain}" >&2
  fi
  rm -rf "${work}"
  exit "${status}"
}
trap cleanup EXIT
trap 'exit 130' INT TERM HUP

# 1. Throwaway keychain holding only the Developer ID Application identity.
printf '%s' "${APPLE_CERTIFICATE}" | base64 --decode > "${work}/certificate.p12" ||
  fail "APPLE_CERTIFICATE is not valid base64"
[[ -s "${work}/certificate.p12" ]] || fail "APPLE_CERTIFICATE decodes to an empty file"
keychain_password="$(openssl rand -hex 32)"
keychain_created=true
security create-keychain -p "${keychain_password}" "${keychain}"
security set-keychain-settings -lut 21600 "${keychain}"
security unlock-keychain -p "${keychain_password}" "${keychain}"
security import "${work}/certificate.p12" -k "${keychain}" -f pkcs12 \
  -P "${APPLE_CERTIFICATE_PASSWORD}" -T /usr/bin/codesign > /dev/null
rm -f "${work}/certificate.p12"
security set-key-partition-list -S apple-tool:,apple:,codesign: -s \
  -k "${keychain_password}" "${keychain}" > /dev/null
security list-keychains -d user -s "${keychain}" ${original_keychains[@]+"${original_keychains[@]}"}

identity_pattern="^[[:space:]]*[0-9]+\) ([0-9A-F]{40}) \"(Developer ID Application: .+ \(${EXPECTED_TEAM_ID}\))\"$"
identity_hash=""
identity_name=""
matches=0
identity_listing="$(security find-identity -v -p codesigning "${keychain}")"
while IFS= read -r line; do
  if [[ "${line}" =~ ${identity_pattern} ]]; then
    matches=$((matches + 1))
    identity_hash="${BASH_REMATCH[1]}"
    identity_name="${BASH_REMATCH[2]}"
  fi
done <<< "${identity_listing}"
[[ "${matches}" == 1 ]] ||
  fail "expected exactly one valid Developer ID Application identity for team ${EXPECTED_TEAM_ID}, found ${matches}"
echo "Signing identity: ${identity_name} (${identity_hash})"

# 2. Sign with hardened runtime and a secure Apple timestamp. --force replaces
# the ad-hoc signature that the Go linker adds to darwin/arm64 binaries.
sign_binary() {
  local file="$1" attempt output
  for attempt in 1 2 3; do
    if output="$(codesign --force --options runtime --timestamp --identifier "${SIGNING_IDENTIFIER}" \
      --keychain "${keychain}" --sign "${identity_hash}" "${file}" 2>&1)"; then
      printf '%s\n' "${output}"
      return 0
    fi
    printf '%s\n' "${output}" >&2
    if [[ "${attempt}" == 3 ]] || ! grep -qi timestamp <<< "${output}"; then
      return 1
    fi
    echo "macOS signing: Apple timestamp service unavailable, retrying" >&2
    sleep $((attempt * 15))
  done
}

verify_signature() {
  local target="$1" file="$2" details cdhash
  codesign --verify --strict --verbose=2 "${file}"
  details="$(codesign --display --verbose=4 "${file}" 2>&1)"
  printf '%s\n' "${details}" > "${evidence_dir}/codesign-${target}.txt"
  grep -Fxq "Identifier=${SIGNING_IDENTIFIER}" <<< "${details}" || fail "${target}: unexpected signing identifier"
  grep -Fxq "Authority=${identity_name}" <<< "${details}" || fail "${target}: not signed by ${identity_name}"
  grep -Fxq "TeamIdentifier=${APPLE_TEAM_ID}" <<< "${details}" || fail "${target}: TeamIdentifier is not ${APPLE_TEAM_ID}"
  grep -Eq '^CodeDirectory .*flags=0x[0-9a-f]+\([^)]*runtime[^)]*\)' <<< "${details}" ||
    fail "${target}: hardened runtime flag is missing"
  grep -Eq '^Timestamp=.+' <<< "${details}" || fail "${target}: secure Apple timestamp is missing"
  cdhash="$(sed -n 's/^CDHash=\([0-9a-f]\{40\}\)$/\1/p' <<< "${details}")"
  [[ "${cdhash}" =~ ^[0-9a-f]{40}$ ]] || fail "${target}: CDHash is missing"
  printf -v "cdhash_${target}" '%s' "${cdhash}"
}

cdhash_darwin_amd64=""
cdhash_darwin_arm64=""
for target in ${TARGETS}; do
  file="${binary_dir}/agentplugins_${version}_${target}"
  sign_binary "${file}" || fail "${target}: codesign failed"
  verify_signature "${target}" "${file}"
done

# 3. Notarize one zip with both binaries. A bare Mach-O cannot be stapled, so the
# ticket stays with Apple and Gatekeeper fetches it online by CDHash.
payload="${work}/agentplugins_${version}_darwin"
mkdir "${payload}"
for target in ${TARGETS}; do
  cp "${binary_dir}/agentplugins_${version}_${target}" "${payload}/"
done
archive="${work}/agentplugins_${version}_darwin.zip"
ditto -c -k --keepParent "${payload}" "${archive}"
api_key="${work}/notary-key.p8"
printf '%s' "${APPLE_API_KEY_BASE64}" | base64 --decode > "${api_key}" ||
  fail "APPLE_API_KEY_BASE64 is not valid base64"
[[ -s "${api_key}" ]] || fail "APPLE_API_KEY_BASE64 decodes to an empty file"
notary_auth=(--key "${api_key}" --key-id "${APPLE_API_KEY_ID}" --issuer "${APPLE_API_ISSUER}")

json_field() {
  python3 -c 'import json, sys; value = json.load(open(sys.argv[1])).get(sys.argv[2]); print(value if isinstance(value, str) else "")' "$1" "$2"
}

submit_status=0
xcrun notarytool submit "${archive}" "${notary_auth[@]}" --wait --timeout "${NOTARY_WAIT_TIMEOUT}" \
  --output-format json > "${evidence_dir}/notary-submission.json" || submit_status=$?
cat "${evidence_dir}/notary-submission.json"
submission_id="$(json_field "${evidence_dir}/notary-submission.json" id 2> /dev/null || true)"
submission_state="$(json_field "${evidence_dir}/notary-submission.json" status 2> /dev/null || true)"
[[ "${submission_id}" =~ ^[0-9A-Fa-f-]{36}$ ]] || fail "notarytool returned no submission ID (exit ${submit_status})"
xcrun notarytool log "${notary_auth[@]}" "${submission_id}" "${evidence_dir}/notary-log.json" ||
  fail "could not fetch the notary log for ${submission_id}"
rm -f "${api_key}"
if [[ "${submit_status}" != 0 || "${submission_state}" != Accepted ]]; then
  cat "${evidence_dir}/notary-log.json" >&2
  fail "notarization ${submission_id} ended with status '${submission_state}' (exit ${submit_status})"
fi
python3 - "${evidence_dir}/notary-log.json" "${version}" "${cdhash_darwin_amd64}" "${cdhash_darwin_arm64}" << 'PY' ||
import json
import sys

log_path, version, amd64, arm64 = sys.argv[1:]
log = json.load(open(log_path))
if log.get("status") != "Accepted":
    sys.exit("notary log status is %r, not Accepted" % log.get("status"))
tickets = log.get("ticketContents") or []
for target, cdhash in (("darwin_amd64", amd64), ("darwin_arm64", arm64)):
    name = "agentplugins_%s_%s" % (version, target)
    if not any(str(ticket.get("path", "")).endswith("/" + name)
               and str(ticket.get("cdhash", "")).lower().startswith(cdhash)
               for ticket in tickets):
        sys.exit("notary ticket does not cover %s with CDHash %s" % (name, cdhash))
PY
  fail "notary log does not bind both signed binaries"

# 4. Bind the exact signed bytes for staging, which hashes and attests them.
sha_darwin_amd64="$(shasum -a 256 "${binary_dir}/agentplugins_${version}_darwin_amd64" | cut -d ' ' -f 1)"
sha_darwin_arm64="$(shasum -a 256 "${binary_dir}/agentplugins_${version}_darwin_arm64" | cut -d ' ' -f 1)"
python3 - "${evidence_dir}/signing-summary.json" "${tag}" "${identity_name}" "${APPLE_TEAM_ID}" \
  "${SIGNING_IDENTIFIER}" "${submission_id}" "${cdhash_darwin_amd64}" "${sha_darwin_amd64}" \
  "${cdhash_darwin_arm64}" "${sha_darwin_arm64}" << 'PY'
import json
import sys

path, tag, identity, team, identifier, submission, amd64_cd, amd64_sha, arm64_cd, arm64_sha = sys.argv[1:]
version = tag[len("agentplugins-v"):]
summary = {
    "schema_version": 1, "tag": tag, "identity": identity, "team_id": team,
    "identifier": identifier, "notary_submission_id": submission, "notary_status": "Accepted",
    "binaries": {
        "agentplugins_%s_darwin_amd64" % version: {"cdhash": amd64_cd, "sha256": amd64_sha},
        "agentplugins_%s_darwin_arm64" % version: {"cdhash": arm64_cd, "sha256": arm64_sha},
    },
}
with open(path, "x") as stream:
    json.dump(summary, stream, indent=2, sort_keys=True)
    stream.write("\n")
PY
{
  echo "darwin_amd64_sha256=${sha_darwin_amd64}"
  echo "darwin_arm64_sha256=${sha_darwin_arm64}"
} >> "${GITHUB_OUTPUT:-/dev/stdout}"
if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    echo "### macOS signing"
    echo
    echo "- Identity: ${identity_name}"
    echo "- Notary submission: ${submission_id} (Accepted)"
    echo "- agentplugins_${version}_darwin_amd64: sha256 ${sha_darwin_amd64}, CDHash ${cdhash_darwin_amd64}"
    echo "- agentplugins_${version}_darwin_arm64: sha256 ${sha_darwin_arm64}, CDHash ${cdhash_darwin_arm64}"
  } >> "${GITHUB_STEP_SUMMARY}"
fi
echo "Signed and notarized: agentplugins ${version} darwin amd64 and arm64"
