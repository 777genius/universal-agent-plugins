# OpenCode Stable Promotion Audit

This ledger records the post-`v1.0.0` promotion of the stable OpenCode local-plugin-loading subset.

Standalone-tools beta evidence is tracked separately in [OPENCODE_TOOLS_BETA_AUDIT.md](./OPENCODE_TOOLS_BETA_AUDIT.md).

## Scope

Promoted to `public-stable` in the current source tree:

- target: `opencode`
- authored surfaces:
  - `targets/opencode/plugins/**`
  - `targets/opencode/package.json`
- supporting authored/generated contract already in scope with this subset:
  - `targets/opencode/package.yaml`
  - `targets/opencode/commands/*.md`
  - `targets/opencode/agents/*.md`
  - `targets/opencode/themes/*.json`
  - portable `skills`
  - portable `mcp_servers`

Explicitly **not** promoted in this audit:

- helper-based `custom_tools` beyond the documented beta contract
- first-class standalone `.opencode/tools/**` beyond its documented beta contract
- broader OpenCode product config beyond the supported package-standard OpenCode surfaces
- JS/TS semantic compilation or universal npm dependency-graph validation

## Stable Boundary

Stable promise for this subset means:

- deterministic repo-local authored/generate/import/validate contract for official-style local JS/TS plugin subtree ownership
- deterministic repo-local authored/generate/import/validate contract for plugin-local dependency metadata in `targets/opencode/package.json`
- stable rejection of the deprecated `export default { setup() { ... } }` scaffold shape
- stable enforcement of the documented `@opencode-ai/plugin` helper dependency check
- stable dependency-free official-style named async plugin scaffold/example shape
- deterministic explicit user-scope import normalization for project-local and `--include-user-scope` OpenCode plugin tree/package metadata
- deterministic marker-based loader smoke through `TestOpenCodeLoaderSmoke`

Stable promise for this subset does **not** mean:

- availability of a local `opencode` binary
- success of external OpenCode startup/auth/provider/network state before plugin loading
- stable support for every possible helper-based custom tool implementation
- stable support for the separate first-class beta standalone `.opencode/tools/**` surface

## Evidence Required

Promotion requires:

- descriptor-backed docs and policy alignment
- scaffold alignment
- generate/import/validate alignment
- production example canary coverage
- unit coverage
- integration coverage
- contract coverage
- real OpenCode loader smoke evidence through the documented opt-in live path

## Promotion Decision

Current status:

- `local_plugin_code`: `stable-approved`
- `local_plugin_dependencies`: `stable-approved`
- `custom_tools`: `stays-beta`

Rationale:

- repo-local plugin subtree ownership and plugin-local dependency metadata now have a fully bounded authored/generate/import/validate contract plus deterministic loader evidence
- helper-based custom tools and the separate standalone tools surface are valuable and supported, but their semantic surface is still broader than the current stable validation boundary
