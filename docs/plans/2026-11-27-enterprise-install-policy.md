# Enterprise installation policy

Status: implementation and repeated independent source review complete. The
combined policy/legacy CLI regression selection passes. Final lint, portable
E2E and current-head supported-runner qualification remain in progress; merge
and actual-main verification follow those gates.
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
- Three production-composition Enterprise archive E2E cases passed before the
  final repair refinements; their final-source rerun is required.
- All four modules passed the lint size/architecture pass before the final
  operation-intent refinement. The first changed-lines pass was killed by host
  resource pressure. A bounded-memory complete rerun is in progress.
- The execution interruption was recovered without losing the working tree.
  The integrated code was published in dabd3f0d; subsequent completed commits are
  published immediately. No stale, skipped, cancelled or queued result counts
  as passing evidence.

Remaining gates: final lint, final-source production/archive E2E, current-head
supported-runner checks and review findings, ordinary expected-head merge, and
verification of the actual merged main. No release or tag.
