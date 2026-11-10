#!/usr/bin/env bash
# Native E2E: build from this checkout, invoke real agy, and verify that a
# project installation never mutates the user's Antigravity plugin directory.
set -euo pipefail

: "${AGY_BIN:?Set AGY_BIN to the pinned real Antigravity CLI executable}"
: "${UAP_BIN:?Set UAP_BIN to the UAP CLI built from the current PR}"
test -x "$AGY_BIN" || { echo "agy is unavailable: cannot qualify native E2E" >&2; exit 1; }
test -x "$UAP_BIN" || { echo "UAP binary is unavailable" >&2; exit 1; }

tmp="$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/uap-antigravity-native.XXXXXXXX")"
trap 'rm -rf "$tmp"' EXIT
export HOME="$tmp/home"
export XDG_CONFIG_HOME="$tmp/config"
export XDG_CACHE_HOME="$tmp/cache"
ln -s "$AGY_BIN" "$tmp/agy"
export PATH="$tmp:$(dirname "$AGY_BIN"):$PATH"
mkdir -p "$HOME" "$XDG_CONFIG_HOME" "$XDG_CACHE_HOME"
"$AGY_BIN" --version

source="$tmp/source"
project="$tmp/project"
mkdir -p "$source/skills/native-smoke" "$project" "$HOME/.gemini/config/plugins"
cat > "$source/plugin.json" <<'JSON'
{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "uap-agy-native-smoke",
  "version": "1.0.0",
  "description": "Isolated real Antigravity native plugin smoke"
}
JSON
cat > "$source/skills/native-smoke/SKILL.md" <<'MD'
---
name: native-smoke
description: Runs a disposable native plugin validation smoke.
---
Do not execute commands or connect to external services.
MD
foreign="$HOME/.gemini/config/plugins/existing-foreign-plugin"
mkdir -p "$foreign"
printf 'foreign must survive\n' > "$foreign/sentinel"

# The public CLI command must create UAP's exact-owned project-native layout.
# --no-save only skips the portable dependency manifest, never ownership.
"$UAP_BIN" add "$source" \
  --target antigravity --scope project --project-root "$project" \
  --no-save --yes --format json > "$tmp/uap-add.json"
python3 - "$tmp/uap-add.json" <<'PY'
import json,sys
with open(sys.argv[1],encoding="utf-8") as stream:
    result=json.load(stream)
assert result, "UAP emitted no structured result"
PY

native="$project/.agents/plugins"
test -d "$native" || { echo "UAP did not create native project plugin root" >&2; exit 1; }
mapfile -t paths < <(find "$native" -mindepth 1 -maxdepth 1 -type d ! -name '.agentplugins-staging-*' -print)
test "${#paths[@]}" -eq 1 || { echo "Expected one owned plugin, got ${#paths[@]}" >&2; exit 1; }
plugin="${paths[0]}"
test -f "$plugin/.antigravity-plugin/plugin.json"
test -f "$plugin/skills/native-smoke/SKILL.md"

# Only this native executable can establish vendor format compatibility.
if ! "$AGY_BIN" plugin validate --strict "$plugin" > "$tmp/agy-validation.txt" 2>&1; then
  cat "$tmp/agy-validation.txt" >&2
  echo "Pinned agy rejected the native plugin in strict mode" >&2
  exit 1
fi
cd "$project"
"$AGY_BIN" plugin list --json > "$tmp/agy-list.json"
python3 - "$tmp/agy-list.json" <<'PY'
import json,sys
with open(sys.argv[1],encoding="utf-8") as stream:
    doc=json.load(stream)
assert "uap-agy-native-smoke" in json.dumps(doc), (
    "Native agy did not discover the generated project plugin"
)
PY
test "$(cat "$foreign/sentinel")" = "foreign must survive"
test ! -e "$HOME/.gemini/config/plugins/uap-agy-native-smoke"

# A normal UAP remove must clean only its scoped owner. No vendor uninstall
# call may run, and the existing global plugin must remain untouched.
"$UAP_BIN" remove uap-agy-native-smoke \
  --target antigravity --scope project --project-root "$project" \
  --yes --format json > "$tmp/uap-remove.json"
test ! -e "$plugin" || { echo "Owned project plugin survived remove" >&2; exit 1; }
test "$(cat "$foreign/sentinel")" = "foreign must survive"
echo "Antigravity native discovery, strict validation and project removal passed."
