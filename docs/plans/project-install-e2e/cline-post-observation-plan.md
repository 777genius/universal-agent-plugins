# Cline project components: concrete post-observation plan

2026-10-24. **Plan only; no project adapter code is authorized by this document
until two independent review rounds are complete.** The owner-authorized
all-client program and original Cline choice in `client-plans.md` remain the
scope. The preserved historical hosted observations are
[recorded with raw output and provenance](../../evidence/project-scope-research/cline-3.0.73-linux-amd64-40017958131/README.md).
Root and the implementing worker independently interpreted them as sufficient
for this concrete plan, not current-head production/native qualification.

## Product boundary and evidence

Pinned Cline CLI 3.0.73 on hosted Linux discovered project `.cline/skills`
markers alongside user skills. `config mcp --json` selected the user settings
file in both project working directories; only explicit
`CLINE_MCP_SETTINGS_PATH` selected a prepared project file. Cross-cwd selection
followed that override, and the next ordinary process returned to user settings.
No same-name precedence, enabled execution, IDE, trust or MCP initialization
was observed. Exact source and native pins, complete raw streams, original false
qualification fields and source-closure differences are preserved in evidence.

Implement owned project **skill directories plus explicit-session MCP
preparation**. All project activation results remain `Prepared`; file receipts
may establish `installation_verified` after exact local verification. That
value means installed component files, never native session discovery or MCP
startup. Initial plans are package-valid; broken/foreign objects fail normally.
Even skills-only results remain conservatively Prepared because production
verification is host-free and no native config enumeration is performed.

| Case | Local delivery | Lifecycle and guidance |
| --- | --- | --- |
| Skill only | Install exact owned skill directories | Prepared; start Cline from the selected project and inspect skills. |
| MCP only or skills + MCP | Install project skills as selected; merge only owned MCP members in the fixed prepared file | Prepared; explicit launch action selects the exact file and cwd. A later ordinary process is not covered. |
| `ActivationComplete` in any project add/resume/update/repair path | Refuse before effects, including composed-provider generic attestation | No attested Active/Installed record. Actual verified file outcomes remain allowed. |
| Foreign/tampered expected target or changed recorded identity | Refuse before mutation | Preserve foreign/project/user bytes and the prior binding. |
| Remove recorded project binding | Verify exact recorded ownership; remove owned skill directories/MCP members only | No native uninstall. Preserve user configuration and unrelated members. Warn that existing sessions can retain state. |

The displayed launch action is data for review, not a command UAP executes.
It uses explicit `CLINE_MCP_SETTINGS_PATH=<absolute prepared file>`,
`CLINE_NO_AUTO_UPDATE=1`, `cline --auto-approve false --cwd <absolute project>`.
Provide correct POSIX and PowerShell quoting or structured instructions for
both platforms; no guessed shell execution, wrapper scripts, changed HOME,
CLINE_DIR, CLINE_DATA_DIR or copied user secrets. Mark CLI-only selection and
IDE behavior separately. No prompt/act/model/MCP-tool command is requested.

## Architecture and exact authority

Use existing `TargetContext`, scoped composition, immutable package store,
receipts, transaction/recovery journal and original-App namespace guards. Add
Cline's explicit project policy as `LifecycleNativeConfig` only with this
adapter. Preserve the entire existing user implementation and user traits.
No generic installer, new settings framework, SDK schema or client-ID branch
in usecase orchestration is needed. Native policy belongs to `clients/cline`;
shared filesystem ownership remains `ProjectComponents`/
`ProjectComponentMutation` and the existing `nativeconfig.CodecCline`.

Add narrow `clients/cline/{scope,project_components,project_lifecycle,
project_identity,project_validation}.go` files and dispatch at the existing
plan/project/identity/activation/removal entry points when either scope field
selects project. New code must validate both fields agree; mismatches fail.

Canonical project paths are:

- Registry namespace: the selected absolute project root (not `.cline` as a
  fabricated user profile), bound separately from original user `ConfigRoot`.
- Managed package: `<project>/.uap/managed/clients/cline/<physical-artifact>`.
- Skills: `<project>/.cline/skills/<skill-name>`.
- Prepared MCP file: `<project>/.uap/clients/cline/mcp.json`.
- Distinct project object kinds: `cline_project_skill_directory` and
  `cline_project_mcp_server`, with `kind:name` IDs and exact digest receipts.

One validator derives those paths from `TargetContext`, validates the selected
client, scope, original user-root identity, physical artifact and containment,
and is reused by planning, projection, observation, activation and removal.
`ProfileBindingValidator` must reject copied user/project kinds, foreign root,
changed artifact/target and out-of-project native objects before applying any
state-loaded operation. Project code must never call ambient-sensitive
`ClineMCPSettingsPath`: hostile `CLINE_MCP_SETTINGS_PATH` and `CLINE_DATA_DIR`
values cannot redirect it. The existing user path behavior stays intact.

`ProjectTargetLayout` supplies the managed path; `NativeRegistryLayout` supplies
the exact project namespace and no native registry executable. Plan refinement
supports only existing portable skill/MCP components, preserves unsupported
commands/agents/hooks as unsupported, and returns truthful Prepared guidance.
No JS/TS plugin, dependency execution, new hook, global alias or native registry
is synthesized. Unknown native config transport/settings schema fails closed
through the current Cline codec; do not broaden it from a guessed native schema.

Projection reuses `ClineNeutralServer` and the existing stdio data contract/
placeholder handling. Project-specific object creation supplies the exact
layout path directly, never the ambient-sensitive user path helper. Use
`ProjectComponents.SkillObject`/`MCPObject` and the current kernel codec for
canonical hashes. Reuse `ProjectComponentMutation` for ownership-first skill
swap + MCP member commit, rollback and recovery. It preserves unrelated JSON
members and can represent skill-only/MCP-only/zero-owned removals without
inventing a second combined transaction implementation.

