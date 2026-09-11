# Skill Examples

These are **portable Skills components with historical v1 tooling metadata**,
not complete root `plugin.json` packages or proof of current standard validation.
See the [prepared, unreleased Skill guide](../../website/source/en/build/skill.md)
for package context and [historical v1, baseline 1.2.4](../../website/source/en/legacy/v1/index.md)
for the preserved generation workflow. Public activation remains gated.

| Component | Source and execution boundary |
| --- | --- |
| [go-command-lint](./go-command-lint) | `skills/lint-repo/SKILL.md` invokes Go code that checks required example files exist; it is not a general linter. |
| [cli-wrapper-formatter](./cli-wrapper-formatter) | `skills/format-changed/SKILL.md` invokes pinned Prettier through npx, may download it, and writes files. |
| [docs-only-review](./docs-only-review) | `skills/review-checklist/SKILL.md` is instruction-only. |

The instructions are reusable Skills material. Fields such as `execution_mode`,
`supported_agents`, and `command` describe historical tooling conventions, not
universal client execution guarantees. The committed Claude/Codex projections
under `generated/skills/` remain historical examples; v1 Skills generation and
external Skills lifecycle commands are not part of the standard authoring MVP.
Project migration is not available in v2 yet. Maintain legacy projects using
the v1 1.2.4 command set. No examples or generated outputs are removed here.

These examples are intentionally small, but each one demonstrates a historical v1 beta-adoption path for `plugin-kit-ai skills`.

- `go-command-lint`
  - canonical `SKILL.md` plus a Go command entrypoint
  - shows the recommended typed executable path
- `cli-wrapper-formatter`
  - canonical `SKILL.md` that wraps an existing external formatter command
  - shows that the subsystem is not Go-only
- `docs-only-review`
  - canonical `SKILL.md` with no executable command
  - shows that a skill can stay instruction-only

Each example keeps the authored source under `skills/<name>/SKILL.md` and commits generated outputs under `generated/skills/...`.
Treat the authored `SKILL.md` as canonical and the generated outputs as disposable generate targets.
