# Codex + Claude Code user/global installation hardening (2026-11-05)

## Goal and scope

Qualify **Agent Plugins 1.0** packages and **Agent Skills 1.0** raw Skills for the two user-level native targets `codex` and `claude`. In the public `agentplugins` CLI, **global means `--scope user`**, the default scope; `--scope global` and the legacy `plugin-kit-ai --global` interface are not equivalent public `agentplugins` contracts. Do not introduce a second command or alias without a separately reviewed compatibility decision.

Source baseline: main `8aa409933baed3886150a24c74ce5908c6eff632`. Native client provenance, auth, and activation remain separate from file installation. The standard is normative for root `plugin.json`, `skills/<leaf>/SKILL.md`, `mcp.json`, containment and component failure boundaries; installation scope and native activation are UAP/client-specific.

## Existing foundations and non-overlap

- `repotests/agentplugins_{codex,claude}_native_e2e_test.go` and commands-native tests already exercise actual signed/digest-pinned client executables, plugin activation, Skills and MCP inventory, repeat/update/repair/remove, foreign-file preservation.
- `cli/internal/agentpluginscli/skills_batch_test.go` verifies raw Skill source/revision and per-client cold repair, but does not by itself prove every user/native lifecycle.
- `cli/cmd/agentplugins/project_skills_e2e_test.go` covers raw Git/HTTPS Skill user/project isolation with Gemini, not both named native clients.
- PR #698 is a separate multi-client 30×30 audit and PR #713 changes legacy-lock boundaries; avoid editing their changed files unless essential. VS Code implementation/fixtures are frozen.

## Implementation order

1. **Red/green public CLI regression**: add user-scope lifecycle tests for raw Skills on Codex and Claude wherever the production composition and existing inert fixtures can prove it, including dry-run, explicit/default scope parity, save+lock, repeat, retained foreign neighbors, per-target update/repair/remove, cross-scope isolation, and fail-closed ambiguity.
2. **Native proof**: extend existing Codex and Claude real-client hosted fixtures (do not invent alternate installers) to exercise explicit user scope alongside the default, pin/check actual native versions and observed activation, and include raw Skills native discovery only when independently observable. Classify any missing native observation as unqualified, not PASS.
3. Fix **only defects reproduced by the above** in the existing seam/adapter. Preserve name/identity, receipts, state, fail-closed path containment, rollback/recovery, public exit/JSON shapes, and source hash. No changes to frozen VS Code, release policies, generated files, feature flags, or unsupported component types.
4. Run exact-HEAD `make lint`, `make test-core`, and scoped native/portable package-manager checks; use hosted Linux, Windows and macOS including relevant architectures. Do not treat skipped/cancelled/draft-only jobs as E2E proof. Review independently from security, race/data loss, architecture, and native compatibility lenses; resolve findings and re-run all applicable checks after any push.
5. Only after the **final commit** passes mandatory checks and genuine native E2E, mark Ready, obtain review and squash merge. Verify post-merge main, report any real external auth/model limitations. Release is out of scope.

## Safety and acceptance matrix

| Surface | Positive | Negative |
| --- | --- | --- |
| User/global selection | default and explicit `--scope user` route to one immutable user namespace | user + `--project-root` refuses before mutation |
| Plugin package | strict Agent Plugins 1.0 Skills/MCP projection; repeat, update, repair, remove | invalid manifest, foreign/colliding native identity, path escape preserve bytes |
| Raw Skill | Agent Skills 1.0 `SKILL.md` plus assets/references retained in pinned lock; multiple targets; cold replay | invalid source, poisoned path, stale plan, unowned skill cannot be adopted |
| Isolation | selected user Codex/Claude only, neighboring project and foreign native state identical | tampering/failed/partial operations do not publish false success |
| Native | real pinned Codex/Claude CLI observes installed/enabled components and eventual removal | no claim of model/MCP execution, OAuth or interactive trust unless actually tested |

## Review log

- **Initial security critique**: never infer native activation from on-disk files; demand exact client listing and actual process evidence. Preserve foreign bytes on all refusals.
- **Initial data-loss critique**: the cross-scope negative paths must be asserted *before* acquisition/write, and no-op must preserve state/managed bytes, not merely print success.
- **Initial architecture critique**: place tests at public CLI/adapter boundaries; use the established source and runner fixtures rather than introducing a new engine or feature flag.

All boxes below require recorded, current-HEAD evidence. Work is not complete merely because this plan or Draft PR exists.

- [ ] Focused public CLI raw Skill lifecycle and refusal tests
- [ ] Native Codex user-scope parity and Skill proof
- [ ] Native Claude Code user-scope parity and Skill proof
- [ ] Linux/Windows/macOS E2E on exact final commit
- [ ] Required/core/lint and review findings resolved
- [ ] Reviewed and squash-merged
