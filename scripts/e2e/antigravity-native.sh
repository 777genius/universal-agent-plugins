#!/usr/bin/env bash
# Native E2E: build from this checkout, invoke real agy, and verify that a
# project installation never mutates the user's Antigravity plugin directory.
set -euo pipefail

: "${AGY_BIN:?Set AGY_BIN to the pinned real Antigravity CLI executable}"
: "${UAP_BIN:?Set UAP_BIN to the UAP CLI built from the current PR}"
test -x "$AGY_BIN" || { echo "agy is unavailable: cannot qualify native E2E" >&2; exit 1; }
test -x "$UAP_BIN" || { echo "UAP binary is unavailable" >&2; exit 1; }

tree_snapshot() {
  python3 - "$1" <<'PY'
import hashlib,json,sys
from pathlib import Path
root=Path(sys.argv[1])
entries={}
for entry in sorted(root.rglob("*")):
    assert not entry.is_symlink(), "Unexpected symlink in native fixture"
    entries[entry.relative_to(root).as_posix()] = (
        "directory" if entry.is_dir() else hashlib.sha256(entry.read_bytes()).hexdigest()
    )
print(json.dumps(entries,sort_keys=True))
PY
}

assert_native_mcp() {
  python3 - "$1" "$tmp" <<'PY'
import json,sys
from pathlib import Path
plugin,fixture=map(Path,sys.argv[1:])
config=json.loads((plugin/"mcp_config.json").read_text(encoding="utf-8"))
servers=config.get("mcpServers",{})
assert set(servers)=={"native-stdio","native-http"}, "Selected MCP servers differ"
stdio=servers["native-stdio"]
assert stdio.get("command")=="node" and not {"type","url","serverUrl"}.intersection(stdio)
assert Path(stdio["cwd"]).resolve()==plugin.resolve(), "MCP cwd retained staging path"
assert Path(stdio["env"]["PLUGIN_ROOT"]).resolve()==plugin.resolve()
data=Path(stdio["env"]["PLUGIN_DATA"])
assert data.is_absolute() and data.is_dir() and fixture.resolve() in data.resolve().parents
assert stdio["env"]["UAP_SMOKE_DATA"]==str(data), "MCP data variable was not expanded"
args=stdio.get("args",[])
assert len(args)==3 and Path(args[0]).resolve()==(plugin/"scripts"/"mcp-smoke.cjs").resolve()
assert args[1]=="--data-dir" and Path(args[2]).resolve()==data.resolve()
assert servers["native-http"]=={
    "serverUrl":"https://example.invalid/uap-native-smoke",
    "headers":{"X-UAP-Smoke":"native-e2e"},
}, "Remote URL or headers were not projected exactly"
assert "${PLUGIN_" not in json.dumps(config), "Unexpanded managed MCP placeholder"
PY
}

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
mkdir -p "$source/scripts"
cat > "$source/scripts/mcp-smoke.cjs" <<'JS'
// Native validation fixture; no network access or model calls.
const lines = require('node:readline').createInterface({ input: process.stdin });
lines.on('line', line => {
  const request = JSON.parse(line);
  if (request.id === undefined) return;
  const result = request.method === 'initialize'
    ? { protocolVersion: request.params.protocolVersion, capabilities: { tools: {} },
        serverInfo: { name: 'uap-native-smoke', version: '1.0.0' } }
    : { tools: [] };
  process.stdout.write(JSON.stringify({ jsonrpc: '2.0', id: request.id, result }) + '\n');
});
JS
chmod 755 "$source/scripts/mcp-smoke.cjs"
cat > "$source/mcp.json" <<'JSON'
{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
  "mcpServers": {
    "native-stdio": {
      "type": "stdio",
      "command": "node",
      "args": ["${PLUGIN_ROOT}/scripts/mcp-smoke.cjs", "--data-dir", "${PLUGIN_DATA}"],
      "cwd": "${PLUGIN_ROOT}",
      "env": {"UAP_SMOKE_DATA": "${PLUGIN_DATA}"}
    },
    "native-http": {
      "type": "streamable-http",
      "url": "https://example.invalid/uap-native-smoke",
      "headers": {"X-UAP-Smoke": "native-e2e"}
    }
  }
}
JSON
source_snapshot="$(tree_snapshot "$source")"
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
test -f "$plugin/plugin.json"
test ! -e "$plugin/.antigravity-plugin" || { echo "Duplicate native manifest remained" >&2; exit 1; }
test -f "$plugin/skills/native-smoke/SKILL.md"
assert_native_mcp "$plugin"

