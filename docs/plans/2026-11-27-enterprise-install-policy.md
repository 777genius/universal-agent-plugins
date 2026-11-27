# Enterprise installation policy

Status: draft design, revision 1. No implementation or new qualification claimed.
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

Keep `schema_version: 1` and existing `sources.allow_hosts`, `sources.deny_hosts`,
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

Omitted allow-list means unrestricted; present empty list means allow none.
Reject null, unknown fields, duplicate keys, YAML aliases/merge keys, invalid
types/values, multiple documents, unsupported schema and oversized documents.
No arbitrary commands or expressions in policy. No warning-only installation
mode initially: use read-only checks for rollout; active restrictions enforce.

Layers in order for explanation (all conjunctive, no weaker override):

1. Machine policy: fixed platform location (`/etc/agentplugins/policy.yaml`,
   `/Library/Application Support/AgentPlugins/policy.yaml`, Windows system-wide
   ProgramData `AgentPlugins/policy.yaml`). Optional until present.
2. User policy: `<user-home>/.config/agentplugins/policy.yaml`.
3. Project policy: `<selected-project-root>/uap-policy.yaml`; for User operations
   use the invocation working directory. No walking arbitrary ancestor trees.
4. Explicit deployment file `UAP_POLICY_FILE`, if configured, is required and
   additive, never replaces machine/user/project rules. Existing `--policy`
   applies additionally in its current dependency commands.

Read discovered files on every operation, not once per daemon. A file present
at review that disappears before apply is a changed-policy refusal. Missing
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
AddInput, including groups. Recheck current policy at the final apply boundary.
Installer preview/apply must preserve policy authority across user confirmation;
long-lived management sessions cannot reuse an old successful decision forever.
No per-target filtering or silent partial install to make policy pass.

Enforcement matrix to complete before implementation approval:

| Path | Required admission point |
| --- | --- |
| Direct/Directory add and standalone Skills | Known source before acquisition where available; actual envelope before host preparation/apply |
| Dependency graph including transitives and warm cache | All prospective nodes before environment intent/lock/manifest publication |
| update/repair/sync/locked replay | Reevaluate actual retained source, requested pin and envelope; no grandfathered allow |
| import --install | Before `transfer.publish`, then normal shared apply gate |
| Browser management | Same preparation/application boundary; revalidate after retained review |
| Installer SDK | Explicit injectable gate, shared usecase enforcement; no global singleton |
| switch/rebind/migrate format | Audit whether they activate content; gate any activation/provenance replacement |
| remove/export/read/rollback | Do not require install permission; retain ownership and recovery checks |

Do not place universal denial in a raw loader that is also used to remove or
inspect a now-forbidden package. Policy may read an already admitted snapshot;
it may not execute package code to decide permission. Source rules describe
canonical identity, not a network firewall.

## User experience

`agentplugins policy init [path]` creates a commented schema-1 example without
overwriting files. `policy validate <path>` checks configuration without fetching
packages. `policy status` displays effective layers and rules with their paths;
`--require-policy` makes an empty configuration fail in CI. `policy check <source>`
checks the actual package for chosen scope/targets and explains all relevant
denials without installation/native activation. Support existing human/JSON
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
- Full native-plugin inventories cannot hide MCP/hooks in a Skills-only claim.
- Approved installation works; tightening blocks subsequent update/repair;
  ownership-safe removal still works, including invalid policy.
- SDK policy injection and current production CLI composition are both exercised.
- No new real credentials, vendor account calls or broad native-client retesting:
  verify policy at the actual installer boundary with existing isolated fixtures.

## Review log

R1–R3 independent critiques pending. Revision 1 is not implementation approval.
