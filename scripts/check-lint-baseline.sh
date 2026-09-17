#!/usr/bin/env bash
# Enforces that the LEGACY SIZE BASELINE block in .golangci.yml only ever shrinks.
#
# The block exempts pre-existing oversized files from the size gate. Adding an
# entry would silently exempt new debt, so every entry present in HEAD must also
# be present in the base revision.
#
# Usage: scripts/check-lint-baseline.sh <base-ref>

set -euo pipefail

BASE_REF="${1:-origin/main}"
CONFIG=".golangci.yml"
BEGIN_MARKER="# BEGIN LEGACY SIZE BASELINE"
END_MARKER="# END LEGACY SIZE BASELINE"

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

extract_entries() {
  awk -v begin="$BEGIN_MARKER" -v end="$END_MARKER" '
    index($0, begin) { inside = 1; next }
    index($0, end)   { inside = 0; next }
    inside && $0 ~ /^[[:space:]]*-[[:space:]]*path:/ {
      sub(/^[[:space:]]*-[[:space:]]*path:[[:space:]]*/, "")
      print
    }
  ' | sort -u
}

if [ ! -f "$CONFIG" ]; then
  echo "check-lint-baseline: $CONFIG not found in the working tree" >&2
  exit 1
fi

head_entries="$(extract_entries <"$CONFIG")"

if ! git cat-file -e "$BASE_REF:$CONFIG" 2>/dev/null; then
  echo "check-lint-baseline: $CONFIG does not exist at $BASE_REF; nothing to compare against"
  echo "check-lint-baseline: HEAD baseline has $(printf '%s' "$head_entries" | grep -c . || true) entries"
  exit 0
fi

base_entries="$(git show "$BASE_REF:$CONFIG" | extract_entries)"

added="$(comm -13 <(printf '%s\n' "$base_entries") <(printf '%s\n' "$head_entries") || true)"

if [ -n "$added" ]; then
  echo "check-lint-baseline: the legacy size baseline may only shrink, but these entries were added:" >&2
  printf '  %s\n' $added >&2
  echo >&2
  echo "Split the file or reduce its complexity instead of exempting it." >&2
  exit 1
fi

removed="$(comm -23 <(printf '%s\n' "$base_entries") <(printf '%s\n' "$head_entries") || true)"
removed_count="$(printf '%s' "$removed" | grep -c . || true)"
head_count="$(printf '%s' "$head_entries" | grep -c . || true)"

echo "check-lint-baseline: ok (${head_count} entries, ${removed_count} removed since ${BASE_REF})"