# Exercise public no-save update and both repair ownership boundaries before
# native validation: partial drift is refused; an absent exact-owned bundle
# is restored from its recorded revision. Neither may affect foreign state.
baseline_manifest="$(sha256sum "$plugin/plugin.json" | cut -d' ' -f1)"
baseline_tree="$(tree_snapshot "$plugin")"
"$UAP_BIN" update uap-agy-native-smoke --target antigravity \
  --scope project --project-root "$project" --yes --format json > "$tmp/uap-project-update.json"
python3 -m json.tool "$tmp/uap-project-update.json" > /dev/null
test "$(sha256sum "$plugin/plugin.json" | cut -d' ' -f1)" = "$baseline_manifest"
rm "$plugin/skills/native-smoke/SKILL.md"
damaged_tree="$(tree_snapshot "$plugin")"
if "$UAP_BIN" repair uap-agy-native-smoke --target antigravity \
  --scope project --project-root "$project" --yes --format json \
  > "$tmp/uap-partial-repair.json" 2> "$tmp/uap-partial-repair.err"; then
  echo "Repair silently replaced a partially changed native package" >&2
  exit 1
fi
grep -Fq 'native identity ownership is indeterminate; refusing repair' \
  "$tmp/uap-partial-repair.err" "$tmp/uap-partial-repair.json"
test "$(tree_snapshot "$plugin")" = "$damaged_tree"
test "$(tree_snapshot "$source")" = "$source_snapshot"
test "$(cat "$foreign/sentinel")" = "foreign must survive"
# Delete only the disposable fixture's exact-owned complete native bundle.
# Keep its state receipt so public repair must recover the recorded revision.
test "$(dirname "$plugin")" = "$native"
rm -rf "$plugin"
"$UAP_BIN" repair uap-agy-native-smoke --target antigravity \
  --scope project --project-root "$project" --yes --format json > "$tmp/uap-project-repair.json"
python3 -m json.tool "$tmp/uap-project-repair.json" > /dev/null
cmp "$source/skills/native-smoke/SKILL.md" "$plugin/skills/native-smoke/SKILL.md"
test "$(sha256sum "$plugin/plugin.json" | cut -d' ' -f1)" = "$baseline_manifest"
test "$(tree_snapshot "$plugin")" = "$baseline_tree"
assert_native_mcp "$plugin"
test "$(cat "$foreign/sentinel")" = "foreign must survive"

# Only this native executable can establish vendor format compatibility.
if ! "$AGY_BIN" plugin validate --strict "$plugin" > "$tmp/agy-validation.txt" 2>&1; then
  cat "$tmp/agy-validation.txt" >&2
  echo "Pinned agy rejected the native plugin in strict mode" >&2
  exit 1
fi
# The IDE discovers project .agents/plugins directories. In contrast,
# "agy plugin list" lists CLI-installed packages, not arbitrary workspace
# directories. Test native CLI import/list independently in a second disposable
# profile; never confuse this with project activation or mutate the UAP profile.
vendor_home="$tmp/vendor-cli-home"
vendor_config="$tmp/vendor-cli-config"
vendor_cache="$tmp/vendor-cli-cache"
mkdir -p "$vendor_home" "$vendor_config" "$vendor_cache"
native_digest="$(sha256sum "$plugin/plugin.json" | cut -d' ' -f1)"
if ! HOME="$vendor_home" XDG_CONFIG_HOME="$vendor_config" XDG_CACHE_HOME="$vendor_cache" \
    "$AGY_BIN" plugin install "$plugin" > "$tmp/agy-install.txt" 2>&1; then
  cat "$tmp/agy-install.txt" >&2
  echo "Native CLI local import did not accept the strictly validated plugin" >&2
  exit 1
