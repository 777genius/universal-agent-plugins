#!/usr/bin/env bash
# Full installer-to-real-Hermes local Skills/MCP smoke; no model/API login.
set -euo pipefail
: "${HERMES_BIN:?Set HERMES_BIN to the real pinned Hermes launcher}"
: "${UAP_BIN:?Set UAP_BIN to the built UAP CLI}"
: "${RUNNER_TEMP:?Use an isolated native CI runner}"
: "${HERMES_HOME:?Choose an isolated Hermes profile}"
case "$HERMES_HOME" in "$RUNNER_TEMP"/*) ;; *) echo "Hermes native test must run in isolated RUNNER_TEMP" >&2; exit 1 ;; esac
test -x "$HERMES_BIN" && test -x "$UAP_BIN" || { echo "Missing native binary: no E2E qualification" >&2; exit 1; }
export PATH="$(dirname "$HERMES_BIN"):$PATH"
tmp="$(mktemp -d "$RUNNER_TEMP/hermes-uap-native.XXXXXXXX")"
trap 'rm -rf "$tmp"' EXIT
"$HERMES_BIN" --version

global_src="$tmp/global-src"
project_src="$tmp/project-src"
project="$tmp/project"
mkdir -p "$global_src/skills/native-smoke-user" "$project_src/skills/native-smoke-project" "$project" "$HERMES_HOME"
git -C "$project" init -q
cat > "$global_src/plugin.json" <<'JSON'
{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"hermes-uap-native-user","version":"1.0.0","description":"Disposable Hermes native User MCP smoke"}
JSON
cat > "$global_src/skills/native-smoke-user/SKILL.md" <<'MD'
---
name: native-smoke-user
description: Validates Hermes User Skills discovery without model access.
---
No model or external tool call is required.
MD
cat > "$global_src/mcp.json" <<'JSON'
{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"uap-native-test-server":{"type":"streamable-http","url":"https://example.test/mcp"}}}
JSON
cat > "$project_src/plugin.json" <<'JSON'
{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"hermes-uap-native-project","version":"1.0.0","description":"Disposable Hermes Project Skills smoke"}
JSON
cat > "$project_src/skills/native-smoke-project/SKILL.md" <<'MD'
---
name: native-smoke-project
description: Validates Hermes project-scoped Skills in the selected Git root.
---
Wait for explicit Hermes project trust; no automatic trust changes.
MD

# This is a disposable test profile, not the user's pre-existing config.
cat > "$HERMES_HOME/config.yaml" <<'YAML'
model: offline-native-smoke
mcp_servers:
  foreign:
    command: echo
    args: [foreign]
YAML
chmod 600 "$HERMES_HOME/config.yaml"

"$UAP_BIN" add "$global_src" --target hermes --scope user \
  --no-save --yes --format json > "$tmp/uap-user-add.json"
test -f "$HERMES_HOME/skills/native-smoke-user/SKILL.md"
"$HERMES_BIN" mcp list > "$tmp/hermes-mcp-list.txt"
grep -F 'uap-native-test-server' "$tmp/hermes-mcp-list.txt" >/dev/null
grep -F 'foreign' "$HERMES_HOME/config.yaml" >/dev/null

"$UAP_BIN" add "$project_src" --target hermes --scope project \
  --project-root "$project" --no-save --yes --format json > "$tmp/uap-project-add.json"
test -f "$project/.hermes/skills/native-smoke-project/SKILL.md"

# A Project request with a required MCP component must never mutate User YAML.
before="$(sha256sum "$HERMES_HOME/config.yaml" | cut -d' ' -f1)"
if "$UAP_BIN" add "$global_src" --target hermes --scope project \
  --project-root "$project" --no-save --yes --format json \
  > "$tmp/uap-project-mcp-refusal.json" 2> "$tmp/uap-project-mcp-error.txt"; then
  echo "Hermes Project MCP was incorrectly installed" >&2
  exit 1
fi
test "$before" = "$(sha256sum "$HERMES_HOME/config.yaml" | cut -d' ' -f1)"

"$UAP_BIN" remove hermes-uap-native-project --target hermes \
  --scope project --project-root "$project" --yes --format json > "$tmp/uap-project-remove.json"
test ! -e "$project/.hermes/skills/native-smoke-project/SKILL.md"
test -f "$HERMES_HOME/skills/native-smoke-user/SKILL.md"

"$UAP_BIN" remove hermes-uap-native-user --target hermes \
  --scope user --yes --format json > "$tmp/uap-user-remove.json"
test ! -e "$HERMES_HOME/skills/native-smoke-user/SKILL.md"
grep -F 'foreign' "$HERMES_HOME/config.yaml" >/dev/null
! grep -F 'uap-native-test-server' "$HERMES_HOME/config.yaml"
echo "Hermes User Skills/MCP, project trust boundary and owned removal passed."
