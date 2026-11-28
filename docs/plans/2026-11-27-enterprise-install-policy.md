# Enterprise installation policy

Status: implementation and repeated independent source review complete. The
qualification record below captures execution checkpoints. Final qualification,
merge and actual-main verification are tracked in
[PR #1130](https://github.com/777genius/universal-agent-plugins/pull/1130).
Owner request: research APM and ToolHive, synthesize a convenient modular design,
review and refine it, implement in one Draft PR, publish each commit immediately,
exercise important end-to-end paths, fix review findings, ordinary merge and verify.

## Outcome and boundaries

One reusable installation-policy feature governs packages, standalone Skills and
MCP configuration delivered by UAP. Administrators can restrict sources, requested
pins, clients, installation scopes and package components. Users can see which
file and rule rejected a request and check a policy before installing anything.
No configured policy preserves current behavior. Configured invalid policy fails
closed. Policy permission never grants native trust, security-risk consent,
credential forwarding or ownership of foreign files.

This is installation governance. It does not constrain what an agent executes
after installation or what users manually edit. The system policy is effective
for this UAP binary when deployed at an administrator-protected location; it is
not a sandbox or a defense against an administrator replacing the binary.

In scope: shared domain rules, strict local configuration, layered composition,
CLI diagnostics/template, add/install/update/repair/sync/import-install admission,
standalone Skills, transitive graph admission, management and installer facade
integration, source-level and final-envelope checks, documentation and bounded E2E.

Out of scope: remote discovery/extends/cache, control plane, SSO/RBAC, Cedar/OPA,
approvals service, runtime proxy/container sandbox, network-egress enforcement,
new scanners, release/tag/npm publishing, client adapter redesign, local Skill
authoring, frozen VS Code, held resource/MCP probes (#1114) and Codex forwarding
(#832). Read/export/removal and rollback remain available after policy denial.

## Research and adaptation

Primary sources checked 2026-11-27:

- APM [organization policies](https://microsoft.github.io/apm/enterprise/apm-policy/),
  [schema](https://microsoft.github.io/apm/reference/policy-schema/),
  [inheritance implementation](https://github.com/microsoft/apm/blob/main/src/apm_cli/policy/inheritance.py).
  Adopt declarative source/component rules and explanations. Avoid user-scope
  exclusions, local disable switches, remote discovery complexity and literal
  string intersection of wildcard allow-lists. Evaluate each layer independently.
- Stacklok [registry policy](https://docs.stacklok.com/platform/enterprise-manager/policies/registry),
  [non-registry policy](https://docs.stacklok.com/platform/enterprise-manager/policies/non-registry-servers),
  [Enterprise CLI](https://docs.stacklok.com/platform/enterprise-cli/),
  [runner gate](https://github.com/stacklok/toolhive/blob/main/pkg/runner/policy_gate.go),
  [plugin verification](https://github.com/stacklok/toolhive/blob/main/pkg/plugins/pluginsvc/verify.go).
  Adopt early shared admission and mandatory settings. Avoid singleton policy
  state, conflating runtime authorization with installation, or claiming commercial
  Enterprise controls are all present in the OSS ToolHive package manager.
- Existing UAP `cli/internal/agentpluginscli/deps_policy.go` already owns schema-1
  YAML source host, requested pin and MCP transport policy. Extract/reuse its
  parser and semantics rather than introduce a competing policy language.

Current base: `864829036a932a8c22ab7074b94b79dc70ff8021`. Eleven open PRs inspected.
#1129 owns batch-update and partial-removal presentation; this work does not
change those outcome semantics. #1078 owns live-audit/native harness changes;
avoid its changed fixtures/workflows. No existing enterprise feature PR found.

## Configuration contract

Reuse the existing YAML language and parser, with `schema_version: 2` for the
new layered policy. Continue accepting schema 1 with its exact legacy semantics
(including rejection of local sources, even with no host rule); new fields require
schema 2. The shared rule model retains `sources.allow_hosts`, `sources.deny_hosts`,
`integrity.require_pinned_git`, `integrity.require_archive_sha256` and
`mcp.allowed_transports` fields. Existing explicit dependency `--policy` remains
an additive local restriction and retains its documented command scope.

Add bounded optional rules:

- `sources.allowed_kinds`: directory, git, archive, web, local.
- `sources.allow_repositories` / `deny_repositories`: host-qualified exact Git
  repositories or a terminal owner namespace wildcard (`github.com/acme/*`).
  No arbitrary regex, glob algebra or hostless identities.
- `install.allowed_targets` / `allowed_scopes`.
- `components.denied`: mcp, skills, commands, apps, extensions. Evaluate the
  package inventory conservatively, even if a target projects fewer components:
  native full-plugin delivery must not smuggle a denied component.
- `mcp.allow_hosts` / `deny_hosts` for remote MCP endpoint hosts. This controls
  declarations, not subsequent redirects, DNS resolution or runtime egress.

For schema 2, omitted allow-list means unrestricted; present empty list means
allow none. Local satisfies source-kind rules but cannot satisfy a present host
or repository allow-list. A host deny-list alone does not deny a local source.
Archive/web/local cannot satisfy repository allow-lists by looking like Git URLs.
Directory uses its signed repository identity; manifest.repository is ignored.
Concrete Git URL precedes shorthand; normalize scheme/host, SSH username, terminal
.git and GitHub owner/repository case. Reject encoded separators, credentials in
HTTPS URLs, ambiguous authorities and nondefault ports for repository matching.
Preserve non-GitHub repository path case. Namespace wildcard matches repository
paths under that prefix at a slash boundary, including nested namespaces; package
subpaths do not alter repository identity. No suffix/substr host matching.
Reject null, unknown fields, duplicate keys, YAML aliases/merge keys, invalid
types/values, multiple documents, unsupported schema and oversized documents.
No arbitrary commands or expressions in policy. No warning-only installation
mode initially: use read-only checks for rollout; active restrictions enforce.

Layers in order for explanation (all conjunctive, no weaker override):

1. Machine policy: fixed platform location (`/etc/agentplugins/policy.yaml`,
   `/Library/Application Support/AgentPlugins/policy.yaml`, Windows system-wide
   ProgramData `AgentPlugins/policy.yaml`, resolved with the Windows Known Folder API,
   not the user-controlled environment). Optional until present. HOME/XDG/UAP
   environment settings never relocate the machine layer. Use an injected path
   list in tests; there is no production machine-policy bypass selector.
2. User policy: `<user-home>/.config/agentplugins/policy.yaml`.
3. Project policy: `<selected-project-root>/uap-policy.yaml`; for User operations
   use the invocation working directory. No walking arbitrary ancestor trees.
4. Explicit deployment file `UAP_POLICY_FILE`, if configured, is required and
   additive, never replaces machine/user/project rules. Existing `--policy`
   applies additionally in its current dependency commands.

File-location configuration belongs to App/Engine; policy snapshots belong to
one reviewed operation. Freeze selected project/invocation roots and all
candidate locations including absent optional paths. Before apply reread the
same locations and compare the semantic fingerprint of rules plus layer
presence. Added/deleted/changed rules invalidate review; comments do not. Fresh
preparation captures a new snapshot and does not poison a long-lived daemon.
UAP does not lock an administrator out of changing policy while an already
authorized transaction runs; its decision is fixed at the last pre-effect gate.
Rollback/recovery of that transaction must finish safely. Missing
explicit paths, permission errors, symlinks/nonregular files and malformed
configured policy refuse installation. Error output avoids full source URLs,
arguments, environment values and untrusted terminal control characters.

Enterprise enforcement requires trusted filesystem ancestry: deployment must
protect the policy file and every containing directory from writes, deletion,
rename or replacement by governed users using OS ownership/ACLs. Bounded leaf
reading and ancestry checks do not supply directory isolation against an actor
who can replace those ancestors. Optional discovery intentionally permits an
absent path at the start of a fresh operation, including absent files beneath
linked home/project roots. A retained review rejects disappearance, including
an ancestor redirect that hides its policy. Deployment that requires absence
to fail uses a protected required path, such as UAP_POLICY_FILE supplied by a
trusted managed launch configuration. An environment variable that governed
users can unset does not establish mandatory organization authority.

## Architecture and integration

Domain types and evaluation stay stdlib-only under the existing domain boundary;
no I/O or client constants in generic rules. A narrow policy port consumes actual
source/envelope/scope/client facts. A file adapter owns strict YAML decoding,
bounded safe reading and layer discovery. Source normalization reuses existing
`sourcespec`/domain identities; Directory authority is never inferred from names.
Composition roots inject the same gate into the CLI, installer facade and
usecase service. No new installer/resolver/store/framework or feature flag.

Usecase admission runs before host preparation and mutation recovery for every
AddInput. For groups, complete the admission pass for every input before any
client host preparation. SDK preparation also checks immediately after package
loading and before prepareOpenCodeHost, which precedes the usecase. Recheck current policy at the final apply boundary.
Installer preview/apply must preserve policy authority across user confirmation;
long-lived management sessions cannot reuse an old successful decision forever.
No per-target filtering or silent partial install to make policy pass.

Enforcement matrix to complete before implementation approval:

| Path | Required admission point |
| --- | --- |
| Direct/Directory add and standalone Skills | Known source before acquisition where available; actual envelope before host preparation/apply |
| Dependency graph including transitives and warm cache | Selected closure including unchanged required nodes before environment publication |
| update/repair/sync/locked replay | Reevaluate actual retained source, requested pin and envelope; no grandfathered allow |
| import --install | Before `transfer.publish`, then normal shared apply gate |
| Browser management | Same preparation/application boundary; revalidate after retained review |
| Installer SDK | Explicit injectable gate, shared usecase enforcement; no global singleton |
| switch/rebind/migrate format | Gate active switch and proposed retained source/envelope before provenance replacement |
| remove/export/read/rollback | Do not require install permission; retain ownership and recovery checks |

Reuse `applyPreparedDependencies` and the existing environmentapp `Recheck` /
`CheckInputs` seams, including per-binding checks. Import-install has a separate
mandatory check in `commitDependencyTransferInstall` before `transfer.publish`.
Do not gate unrelated retained packages when an operation selects a smaller
closure; remove-only work does not acquire install permission.

Requested integrity is not resolved integrity. CLI/source/graph checks use the
original selector/checksum and recheck authoritative locked intent. The shared
request may carry these explicit facts when they are available; SDK checks do
not infer an original request from a resolved commit. If a Git/archive pin rule
requires unavailable original intent, refuse rather than claim it was pinned.
Directory is a distinct source kind with existing signed immutable provenance.
Local/normalized web Skills do not acquire imaginary Git pin requirements.

Existing loaders already reject auto-discoverable hooks directories and explicit
native hook declarations. Preserve those checks and their regressions. This PR
does not add a hooks detector or claim to identify all executable behavior;
`components.denied: [extensions]` includes raw/unknown extension declarations.

Do not place universal denial in a raw loader that is also used to remove or
inspect a now-forbidden package. Policy may read an already admitted snapshot;
it may not execute package code to decide permission. Source rules describe
canonical identity, not a network firewall.

## User experience

`agentplugins policy init [path]` creates a commented schema-2 example without
overwriting files. `policy validate <path>` checks configuration without fetching
packages. `policy status` displays effective layers and rules with their paths;
`--require-policy` requires at least one effective restriction (an all-unrestricted
template is reported as such). `policy check` checks the existing verified declared
closure without install effects; it can reuse offline dependency-check machinery.
`policy check <source>` is a package inspection with explicit target(s). It reports
its coverage; if dependencies are declared, root-only inspection is incomplete
and nonzero rather than falsely claiming closure approval. `--policy <path>` on
checks adds a candidate policy without replacing active layers. Checks may fetch
immutable bytes into temporary snapshots/cache but never prepare hosts, recover
mutating journals, write intent/receipts/trust or activate clients. A check is not
a saved authorization for later installation. Support existing human/JSON
conventions and nonzero refusal status. The existing `deps check --policy`
continues to audit a verified offline closure using the same rules.

Messages identify subject, rule, policy source and next action, e.g. remove the
dependency or ask the policy owner to update that file. `--yes` and
`--accept-security-risk` never override enterprise rules. Ordinary users with no
policy see no extra wizard, prompt or repeated success output.

## Work plan and estimates

| Slice | Production LOC | Test/document LOC | Owner |
| --- | ---: | ---: | --- |
| Shared domain rules and file adapter | 800–1,200 | 450–700 | policy worker |
| Shared usecase/facade enforcement | 250–500 | 400–650 | integration worker |
| CLI composition, graph admission and migration of local policy | 650–1,000 | 450–700 | root |
| Policy CLI, user guide and key E2E | 450–750 | 500–800 | UX worker/root |
| Total | 2,150–3,450 | 1,800–2,850 | one PR |

Complexity: 6/10. Target maintainability/reliability: 9/10. Initial confidence:
7/10 pending complete path audit. Estimates are a budget, not a line-count goal.

1. Independent design critiques: source/security; architecture/enforcement;
   configuration/UX. Revise concrete findings, then second review of revised plan.
2. Implement disjoint modules in parallel; root integrates and publishes each
   coherent commit immediately. Do not commit another worker's in-progress files.
3. Focused adversarial/core tests and production CLI E2E; review actual combined
   diff in multiple rounds and fix all critical findings.
4. Run required lint/core and affected portable package-manager gates. Inspect
   real failures and review threads; no skipped/queued-as-PASS, weakened refusals,
   duplicated CI runs or unrelated fixture expansion.
5. Mark ready after review/qualification, ordinary expected-head merge, verify
   actual merged code and applicable postmerge checks. No release is authorized.

## Essential acceptance cases

- No-policy installation preserves existing behavior and public contracts.
- Machine deny cannot be weakened by project/user/explicit allow or consent flags;
  both User and Project scopes obey it. Narrower repository patterns combine.
- Denied source/component/target/scope/MCP endpoint fails with actionable human
  and JSON diagnostics; client config/trust, receipts, manifests and lock unchanged.
- Transitive denial and warm-cache denial happen before any graph apply/publish.
- Missing required policy, malformed/unknown/null/duplicate config, symlinks and
  changed/deleted policy between review/apply fail closed with no managed changes.
- Updates, repair, sync, import-install and management do not bypass policy.
- Full native-plugin inventories cannot hide MCP/extension components in a
  Skills-only claim; existing hook-source refusals remain intact.
- Approved installation works; tightening blocks subsequent update/repair;
  ownership-safe removal still works, including invalid policy.
- SDK policy injection and current production CLI composition are both exercised.
- No new real credentials, vendor account calls or broad native-client retesting:
  verify policy at the actual installer boundary with existing isolated fixtures.

## Review log

R1 source/schema review: found legacy local semantics, canonical-repository and
requested-pin ambiguities. Revision 2 uses one shared model/parser with explicit
schema-1 compatibility and schema-2 neutral defaults; documents canonical identity
and original intent rather than resolved hashes.

R2 architecture/enforcement review: found early SDK host work, group preparation
ordering, warm-cache/unchanged dependencies, import publication and retained
provenance paths. Revision 2 names both publication gates and the before-host
admission points, and fixes snapshot lifetime and removal semantics.

R3 ToolHive/UX review: found Windows environment path bypass, hook claim mismatch,
partial check semantics and unrestricted-template ambiguity. Revision 2 requires
Known Folder lookup, preserves existing loader hook rejection, explicitly reports
check coverage, supports additive candidate checks and useful status discovery.

R4 revised source review: GO with explicit partial-source semantics, rejection of
conflicting envelope/source identities, and no classification for an unrestricted
schema-2 configuration. Accepted. Original pin facts remain trusted orchestration
inputs, never invented from resolved commits.

R5 revised UX/system review: GO. Candidate paths and absent layers are visible;
validate is independent of broken active policy; package-only check cannot claim
closure approval. No additional presets, wizard or elevated writer.

R6 integration refinement: usecase/facade worker owns early all-input admission,
per-review fingerprint and final pre-effect gate; root owns both environment
publication paths, original pin facts and CLI/management session composition.

R7 root scope/API review: one `ports.InstallationPolicy.Check(ctx, request)`
returns a semantic fingerprint or typed refusal. `domain.InstallPolicyRequest`
contains Source, OriginMode, optional Envelope, Scope, Targets and optional
original GitPinned/ArchivePinned facts. Source-only calls defer component/MCP and
missing target/scope rules; final calls reject missing constrained facts. A nil
gate preserves compatibility. Files adapter owns candidate layers and strict
loading; fresh operation sessions own comparison, never the long-lived engine.
The SDK exposes an immutable WithInstallationPolicy clone for composition.
Workers edit disjoint domain/adapter, usecase/facade and CLI command/doc slices;
root publishes coherent commits and owns integration. No second policy engine.

## Final implementation and review

The shared domain, narrow policy port, layered adapter, CLI composition,
SDK/usecase admission, policy commands, safe diagnostics and configuration guide
are implemented. No new dependency, policy runtime, installation-state format, resolver,
feature flag, client implementation or release was introduced.

R8 combined scope review: selected additions and their dependency closure are
admitted, including cached and unchanged packages. Unrelated retained roots do
not block a selected operation. Cold acquisition remains source-gated. Explicit
removal-only operations preserve ownership/integrity and remain available with
invalid or tightened policy. Ordinary package validation stays read-only.

R9 session/output review: fresh operations get fresh sessions; retained reviews
reuse their policy fingerprint through apply. Group inputs are all admitted
before host preparation. Public diagnostics sanitize injected checker errors and
emit exactly one JSON result, preserving typed causes and writer failures.

R10 original-pin review: saved and legacy imperative repair no longer treat a
resolved replay commit/checksum as evidence of an originally pinned request.
Saved intent requires matching alias, source key, tree digest, manifest digest
and immutable revision; unknown evidence stays unpinned. Acquisition coordinates
remain exact. Cold recorded Directory repair retains its established repair
eligibility and executable inventory.

R11 dependency-revision review: standalone CLI/management repair evaluates the
actual recorded root and selected transitive closure. A missing, duplicate,
pending or divergent installed child refuses with reconciliation guidance;
a newer desired child cannot stand in for older installed bytes. Actual package
declarations and envelopes remain verified.

R12 convergence review: ordinary install/recovery can repair root drift while
converging to admitted desired dependencies. Operation intent distinguishes that
path from standalone repair. Captured input/policy checks run before each
preliminary Prepare/Apply; the graph retains complete desired-closure admission.
The regression both refuses standalone repair and successfully reconciles the
same graph with root drift through install --locked.

R13 final independent source review: source normalization, schema-1 compatibility,
local archive pin facts, session boundaries, UX/JSON diagnostics and the final
repair distinction were independently reread. Identified blocking findings were
fixed. Source review is not a substitute for the execution evidence below.

R14 external review: all ancestor components of active/required policy files are
checked around the bounded read; optional absent policies retain no-policy
behavior and macOS fixed system aliases retain configured path diagnostics.
Valid JSON emptiness is semantic rather than formatting-dependent. Group apply
rechecks policy after the final configuring callback and identity observation,
before desired state/kernel effects. Independent reread approved these fixes;
focused domain/adapter and core policy/SDK regressions passed. The SDK reason
mapping comment was already addressed and its callback regression was rerun.

R15 follow-up review: deployment requires trust in the policy file and every
ancestor, enforced by OS ownership/ACLs; optional absence retains the accepted
no-policy behavior. A new regression covers a retained restrictive policy hidden
by an ancestor redirect, required-path refusal and fresh optional discovery.
This does not claim protection from transient concurrent directory replacement.
The SDK now preserves policy-error origin through a private wrapper, retaining
errors.Is/errors.As while mapping late configuration/gate failures to
policy_denied after single-target and group callbacks. An eight-case regression
was added. These follow-up changes require their own qualification after the
executor interruption; earlier passing results are explicitly tied to 5cbd057e.

R16 source-admission review: a full commit in the first browser-URL selector
segment is original pin intent at both the early source and final envelope
gates. The helper follows the existing resolver and preserves explicit
commit/ref precedence; moving refs never inherit pin status from resolved bytes.
Ambiguous HTTPS keeps archive-first discovery when archive policy allows it.
A pure typed archive denial can select independently admitted Git without an
archive request; configuration, changed-policy and mixed operational errors
cannot select another transport. Explicit source kinds retain their behavior.
Focused regressions compare browser admission with the offline resolver and
observe actual HTTPS requests and injected Git acquisition separately.

### R17 — Windows machine path under isolated process environments

The bf12c87c Pi Windows job 140367431796 exposed a production regression:
public skills add fails during composition when the child environment omits
SystemDrive, ProgramData and ALLUSERSPROFILE. Its scoped SDK preflight and genuine
Pi project trust/A/B lifecycle had already passed. The public child isolates user and
application roots; that isolation remains part of the acceptance test.

Correction boundary: Windows CLI composition, focused regression coverage, the
existing Pi preflight selection, and deployment documentation. Read the protected
64-bit HKLM ProfileList ProgramData registration, retain a configured absolute
directory, and expand only supported machine tokens from the trusted system
Windows directory. Preserve registry string-type semantics and administrative
relocation through the existing x/sys string decoder. Validation covers the
decoded path, not raw UTF-16 encoding or trailing registry data after a NUL.
Never expand caller-controlled environment variables, mutate the process
environment, select a default folder instead of the registered folder, or turn
a lookup error into an absent policy. Validate the resolved directory and retain
genuine registry, API, close and filesystem errors. The existing policy adapter
continues to own optional-file discovery and ancestor/leaf checks.

Validation: fresh test subprocesses with omitted and poisoned machine aliases,
isolated profile/application roots, and an unchanged authoritative machine layer;
native normal-environment parity with the Windows known folder; small registered
path cases for relocation, trusted expansion and malformed/unresolved refusal.
Run the focused EnterprisePolicy composition selection from the repository root
in the existing Pi native preflight on its three hosts. Both PR and main-push
path filters include the policy composition files. The real public Pi Skills/MCP
journey remains unchanged and must pass on the corrected source. No new CI job,
runner route, dependency, permission, exception, flag or runtime policy engine
is introduced.

Estimated additional scope: under 120 production lines and 220 test lines, plus
one focused CI selection and documentation. Complexity 3/10; intended reliability
8/10; confidence 7/10 pending native execution. Two independent reviews of the
correction and its qualification boundary precede publication.

Primary references: [machine ProgramData registration](https://learn.microsoft.com/en-us/troubleshoot/windows-server/remote/internal-error-has-occurred-connecting-remote-machine#check-registry-configurations),
[trusted shared Windows directory](https://learn.microsoft.com/en-us/windows/win32/api/sysinfoapi/nf-sysinfoapi-getsystemwindowsdirectoryw),
and [known-folder retrieval flags](https://learn.microsoft.com/en-us/windows/win32/api/shlobj_core/ne-shlobj_core-known_folder_flag).
This section records a planned correction, not a passing execution result.

### R18 — Qualified runtime bytes remain independently bounded

The original 30-source Windsurf prepared-installation job 140491530891 failed
on Firebase case 6 at head 713053bf (checkout c5f816bd, identical source tree).
The unchanged strict audit attempted all 30 cases and refused qualification:
`HeldTree._capture` exceeded the combined 256 MiB byte bound. Artifact
15203313236 was independently read and SHA-256 verified as
`a24071579a84596a729c2fcfc7bbfe839c914f7e12d8a9d7a3246d0a9c2aa3d9`.
The error row contains no completed footprint; the failure is not evidence
that Firebase's authenticated runtime alone exceeds 256 MiB.

The earlier same-pin, script-disabled fixture measurement in merged
[#1126](https://github.com/777genius/universal-agent-plugins/pull/1126)
records 242,615,124 regular-file bytes across the owned fixture, 19,465 runtime
entries and 23 npm bin links. This supports a bounded accounting correction,
not a claim that the currently failed native tree was completely measured.

Correction boundary: the shared retained-tree reader, its focused regression
tests, and this plan. Reuse the existing canonical one-prefix classification
and the caller's unchanged receipt/source/manifest/configuration qualification.
Retain one aggregate 256 MiB ordinary-data limit and a separate fixed 256 MiB
limit for that qualified runtime subtree. Neither class borrows unused bytes
from the other; no prefix gives no additional capacity. Keep the complete
`self.total` and `fully_read_bytes` evidence across both classes. Check a
file's class bound before its full read; charge both class and aggregate
counters only after that read succeeds. A refusal may report its resource
class, partial fully-read counters, next stat size and fixed bound, without
paths or contents. Those counters are not a completed-tree measurement.

Keep the per-file bound, five-second cooperative deadline, depth, directory,
member and aggregate descriptor quotas, exclusive regular-file requirements,
authenticated bin links, full hashing, held descriptors, identity/membership
checks and closing verification unchanged. Capture the entire data tree in
one custody graph. Do not alter Windsurf source qualification, observations,
native acceptance, fixtures, expected refusals, workflow routing, timeouts,
permissions or dependencies. This does not grant installation authority or
claim atomic snapshots or writer closure.

Validation: small real-filesystem tests with scaled byte constants prove both
classes can simultaneously reach their own bound while retaining every object
and byte; either class exceeding its bound refuses despite unused capacity in
the other; default/no-prefix and near-prefix siblings remain ordinary; all
failure-owned descriptors close. Verify complete aggregate evidence, runtime
mutation refusal, and the unchanged per-file guard. Existing invalid/multiple/
missing-prefix, source-tuple, link, timing and custody regressions remain in
force. Run the shared-reader and Windsurf observer contract suites.

The original full Firebase reader and prepared lifecycle must then pass on the
published correction under the unchanged strict audit. Successful full capture
will establish current fit within both fixed class limits; until then this is
a candidate correction, not native qualification. If a class still exceeds
its cap, retain FAIL and investigate the measured offending class rather than
raising the ordinary bound or bypassing custody. The combined source includes
main a88b22c2 and retains its eight batch-update/removal fixes.

Pre-implementation review: independently approved the bounded accounting plan
and explicit qualification boundary; a second pass preserved aggregate evidence,
post-read charging, exact prefix boundaries and diagnostic honesty. Review the
actual diff before publication. No source commit is withheld after creation.

Local qualification of the R18 candidate: all 47 shared-reader tests passed
in 3.697 seconds, including the five new byte-accounting regressions and the
existing 10,000-member custody controls. Independent actual-diff review found
no remaining issue. The existing Windsurf runtime/source-profile controls
passed, but its complete 60-test invocation did not pass: the projection oracle
could not build. An explicit build with the existing Go toolchain and downloads
disabled confirmed that golang.org/x/sys v0.44.0 is absent from the local cache.
No dependency was installed or substituted. The complete observer contracts and
actual Firebase lifecycle remain required on the supported CI runner.

### R19 - Preserve Kiro finalization failure diagnostics

Current-source Kiro plugin job 140615547142 in run 46017064716 attempted all
30 cases on checkout 7c5b81fa (the same tree as head 731d8c7). Cases 1, 2, 3
and 16 reached the unchanged six-observation custody assertion without a
complete lifecycle. The outer handler replaced their result rows with a
generic exception, losing the initial add/list outcomes. Artifact 15222303169
has verified ZIP SHA-256
`31bb81637f14d9f0d274485d1b9f993e83f003cdbd0beb02db8900a75fc7d398`.
The other 25 positive cases and the one expected negative retained their
custody evidence; the complete audit failed and remains unqualified.

Independent inspection of production signed snapshot 49, its signature, all
four source tuples and immutable manifests found no fixture drift. The
artifact cannot establish the original installation refusal or source/state
mismatch. Do not change pins or classify these cases as expected negatives.

The correction preserves bounded public command outcomes, signed-source
observations, public states and the observation count when Kiro finalization
raises. It attaches diagnostics to the same exception and re-raises it; the
outer handler records them alongside the unchanged harness-error status.
It does not export full streams, decoded private receipts or native images,
read more files, perform more commands, or change six-observation, source,
custody, lifecycle or final acceptance requirements. Earlier guard failures
remain outside this narrowly scoped diagnostic correction.

Budget: about 20 harness lines and 60-120 regression/documentation lines.
Complexity 3/10; target reliability 9/10; confidence 8/10 pending execution.
Two independent plan critiques approved this boundary. Regressions exercise
the actual case/finalization and report path for refused or unmatched initial
installation, retaining failure and skipped phases. Review the written diff,
publish in this PR, then use supported CI to identify and address the original
cause. This diagnostic correction is not a Kiro lifecycle PASS.

Written-diff review found no remaining issue. All 33 existing and added Kiro
observer/runner tests passed locally in 7.079 seconds with no skips; the two
focused diagnostic tests also passed independently in 0.850 seconds. They
exercise the actual case/finalizer for an initial refusal and an unmatched
signed receipt, and the actual report handler with unsuccessful acceptance.
Whitespace and Python compilation checks passed. Fresh supported-runner
execution, the original failure diagnosis and actual-main qualification remain
outstanding.

### R20 - Bound Claude temporary-file refusal diagnostics

Head `31cb7e2e` passed all 15 Core fast and all 10 Required jobs. Its supported
package-manager four-host and Pi three-host artifacts were independently checked.
Kiro plugin job `140711595610` completed 29 installation lifecycles and one exact
signed refusal; artifact `15233039162` matched its ZIP SHA-256 and the current
acceptance function independently accepted its report. The four earlier failing
Kiro cases passed with unchanged pins and custody requirements. This establishes
installation coverage, not Kiro native runtime, authentication or tools. All 60
Windsurf observer tests also ran successfully without skips on the current Linux
Cursor job. These are evidence for that head, not automatic qualification of a
later source revision.

Claude Marketing job `140673480347` then failed before the published `0.1.116`
lifecycle began. Its genuine pinned Claude `2.1.263` regression passed in the
first 38-test selection, then failed when repeated in the 48-test Claude selection.
The main passive document passed; a post-uninstall `.claude.json.tmp.*` failed
the existing file-custody predicate. The traceback does not distinguish its mode,
link count, owner, size or descriptor identity. This is not evidence of a network
failure and does not justify normalizing permissions, changing pins or relaxing
the refusal.

The correction preserves the exact short-circuit predicate and the same raised
ValueError. It retains the metadata snapshot already used by the predicate and
appends a fixed diagnostic object: file type, mode, link count, UID/GID, size and
identity equality. The descriptor snapshot is explicitly unavailable when an
earlier condition short-circuits. No extra filesystem probe, pathname, inode
number, file bytes, retry or acceptance exception is introduced. Regressions use
the actual finish/cleanup path for unsafe mode, an empty temporary and a replaced
pathname, preserving refusal and closed descriptor custody.

The inspected main remains `48793197`. Open PR #1133 owns separate functional
Claude retirement and User-profile corrections; this diagnostic does not import
or claim those changes. Budget: about 25 diagnostic lines and 50 regression lines;
complexity 2/10, confidence 8/10 pending supported native execution. Root and an
independent actual-diff reviewer identified and corrected a second-snapshot
diagnostic ambiguity before publication. This correction supplies failure
evidence; it is not a successful Marketing Kit lifecycle.

The focused regression passed all three real-filesystem subcases. The complete
affected local selection ran 39 tests in 31.279 seconds: 38 passed and the genuine
native Claude case skipped because the pinned executable is unavailable. Root
and independent final-diff review passed; whitespace checks passed. The genuine
Claude case and full lifecycle remain required on the final supported CI source.

### R21 - Integrate merged native custody and byte-pool diagnostics

Main `c9e52bf9` now includes merged #1133. Its functional Claude custody and
runtime-byte diagnostic changes overlap R18/R20 and make head `64e62f12`
unmergeable. Integrate this main into the existing PR; do not import unmerged
#1078 or #1135 work. The other inherited workflow, discovery, orchestration and
Copilot refusal changes remain exactly as merged upstream. R19 Kiro diagnostics
and the Enterprise production implementation remain unchanged.

Keep main's single retained-tree reader, including independent fixed 256 MiB
ordinary and qualified-runtime pools, its captured limits and structured partial
byte checkpoint. Preserve main's four pool regressions and six Windsurf
diagnostic regressions, plus all five R18 regressions. Adapt three R18 message
assertions to the stable upstream refusal and verify the checkpoint's refused
pool, fixed limit, attempted size, partial fully-read counters and descriptor
cleanup. Do not grant unqualified or near-prefix data extra runtime capacity.

Keep main's rooted no-follow Claude member walk, original-descriptor `pread`,
offset preservation, User receipt transitions, capture-one/finish-two temporary
bound, ABA defenses and late held-descriptor checks. Rehome bounded R20 metadata
diagnostics at the existing named-member, held-descriptor and initial descriptor
metadata guards. Named members still require one link; an original held inode
may have zero or one after an authorized replacement. Preserve lazy comparisons
and owner-before-mode order. Cache the existing owner comparison instead of
calling `geteuid()` again. Annotation performs no new filesystem observations or
content reads and adds no paths, filenames or inode numbers. Original I/O
exceptions, refusal type and acceptance remain unchanged. This supplies failure
evidence; it does not establish the cause of the earlier Claude refusal.

Three independent plan critiques approved this integration boundary. Local
shared-reader tests passed 51/51 in 2.685 seconds and Copilot refusal tests
passed 13/13 in 0.405 seconds, with no skips. The complete Claude retirement and
discovery selection ran 82 tests in 45.742 seconds: 81 passed and the genuine
native case explicitly skipped because the pinned executable is unavailable.
Three focused methods also passed, covering actual finish refusals, lazy named
observations and unchanged I/O exceptions. The initial full Windsurf invocation
did not pass because `/usr/bin/go` is not a Go compiler. After selecting an
existing Go 1.26.11 installation, the canonical projection oracle built with
`GOTOOLCHAIN=local` and `GOPROXY=off`: all 66 Windsurf tests passed in 25.097
seconds without skips. No dependency download or pin substitution was needed.
Independent actual-diff review confirmed all 46 main reader tests and 68 main
Claude retirement tests remain unchanged, R19 remains exact HEAD, and all 11
other integration files are byte-identical to main. Root review, Python parsing
and whitespace checks also passed.

The complete Kiro, prepared-installation and environment selection ran 468 tests
in 104.302 seconds: 455 passed and 13 Windows-only contracts skipped on Linux.
Both R19 diagnostic regressions and both new main Claude routing/failure tests
executed and passed. These local contracts do not claim native lifecycle proof.

Review the actual combined diff, publish the coherent merge commit in this PR,
and qualify that source through the required supported-runner gates. Results
from `64e62f12` and earlier revisions remain historical. The genuine Claude and
original full Firebase lifecycles, final required/core/package-manager and Pi
checks, ordinary expected-head merge, and actual-main verification remain
required. No release, tag or npm publication is authorized by this work.

### R22 - Accepted main collection and Claude backup integration

Accepted main `11ac0bc0` includes #1135 Skill collections and #1140 Claude
initialize-backup custody. Published heads `28ad4f84` and `a54d1d27` retain that
source and add early HTTPS publisher admission, including an actual `--all`
zero-request regression. Subsequent heads `8903c895` and `06008851` add
known Git/local source admission and preserve the distinct read-only listing
entrypoint. These existing branch changes are retained. Independent source review found the existing rooted
descriptors, bounded backup admission, original held-main evidence and native
proof checks preserved. No discovery or writer-closure claim was widened.

The combined Claude retirement/discovery and three package-manager Python
contract suites ran 130 tests: 129 passed and the genuine pinned-Claude case
was explicitly skipped. The tested Python paths match `28ad4f84`; the only
subsequent `a54d1d27` change is the Go HTTPS preflight regression.

New full package-manager evidence must include 23 phases, 46 stream hashes,
140 required Go roots and the collection command evidence. Its expected raw
case inventory is 463 RUN/PASS nodes. The original binary installation's scanner
receipt does not become an independent collection scan receipt. Genuine Claude
and Codex raw-Skill qualification also requires the new collection install,
repeat and removal stages. These are required observations, not claimed passes.

### R23 - Admit Git collection targets before raw prefetch

Combined-source review found that multi-Skill Git prefetch checked each original
source with a nil target list before later resolution checked the selected
targets. A two-Skill public `PrepareSkills` regression reproduced both raw-tree
acquisitions before a typed refusal of the requested Claude target.

Pass the selected targets through the existing prefetch admission call. This is
the same installation-policy authority used by per-selection resolution and
final apply. A denied target starts no selected raw acquisition; an allowed
Codex control still starts both acquisitions through the existing prefetch seam.
Independent review caught a test synchronization race: wait for both recorded
starts before asserting the atomic call count, rather than the reverse order.
No new policy engine, source exception, authority or client-specific behavior.

Additional real-TLS `--all` regressions cover denied source kind, host, scope and
target plus malformed policy. They require zero publisher requests and unchanged
durable files under the fixture root, then prove that explicit read-only listing still obtains the
two-entry catalog without durable changes under each restriction.

The old-code regression failed as intended. Local post-fix compilation met
resource failures; a subsequent memory-bounded combined run lost its executor
connection, so no passing result is claimed for that run. Final-source Required,
Core, full PM/Pi and applicable genuine native E2E remain mandatory before the
ordinary expected-head squash merge, followed by actual-main verification.
Earlier-source successful checks remain historical. No release or tag.

### R26 - Preserve explicit ref intent and complete collection refusals

Adopt `8f56f768` and all earlier accepted concurrent changes. Keep its selected-
target admission before raw Git Skill prefetch and its richer real-TLS refusal/
read-only matrix. Keep the separate read-only Git catalog wrapper and existing
Git/local/HTTPS source guards unchanged. The only remaining production correction
is in RequestedGitIntent.

An explicit `--ref` is an exact ref request, even with a forty-hexadecimal
spelling. The actual resolver sends explicitRef to ResolveGitHubRef or
ResolveGit with exact-ref selection; only a Commit request uses the non-ref
immutable-commit path. After the existing conflict check, clear Commit and
preserve explicitRef as Ref. Do not promote a commit-shaped ref to requested
pin authority. Keep the actual resolver, source grammar, URL/ref boundary
validation and optional acquirer interfaces unchanged.

Independent review verified these distinctions in the immutable resolver
sources and approved this minimal correction. Expand the existing pin table for
native, browser and raw source forms. Add composed regressions for local denial
before a missing root can be scanned, five denied Git spellings with no source
commands, public picker admission, original pin intent and successful read-only
Git/local listing with unchanged durable state. The batch regression admits the
first actual Skill, denies the second Skill's component envelope in a two-target
selection, and requires zero native, durable and source effects. Reuse the
already accepted richer HTTPS matrix instead of adding duplicate website cases.

An earlier, larger unpublished correction was discarded in favor of the valid
concurrent architecture. The executor became unavailable before the final
correction was published. Recover the retained tests from their exact recorded
patch inputs, adapt their pin check to RequestedGitIntent plus the existing
policy helper, and compare immutable Git object identities before publication.
Preserve the incoming plan sections and all unrelated source changes.

Additional local Go evidence applies only to that pre-recovery candidate:
targeted parser/local-scanner, usecase-policy and installer-policy selections
produced 20, 21 and 18 PASS test/subtest events, with no failures or skips. The
full enterprisepolicy package produced 57 PASS events and one Darwin-only skip.
The full skillsource package produced 24 PASS events but failed ten top-level
Git-backed tests (15 failure events with subtests). An OSRunner probe refused
the environment's missing live procfs task-children interface; those failed
results are retained and no containment requirement was relaxed.

Overlaying the incoming unguarded discovery routes demonstrated the intended
red failures: local discovery returned lstat, five Git spellings each launched
one source command, and the public picker launched one source command. That
auxiliary CLI build disabled inlining for the package after ordinary compilation
was killed. Fixed-source green compilation was also killed before tests ran.
No green CLI result is claimed, no compiler option changes in the repository,
and the interrupted production-only attempt produced no binary or journey result.

Publish the reviewed final correction with an expected-head lease. Standard
current-source Required/Core/package-manager, Pi and applicable native execution
remain mandatory. Superseded revisions, source reviews, skips and queued or
cancelled jobs do not qualify the final source. Follow an ordinary expected-head
squash merge with actual-main verification, including all six package-manager
hosts. Preserve pins, runners, timeouts, custody, lane selection, installation
authority and acceptance. No release or tag is authorized.

### R31 - Correct composed refusal fixtures and keep prefetch within the size gate

The supported Core run on `4c34ae41` exposed two concrete problems in the
new tests. A bare `newCLIFixture` has no installer, so explicit `codex` target
admission refused with an empty supported-target list before the intended
enterprise policy check. The whole-collection callback compared synthetic
package manifest names with raw Skill names, so neither its first-admission
nor its later-denial branch ran. The compound failure assertion alone did not
establish which individual state predicates changed.

The corrected local and HTTPS fixtures use the existing `newSavedJourney`
composition. They retain typed policy/configuration refusal, zero publisher
requests before refusal, successful read-only catalog listing and complete
saved-file snapshots; native-call checks are also explicit. The collection
callback identifies members through `Envelope.Source.Skill.Name`, requires
the first admission before the second member, and retains the real component
policy adapter for that later refusal. It still requires no partial native,
durable or source effects. No production admission or assertion is weakened.

The CLI full-file gate independently reported 41 statements in
`prefetchGitRoots`, exceeding its unchanged limit of 40. Building the same
ordered raw target strings directly with an append loop removes the temporary
target-ID assignment. No guard, target identity or policy authority changes,
and no helper or lint exemption is introduced.

The actual five-file correction received independent source review; the pinned
formatter and whitespace check passed. The recovered earlier local Go attempt
had been killed during compilation and ran zero tests. It is not a PASS; the
new source still requires supported-runner Core/lint and all applicable E2E.

The preceding `4c34ae41` Intel package-manager artifact `15243575525` was
independently byte-audited: SHA-256
`a80f4b204fc240e0bda0cfaad38e18543727ce034d39cbbd936882409eab5c61`,
valid ZIP CRC, all 23 phases and 46 stream hashes, all 140 required roots,
463 raw RUN/PASS nodes and 17 agreement roots, without failed/skipped inventory.
The six production collection commands passed. This remains evidence for the
preceding tree, with native-runtime and independent collection-scanner limits
unchanged; it does not qualify the new correction or actual merged main.

### R32 - Cover every captured symlink representation in the refusal regression

The accepted main diagnostics merged in `7789e68f` preserve the three existing
cold Copilot Project custody refusals. On that exact source, the focused local
suite ran 17 tests: 16 passed and the new singleton regression failed because
it expected only metadata/link keys while the actual rejected key had the
ordinary file prefix. The unchanged shared snapshot records the same symlink
as a file-key `symlink` marker, a private link-target string and a metadata tuple.
The refusal iterates a set union, so any of those captured entries can be first.

Correct only the test expectation to require one of the three exact captured
key/value shape triples. Keep the actual refusal, exact singleton key, one
delta call/item, zero omitted changes, bounded private diagnostics and unchanged
filesystem assertions. No production predicate, snapshot authority, source pin,
runner or acceptance gate changes. Independent review confirmed this cause
and the bounded test-only correction before implementation.

The corrected complete Copilot refusal suite passed all 17 tests with no skips
or failures. Three bounded hash-seed probes of the affected regression also
passed: seeds 0 and 2 observed the metadata shape, and seed 1 observed the
previously omitted file-key shape. No claim is made that these probes executed
the third, independently source-verified link-target shape. The complete suite
log SHA-256 is
`b60990251389487f8c40f742a07b8fb6dfdb211b3bd8cfb5f3fc515d56498188`.
These inert filesystem/diagnostic tests do not qualify a native lifecycle. Final-source
Required/Core, full package-manager/Pi and applicable native E2E still precede
the ordinary expected-head squash merge and actual-main qualification. Earlier
successful steps remain tied to their original source; no release or tag.

## Qualification record

- The combined EnterprisePolicy, LegacyRepair and legacy local-policy CLI test
  selection passed: CLI internal package 5.163s, production composer 0.012s.
  This includes meaningful positive and negative real HTTPS archive journeys,
  original pin intent, changed policy, removal, selected scope, safe diagnostics,
  recorded child divergence and successful graph reconciliation.
- Initial shared domain/adapter/core tests passed; the required core run also
  passed domain, policy adapter, environment, environmentapp, architecture,
  transaction and usecase packages. The full local make test-core failed on
  unavailable native process observation/Unix sockets and a killed environment
  store test process. It is not counted as a successful full-core run. The
  supported GitHub Core fast runner supplies full native/core qualification.
- Published commit 5cbd057e passed the full domain and enterprisepolicy adapter
  packages and the focused usecase/SDK policy selections. Its three production
  Enterprise archive E2E cases passed in 2.814s. These results do not qualify
  later changes until those changes are verified separately.
- Lint baseline verification at 5cbd057e passed with 137 unchanged existing
  entries. Earlier all-module size/architecture checks passed; a first
  changed-lines attempt was killed by host resource pressure.
- The official archive suite exercised the 5cbd057e binary with the real
  scanner: lifecycle checks passed, installation checks passed in 18.42s and
  production composition passed in 8.995s. Portable catalog compilation was
  killed by SIGKILL, so the overall official suite is not counted as PASS.
- The GitHub Core fast test-core job
  [140293623466](https://github.com/777genius/universal-agent-plugins/actions/runs/45934226946/job/140293623466)
  completed successfully for 5cbd057e, including CLI internal/composition,
  usecase, domain and policy adapter packages. This is job-level evidence;
  it does not count the workflow's other queued jobs as passing.
- After the executor recovered, the published b993da43 domain and file-adapter
  packages passed locally, as did the focused SDK/usecase policy regressions
  (0.826s and 0.052s), including all eight SDK callback cases. That published
  commit also passed the complete GitHub test-core and macOS Intel package-manager
  jobs; the other queued jobs are not counted as passing evidence.
- The subsequent source-admission and diagnostic refinements passed the combined
  EnterprisePolicy/LegacyRepair/local-policy CLI selection (6.249s internal,
  0.011s composer). Full make lint passed all four modules, both gate passes,
  and the legacy no-growth baseline with 137 unchanged exclusions. No lint
  suppression or safety check was weakened.
- The execution interruption was recovered without losing the working tree.
  The integrated code was published in dabd3f0d; subsequent completed commits are
  published immediately. No stale, skipped, cancelled or queued result counts
  as passing evidence.

Remaining at this checkpoint: final-source portable catalog and supported-runner
checks, ordinary expected-head merge, and verification of the actual merged main.
No release or tag.