fi
HOME="$vendor_home" XDG_CONFIG_HOME="$vendor_config" XDG_CACHE_HOME="$vendor_cache" \
  "$AGY_BIN" plugin list --json > "$tmp/agy-list.json"
python3 - "$tmp/agy-list.json" <<'PY'
import json,sys
with open(sys.argv[1],encoding="utf-8") as stream:
    doc=json.load(stream)
assert isinstance(doc,list) and any(
    isinstance(entry,dict) and entry.get("name")=="uap-agy-native-smoke"
    for entry in doc
), "Native CLI did not list its explicitly installed local test plugin"
PY
if ! HOME="$vendor_home" XDG_CONFIG_HOME="$vendor_config" XDG_CACHE_HOME="$vendor_cache" \
    "$AGY_BIN" plugin uninstall uap-agy-native-smoke > "$tmp/agy-uninstall.txt" 2>&1; then
  cat "$tmp/agy-uninstall.txt" >&2
  echo "Native CLI failed to uninstall its disposable imported plugin" >&2
  exit 1
fi
test "$(sha256sum "$plugin/plugin.json" | cut -d' ' -f1)" = "$native_digest"
test "$(cat "$foreign/sentinel")" = "foreign must survive"
test ! -e "$HOME/.gemini/config/plugins/uap-agy-native-smoke"

# A separate disposable UAP User profile must discover and remove its own
# native plugin without changing the previously installed Project instance.
(
  export HOME="$tmp/native-user-home"
  export XDG_CONFIG_HOME="$tmp/native-user-config"
  export XDG_CACHE_HOME="$tmp/native-user-cache"
  mkdir -p "$HOME" "$XDG_CONFIG_HOME" "$XDG_CACHE_HOME" "$tmp/independent-project"
  cd "$tmp/independent-project"
  "$UAP_BIN" add "$source" --target antigravity --scope user \
    --no-save --yes --format json > "$tmp/uap-user-add.json"
  user_native="$HOME/.gemini/config/plugins"
  test -d "$user_native"
  mapfile -t user_paths < <(find "$user_native" -mindepth 1 -maxdepth 1 -type d ! -name '.agentplugins-staging-*' -print)
  test "${#user_paths[@]}" -eq 1
  user_plugin="${user_paths[0]}"
  test -f "$user_plugin/plugin.json"
  test -f "$user_plugin/skills/native-smoke/SKILL.md"
  assert_native_mcp "$user_plugin"
  "$AGY_BIN" plugin validate --strict "$user_plugin" > "$tmp/agy-user-validation.txt"
  "$AGY_BIN" plugin list --json > "$tmp/agy-user-list.json"
  python3 - "$tmp/agy-user-list.json" <<'PY'
import json,sys
with open(sys.argv[1],encoding="utf-8") as stream:
    doc=json.load(stream)
# Vendor registration is separate from UAP's owned prepared User bundle.
# This observation is not proof of runtime enablement.
print("Pinned agy User registration observed:",
      "uap-agy-native-smoke" in json.dumps(doc))
PY
  "$UAP_BIN" remove uap-agy-native-smoke --target antigravity \
    --scope user --yes --format json > "$tmp/uap-user-remove.json"
  test ! -e "$user_plugin" || { echo "Owned User plugin survived remove" >&2; exit 1; }
)
test -d "$plugin" || { echo "User scope removed Project plugin" >&2; exit 1; }

# A normal UAP remove must clean only its scoped owner. No vendor uninstall
# call may run, and the existing global plugin must remain untouched.
"$UAP_BIN" remove uap-agy-native-smoke \
  --target antigravity --scope project --project-root "$project" \
  --yes --format json > "$tmp/uap-remove.json"
test ! -e "$plugin" || { echo "Owned project plugin survived remove" >&2; exit 1; }
test "$(cat "$foreign/sentinel")" = "foreign must survive"
test "$(tree_snapshot "$source")" = "$source_snapshot"
echo "PASS: real agy strict Skills + stdio/HTTP MCP validation; separate native CLI import/list/uninstall; User/Project add/remove, update, partial-drift refusal and exact absent-bundle repair. IDE activation and live MCP execution were not observed."
