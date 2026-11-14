# OpenCode Tools Beta Audit

This ledger records the current first-class `public-beta` contract for standalone OpenCode tools.

## Scope

Covered in the current beta contract:

- target: `opencode`
- authored surface:
  - `targets/opencode/tools/**`
- shared supporting authored surface:
  - `targets/opencode/package.json`

This beta surface is intentionally separate from:

- the stable OpenCode local-plugin-loading subset in [OPENCODE_STABLE_PROMOTION_AUDIT.md](./OPENCODE_STABLE_PROMOTION_AUDIT.md)
- the broader `custom_tools` umbrella capability spanning standalone tools and plugin code

## Beta Boundary

`tools=beta` currently means:

- deterministic repo-local authored/generate/import/validate contract for `targets/opencode/tools/**`
- deterministic generate into `.opencode/tools/**`
- explicit import support for project-local and `--include-user-scope` OpenCode tool directories
- deterministic validation for path ownership, traversal rejection, symlink rejection, duplicate normalized paths, case-folded collisions, JS/TS tool-file presence, and `@opencode-ai/plugin` dependency checks
- dedicated opt-in live smoke evidence through `TestOpenCodeStandaloneToolsSmoke`

`tools=beta` currently does **not** mean:

- stable semantic guarantees for every possible standalone tool implementation
- AST/typecheck validation
- general npm import-graph validation
- stable promotion of the broader `custom_tools` umbrella capability

## Evidence Required

Current beta confidence requires:

- descriptor-backed docs and matrix wording
- scaffold alignment
- generate/import/validate coverage
- production example canary coverage
- dedicated opt-in standalone-tools smoke evidence

## Promotion Notes

Current status:

- `tools`: `beta-with-live-evidence`

Future stable promotion would require:

- sustained standalone-tools live evidence
- clearly bounded semantic expectations for runtime behavior
- a documented decision on whether standalone-tool semantics can be promoted independently of the broader `custom_tools` capability
