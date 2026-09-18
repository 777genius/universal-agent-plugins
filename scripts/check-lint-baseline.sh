#!/usr/bin/env bash
# Enforces that the lint exclusions in .golangci.yml only ever shrink.
#
# An exclusion is amnesty: it says "this file may violate the size gate". So
# every exclusion in HEAD must already exist in the base revision, and it may
# only get narrower - fewer linters, fewer message patterns - never wider. A new
# path, a new linter on an existing path, or a new message pattern all fail
# here, including entries placed outside the LEGACY SIZE BASELINE markers.
#
# A `git mv` of an exempt file is indistinguishable from a new exemption here,
# so moves are declared in scripts/lint-baseline-renames.txt and applied to the
# base side before the comparison. A declared move still has to keep the same
# linters and message patterns; the rename file only maps the path.
#
# This is a speed bump, not a proof. It understands the flat three-line entry
# shape the generator emits and will not follow arbitrary YAML restructuring.
# Its job is to catch accidental widening and to push the deliberate kind
# through review.
#
# Usage: scripts/check-lint-baseline.sh <base-ref>

set -euo pipefail

BASE_REF="${1:-origin/main}"
CONFIG=".golangci.yml"
RENAMES="scripts/lint-baseline-renames.txt"

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

# One tab separated "path<TAB>linters<TAB>text<TAB>region" record per exclusion
# rule. region is "baseline" between the markers, "other" anywhere else.
extract_rules() {
  awk '
    function trim(s) { sub(/^[[:space:]]+/, "", s); sub(/[[:space:]]+$/, "", s); return s }
    function unquote(s,   q) {
      s = trim(s)
      q = substr(s, 1, 1)
      if ((q == "\"" || q == "'"'"'") && substr(s, length(s), 1) == q) { s = substr(s, 2, length(s) - 2) }
      return s
    }
    function flush() {
      if (have) { printf "%s\t%s\t%s\t%s\n", p, l, t, r }
      have = 0; p = ""; l = ""; t = ""; r = ""
    }
    /^[[:space:]]*#/ {
      if (index($0, "BEGIN LEGACY SIZE BASELINE")) { region = "baseline" }
      else if (index($0, "END LEGACY SIZE BASELINE")) { flush(); region = "other" }
      next
    }
    /^  exclusions:[[:space:]]*$/ { in_exclusions = 1; next }
    in_exclusions && /^    rules:[[:space:]]*$/ { in_rules = 1; next }
    in_rules && /^[[:space:]]{0,4}[A-Za-z]/ { flush(); in_rules = 0; in_exclusions = 0 }
    !in_rules { next }
    /^[[:space:]]*-[[:space:]]*path:/ {
      flush()
      have = 1; r = (region == "baseline" ? "baseline" : "other")
      line = $0; sub(/^[[:space:]]*-[[:space:]]*path:/, "", line); p = unquote(line)
      next
    }
    /^[[:space:]]*linters:/ {
      line = $0; sub(/^[[:space:]]*linters:/, "", line)
      gsub(/[][[:space:]]/, "", line); l = line
      next
    }
    /^[[:space:]]*text:/ { line = $0; sub(/^[[:space:]]*text:/, "", line); t = unquote(line); next }
    END { flush() }
  ' | sort
}

# A rule may only narrow: its linters and its message patterns must both be
# subsets of what the base revision already granted for the same path. An empty
# list means "everything", so it is the widest value, not the narrowest. Several
# rules can share a path, so the base side is unioned per path.
compare_rules() {
  awk -F'\t' '
    function union(old, add, sep) {
      if (old == "" || add == "") { return "" }
      return old sep add
    }
    function widened(head, base, sep,   n, i, parts, seen, m, j, bparts) {
      if (base == "") { return 0 }
      if (head == "") { return 1 }
      m = split(base, bparts, sep)
      for (j = 1; j <= m; j++) { seen[bparts[j]] = 1 }
      n = split(head, parts, sep)
      for (i = 1; i <= n; i++) { if (!(parts[i] in seen)) { return 1 } }
      return 0
    }
    NR == FNR {
      if ($1 in base_seen) {
        base_l[$1] = union(base_l[$1], $2, ",")
        base_t[$1] = union(base_t[$1], $3, "|")
      } else {
        base_l[$1] = $2; base_t[$1] = $3; base_seen[$1] = 1
      }
      next
    }
    {
      if (!($1 in base_seen)) { print "new\t" $4 "\t" $1; bad = 1; next }
      if (widened($2, base_l[$1], ",")) { print "wider-linters\t" $4 "\t" $1 "\t[" $2 "] was [" base_l[$1] "]"; bad = 1; next }
      if (widened($3, base_t[$1], "|")) { print "wider-text\t" $4 "\t" $1 "\t" $3 " was " base_t[$1]; bad = 1 }
    }
    END { exit(bad ? 1 : 0) }
  ' "$1" "$2"
}

if [ ! -f "$CONFIG" ]; then
  echo "check-lint-baseline: $CONFIG not found in the working tree" >&2
  exit 1
fi

if ! git rev-parse --verify --quiet "$BASE_REF^{commit}" >/dev/null; then
  echo "check-lint-baseline: base ref '$BASE_REF' does not resolve to a commit" >&2
  echo "Fetch it first (CI needs fetch-depth: 0) or pass a valid ref, for example" >&2
  echo "  make lint LINT_BASE=origin/main" >&2
  exit 1
fi

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

extract_rules <"$CONFIG" >"$work_dir/head"
head_count="$(grep -c . "$work_dir/head" || true)"

if ! git cat-file -e "$BASE_REF:$CONFIG" 2>/dev/null; then
  echo "check-lint-baseline: $CONFIG does not exist at $BASE_REF; this run introduces it"
  echo "check-lint-baseline: HEAD carries ${head_count} exclusion rules"
  exit 0
fi

git show "$BASE_REF:$CONFIG" | extract_rules >"$work_dir/base"
base_count="$(grep -c . "$work_dir/base" || true)"

if [ -f "$RENAMES" ]; then
  awk -F'\t' '
    NR == FNR {
      line = $0
      sub(/#.*/, "", line)
      split(line, field, /[[:space:]]+/)
      if (field[1] != "" && field[2] != "") { moved[field[1]] = field[2] }
      next
    }
    { if ($1 in moved) { $1 = moved[$1] } ; print $1 "\t" $2 "\t" $3 "\t" $4 }
  ' "$RENAMES" "$work_dir/base" | sort >"$work_dir/base.renamed"
  mv "$work_dir/base.renamed" "$work_dir/base"
fi

if ! compare_rules "$work_dir/base" "$work_dir/head" >"$work_dir/violations"; then
  echo "check-lint-baseline: lint exclusions may only shrink, but these are new or wider:" >&2
  sed 's/^/  /' "$work_dir/violations" >&2
  echo >&2
  echo "Split the file or reduce its complexity instead of exempting it." >&2
  echo "'wider-linters' or 'wider-text' means an existing entry now covers more than it did." >&2
  exit 1
fi

echo "check-lint-baseline: ok (${head_count} exclusion rules, was ${base_count} at ${BASE_REF})"
