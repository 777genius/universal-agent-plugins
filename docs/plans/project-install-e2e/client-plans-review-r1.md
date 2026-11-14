# Client project plans — independent R1 review

Reviewer: source lane. Reviewed all 14 sections of `client-plans.md` at the revised implementation base `3ea24e3014ddf6679ca86729c56f1069112b0d3a`, relevant existing adapters, the accepted core TargetContext/layout/deactivation contract, and the linked first-party docs. No native client was executed and no code was changed.

## Verdict

The proposed client-specific paths and the separation of native registration, component configuration, prepared sessions and remote/manual installation are substantially correct. Changes below must be applied before R2. Most are small capability/output/verification clarifications; do not turn them into new generic frameworks or second fallback implementations.

## Shared required clarifications

**C1 — choose one pinned lane before adapter coding.** Codex native marketplace versus component fallback, Grok plugin versus direct MCP fallback, and Devin version layouts are currently alternatives. Each PR must record the actual selected official artifact/version and one primary implementation before code. Unsupported/unobserved versions return prepared/unsupported results, preserving user behavior; do not implement two speculative lifecycle systems to make the plan look universally complete. The decision can be an initial read-only pinned native probe recorded in that client plan. This is still consistent with a project capability that supports multiple explicitly verified versions later.

**C2 — effective configuration, not file presence, decides activation.** Each lane must identify higher-precedence project/user/admin/session sources for the exact configured keys and preserve them. If a managed project entry is shadowed, disabled or not loaded by the selected harness, status remains configured/prepared/blocked with a precise reason; neither static file verification nor `--activation-complete` may manufacture durable automatic discovery. The fixture matrix must cover the meaningful specific precedence for each client, not a generic arbitrary foreign file. Direct component adapters must reuse existing rendered command-skill projection and component support decisions when applicable rather than copying only raw `skills/` and dropping the current command behavior.

**C3 — keep no native evidence distinct from unsupported product completion.** When artifact/account/UI evidence is unavailable, preserve the bounded fixture/config result and explicitly report missing native proof. Do not mark a real-client E2E complete from a fake executable or a generic MCP harness. Pin/record the package and exact help/list schema used for adapter decisions; no model task, trust write, source upload or credential creation is required to obtain a truthful partial observation.

## Per-client ledger and findings

