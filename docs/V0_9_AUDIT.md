# `v0.9` Stable-Candidate Audit

This document tracks the declared `v1` candidate set through freeze, rehearsal, and final approval evidence recording.

`public-stable` promotion stayed blocked until this audit was completed and release evidence was recorded for the promoted set.

## Declared `v1` Candidate Set

SDK root API:

- `plugin-kit-ai.New`
- `plugin-kit-ai.Config`
- `plugin-kit-ai.App`
- `(*plugin-kit-ai.App).Use`
- `(*plugin-kit-ai.App).Claude`
- `(*plugin-kit-ai.App).Codex`
- `(*plugin-kit-ai.App).Run`
- `(*plugin-kit-ai.App).RunContext`
- `plugin-kit-ai.Supported`

Event surfaces:

- `claude/Stop`
- `claude/PreToolUse`
- `claude/UserPromptSubmit`
- `codex/Notify`

CLI surfaces:

- `plugin-kit-ai init`
- `plugin-kit-ai validate`
- `plugin-kit-ai capabilities`
- `plugin-kit-ai install`
- `plugin-kit-ai version`

Generated scaffold contract:

- Codex: `go.mod`, `README.md`, `AGENTS.md`, `.codex/config.toml`, generated `cmd/<project>/main.go`
- Claude: `go.mod`, `README.md`, `.claude-plugin/plugin.json`, `hooks/hooks.json`, generated `cmd/<project>/main.go`

## Audit Gates

Every candidate surface must answer these questions before promotion:

- Is it declared in support/stability policy?
- Does it have generated support metadata where applicable?
- Do scaffold and validate claims agree?
- Does deterministic coverage exist in unit/integration/contract tests?
- Does required external smoke exist where applicable?
- Are user-facing diagnostics reviewed?
- Is a change note required?

## Final Statuses

- `candidate-ready`: technical and policy prerequisites are present, but release evidence is not recorded yet.
- `stable-approved`: approved for `v1` stable promotion after rehearsal evidence is recorded.
- `stays-beta`: shipped and supported, but intentionally not promoted in `v1`.
- `blocked`: cannot be promoted and blocks `v1` if it belongs to the core stable set.

## Release Evidence Fields

Every candidate group must record:

- `required`: required lane result recorded
- `extended`: external smoke result recorded, or `n/a`
- `live/waiver`: live result recorded, explicit waiver recorded, or `n/a`
- `rehearsal`: release rehearsal decision recorded

Core stable set that must not remain `blocked`:

- SDK root API
- Claude event set
- Codex `Notify`
- `plugin-kit-ai init`
- `plugin-kit-ai validate`
- generated scaffold contract

## Audit Ledger

| Surface | Policy Declared | Generated / Descriptor Backed | Deterministic Coverage | External Smoke Policy | Diagnostics Review | Migration Note | Required | Extended | Live / Waiver | Rehearsal | Final Status | Notes |
|--------|------------------|-------------------------------|------------------------|-----------------------|--------------------|----------------|----------|----------|---------------|-----------|--------------|-------|
| SDK root API (`New`, `Config`, `App`, `Use`, `Claude`, `Codex`, `Run`, `RunContext`, `Supported`) | yes | yes | yes | n/a | yes | existing | pass | n/a | n/a | done | stable-approved | Runtime failure families are documented in `DIAGNOSTICS.md` and covered by runtime regression tests. |
| Claude event set (`Stop`, `PreToolUse`, `UserPromptSubmit`) | yes | yes | yes | yes | yes | existing | pass | pass | n/a | done | stable-approved | Real Claude CLI smoke asserts all three declared events through the repository-owned hook harness and now passes in the latest evidence refresh. |
| Codex event set (`Notify`) | yes | yes | yes | yes | yes | existing | pass | pass | n/a | done | stable-approved | Real `codex exec` smoke passed in rehearsal, and current live evidence also includes real `codex mcp get --json` preflight through explicit override wiring plus generated `codex-package` `.mcp.json` sidecar projection into the same live CLI path. Known external Codex runtime panics remain environment-health skips rather than plugin-kit-ai regressions. Current live evidence also records that Codex CLI `v0.117.0` does not reliably honor project-local `.codex/config.toml` for `exec` or `mcp get`, so those probes stay evidence-only rather than shipped guarantees. |
| CLI command set (`init`, `validate`, `capabilities`, `install`, `version`) | yes | partial | yes | partial | yes | existing | pass | pass | pass | done | stable-approved | `init`, `validate`, `capabilities`, and `install` have integration coverage. `version` is covered in required, and live install checks now pass in the latest evidence refresh. |
| Generated scaffold contract (Codex + Claude required files and generated entrypoints) | yes | yes | yes | n/a | n/a | existing | pass | n/a | n/a | done | stable-approved | Scaffold and validate claims are generated from descriptors and covered by init/validate integration tests. |

## Remaining Gaps Before Promotion

- Carry the stable-approved set through the final `v1.0` release notes.
- Keep future additions outside the approved set in `public-beta` until a new planning and approval cycle completes.
