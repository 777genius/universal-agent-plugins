#!/usr/bin/env bash
# Full installer-to-real-Hermes local Skills/MCP smoke; no model/API login.
set -euo pipefail
: "${HERMES_BIN:?Set HERMES_BIN to the real pinned Hermes launcher}"
: "${UAP_BIN:?Set UAP_BIN to the built UAP CLI}"
: "${RUNNER_TEMP:?Use an isolated native CI runner}"
: "${HERMES_HOME:?Choose an isolated Hermes profile}"
case "$HERMES_HOME" in "$RUNNER_TEMP"/*) ;; *) echo "Hermes native test must run in isolated RUNNER_TEMP" >&2; exit 1 ;; esac
test -x "$HERMES_BIN" && test -x "$UAP_BIN" || { echo "Missing native binary: no E2E qualification" >&2; exit 1; }
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if [[ "${1:-}" != --lifecycle ]]; then
  exec python3 "$script_dir/hermes-native.py" run "$script_dir/hermes-native.sh"
fi
: "${HERMES_NATIVE_MCP_URL:?Run through the loopback fixture wrapper}"
: "${HERMES_NATIVE_EVIDENCE:?Missing evidence directory}"
export PATH="$(dirname "$HERMES_BIN"):$PATH"
tmp="$(mktemp -d "$RUNNER_TEMP/hermes-uap-native.XXXXXXXX")"
trap 'rm -rf "$tmp"' EXIT
"$HERMES_BIN" --version

global_src="$tmp/global-src"
project_src="$tmp/project-src"
project="$tmp/project"
project_b="$tmp/project-b"
mkdir -p "$global_src/skills/native-smoke-user" "$project_src/skills/native-smoke-project" "$project" "$project_b" "$HERMES_HOME"
git -C "$project" init -q
git -C "$project_b" init -q
observe() {
  local stage="$1" cwd="$2" marker="$3" project_count="$4"
  (cd "$cwd" && TERMINAL_CWD="$cwd" python3 \
    "$script_dir/hermes-native.py" observe "$marker" "$project_count") \
    | tee "$HERMES_NATIVE_EVIDENCE/$stage-skills.json"
}
probe_mcp() {
  "$HERMES_BIN" mcp test uap-native-test-server \
    > "$HERMES_NATIVE_EVIDENCE/$1-mcp.txt"
}
run_uap() {
  local stage="$1"
  shift
  "$UAP_BIN" "$@" --target hermes --yes --format json \
    > "$HERMES_NATIVE_EVIDENCE/$stage-uap.json"
}
advance_package() {
  python3 - "$1" "$2" <<'PY'
import json, pathlib, sys
root, scope = pathlib.Path(sys.argv[1]), sys.argv[2]
manifest = root / "plugin.json"
value = json.loads(manifest.read_text())
value["version"] = "1.1.0"
manifest.write_text(json.dumps(value) + "\n")
skill = root / f"skills/native-smoke-{scope}/SKILL.md"
prefix = "UAP_NATIVE_" + scope.upper()
skill.write_text(skill.read_text().replace(prefix + "_V1", prefix + "_V2"))
PY
}
cat > "$global_src/plugin.json" <<'JSON'
{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"hermes-uap-native-user","version":"1.0.0","description":"Disposable Hermes native User MCP smoke"}
JSON
cat > "$global_src/skills/native-smoke-user/SKILL.md" <<'MD'
---
name: native-smoke-user
description: Validates Hermes User Skills discovery without model access.
---
No model or external tool call is required.
UAP_NATIVE_USER_V1
MD
python3 - "$global_src/mcp.json" <<'PY'
import json, os, sys
with open(sys.argv[1], "w") as out:
    json.dump({"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
               "mcpServers": {"uap-native-test-server": {"type": "streamable-http",
                              "url": os.environ["HERMES_NATIVE_MCP_URL"]}}}, out)
PY
cat > "$project_src/plugin.json" <<'JSON'
{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"hermes-uap-native-project","version":"1.0.0","description":"Disposable Hermes Project Skills smoke"}
JSON
cat > "$project_src/skills/native-smoke-project/SKILL.md" <<'MD'
---
name: native-smoke-project
description: Validates Hermes project-scoped Skills in the selected Git root.
---
Wait for explicit Hermes project trust; no automatic trust changes.
UAP_NATIVE_PROJECT_V1
MD
mkdir -p "$HERMES_HOME/skills/native-smoke-foreign"
cat > "$HERMES_HOME/skills/native-smoke-foreign/SKILL.md" <<'MD'
---
name: native-smoke-foreign
description: Retained foreign Skill in a disposable Hermes profile.
---
Do not change this foreign Skill.
MD
cp "$HERMES_HOME/skills/native-smoke-foreign/SKILL.md" "$tmp/foreign-skill.md"

# This is a disposable test profile, not the user's pre-existing config.
cat > "$HERMES_HOME/config.yaml" <<'YAML'
model: offline-native-smoke
mcp_servers:
  foreign:
    command: echo
    args: [foreign]
YAML
chmod 600 "$HERMES_HOME/config.yaml"

run_uap user-add add "$global_src" --scope user --no-save
observe user-add "$tmp" UAP_NATIVE_USER_V1 0
probe_mcp user-add
"$HERMES_BIN" skills list --source local --enabled-only \
  > "$HERMES_NATIVE_EVIDENCE/user-cli-skills.txt"
grep -F native-smoke-user "$HERMES_NATIVE_EVIDENCE/user-cli-skills.txt" >/dev/null
"$HERMES_BIN" mcp list > "$tmp/hermes-mcp-list.txt"
grep -F 'uap-native-test-server' "$tmp/hermes-mcp-list.txt" >/dev/null
grep -F 'foreign' "$HERMES_HOME/config.yaml" >/dev/null

run_uap user-repeat add "$global_src" --scope user --no-save
observe user-repeat "$tmp" UAP_NATIVE_USER_V1 0
probe_mcp user-repeat
advance_package "$global_src" user
run_uap user-update update hermes-uap-native-user --scope user
observe user-update "$tmp" UAP_NATIVE_USER_V2 0
probe_mcp user-update
rm "$HERMES_HOME/skills/native-smoke-user/SKILL.md"
rmdir "$HERMES_HOME/skills/native-smoke-user"
observe user-missing "$tmp" absent 0
run_uap user-repair repair hermes-uap-native-user --scope user
observe user-repair "$tmp" UAP_NATIVE_USER_V2 0
probe_mcp user-repair

# All project operations must preserve User YAML byte-for-byte, including trust.
cp "$HERMES_HOME/config.yaml" "$tmp/user-before-projects.yaml"
run_uap project-a-add add "$project_src" --scope project --project-root "$project" --no-save
run_uap project-b-add add "$project_src" --scope project --project-root "$project_b" --no-save
run_uap project-a-repeat add "$project_src" --scope project --project-root "$project" --no-save
observe project-a-add "$project" UAP_NATIVE_USER_V2 1
observe project-b-add "$project_b" UAP_NATIVE_USER_V2 1
cp "$project_b/.hermes/skills/native-smoke-project/SKILL.md" "$tmp/project-b-skill.md"
advance_package "$project_src" project
run_uap project-a-update update hermes-uap-native-project --scope project --project-root "$project"
grep -F UAP_NATIVE_PROJECT_V2 "$project/.hermes/skills/native-smoke-project/SKILL.md" >/dev/null
cmp "$tmp/project-b-skill.md" "$project_b/.hermes/skills/native-smoke-project/SKILL.md"
rm "$project/.hermes/skills/native-smoke-project/SKILL.md"
rmdir "$project/.hermes/skills/native-smoke-project"
observe project-a-missing "$project" UAP_NATIVE_USER_V2 0
run_uap project-a-repair repair hermes-uap-native-project --scope project --project-root "$project"
observe project-a-repair "$project" UAP_NATIVE_USER_V2 1
cmp "$project_src/skills/native-smoke-project/SKILL.md" "$project/.hermes/skills/native-smoke-project/SKILL.md"
cmp "$tmp/project-b-skill.md" "$project_b/.hermes/skills/native-smoke-project/SKILL.md"

# A Project request with a required MCP component must never mutate User YAML.
before="$(sha256sum "$HERMES_HOME/config.yaml" | cut -d' ' -f1)"
if "$UAP_BIN" add "$global_src" --target hermes --scope project \
  --project-root "$project" --no-save --yes --format json \
  > "$HERMES_NATIVE_EVIDENCE/project-mcp-refusal.json" \
  2> "$HERMES_NATIVE_EVIDENCE/project-mcp-refusal-stderr.txt"; then
  echo "Hermes Project MCP was incorrectly installed" >&2
  exit 1
fi
python3 "$script_dir/hermes-native.py" refusal "$HERMES_NATIVE_EVIDENCE/project-mcp-refusal.json"
test "$before" = "$(sha256sum "$HERMES_HOME/config.yaml" | cut -d' ' -f1)"

run_uap project-a-remove remove hermes-uap-native-project --scope project --project-root "$project"
observe project-a-remove "$project" UAP_NATIVE_USER_V2 0
observe project-b-retained "$project_b" UAP_NATIVE_USER_V2 1
cmp "$tmp/project-b-skill.md" "$project_b/.hermes/skills/native-smoke-project/SKILL.md"
run_uap project-b-remove remove hermes-uap-native-project --scope project --project-root "$project_b"
observe project-b-remove "$project_b" UAP_NATIVE_USER_V2 0
cmp "$tmp/user-before-projects.yaml" "$HERMES_HOME/config.yaml"

run_uap user-remove remove hermes-uap-native-user --scope user
observe user-remove "$tmp" absent 0
grep -F 'foreign' "$HERMES_HOME/config.yaml" >/dev/null
! grep -F 'uap-native-test-server' "$HERMES_HOME/config.yaml"
"$HERMES_BIN" mcp list > "$HERMES_NATIVE_EVIDENCE/removed-mcp-list.txt"
! grep -F uap-native-test-server "$HERMES_NATIVE_EVIDENCE/removed-mcp-list.txt"
cmp "$tmp/foreign-skill.md" "$HERMES_HOME/skills/native-smoke-foreign/SKILL.md"
echo "Hermes native User lifecycle, Project A/B isolation and trust boundary passed."