| Client | R1 result | Required changes / exact review focus |
|---|---|---|
| Claude | GO after shared C2/C3 clarification | The documented `name@skills-dir` project listing and manifest-version contract supports this narrow approach. Preserve admin allow/block policy and scope-specific disabled entries. No extra client-specific architecture finding. |
| Codex | Fix C1 | Freeze native repo-marketplace **or** component implementation after pinned actual CLI inspection. The changelog's repo source does not prove that the currently installed CLI isolates installed/enable identities. If native proof fails, conservative component/prepared behavior is acceptable; do not add a speculative global-cache management path. |
| Cursor | GO after shared C2/C3 | Project `.cursor/skills` + `.cursor/mcp.json` component path is a bounded choice. Keep UI installed plugins separate; selected official editor/agent evidence must identify actual project components. No guessed project plugin registry or extra Stop/hook claim. |
| Gemini | GO after shared C2/C3 | Existing plan already tests `.agents/skills` precedence. Make the higher-precedence same-name alias produce shadowed/inactive observation rather than successful managed activation; preserve disabled workspace settings without `--consent` autoenable. No additional architecture finding. |
| OpenCode | Fix C2 concretely | Add actual effective-config observation (e.g. pinned `opencode debug config`) and the high-precedence inline/managed/custom-directory override case. Current official config docs distinguish standard project precedence from inline/admin overrides; writing root `opencode.json[c]` alone cannot prove the selected MCP is effective. Preserve JSON/JSONC selection and rooted ancestor search. |
| Copilot CLI | Add format-preserving project MCP contract | Official per-repository MCP accepts both `{mcpServers:{...}}` and a bare server map; same-directory `.mcp.json` wins and closer ancestor files override. Declare support for both existing formats through one codec or fail safely before mutation for a bare map; never silently wrap a bare document and lose foreign servers. Remove project entries by exact transactional key/path, not `copilot mcp remove`, which docs say edits user config. Existing nested override/trust boundaries are correct. |
| Kiro | GO after shared C2/C3 | Keep IDE/CLI surface qualification separate. Agent overrides/include requirements must appear in readiness output; no agent permission/config rewrites. Existing workspace path, same-name/agent-override and trust tests are appropriate. No extra architecture finding. |
| Grok Build | Fix C1 and C2 concretely | Choose project plugin directory as primary only after pinned list/source observation. Test cross-read Claude-compatible plugin/skill same-name sources too: official Grok docs read Claude conventions in addition to `.grok`, so A/B/user `.grok` alone does not exhaust native collisions. Do not implement a second direct-TOML fallback speculatively; return a precise unsupported/prepared MCP result until separately qualified. |
| Qoder | Add retained-native-data command contract | All project mutating native commands need `--scope project`, including replacement uninstall and remove. Use documented `--keep-data` on project uninstall/reinstall whenever data retention is promised. Existing user adapter uninstalls without it; keep unrelated user behavior untouched and explicitly qualify project native/UAP data markers. Listing has no documented scope flag; inspect returned scope/source from fixed cwd rather than inventing `list --scope`. |
| Cline | Make session preparation non-durable and safe launch explicit | `CLINE_MCP_SETTINGS_PATH` selects a process configuration; the ordinary IDE/global path is not project automatic discovery. Prepared MCP remains prepared after a one-session attestation/restart and must display launch instructions again. Do not copy global credentials. The current CLI defaults a prompt/act invocation to autoapproval, so proof must use inert documented subcommands or a launch action that explicitly disables autoapproval; generic `cline --cwd A` cannot be the unattended MCP proof. |
| Windsurf / Devin | Fix C1 and C2 concretely | Record the actual selected harness/version and canonical version source for the v3000.3/Local3.6 transition; the currently opened canonical MCP page documents dedicated files but does not itself state those version numbers. Test `.devin/mcp_config.local.json` shadowing the project file, and never invoke an unscoped `devin mcp remove` that defaults to local while intending project removal. Unknown/legacy Cascade remains bounded, no editor-install inference. |
| Kimi Code | GO after current-doc correction below | Current Kimi Code supports native project `.kimi-code/mcp.json` and `.kimi-code/skills/`; preserve trust, exact owned config and project-over-user precedence. Nearest `.git` skill root mismatch requires a gate. Legacy `.kimi`/`--mcp-config-file` instructions are superseded and must not reach code. See current-doc re-verification below. |
| ChatGPT | Fix saved-admission/preparation distinction | Core excludes remote-only ChatGPT from dependency install admission. Do not add native `ScopeProject` or fake NativeRegistryLayout merely to satisfy the shared project gate. Keep supported project preparation on the existing explicit/manual path (normally `--no-save`) and return unsupported for saved dependency sync/install. The common add/sync/update proof matrix must mark these operations unsupported/preparation-only, not claim a full project lifecycle. No personal/workspace account mutation. |
| VS Code | GO after shared C2/C3 and explicit single-folder limit | Local-harness `.vscode/mcp.json` `servers` is supported compatibility behavior; portable `.mcp.json` is separate experimental behavior. State that first project qualification is a single selected filesystem folder, or explicitly reject multi-root/remote workspace contexts until their authority is qualified. A `code --version`/Copilot listing/fake host must not qualify Local UI discovery. Preserve frozen hooks and unrelated user behavior. |

## Concrete source seams

Source lane introduces typed URL Git and archive pins but no new project paths in manifest/lock/source identity. Client project tests can use local source fixtures for most cases; one qualified project scenario must later exercise a generic remote/archive lock through the selected App to catch accidental user-runtime reconstruction. No transport-by-14-client cross product is needed.

Same-tree shared store entries can carry another source's historical metadata. All client requests must consume provenance rebuilt by source core from the selected validated lock; never derive native identity/repair coordinates from first-writer store metadata or the physical archive wrapper. This is already a fixed source-plan R1 invariant.

## Primary references opened during this review

