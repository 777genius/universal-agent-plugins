# Automatic native client installation — implementation and E2E delivery plan

Created 2026-11-09; repository baseline `main` `00bcdf486dabef2dc2a4e6cb239bae818a451ab0`.

## Goal and precise definition

Deliver reliable, user/project-scoped Skills/MCP installation for Kimi Code, Grok Build, Cline, Windsurf/Devin and Kiro, reusing UAP's existing installer ownership and transaction infrastructure. **Automatic** means that UAP writes/updates/removes selected *native* components, records exact ownership receipts, and verifies installed bytes/entries. It does **not** automatically log into third-party accounts, approve untrusted workspaces, accept permissions, execute vendor models or prove MCP `tools/call`. Treat `Installed`, `Prepared`, `Activated`, and `Needs user action` as separate facts.

Do not change a site badge without real implementation, no-effect failure tests, and observed native evidence. The signed Directory compatibility/transport gate is separate from client capability. Preserve all existing user-scope, plugin-manager, custom-profile, offline/local and scope-selection semantics.

## Priorities, PR/CI strategy and budget

| Order | Deliverable PR | Real gap | Approx. changed production LOC | Acceptance |
| --- | --- | --- | ---: | --- |
| P0 | **Kimi standalone** — this PR | Already displays Automatic, but Project only admits `2.3.1/2.3.2`; current stable `2.5.2` is rejected | 2–30 | Exact current version + secure project Skills/MCP E2E |
| P1 | **Grok standalone** | Project Plugins/Skills exist; Project MCP currently unqualified | 100–450 | Trusted project plugin MCP native proof, or truthful Guided |
| P2 | **Cline standalone** | CLI can use selected MCP settings path; IDE path selection differs | 50–220 | Config route correctly identified, no accidental IDE/global rewrite |
| P3 | **Windsurf/Devin standalone** | Legacy Windsurf != current Devin; version/channel/trust constraints | 130–500 | Both surfaces independently qualified and documented |
| P4 | **Kiro standalone** | Native files partially written; login/agent/IDE activation not qualified | 130–500 | Config delivery verified without auth bypass or false activation |

**Why multiple small PRs:** one huge PR blocks a completed Kimi fix on an unrelated Grok/Kiro issue and triggers entire suites on every late edit. Using parallel unrelated PRs creates extra CI queue load, so prepare them in priority order, minimize commits after requesting review, and do not duplicate global tests. Merge each independently when its exact-head evidence passes; don't wait for every agent.

