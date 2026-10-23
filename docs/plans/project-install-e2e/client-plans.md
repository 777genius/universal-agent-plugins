# Project installation: separate plans for all 14 supported clients

Status: **both independent plan review/fix rounds GO; implementation is
not authorized by this document alone**. Prepared 2026-10-23 against source
`2bf6c3d703640ffbecc14346996709b4aec36ddc`; parent refreshed the implementation
base to `3ea24e3014ddf6679ca86729c56f1069112b0d3a`. Recheck every touched source
against that base before coding. Every client is a separate PR stacked on the
project core PR, with incremental reviewed commits. No native-client proof was
executed while preparing these plans. PATH inventory found only `code`; that
alone is not client identification or compatibility evidence.

## Governing decisions and shared contract

Read repository AGENTS, capability preservation, and active owner decisions.
The latest explicit owner request covers all supported agents; parent records
the narrow exception for **VS Code project-scope installation** while retaining
the unrelated VS Code freeze. All workers/reviewers use the owner-selected
GPT-6.1 high/xhigh. Preserve user behavior, existing useful source/tests, one
standard authoring engine, and historical release evidence.

Use core's validated `TargetContext` (scope, project root, original user config
root), `ProjectTargetLayout`, project-aware native registry selection, and
context propagation through activation **and deactivation**. Never encode a
project as a fake user profile, change HOME for product behavior, use an
unrelated editor executable as a CLI, or infer project authority from a package
path. Client stacks add project scope to domain capabilities only when the
implemented capability is truthful. Package support is separate from native
discovery and runtime verification.

Each stack reuses the existing projector, exact native-config kernel,
transaction/receipt, immutable package and security gates. UAP mutable state,
package data, and prepared packages are project-local under core's `.uap/`;
host-discovered skills/configuration use the documented paths below. Native
object receipts own precise entries/directories, never entire shared config
files. Refuse foreign names, disabled entries, unsafe aliases and drift rather
than overwrite them. Direct JSON/JSONC/TOML edits preserve foreign data and
comments through a suitable existing codec; add only a narrowly needed codec.

For every adapter, observe the effective selected configuration and meaningful
higher-priority admin/user/project/session sources. A shadowed, disabled or
unloaded entry stays configured/prepared/blocked with a specific reason;
neither a file receipt nor operator attestation manufactures durable native
discovery. Reuse existing rendered command-skill projection and supported
component selection, including command behavior, rather than copying raw
`skills/` alone. Record pinned official artifact/version and inspected help/list
schema before each adapter implementation. Where a version/harness is not
observed, return prepared/unsupported instead of implementing speculative
fallback lifecycle systems. Initial read-only native probes do not authorize
model tasks, account mutation, auto-trust, credential creation or scanner upload.
For any known shadowed/disabled project result, use the existing activation
preflight before generic attestation to refuse `ActivationComplete` as a bypass;
preserve user attestation behavior. Composed-provider regression coverage must
prove that a positive flag cannot override this recognized negative evidence.

### Core coordination required for project lifecycle policy

Core owns one narrow scoped effective-policy helper before client implementation.
Current `SelectedDelivery.EffectiveTraits` and zero-mode backend sharing default
to each client's user traits. Codex/Cursor/Copilot/VS Code user CLI-registry
traits therefore cannot govern project direct-config delivery. Add declarative
project policy to the existing client definitions (qualified client stacks fill
their rows) and a scoped helper selecting it from validated target context and
the existing persisted binding scope. User scope retains the exact existing
selected-delivery/default traits. Project scope has no fallback to user policy
or an old TEST user-profile selected mode.

Direct project component lanes select native-config lifecycle/attempt/ownership
and recovery policy; actual CLI plugin lanes retain their real CLI policy.
The helper must govern `providers` native-kernel admission, usecase
`nativeLifecycleClient`, pre-mutation `NativeActivationAttempt`, native
projection/readiness, repair/remove/recovery, and installer binding/profile
validation. It also governs physical backend selection, group collision/adoption,
and data receipts. Core updates these shared consumers once; each client does
not introduce a separate engine or policy branch. Operational policy/root data
does not enter portable manifest/lock/source identity or public path output.

For project Copilot and VS Code, declare separate physical backend/registry
identity because root `.mcp.json` and `.vscode/mcp.json` are distinct. Scoped
backend/sibling helpers must fence adoption/removal by actual context/layout;
zero-mode `SharesBackend` must not borrow the user's Copilot/VS Code alias.
Preserve current user backend sharing. Real overlapping skill paths retain
exact collision/ownership checks and cannot be silently adopted.

