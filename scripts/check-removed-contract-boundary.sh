#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

has_error=0

check_pattern() {
  local label="$1"
  local pattern="$2"
  shift 2

  local args=(
    -n
    --hidden
    --glob=!**/node_modules/**
    --glob=!**/.git/**
    --glob=!scripts/check-removed-contract-boundary.sh
  )

  for exclude in "$@"; do
    args+=("--glob=!${exclude}")
  done

  local matches
  local status
  if command -v rg >/dev/null 2>&1; then
    if matches="$(env RG_GUARD_DISABLE=1 RIPGREP_CONFIG_PATH= rg --no-config "${args[@]}" -- "${pattern}" .)"; then
      status=0
    else
      status=$?
    fi
  else
    local pathspecs=(
      .
      ':(exclude,glob)**/node_modules/**'
      ':(exclude,glob)**/.git/**'
      ':(exclude,glob)scripts/check-removed-contract-boundary.sh'
    )
    for exclude in "$@"; do
      pathspecs+=(":(exclude,glob)${exclude}")
    done
    if matches="$(git grep --untracked --exclude-standard -n -E -e "${pattern}" -- "${pathspecs[@]}")"; then
      status=0
    else
      status=$?
    fi
  fi
  if [[ "$status" -gt 1 ]]; then
    echo "removed-contract scan failed for ${label} with status ${status}" >&2
    exit 1
  fi
  if [[ -n "${matches}" ]]; then
    echo "forbidden ${label} references found:" >&2
    echo "${matches}" >&2
    has_error=1
  fi
}

check_pattern "removed Cursor rules import" '\.cursorrules' 'docs/research/**'
check_pattern "removed OpenCode env-config compatibility" 'OPENCODE_CONFIG(_DIR)?' 'install/integrationctl/**' 'docs/UNIVERSAL_INSTALL_UPDATE_CONTROL_PLANE_PLAN.md'
check_pattern "removed Gemini binary aliases" 'PLUGIN_KIT_AI_GEMINI_BIN|GEMINI_BIN'
check_pattern "Gemini migratedTo field outside research or runtime codec" 'migratedTo|migrated_to' 'docs/research/**' 'docs/UNIVERSAL_INSTALL_UPDATE_CONTROL_PLANE_PLAN.md' 'cli/internal/geminimanifest/**' 'cli/internal/platformexec/gemini*.go' 'cli/internal/validate/validate_test.go' 'install/integrationctl/**' 'repotests/plugin_manifest_lifecycle_integration_test.go' 'sdk/platformmeta/**'
check_pattern "deleted maintainer docs tree" 'maintainer-docs' 'website/tools/quality/check-output.mjs'
check_pattern "removed guide slug" 'migrate''-existing-config'

if [[ "${has_error}" -ne 0 ]]; then
  echo "removed-contract boundary check failed" >&2
  exit 1
fi

echo "removed-contract boundary intact"
