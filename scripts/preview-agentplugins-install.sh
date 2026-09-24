#!/usr/bin/env bash

# Build the current checkout and preview a first-time interactive install in a
# fresh, isolated user profile. Keep the sandbox so the result can be inspected.
set -euo pipefail

if [[ $# -ne 1 || -z "$1" ]]; then
  printf 'Usage: bash scripts/preview-agentplugins-install.sh SOURCE\n' >&2
  exit 2
fi

source_ref=$1
if [[ ( "$source_ref" == ./* || "$source_ref" == ../* ) && -e "$source_ref" ]]; then
  source_ref="$(cd "$(dirname "$source_ref")" && pwd)/$(basename "$source_ref")"
fi
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
temp_base=${TMPDIR:-/tmp}
temp_base=${temp_base%/}
if [[ -z "$temp_base" ]]; then temp_base=/tmp; fi
sandbox_root=$(mktemp -d "$temp_base/uap-install-preview.XXXXXX")
test_home="$sandbox_root/home"
mkdir -p "$test_home/.gemini" "$test_home/.config/opencode" "$sandbox_root/project"

printf 'Building current source in %s\n' "$repo_root"
(cd "$repo_root" && go build -trimpath -ldflags='-X main.version=0.1.72-dev' -o "$sandbox_root/agentplugins" ./cli/cmd/agentplugins)

printf 'Fresh test profile: %s\n' "$sandbox_root"
printf 'The agent chooser is interactive. Only this test profile will receive configuration.\n\n'
cd "$sandbox_root/project"
preview_env=(
  PATH="$PATH"
  HOME="$test_home"
  XDG_CONFIG_HOME="$test_home/.config"
  XDG_DATA_HOME="$test_home/.local/share"
  GEMINI_CLI_HOME="$test_home"
  AGENTPLUGINS_HOME="$sandbox_root/state"
  TERM="${TERM:-xterm-256color}"
  LANG="${LANG:-en_US.UTF-8}"
)
env -i "${preview_env[@]}" "$sandbox_root/agentplugins" add "$source_ref"
printf '\nFinal read-only status:\n'
env -i "${preview_env[@]}" "$sandbox_root/agentplugins" doctor