Required core contracts: interrupt a CLI-user/native-config-project lane after
its native edit and before acknowledgment; the persisted unresolved attempt
must force safe recovery/refusal rather than blind rewrite or user CLI fallback.
In one project install distinct Copilot and VS Code MCP entries, then maintain
or remove one while preserving the other's entries and receipts; repeat a user
case to prove existing shared-backend behavior unchanged. Production project
composition, scope-aware saved admission, and configured/attestation boundaries
must be tested through the real provider path, not only direct adapter calls.

## Delivery classification

| Client | Project skills | Project MCP | Preferred first implementation |
| --- | --- | --- | --- |
| Claude | Native skills-directory plugin | Bundled native plugin MCP | Existing package layout under project `.claude/skills/`, exact project listing |
| Codex | `.agents/skills/` | `.codex/config.toml` | Project component projection; repo marketplace remains a separate deferred qualification |
| Cursor | `.cursor/skills/` | `.cursor/mcp.json` | Native project component projection; UI plugin installation remains separate/manual |
| Gemini | `.gemini/skills/` | `.gemini/settings.json` | Native project component projection; no project extension-registry claim |
| OpenCode | `.opencode/skills/` | Root `opencode.json[c]` | Native project component projection with existing transactional ownership |
| Copilot CLI | `.github/skills/` | Root `.mcp.json` or `.github/mcp.json` | Native project component projection, not global plugin install |
| Kiro | `.kiro/skills/` | `.kiro/settings/mcp.json` | Native project component projection, IDE/CLI version gated |
| Grok Build | Project plugin skills | Bundled project plugin MCP | One pinned project plugin-directory lane; unobserved support stays prepared/unsupported |
| Qoder CLI | Native project plugin | Native project plugin | Native `plugins install --scope project` and scoped removal |
| Cline | `.cline/skills/` | Explicit session-selected file only | Native project skills; MCP prepared with documented `CLINE_MCP_SETTINGS_PATH` launch action |
| Windsurf | `.devin/skills/` | Current Devin Local dedicated project MCP config | One pinned newer Devin Local lane; older/unknown/Cascade project MCP unsupported |
| Kimi Code | `.kimi-code/skills/` | `.kimi-code/mcp.json` | Current native project components; legacy kimi-cli and project plugin registry excluded |
| ChatGPT | No native project install | No native project install | Explicit pre-acquisition unsupported for project requests; existing user manual preparation preserved |
| VS Code | Supported workspace skill paths | `.vscode/mcp.json` for Local harness | Project Local-harness components; keep Copilot CLI and experimental portable config distinct |

## Common validation required in every client PR

**Contract/fixture coverage:** use the public installer and production CLI
composition with disposable user profile and projects A/B. Exercise add,
unchanged sync, moving-ref update, exact-pin repair after drift, status,
interrupted operation/recovery, and remove. Assert that the selected native
paths, cwd, scope flags, receipts and data refer to A; B and user files remain
byte-identical. Verify repeat removal, foreign same-name/disabled entries,
malformed configuration, symlink escape, changed ambient cwd/home, copied
mutable state, and unsupported/mixed component selections. A failed native
operation must retain uncertain/pending ownership until exact verification or
the existing explicit completion contract resolves it. Fixtures do not prove
that a real client discovers or executes a component.

**Native evidence:** acquire a pinned official binary/package in the existing
disposable-host test policy; record source URL, version, artifact integrity,
binary digest, platform, command/cwd/config scope, and sanitized observations.
Use a local harmless marker MCP fixture and a distinctive skill with recorded
server handshake/tool result and real client discovery. Prove fresh restart
behavior, A/B/user precedence, targeted update/repair/removal and no residual
managed registration. Do not invoke model tasks, grant trust, add credentials,
use YOLO/AFK or upload source to scanners to make a test pass. If native
inspection needs login/UI or an unavailable official artifact, report that
boundary explicitly: configuration/fixture proof can pass while runtime proof
is pending. Native installed listing, native discovery, MCP initialization,
tool invocation, authentication and release qualification are separate facts.

The current native matrix provisions only Claude/Codex/OpenCode. Extend its
pin/evidence plumbing only for a client actually being tested; use separate
manual UI evidence for editor-only behavior. A third-party imitation binary,
shell script fixture or locally authored parser never counts as that client.

