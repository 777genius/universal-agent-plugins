#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

STARTER="${STARTER:-}"
if [[ -z "$STARTER" ]]; then
  echo "set STARTER=<starter-name|all>" >&2
  exit 1
fi

PRIMARY_MAPPING_FILE="${STARTER_TEMPLATE_MAPPING_FILE:-$ROOT/examples/starters/template-repos.txt}"
RUNTIME_PACKAGE_MAPPING_FILE="${STARTER_TEMPLATE_RUNTIME_PACKAGE_MAPPING_FILE:-$ROOT/examples/starters/runtime-package-template-repos.txt}"
OWNER="${STARTER_TEMPLATE_REPO_OWNER:-777genius}"
REMOTE_BASE="${STARTER_TEMPLATE_REMOTE_BASE:-}"
TOKEN="$(printf '%s' "${STARTER_TEMPLATE_SYNC_TOKEN:-${GITHUB_TOKEN:-}}" | tr -d '\r\n')"

if [[ ! -f "$PRIMARY_MAPPING_FILE" ]]; then
  echo "mapping file not found: $PRIMARY_MAPPING_FILE" >&2
  exit 1
fi

if [[ -z "$REMOTE_BASE" && -z "$TOKEN" ]]; then
  echo "set STARTER_TEMPLATE_SYNC_TOKEN (or GITHUB_TOKEN) with push access to starter template repos" >&2
  exit 1
fi

lookup_repo() {
  local starter="$1"
  local repo=""
  repo="$(awk -v name="$starter" '$1 == name { print $2 }' "$PRIMARY_MAPPING_FILE")"
  if [[ -n "$repo" ]]; then
    printf '%s\n' "$repo"
    return 0
  fi
  if [[ -f "$RUNTIME_PACKAGE_MAPPING_FILE" ]]; then
    repo="$(awk -v name="$starter" '$1 == name { print $2 }' "$RUNTIME_PACKAGE_MAPPING_FILE")"
  fi
  printf '%s\n' "$repo"
}

sync_one() {
  local starter="$1"
  local repo=""
  repo="$(lookup_repo "$starter")"
  if [[ -z "$repo" ]]; then
    echo "unknown starter: $starter" >&2
    exit 1
  fi

  local source_dir="$ROOT/examples/starters/$starter"
  if [[ ! -d "$source_dir" ]]; then
    echo "starter source not found: $source_dir" >&2
    exit 1
  fi

  local remote_url=""
  if [[ -n "$REMOTE_BASE" ]]; then
    remote_url="${REMOTE_BASE%/}/${repo}.git"
  else
    remote_url="https://x-access-token:${TOKEN}@github.com/${OWNER}/${repo}.git"
  fi

  local tmp
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' RETURN

  git clone "$remote_url" "$tmp/repo"
  if git -C "$tmp/repo" show-ref --verify --quiet refs/remotes/origin/main; then
    git -C "$tmp/repo" checkout -B main origin/main >/dev/null 2>&1
  else
    git -C "$tmp/repo" checkout --orphan main >/dev/null 2>&1
  fi

  find "$tmp/repo" -mindepth 1 -maxdepth 1 ! -name '.git' -exec rm -rf {} +
  cp -R "$source_dir"/. "$tmp/repo"/

  pushd "$tmp/repo" >/dev/null
  git config user.name "plugin-kit-ai-bot"
  git config user.email "actions@users.noreply.github.com"
  git add -A
  if git diff --cached --quiet; then
    echo "starter template already up to date: ${starter} -> ${OWNER}/${repo}"
    popd >/dev/null
    return 0
  fi
  git commit -m "Sync ${starter} starter"
  git push origin HEAD:main
  popd >/dev/null
}

if [[ "$STARTER" == "all" ]]; then
  while read -r starter _repo; do
    [[ -n "$starter" ]] || continue
    sync_one "$starter"
  done < "$PRIMARY_MAPPING_FILE"
  exit 0
fi

if [[ "$STARTER" == "all-runtime-package" ]]; then
  if [[ ! -f "$RUNTIME_PACKAGE_MAPPING_FILE" ]]; then
    echo "runtime-package mapping file not found: $RUNTIME_PACKAGE_MAPPING_FILE" >&2
    exit 1
  fi
  while read -r starter _repo; do
    [[ -n "$starter" ]] || continue
    sync_one "$starter"
  done < "$RUNTIME_PACKAGE_MAPPING_FILE"
  exit 0
fi

sync_one "$STARTER"