`RegistryInspector` and `PreparedRegistryInspector` verify only exact scoped
file authority. Owned and unchanged is Expected; a missing owned object is
repairable under the existing checked path; unowned same-name targets collide;
malformed, symlinked, hardlinked, changed-digest or mixed identity is refused.
A discovered native user row never authorizes a project object. Original user
settings are not inspected or copied to infer effective Cline policy.

`ActivationPreflighter` rejects project `ActivationComplete` before the
provider's generic attestation branch and is called again by direct activation.
It validates target/delivery identities, supported components, previous/desired
object kinds and exact file paths. Activation uses shared native-config
completion for actual component mutation/verification, then keeps Prepared and
clears attestation. Auth flags cannot complete project native activation.
A per-session read-only observation never changes this result to Active.
`VerifierAvailable` means exact local file verification only. `Engine.Inspect`
remains host-free and reports saved Prepared values plus general next action.
Fresh lifecycle operations return current reviewable launch guidance.

Removal uses only the recorded validated project layout and receipts. It must
work without Cline or an accepted current version present, must not run native
uninstall or user lifecycle helpers, and must preserve every unrelated member,
user/A/B state and sibling directory. Symlink/hardlink substitutions and changed
owned files fail with prior state intact. Recovery uses the shared ownership
journal, including interruption between skill delivery and MCP commit.

## Incremental implementation and inert verification

After both plan rounds GO, first add the project policy/layout/validators and
projection, with table tests for exact identities, zero-value policy, unchanged
user behavior and hostile ambient overrides. Then add lifecycle/registry hooks
and composed-provider tests. Do not edit shared helpers unless a concrete
unrepresentable case is found and separately reviewed with all existing users.

Exercise public composed SDK/CLI operations on disposable directories: same
plugin at original user + A + B; skill-only/MCP-only/mixed; foreign skills and
MCP collisions; exact-repeat add/update; invocation-change update; intact and
missing-object repair; removal and persisted reopen. Snapshot original user,
other project, unrelated JSON and source package bytes before/after every
operation. Test wrong scope/client/artifact/root, copied/tampered binding,
symlink and hardlink roots, file-mode/JSON corruption, runtime/stdio data-path
bindings, and failed MCP commit rollback. Reuse shared real interruption tests;
add a Cline integration case only if its prepared path introduces an uncovered
boundary. No test may mock a native success and label it native evidence.

Real composed `providers.Activator` tests must reject `ActivationComplete`
for first add, replay, repair and reloaded bindings **before** any filesystem
or state mutation. Also cover mismatched scope fields/direct adapter entry,
AuthComplete, mixed unsupported components, and no native runner calls during
host-free status. Prepared repeated outcomes should be stable under the shared
native-config idempotence contract; no global LifecycleConverged change.

Run affected Cline/domain/provider/installer/SDK/CLI tests, unchanged user Cline
regressions, meaningful shared component/kernel recovery tests, go vet and
format/diff checks; perform Windows compile checks for the dedicated tests.
Report environment/tool limitations honestly. Obtain two independent code GOs
on the frozen product/fixture source before any hosted native publication.

## Current-source hosted acceptance and remaining gates

Keep the existing twelve-probe historical collector unchanged. After adapter
and fixture code reviews, add a dedicated pinned Cline project evidence lane
using its existing exact acquisition and no-updater controls, real hosted guard,
exact checkout, contents-read workflow, clean child env, finite commands,
independent stream/total budgets, bounded cleanup and separately admitted
bounded upload envelope. Review this collector code before it can run.

The new fixture may use public UAP operations to mutate **its own disposable**
project/user files. Native Cline commands are strictly `--version` and
`config {skills,mcp} --json`, always `--auto-approve false --cwd <selected root>`
plus mandatory no-update. Fixture MCP registrations stay disabled loopback
remote markers; no stdio executable, enabled connection, prompt, trust, auth,
model, hook or plugin code is launched. No hosted environment is spoofed locally.

Use separate source and native stages. Public UAP add/replay/update/repair/remove
must return the truthful Prepared lifecycle and exact project file receipts;
record raw CLI JSON and state/file hashes. Native listing after delivery must
show selected A/B unique skill marker paths, with the other's marker absent;
default MCP must still select only the user fixture, explicit A/B overrides
must select exact corresponding prepared files, cross-cwd selection must follow
the explicit override, and ordinary restart must return to the user file.
After removal, the exact owned skill path and explicit-file MCP member must be
absent while unrelated/user/B markers survive. Capture both stdout/stderr and
path-bearing rows. Missing, malformed, nonzero, truncated, ambiguous or changed
fixture output fails its stage and preserves raw diagnostics. Do not turn
empty-array collection into successful discovery.

Separately exercise same-plugin user+A+B UAP ownership with identical portable
names, preserving bytes; if native same-name rows collapse, record the actual
winner and mark same-name active isolation blocked/unqualified. Do not infer
three active installations from three separate receipts. The positively proved
unique-marker discovery stage and this precedence stage remain distinct.

Native MCP initialization/tool output, IDE behavior, enabled skill execution,
trust/managed/session effective policy and model/auth remain explicitly
not_evaluated. Session-selection configuration is the supported product boundary
and remains Prepared even if its read-only config list succeeds. This lane
qualifies only the pinned Linux CLI behavior/current source actually measured;
other platforms and versions need their own observations. Final per-client
matrix must name these limits. A failed native stage is not repaired by relaxing
assertions, changing pins or substituting the historical artifact.