* Claude plugin command/list/skills-dir contract: https://code.claude.com/docs/en/plugins/cli-reference
* Codex repo plugin changelog: https://developers.openai.com/codex/changelog (currently redirects to https://learn.chatgpt.com/docs/changelog)
* Cursor native/project UI distinction: https://cursor.com/docs/plugins
* Gemini skill precedence: https://geminicli.com/docs/cli/skills/
* OpenCode config merge/precedence and `debug config`: https://opencode.ai/docs/config/
* Copilot native repo formats, ancestor precedence and user-only `mcp remove`: https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-mcp-servers
* Kiro workspace MCP: https://kiro.dev/docs/mcp/configuration/
* Grok `.grok` and Claude-compatible discovery: https://docs.x.ai/build/features/skills-plugins-marketplaces
* Qoder scope/list/uninstall `--keep-data`: https://docs.qoder.com/cli/plugins
* Cline project paths and current CLI autoapproval defaults: https://docs.cline.bot/getting-started/config and https://docs.cline.bot/cli/cli-reference
* Devin dedicated project/local MCP config and default local scope: https://docs.devin.ai/cli/extensibility/mcp/configuration
* Kimi nearest Git root and overriding extra skill flag: https://moonshotai.github.io/kimi-cli/en/customization/skills.html ; session MCP: https://moonshotai.github.io/kimi-cli/en/customization/mcp.html
* ChatGPT account/workspace plugin boundary: https://help.openai.com/en/articles/20001256-plugins-in-chatgpt
* VS Code Local versus portable MCP destination: https://code.visualstudio.com/docs/agent-customization/mcp-servers

All source facts above are paraphrased narrowly; no native runtime proof is implied by documentation. Author applies fixes to each client section and ledger, reviewer verifies the revision, then project_core performs independent R2. Root owns implementation authorization.

## Applied-fix verification

Verified the author's stable revised `client-plans.md` on 2026-10-23. All shared C1–C3 and per-client findings above are explicitly addressed: one Codex component/Grok plugin/current Devin lane; effective configuration and rendered command skills; Copilot formats/exact removal; Qoder scoped retained-data commands; Cline explicit autoapproval-off action and inert proof; Kimi root/session gates; ChatGPT preparation outside saved admission; single-folder Local VS Code qualification. **R1 GO for all 14 client plans.** Their actual pinned native artifact/help observation remains the stated pre-coding qualification gate. Independent R2 and root authorization remain required.


## Kimi current-documentation correction and R1 re-verification

The archived `kimi-cli` documentation used during initial R1 describes a different product generation. It must not define the current adapter. Re-opened the current official `kimi-code` MCP, skills, plugins and command reference and verified the author's stable corrected section 12, matrix and common lifecycle exceptions.

* Current native project MCP uses `.kimi-code/mcp.json`; user MCP uses `~/.kimi-code/mcp.json`/KIMI_CODE_HOME. Project definitions override same-name user entries, but folder trust is required; headless untrusted runs do not load them. Do not grant trust or use KIMI_CODE_TRUST_WORKSPACE to force evidence.
* Project skills use `.kimi-code/skills` plus `.agents/skills`, based on nearest `.git` ancestor. Reject/report a selected-root mismatch; --skills-dir replaces auto-discovery and is not a silent fix.
* Plugin installation remains user-only; project component delivery does not install a project plugin or mutate user plugins/managed. Existing UAP `.kimi-code` user-root handling does not need an unrelated legacy migration.
* Deletion verifies the exact owned project config entry and fresh-session absence. Existing sessions retain removed entries/tools with a removal notice, so absence from that old list is not the success gate.
* Legacy --mcp-config-file/session-preparation language is removed for current Kimi Code; Cline alone retains that session-selected MCP exception. No archived no-argument launcher, model task, --auto/--yolo/prompt automatic approval or trust override supplies native proof.

**Kimi R1 re-verification GO.** This supersedes earlier Kimi session-preparation review wording. Independent R2 and the parent code-authorization gate remain required.

Current primary references: https://moonshotai.github.io/kimi-code/en/customization/mcp ; https://moonshotai.github.io/kimi-code/en/customization/skills ; https://moonshotai.github.io/kimi-code/en/customization/plugins ; https://moonshotai.github.io/kimi-code/en/reference/kimi-command .

## Scoped lifecycle supplement verification

Verified the final new shared/client clauses after core R2: one core-owned scoped effective-policy helper covers the existing provider native-kernel gate, lifecycle/attempt persistence, projection/readiness, recovery/repair/remove, physical backend/sibling ownership, validation and data receipts. Its project policy has no fallback to user CLI traits, and user behavior remains unchanged. The Copilot/VS Code independent MCP backend test plus unresolved native-attempt crash test cover the concrete risks. Cline's existing ActivationPreflighter runs before the composed generic attestation branch, so its session-only MCP cannot be falsely promoted by a flag. ChatGPT now truthfully rejects all project requests because no accepted preparation artifact route exists. VS Code uses a new project-only adapter/constructor, leaving New/NewLocal and user saved admission unchanged. **R1 supplement GO; no additional findings.**
