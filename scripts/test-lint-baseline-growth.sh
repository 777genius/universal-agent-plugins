#!/usr/bin/env bash
# End-to-end test of the actual lint baseline checker against disposable git history.
set -euo pipefail
repo_root="$(cd "$(dirname "$0")/.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
cd "$fixture"
mkdir -p scripts/legacy-go-lines legacy
cp "$repo_root/scripts/check-lint-baseline.sh" scripts/check-lint-baseline.sh
cp "$repo_root/scripts/legacy-go-lines/main.go" scripts/legacy-go-lines/main.go
cat > .golangci.yml <<'CFG'
version: "2"
linters:
  exclusions:
    rules:
      # BEGIN LEGACY SIZE BASELINE (shrink-only; scripts/check-lint-baseline.sh)
      - path: ^legacy/old_linux\.go$
        linters: [revive]
        text: 'file length is'
      - path: ^legacy/old_windows\.go$
        linters: [funlen]
        text: 'is too long'
      # END LEGACY SIZE BASELINE
CFG
cat > legacy/old_linux.go <<'GO'
//go:build linux
package fixture
var original = 1
GO
cat > legacy/old_windows.go <<'GO'
//go:build windows
package fixture
var original = 1
GO
: > scripts/lint-baseline-renames.txt
git init -q
git config user.name 'CI architecture test'
git config user.email 'architecture-test@example.invalid'
git config commit.gpgsign false
git add .
git commit -qm 'baseline'
run_gate() { bash scripts/check-lint-baseline.sh HEAD >"$fixture/log" 2>&1; }
expect_fail() {
  local message="$1"
  if run_gate; then
    echo "gate unexpectedly passed: $message" >&2; exit 1
  fi
  if ! grep -Fq "$message" "$fixture/log"; then
    cat "$fixture/log" >&2
    echo "expected refusal missing: $message" >&2; exit 1
  fi
}
run_gate
printf '\n// extra comment\n\n' >> legacy/old_linux.go
run_gate
printf 'var grown = 2\n' >> legacy/old_linux.go
expect_fail 'grew from'
# Reductions are permitted even if the file stays in the legacy baseline.
printf '//go:build linux\npackage fixture\n' > legacy/old_linux.go
run_gate
git checkout -- legacy/old_linux.go
printf 'var platformGrowth = 2\n' >> legacy/old_windows.go
expect_fail 'grew from'
git checkout -- legacy/old_windows.go
printf 'package fixture\n' > "$fixture/outside.go"
rm legacy/old_windows.go
ln -s ../outside.go legacy/old_windows.go
expect_fail 'refusing symlinked legacy source'
rm legacy/old_windows.go
git checkout -- legacy/old_windows.go
cat >> .golangci.yml <<'CFG'
      - path: ^legacy/new.go$
        linters: [revive]
        text: 'file length is'
CFG
expect_fail '^legacy/new.go$'
git checkout -- .golangci.yml
git mv legacy/old_linux.go legacy/renamed_linux.go
sed 's@old_linux@renamed_linux@g' .golangci.yml > "$fixture/config-renamed"
cp "$fixture/config-renamed" .golangci.yml
expect_fail '^legacy/renamed_linux\.go$'
printf '%s\t%s\n' '^legacy/old_linux\.go$' '^legacy/renamed_linux\.go$' > scripts/lint-baseline-renames.txt
run_gate
printf '%s\n' 'legacy baseline growth integration E2E: PASS'