The common saved lifecycle applies only to admitted saved dependencies.
ChatGPT is excluded: all project requests, including `--no-save`, fail before
acquisition with its precise unsupported reason; existing user preparation stays
unchanged. Cline
session MCP preparation remains prepared across restart and any one-session
attestation; display the explicit launch action again. Native project skills
can be installed independently. Do not persist a session observation as a
durable host registration. One qualified project source-seam test later consumes
a generic Git/archive lock through the selected App; the 14 clients do not need
a transport-by-client cross product.

## 1. Claude Code — native project skills-directory plugin

**Evidence/choice.** Official plugin list documents skills-directory plugins
with `id=name@skills-dir`, `scope=user|project`, `enabled`, and `installPath`.
This matches existing UAP's user layout and avoids a new marketplace/cache
lifecycle. Prefer `<project>/.claude/skills/<artifact>` with existing projected
plugin/MCP layout; use project cwd for all native list/probes. Official
marketplace install `--scope project` is an alternative only if this directory
contract is unavailable in the pinned client, and must be separately qualified.
Sources: [CLI reference](https://code.claude.com/docs/en/plugins/cli-reference),
[install scopes](https://code.claude.com/docs/en/plugins/install).

**Changes.** `clients/claude/{plan,project,identity,list,probe,lifecycle}.go` and
dedicated tests: add project layout/registry selection, preserve staging outside
the watched `skills` directory, parameterize exact listing scope without
weakening user-only callers, and pass immutable project cwd/context. Keep user
`CLAUDE_CONFIG_DIR` semantics; never set it to the project. Scope-aware
deactivation verifies exact project identity before owned-directory removal.

**Proof/gates.** Real `claude plugin list --json` must find project identity at
the exact path, reject same-name user/path collisions, and stop listing it after
removal. A harmless skill and MCP runtime fixture prove discovery separately.
Project trust/security policy remains a user action; admin allow/block and
scope-specific disabled entries remain authoritative. Unknown listing versions
stay manual/unverified. Test worktree and nested cwd behavior, because plugin
marketplace relative paths and skills-directory discovery can differ.

## 2. Codex — native project components

**Evidence/choice.** Choose one first implementation: native project skills in
`.agents/skills/` and MCP in trusted-project `.codex/config.toml`. Official
changelog also documents repo marketplaces, but cache/install scope isolation
is a separate qualification and will not be implemented speculatively here.
Sources: [skills](https://developers.openai.com/codex/build-skills),
[MCP](https://developers.openai.com/codex/mcp),
[project config](https://developers.openai.com/codex/config-file/config-advanced).

**Changes.** `clients/codex/{plan,project,identity,lifecycle,lifecycle_deactivate,
command,profile,list}.go` only as needed plus scoped tests. Keep current user
CLI marketplace path intact. Inspect pinned native project config/help first;
write exact project MCP TOML entries and skill objects transactionally with
foreign members/comments preserved. All CLI commands preserve selected user
`CODEX_HOME` and set `Dir=ProjectRoot`. Project operations do not invoke user-only
`plugin add`, marketplace remove or global-cache cleanup.

**Limits.** Unknown/unobserved versions stay prepared/unsupported for components
not proved; no second plugin lifecycle fallback. Exact TOML member names and
skill collision behavior must come from pinned native help/source/probe, not
guessed from removed documentation routes. Repo/user/admin skills with the same
name may coexist in selectors; do not claim the managed one won by filename.
No auto-trust writes.

**Proof.** Native A/B/user different versions, restart skill discovery, effective
MCP config and scoped entry removal prove component isolation. This makes no
installed-plugin isolation claim. Fixture config/command contracts remain
separate from genuine `codex` proof.

## 3. Cursor — project components, plugin UI remains separate

**Evidence/choice.** Official docs support `.cursor/mcp.json` and CLI/editor
shared MCP configuration. Plugin UI offers project/user scope but documents no
stable unattended project installer. Project skill discovery uses
`.cursor/skills/`; verify with the selected actual editor/agent version.
Sources: [MCP](https://cursor.com/docs/mcp),
[CLI MCP](https://cursor.com/docs/cli/mcp),
[plugins](https://cursor.com/docs/plugins).

**Changes.** `clients/cursor/{plan,profile,project,identity,lifecycle}.go`, README
and new dedicated project tests. Retain existing manual editor package contract
for user scope. Implement owned project skills and exact `.cursor/mcp.json`
members, with existing managed stdio launcher and project data. Use project
registry authority and a matching codec; never guess a `.cursor/plugins/local`
project registry, use editor `--user-data-dir` as HOME, or invoke ambiguous
`agent` as a detected CLI. Component configuration can be installed while
native activation remains manual until observed.

**Proof.** Fixture E2E checks merge/no-op/update/remove, foreign disabled entries,
user hooks/config unchanged, and editor/agent-only detection. Real pinned
editor or official `cursor-agent`/`agent` version must load project MCP/skill
in A and not B; CLI and UI evidence are separate. No genuine Stop or plugin
discovery claim follows from this component work.

## 4. Gemini CLI — native project skills and settings

**Evidence/choice.** Project settings are `.gemini/settings.json`; project skills
are `.gemini/skills/` or `.agents/skills/`. Extensions use a separate installed
extension lifecycle, so project install projects components instead of inventing
a project extension registry. Sources:
[configuration](https://geminicli.com/docs/reference/configuration/),
[skills](https://geminicli.com/docs/cli/skills/),
[extension reference](https://geminicli.com/docs/extensions/reference/).

**Changes.** `clients/gemini/{plan,project,identity,native_apply,native_mcp,
native_skills,native_objects,profile}.go` and dedicated project tests. Resolve
actual project `.gemini` root from context; preserve detected user root and
GEMINI_CLI_HOME as user-profile authority. Reuse exact settings MCP codec,
skill ownership, rollback and no-op receipts. Keep package projection under
`.uap/` distinct from the native project settings/skills tree.

**Proof/limits.** Native `gemini mcp list` and skills list from A plus fresh
restart must observe selected project components; B and user remain unchanged.
Test alias precedence when `.agents/skills` has a foreign same-name skill;
report the managed skill shadowed/inactive rather than successfully activated.
Preserve disabled workspace settings without consent/autoenable. Also test
malformed/commented settings, conflicting user server and trust gating. A
settings-file receipt proves configuration, not initialized MCP/tools or
authentication. No extension install/global profile redirection is needed.

**Implemented checkpoint (2026-10-23).** `65b8f17a` qualifies the native
project component lane with independent implementation
[R1](gemini-code-review-r1.md) and [R2](gemini-code-review-r2.md) GO. Project
configuration success remains prepared with installed exact objects. Native
session discovery, workspace trust, system policy and MCP initialization remain
unobserved; activation attestation cannot upgrade that status. Bounded
user/workspace restriction and alias warnings preserve policy and foreign
files. [Pinned evidence and verification](gemini-implementation-evidence.md)
separate actual version/help research, production composition contracts and
remaining hosted native runtime qualification.

## 5. OpenCode — project config at repository root

**Evidence/choice.** Native project MCP uses `<project>/opencode.json` or
`opencode.jsonc`; skills use `.opencode/skills/`. These roots are different,
so a project cannot be represented by substituting one fake config root into
the user adapter. Sources: [config](https://opencode.ai/docs/config/),
[skills](https://opencode.ai/docs/skills/),
[MCP](https://opencode.ai/docs/mcp-servers/).

**Changes.** `clients/opencode/{plan,native*,identity,profile,project}.go` only
as required. Add an internal resolved native layout containing exact config
paths and skill directory, selected from context. Feed that layout through
existing codec/namespace/transition/recovery logic. Preserve JSON-vs-JSONC
selection rules, comments, exact owned MCP keys, tool exclusions and existing
user v1/v2 behavior. Do not widen decoder precedence or transplant state.

**Proof.** Extend genuine native matrix using pinned current OpenCode artifact:
real config read/MCP list/skill discovery, local marker tool, A/B/user versions,
restart and removal. Contract tests inject crashes across settings+skill
transition, check exact repair/no-op/foreign edits and preserved plugin/tool
namespaces. Project config/native roots must remain stable when operation cwd
changes. Observe actual effective config through pinned `opencode debug config`
or its documented equivalent; cover higher-priority inline/managed/custom
directory overrides and rooted ancestor search. A shadowed managed entry remains
configured/prepared, never active solely because root config exists.
Source/package extension work remains separate from native JS plugin
APIs, which this task does not expand.

## 6. GitHub Copilot CLI — repository components

**Evidence/choice.** Official per-repository MCP docs load root `.mcp.json` and
`.github/mcp.json`, with `.mcp.json` precedence in the same directory; project
files require folder trust. Skills use `.github/skills/`. Copilot's installed
plugin command is not a documented project-scope installer. Sources:
[CLI MCP](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-mcp-servers),
[plugin reference](https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-plugin-reference).

**Changes.** `clients/copilot/{plan,project,identity,lifecycle}.go` and scoped
tests. Project branch must bypass shared global Copilot plugin activation and
deactivation. Select the existing effective repository MCP file conservatively
(root `.mcp.json` if present; otherwise `.github/mcp.json`, preserving foreign
members), manage exact owned servers and skills, and run inspection from fixed
project cwd. Existing project documents can be nested `mcpServers` or bare
server maps: support both through one format-preserving codec or reject a bare
map before effects; never wrap it and lose foreign servers. Remove exact
transactional project keys, never user-config `copilot mcp remove`.
Keep `.vscode/mcp.json` distinct; CLI does not read its `servers`
shape. Refuse ambiguous nested overriding configs rather than verify the
wrong effective server.

**Proof/limits.** Genuine pinned Copilot CLI listing/config inspection plus
local MCP handshake and skill discovery after explicit trust. Do not set
GITHUB_COPILOT_PROMPT_MODE_WORKSPACE_MCP to bypass trust. Native auth-required
inspection remains pending when account is absent. Fixtures assert no global
plugin install/uninstall, no user config writes and correct precedence.

## 7. Kiro — workspace components with explicit surface support

**Evidence/choice.** Official workspace paths are `.kiro/settings/mcp.json` and
`.kiro/skills/`. Agent `mcpServers` overrides workspace MCP; custom agents may
need `includeMcpJson`. Sources: [MCP config](https://kiro.dev/docs/mcp/configuration/),
[configuration scopes](https://kiro.dev/docs/configuration/),
[agent configuration](https://kiro.dev/docs/custom-agents/configuration-reference/).

**Changes.** `clients/kiro/{plan,project,identity,activate,lifecycle,native*}.go`
as applicable, and project tests. Preserve IDE-vs-CLI detection. Use exact
project root paths via layout/context for MCP+skills and retain existing
transaction/capacity/rollback checks. Remove current user-only prepared-scope
assumption only in the project branch. Do not edit active custom-agent tool
permissions/include flags to force loading; report that surface requirement.

**Proof/limits.** Native Kiro CLI/IDE discovery at the pinned version must show
workspace MCP and skills; test same-name user server plus agent override,
workspace trust, restart, project remove and disabled config. A CLI check
cannot qualify the IDE, mobile, web or Powers install. An unavailable official
artifact/account yields explicitly bounded configuration/fixture evidence.

## 8. Grok Build — native project plugin directory

**Evidence/choice.** Official docs discover `./.grok/plugins/`, separate user
plugins, and project `.grok/config.toml`; `grok inspect` reports loaded
configuration. Prefer project plugin directory delivery if current real
`plugin list` proves its exact project source. Sources:
[plugins](https://docs.x.ai/build/features/skills-plugins-marketplaces),
[settings](https://docs.x.ai/build/settings),
[MCP scope](https://docs.x.ai/build/features/mcp-servers).

**Changes.** `clients/grok/{plan,project,registry,lifecycle}.go` and dedicated
tests. Place package under `.grok/plugins/<artifact>`, preserve the current
projector, add scoped registry/list classification and fixed project cwd.
Project install/remove must not execute current global `plugin install --trust`
or global uninstall. Freeze one project-directory lane after pinned native
observation; if bundled MCP is not proved, leave it prepared/unsupported with
its exact reason. Do not implement a second direct-TOML fallback speculatively.
Never auto-run `/hooks-trust`.

**Proof.** Official pinned Grok artifact, actual project plugin list/inspect,
skill discovery and harmless MCP initialization. Verify user same-name plugin,
disabled project plugin, nested config precedence, A/B versions and removal.
Include Claude-compatible project/user plugin and skill same-name discovery,
which Grok reads in addition to `.grok`; distinguish shadowed/disabled results.
No claim about remote Grok Bot, arbitrary `grok`-named binaries, or hook trust
follows from local project installation.

## 9. Qoder CLI — native project registry lifecycle

**Evidence/choice.** Official `qoder plugins install PATH --scope project`
writes project settings; `uninstall NAME --scope project` is documented.
Local-scope settings are a distinct optional UX, not the same as project.
Source: [Qoder plugins](https://docs.qoder.com/cli/plugins).

**Changes.** `clients/qoder/{plan,project,registry,lifecycle}.go` and dedicated
tests. Keep user paths/commands unchanged. Project package source stays under
`.uap/`; run install/list/uninstall from immutable project cwd with explicit
project scope on every mutating command, including replacement uninstall and
final remove. Add documented `--keep-data` whenever retaining native plugin
data, and prove separate UAP/native data markers survive replacement. Listing
has no assumed scope flag: inspect actual scope/source returned from fixed cwd.
Match qualified plugin ID, source,
effective enabled state, scope and version; refuse foreign same-name entries.
Audit scoped listing schema against pinned CLI rather than assume adding a
flag to the existing user parser suffices. Capture exact project settings/native
authority for recovery and removal.

**Proof/limits.** Genuine current Qoder CLI must confirm project settings,
scope/source/version, skills and MCP after restart, with A/B and user at
different versions. No-version output remains manual/unverified under existing
policy. Failed reinstall cannot succeed from a stale listing. Tests cover
disabled registration, same-version changed bytes, partial install/uninstall
and unrelated cwd. No direct raw settings edit unless native commands cannot
express an already documented operation.

## 10. Cline — native skills, explicit session MCP preparation

**Evidence/choice.** Official config lists project `.cline/skills/`, hooks,
agents and plugins; MCP settings remain global under
`~/.cline/data/settings/cline_mcp_settings.json`. CLI documents
`CLINE_MCP_SETTINGS_PATH`. This is an explicit process-selected MCP file, not
automatic project discovery. Sources:
[configuration](https://docs.cline.bot/getting-started/config),
[CLI reference](https://docs.cline.bot/cli/cli-reference).

**Changes.** `clients/cline/{plan,project,identity,lifecycle,native*}.go` plus
project tests. Use `.cline/skills/<skill>` for native project skills. Prepare
owned MCP config under `.uap/clients/cline/` and a reviewable launch action
fixing cwd and CLINE_MCP_SETTINGS_PATH to that file. Keep mixed MCP install
prepared/manual even after a one-session observation; ordinary restart again
requires the explicit launch action. Do not redirect CLINE_DIR or HOME,
copy global secrets into a project, or claim ordinary Cline startup loads the
file. Cline JS/TS plugins differ from portable MCP plugins and are outside this
initial projection.

Guard attestation at the real composed provider boundary. Implement Cline's
existing `ActivationPreflighter` hook to reject `ActivationComplete` whenever
project MCP is session-selected; `providers.Activator.Activate` invokes that
hook before its generic manual-attestation branch. Reuse the same validation
for direct adapter activation. An adapter's eventual prepared return alone
does not prevent generic attestation from bypassing it. No new policy framework
or generic client-ID switch is needed. Real composed-provider tests cover this
flag during add/repair, restarted sessions and persisted binding reload, proving
that no active/installation-verified record is fabricated.

**Proof.** Actual pinned Cline CLI with documented environment override must
load marker MCP through inert documented MCP subcommands; a generic prompt/act
invocation defaults to autoapproval. Any displayed session launch action must
include documented `--auto-approve false` and fixed cwd. Do not run an unattended
prompt/act invocation as MCP proof. Normal startup must not be called
project-MCP activation.
Prove project skill discovery independently. Verify user global settings
unchanged, explicit override behavior, project removal and UI-vs-CLI limitation.

## 11. Windsurf / Devin Desktop — harness- and version-aware project path

**Evidence/choice.** Devin Local is distinct from legacy Cascade. Canonical
Devin MCP/configuration docs identify v3000.3/Local3.6 moving entries to dedicated project
`.devin/mcp_config.json` and local `.devin/mcp_config.local.json`; older main
config entries auto-migrate. Cascade retains a global `mcp_config.json`, so it
cannot be advertised as project MCP. Sources:
[canonical MCP config](https://docs.devin.ai/cli/extensibility/mcp/configuration),
[Desktop Local harness](https://docs.devin.ai/desktop/devin-local),
[versioned file locations](https://docs.devin.ai/cli/reference/configuration/config-file),
[skills](https://docs.devin.ai/cli/extensibility/skills/overview).

**Changes.** `clients/windsurf/{detect,plan,project,identity,lifecycle,native*}.go`
plus dedicated tests. Add explicit detected/selected harness capability and
version, keeping legacy Cascade user mutation behavior. Freeze the first lane
to one pinned official newer Devin Local artifact, with `.devin/skills/` and
dedicated `.devin/mcp_config.json`. Older, unknown or unobserved
harness/version stays prepared/manual or rejects MCP with a precise reason;
never infer Devin Local merely from an editor install. Avoid triggering native
migration by writing obsolete project config on new clients.

**Proof.** Genuine official Devin CLI can qualify shared harness configuration;
Desktop UI evidence separately qualifies editor discovery. Test selected newer
layout and explicit older/unknown rejection, no duplicate active MCP source,
`.devin/mcp_config.local.json` shadowing, same-name Cascade/user server,
A/B isolation, restart and removal. Plugin install remains user/account-level;
component projection avoids changing personal cloud plugin manifests.
Remove only exact transactional project entries; never invoke unscoped
`devin mcp remove` while intending project removal, because its default is local.

## 12. Kimi Code — current native project components

**Evidence/choice.** The former `MoonshotAI/kimi-cli` was archived in September
2026; its `.kimi` paths and `--mcp-config-file` instructions must not define the
current client. The official migration link leads to `MoonshotAI/kimi-code`.
Current docs explicitly support project `.kimi-code/mcp.json` and
`.kimi-code/skills/` (plus `.agents/skills/`); user data defaults to
`~/.kimi-code`, matching UAP's existing detected root. Plugin installation
remains user-only. Sources:
[current MCP](https://moonshotai.github.io/kimi-code/en/customization/mcp),
[current skills](https://moonshotai.github.io/kimi-code/en/customization/skills),
[current plugins](https://moonshotai.github.io/kimi-code/en/customization/plugins),
[migration](https://moonshotai.github.io/kimi-code/en/guides/migration).

**Changes.** `clients/kimi/{adapter,project,identity,lifecycle,registry}.go` and
project tests. Choose one first lane: current pinned Kimi Code project component
projection, exact owned MCP members in `.kimi-code/mcp.json` plus skills under
`.kimi-code/skills/`, using the existing no-follow/native-config transactions.
Do not install a project plugin in the user `plugins/managed` registry, rewrite
KIMI_CODE_HOME, or apply legacy CLI flags. Preserve disabled foreign MCP and
user config. Observe `enabled`, effective project-over-user precedence, trust,
component naming, and actual native startup before promoting readiness.

**Proof/limits.** Pin the current official Kimi Code artifact and help/schema;
do not install or launch archived `kimi-cli` (its final no-argument entrypoint
can download a replacement installer). Native project MCP requires independently
established folder trust. Never set KIMI_CODE_TRUST_WORKSPACE or use `--auto`/
`--yolo` to force proof; untrusted/headless discovery stays configured/blocked.
Real fresh-session MCP/skill discovery, A/B/user collision, update/repair/remove,
no-session residual registration, and retained data are required. Existing open
sessions retain removed MCP entries and visible tools that fail with a removal
notice; new sessions omit the tools. Removal verifies exact project entry
deletion and fresh-session absence, never absence from an already-open session.
Skills resolve at the nearest `.git` ancestor;
if that differs from selected ProjectRoot, reject automatic skill activation
before acquisition/effects or report the precise prepared/unsupported state.
`--skills-dir` replaces ordinary discovery and must not be inserted silently.

## 13. ChatGPT — explicit unsupported project installation

**Evidence/choice.** ChatGPT plugins use personal/workspace installation and
authorized app connections. Local Work/Codex folder access is not a native
ChatGPT filesystem plugin scope. The accepted core gate has no artifact-only
project-preparation route. Choose explicit unsupported project installation
instead of adding a fake native layout or user context. Sources:
[plugins](https://help.openai.com/en/articles/20001256-plugins-in-chatgpt),
[Work and Codex](https://help.openai.com/en/articles/20001275-chatgpt-work-and-codex).

**Changes.** Dedicated ChatGPT project capability/CLI tests and precise
unsupported guidance, plus `clients/chatgpt/{plan,identity,lifecycle}.go` only
where a defensive diagnostic is needed. Reject `--scope project` before source
acquisition or effects for both saved dependencies and imperative `--no-save`.
Do not add native `ScopeProject`, `ProjectTargetLayout`, `NativeRegistryLayout`
or a bypass of the shared project gate. Keep existing user-scoped validated
personal/manual preparation behavior unchanged; a future artifact-only project
preparation path needs its own reviewed contract and is not this PR.

**Proof/limits.** E2E proves project requests fail with a stable unsupported
reason, make zero acquisition calls, and do not create `.uap/`, intent, lock,
state, native or remote mutations. Verify existing user preparation still
works under its prior constraints and lifecycle. No generic MCP harness, Codex
CLI result or operator attestation qualifies a ChatGPT app connection,
authentication, workspace publication or project installation. This client PR
reports the product limitation rather than a false completed project lifecycle.

## 14. VS Code — narrowly reopened project Local-harness components

**Evidence/choice.** Current VS Code docs retain `.vscode/mcp.json` with top-level
`servers` for the Local harness. The newer Copilot harness portable root
`.mcp.json` is experimental/gated and is a separate contract. Docs label the
older guided-flow destination deprecated but preserve compatibility.
Source: [MCP destinations](https://code.visualstudio.com/docs/agent-customization/mcp-servers).

**Changes.** Add a project-only VS Code component adapter/constructor in new
`clients/vscode/project_*.go` files, reusing neutral rendering/native-config
helpers only where safe. The project App composition registers that adapter
for the VS Code ID; user composition keeps `vscode.New()` unchanged. Existing
`NewLocal` is an explicitly TEST-qualified profile plugin-registration route,
not a product workspace MCP adapter, and must not be relabeled as one. Project
composition/registry tests must prove actual provider dispatch reaches this
new adapter instead of the default Copilot plugin route. Update the shared
scope-aware saved admission only for the new qualified project adapter:
`ScopeProject` plus both required project hooks qualifies it; legacy
`SavedDependenciesDeferred`/user saved admission stays unchanged. Generic code
uses the existing scope/hook declarations rather than a VS Code ID switch.
Select actual Local harness explicitly; use owned
`.vscode/mcp.json` entries and documented workspace skills. Do not invoke
Copilot global plugin install/uninstall for this path, silently enable an
experimental setting, or reinterpret user profile as project. Maintain exact
scope/harness authority across recovery/remove.
First qualification is one selected local filesystem folder. Reject multi-root
and remote workspace contexts until their authority is separately qualified.

**Proof/limits.** Real pinned VS Code extension-host/UI discovery must load
workspace MCP and skill after trust and restart; `code --version` or Copilot
CLI listing is insufficient. Headless extension-host fixture proves protocol
only when it is not the actual shipped host. Test `.vscode` server shape,
foreign inputs/secrets/disabled entries, no new trust grants, A/B/user isolation
and remove. Native VS Code hooks and other frozen implementation remain out of
scope.

## Review ledger

R1 by sources_lead: findings at `client-plans-review-r1.md`; author applied the
shared effective-config/command-skill/evidence rules and each client-specific
fix on 2026-10-23. Codex now has one component lane, Grok one project-plugin
lane, and Devin one newer dedicated-MCP lane; no speculative dual lifecycle.
Pinned native artifact/help observation remains a pre-coding gate in each lane.
A later migration-link check corrected Kimi to current native project components;
its old kimi-cli sources were archived. sources_lead independently verified the
current Kimi correction R1 GO.
Copilot preserves bare/nested formats and removes exact project keys; Qoder
retains native data with scoped `--keep-data`; Cline remains session-prepared; current Kimi Code uses native project components;
ChatGPT rejects all project requests before acquisition; VS Code first scope is one local
folder. sources_lead verified R1 fixes; project_core independently verified all
R2 integration fixes and granted all-14 R2 GO.

R2 integration fixes applied: Cline's existing preflight refuses session MCP
completion before generic attestation; core owns scoped effective lifecycle and
physical backend policy; VS Code gets explicit project registry composition and
project-only saved qualification; ChatGPT's unreachable artifact-only proposal
is replaced by explicit unsupported. R2 fixes independently verified GO in
`client-plans-review-r2.md`; root assigns implementation only after gates.

| Client | Round 1 (sources_lead) | Fixes | Round 2 (project_core) | Implementation |
| --- | --- | --- | --- | --- |
| claude | GO | Verified | GO | Root assignment required |
| codex | GO | Verified | GO | Root assignment required |
| cursor | GO | Verified | GO | Root assignment required |
| gemini | GO | Verified | GO | Implemented at65b8f17a; independent implementation R1/R2 GO; native discovery pending |
| opencode | GO | Verified | GO | Implemented locally; implementation R1/R2 pending; public CLI/native discovery pending |
| copilot | GO | Verified | GO | Root assignment required |
| kiro | GO | Verified | GO | Root assignment required |
| grok | GO | Verified | GO | Root assignment required |
| qoder | GO | Verified | GO | Root assignment required |
| cline | GO | Verified | GO | Root assignment required |
| windsurf | GO | Verified | GO | Root assignment required |
| kimi | GO (current docs) | Verified | GO | Root assignment required |
| chatgpt | GO | Verified | GO | Root assignment required |
| vscode | GO | Verified | GO | Root assignment required |
