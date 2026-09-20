#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

source "$ROOT/scripts/version-contract.env"

if [[ -z "${GO_SDK_VERSION:-}" || -z "${RUNTIME_PACKAGE_VERSION:-}" ]]; then
  echo "version contract is incomplete; expected GO_SDK_VERSION and RUNTIME_PACKAGE_VERSION" >&2
  exit 1
fi

files=(
  README.md
  cli/README.md
  sdk/README.md
  docs/CHOOSING_HELPER_DELIVERY_MODE.md
  examples/starters/README.md
  examples/starters/codex-go-starter/README.md
  examples/starters/codex-go-starter/go.mod
  examples/starters/codex-go-starter/go.sum
  examples/starters/claude-go-starter/README.md
  examples/starters/claude-go-starter/go.mod
  examples/starters/claude-go-starter/go.sum
  examples/starters/codex-python-runtime-package-starter/README.md
  examples/starters/codex-python-runtime-package-starter/requirements.txt
  examples/starters/claude-node-typescript-runtime-package-starter/README.md
  examples/starters/claude-node-typescript-runtime-package-starter/package.json
  examples/plugins/codex-basic-prod/go.mod
  examples/plugins/codex-basic-prod/go.sum
  examples/plugins/claude-basic-prod/go.mod
  examples/plugins/claude-basic-prod/go.sum
  cli/internal/scaffold/version_contract.go
  cli/internal/scaffold/templates/go.mod.tmpl
  cli/internal/scaffold/templates/codex.go.mod.tmpl
  cli/internal/scaffold/templates/README.md.tmpl
  cli/internal/scaffold/templates/codex-runtime.README.md.tmpl
  cli/internal/scaffold/templates/python.requirements.txt.tmpl
  cli/internal/scaffold/templates/node.package.json.tmpl
)

perl_expr=(
  -0pi
  -e "s{github\\.com/777genius/plugin-kit-ai/sdk\\@v\\d+\\.\\d+\\.\\d+}{github.com/777genius/plugin-kit-ai/sdk\\@${GO_SDK_VERSION}}g;"
  -e "s{github\\.com/777genius/plugin-kit-ai/sdk v\\d+\\.\\d+\\.\\d+}{github.com/777genius/plugin-kit-ai/sdk ${GO_SDK_VERSION}}g;"
  -e 's{Use `v\d+\.\d+\.\d+` or newer}{Use `'"${GO_SDK_VERSION}"'` or newer}g;'
  -e "s{--runtime-package-version \\d+\\.\\d+\\.\\d+}{--runtime-package-version ${RUNTIME_PACKAGE_VERSION}}g;"
  -e "s{plugin-kit-ai-runtime==\\d+\\.\\d+\\.\\d+}{plugin-kit-ai-runtime==${RUNTIME_PACKAGE_VERSION}}g;"
  -e "s{plugin-kit-ai-runtime\\@\\d+\\.\\d+\\.\\d+}{plugin-kit-ai-runtime\\@${RUNTIME_PACKAGE_VERSION}}g;"
  -e "s{\"plugin-kit-ai-runtime\": \"\\d+\\.\\d+\\.\\d+\"}{\"plugin-kit-ai-runtime\": \"${RUNTIME_PACKAGE_VERSION}\"}g;"
  -e 's{pin `plugin-kit-ai-runtime` to `\d+\.\d+\.\d+`}{pin `plugin-kit-ai-runtime` to `'"${RUNTIME_PACKAGE_VERSION}"'`}g;'
  -e "s{DefaultGoSDKVersion          = \"v\\d+\\.\\d+\\.\\d+\"}{DefaultGoSDKVersion          = \"${GO_SDK_VERSION}\"}g;"
  -e "s{DefaultRuntimePackageVersion = \"\\d+\\.\\d+\\.\\d+\"}{DefaultRuntimePackageVersion = \"${RUNTIME_PACKAGE_VERSION}\"}g;"
)

perl "${perl_expr[@]}" "${files[@]}"

echo "updated pinned version references from scripts/version-contract.env"
