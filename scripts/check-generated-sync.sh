#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

before="$(mktemp)"
after="$(mktemp)"
trap 'rm -f "$before" "$after"' EXIT

generated_files=(
  "cli/plugin-kit-ai/internal/scaffold/platforms_gen.go"
  "cli/plugin-kit-ai/internal/validate/rules_gen.go"
  "sdk/internal/descriptors/gen/completeness_gen_test.go"
  "sdk/internal/descriptors/gen/registry_gen.go"
  "sdk/internal/descriptors/gen/resolvers_gen.go"
  "sdk/internal/descriptors/gen/support_gen.go"
  "sdk/internal/descriptors/gen/support_gen_claude.go"
  "sdk/internal/descriptors/gen/support_gen_codex.go"
  "sdk/internal/descriptors/gen/support_gen_gemini.go"
  "sdk/claude/registrar_gen.go"
  "sdk/codex/registrar_gen.go"
  "sdk/gemini/registrar_gen.go"
  "docs/generated/support_matrix.md"
  "docs/generated/target_support_matrix.md"
)

for f in "${generated_files[@]}"; do
  shasum "$f"
done >"$before"

GOCACHE="${GOCACHE:-/tmp/plugin-kit-ai-gocache}" go run ./cmd/plugin-kit-ai-gen >/tmp/plugin-kit-ai-gen.out 2>/tmp/plugin-kit-ai-gen.err

for f in "${generated_files[@]}"; do
  shasum "$f"
done >"$after"

if ! diff -u "$before" "$after"; then
  echo "generated files drifted; rerun generation and review tracked changes" >&2
  exit 1
fi

echo "generated artifacts in sync"