Measured CI baseline: `main` Required run [42633502352](https://github.com/777genius/universal-agent-plugins/actions/runs/42633502352) completed in ~41 minutes, while Pages [42633501816](https://github.com/777genius/universal-agent-plugins/actions/runs/42633501816) took ~108 minutes including queue time. Another main Required [42617871468](https://github.com/777genius/universal-agent-plugins/actions/runs/42617871468) finished in ~17 minutes. Queue contention dominates some runs. Do not skip mandatory checks or repeatedly rerun queued work. A new focus-only native job should be path-scoped, run in parallel with existing Required, and retain original structured receipts.

Site PR ownership stays separate: #807 (auth/Pi/Web), #808 (temporarily hide VS Code), #809 (Sources/Owners row). Do not duplicate their changes.

## P0 Kimi — researched source contract

**Evidence:**
- Current UAP `install/integrationctl/agentplugins/clients/kimi/scope.go::currentKimi` admits exactly `2.3.1/2.3.2` for Project, regardless of its Automatic display. Existing `version_profile.go` probes a selected client using an isolated disposable profile, since genuine `kimi --version` may initialize caches. Existing selected-project probe is invoked in `cli/internal/agentpluginscli/skills_batch_prepare.go`.
- Official stable `@moonshot-ai/kimi-code@2.5.2` available on npm on 2026-11-09; do not treat canary `2.6.0` as stable.
- Verified **identical Git blob SHA** `93879a95c8e0b83cb6153981cad969f321b6205b` of upstream `packages/agent-core-v2/src/app/mcpConfig/configLoader.ts` in tags `@moonshot-ai/kimi-code@2.3.2` and `@moonshot-ai/kimi-code@2.5.2`. It reads user `mcp.json`, Git-root compatibility `.mcp.json`, and cwd `.kimi-code/mcp.json`.
- Tagged upstream `skillRoots.ts` changed (`3db2239db6bbff42ec3e3eedb228cb83904594e7` -> `41fbf01965063bc442d6953d5586c46cf83137b6`), but still recognizes nearest Git root and `.kimi-code/skills`; new host FS/extra-workspace logic needs native proof.
- Publisher documentation: [MCP](https://moonshotai.github.io/kimi-code/en/customization/mcp.html), [Skills](https://moonshotai.github.io/kimi-code/en/customization/skills.html), [Plugins](https://www.kimi.com/code/docs/en/kimi-code-cli/customization/plugins). Kimi's own native **plugin manager remains user-only**; UAP Project Skills/MCP is a distinct supported route.

**Implementation, narrow and reversible:**
1. Admit precisely tested **2.5.2** alongside previously allowed 2.3.1/2.3.2; do not wildcard all 2.x, change executable/path checks, probe an ambient user profile, or accept prerelease/garbled versions.
2. Extend `providers/project_kimi_admission_test.go` positive 2.5.2 and fail-closed negatives (2.5.2-alpha, 2.6.0, bad executable, unknown). Preserve no-effect and nearest-Git failure assertions.
3. Run existing source/Go project lifecycle/repair/remove/foreign-preservation suites. Qualify with a **real pinned stable 2.5.2 CLI**: temporary private HOME, real git init, exact `--version`, UAP Project Skill and inert Project stdio/HTTP MCP where transport is supported, native discovery on new process if authorization permits, repeat add, repair, cold state, removal; preserve raw JSON artifact and explicitly separate bytes from native activation.
4. Do **not** modify the already Automatic site row; only extend supported version and update evidence after a successful run. A source-built CLI is not a released npm CLI: post-release smoke is separate.
5. If actual 2.5.2 native installation fails, **do not merge the admission change**. Reproduce a single source case, correct the smallest verified cause, re-run exact-head evidence.

## P1 Grok — exploit existing project plugin lane, not speculative TOML

Official references: [Grok MCP](https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/07-mcp-servers.md), [Grok Plugins](https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/09-plugins.md), [Grok configuration](https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/05-configuration.md).

Two real native routes exist:
- Project `.grok/config.toml` with `[mcp_servers.<name>]`, cwd/git-root merging, project > user; stdio and native HTTP.
- Project `.grok/plugins/` **trusted** plugin providing a `.mcp.json` MCP sidecar. An untrusted project plugin shows MCP as **blocked**, not active.

UAP already builds `.grok-plugin/plugin.json` and bundled MCP projection in `clients/grok/project.go`, but `scope.go` and `project_lifecycle.go` reject Project components other than Skills and exact pinned inspect parser only accepts 1.0.67/1.0.70. Investigate official native current version and **prove plugin sidecar JSON shape, filenames and discovery first**. If sufficient, extend that lane and existing ownership verification without introducing TOML parsing/writing. If native cannot discover this route, write a second scoped plan for a minimal TOML component with strict foreign keys, comments, cross-file precedence, atomic rollback, no-follow paths and pinned source tests. Do not implement both routes speculatively.

Acceptance: no user trust bypass, no auto-enable; test trusted and untrusted project separately, real `grok inspect --json`, installed MCP projection after restart, one inert stdio initialize/list if feasible without a provider, user/project A/B and foreign preservation, repeat/update/remove, mismatched profile/unknown versions no-effect. Do not change `Skills only` or Guided until proven.

## P2 Cline — CLI and IDE split

Official [Cline MCP docs](https://github.com/cline/cline/blob/main/docs/mcp/mcp-overview.mdx): CLI settings `~/.cline/data/settings/cline_mcp_settings.json` or explicit `CLINE_MCP_SETTINGS_PATH`; IDE extension's actual MCP settings file is selected in its UI. Existing UAP `clients/cline/{plan,project,lifecycle_mcp,scope}` already installs files and its project native evidence contains `tools/call` under explicit selected settings path.

Identify chosen CLI vs IDE route from real detected surface; do not claim the IDE uses CLI's path. For CLI, apply existing owned MCP member mutation and verify native `cline mcp` in a new process; where scope-specific override is not exposed to the host, retain Guided. No "scan every VS Code extension" or create a global setting silently. Test foreign override, symlink, explicit env path and removal.

## P3 Windsurf/Devin — distinct profiles

Existing [adapter contract](../../install/integrationctl/agentplugins/clients/windsurf/README.md): current Devin project `3000.11.205`, `.devin/skills/<name>`, `.devin/mcp_config.json`; legacy Cascade user lane has channel-specific MCP, Skills remain prepared. Native user/project projection exists but actual trust/team/cloud precedence is not fully observed.

Determine official documented **version capability**, not one arbitrary version, through pinned source/native observations. Preserve strict primary executable selection, legacy channel and coexistence behavior. Treat project file installation separately from native session activation. No system-wide command to force trust; no touching cloud/team config. Explicit no-effect tests with mixed Windsurf/Devin installations.

## P4 Kiro — configuration vs account boundary

Publisher [Kiro MCP](https://kiro.dev/docs/mcp/configuration/) and [scope docs](https://kiro.dev/docs/cli/chat/configuration/): project `.kiro/settings/mcp.json`, user `~/.kiro/settings/mcp.json`, project `.kiro/skills`; agent-specific settings supersede workspace, workspace supersedes user. Changes to MCP JSON reconnect after save. Existing UAP project/native code is substantial; [native authentication refusal PR #686](https://github.com/777genius/universal-agent-plugins/pull/686) is still independent Draft.

Complete owned, atomic local config/Skill projection, preserve foreign agent/user settings and disabled entries. Qualify from read-only native client session if available. Authentication/OAuth/workspace approvals remain explicitly human actions. Never map mere successful JSON writes to native Activation.

## Clean Architecture / SOLID / DRY implementation map

- Keep vendor-specific paths/version rules/codec in `clients/{kimi,grok,cline,windsurf,kiro}`.
- Reuse `clients.PlanRefiner`, `clients.PlanQualifier`, `clients.ProjectTargetLayout`, `clients.Projector` and lifecycle interfaces; preserve `shared.ProjectComponentMutation`, `nativeconfig` kernel, existing exact object-ownership receipts, no-follow validation, bounded probes, repair/cold restore.
- No duplicate installer, generic switch on agent name, new service locator, new feature flags, configuration rewriting of unknown members, unsafe symlink following, silent `--trust` or permission changes, or new runtime dependency unless unavoidable for verified format.
- UI is a presentation of qualified scope/status. README/site matrix parity tests must pass after each capability change; don't spread agent-specific status by copying arbitrary strings.

## Test and release gates (all agents)

**Unit/contract:** supported/unsupported component and transport, exact selected scope/profile/version, malformed/foreign/linked native state refusal, idempotence, deterministic mutation and no-effect refusal.

**E2E:** install fresh; inspect original files/receipts; repeat; update; break owned state then repair; cold locked install; source identity preserved; remove only owned component; project A/B isolation and User target unaffected. At least representative Windows/macOS/Linux native install/discovery where runners and verified client binaries exist. Stdio/HTTP/SSE independently only if selected client supports them.

**Evidence:** native executable version and artifact checksum, source commit, exact UAP PR HEAD, JSON output, real probe exit, expected vs observed artifacts; classify `Prepared` vs `Installed` vs `tools/list` vs `tools/call` and any auth gate. Do not claim paid model integration from a scripted fixture.

**Merge:** two code review passes (behavior/security and UX/source compatibility), fix actionable findings, required CI + CodeQL/dependency/security and focused original E2E ZIP at exact HEAD, then SHA-guarded merge. Publication/npm/site require separate actual release/Pages verification. Do not consider queued CI a blocker or a pass.

## Critical review passes on this plan

1. **Scope/functional:** removed initially proposed badge-only "Automatic for all" approach; it would lie about Kimi (already Automatic), Kiro auth, Grok trust and separate Windsurf/Devin.
2. **Security/DRY:** replaced speculative Grok TOML editor with research-first re-use of already projected trusted plugin MCP sidecar; kept Kimi 2.5.2 admission exact until real native evidence, rather than wildcard unknown major/minor versions.
3. **CI/release:** rejected one mega PR and one docs-only PR. Land plan with first Kimi feature PR, split only independent client functionality, avoid CI reruns per narrative edit, and preserve full exact-head checks.
