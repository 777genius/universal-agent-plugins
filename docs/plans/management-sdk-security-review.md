# Management SDK and Agent Teams security review

Date: 2026-10-25. Independent source review; no tests, builds or native clients were
run by this reviewer. No production code, scanner configuration, alert state or
repository protection was changed.

## Current follow-up audit

The follow-up source review covers UAP executable source
`5224ebd5f66bbdfab0bef906dba0b089062f6073` and Agent Teams
`501916f1532d0c2bd48eb4867a09228116009c5f`. The files containing all 15 reported
UAP sinks, the receipt validator and `managementAdd` are byte-unchanged from BBC7, verified
by the targeted Git diff. The dispositions below remain applicable to those
paths. The app follow-up source and native-tool fixture received independent
security/application review with no remaining concrete P1/P2 identified.

The app now requires a separate random sidecar capability on managed HTTP
methods before the facade. Only owned main-frame IPC grants an authorized browser
link. Browser storage scoped to the exact origin and tab, URL/bug-report fragment removal, regrant and
stop/restart/startup-failure revocation supplement Host/Origin/socket checks.
The private Go bearer is distinct and remains in main composition. This corrects
the broader HTTP-authority concern; it does not rely on forged local headers as
authentication. The historical app authority description below is superseded.

The source-reference change retains bounded input and obvious local-path refusal
while leaving the remote-source grammar to Go. The new isolated MCP fixture uses
real pinned OpenCode with a controlled local provider; UAP owns MCP configuration
and ordinary installation security assessment. No live model/OAuth pass is claimed.

At this checkpoint UAP aggregate `121152944456` is neutral only because the Go
configuration is still missing while analysis `121149748693` is queued/running.
App aggregate `121154146461` is also unfinished. Neither is a security pass.
Existing UAP alerts 46/47 still require formal disposition despite their source
responses. The other historical annotation numbers are not invented from sink
locations; the completed new scan must be checked. The connector has no alert
dismissal operation and the available browser is not responding. No scanner,
workflow permission, protection or alert state has been modified.

This follow-up records source review, not additional test execution. Actual
Required/desktop/package results are recorded in the plan and live PR bodies.

## Historical scope and gate status

| Repository / source reviewed | Aggregate check | Annotations reviewed |
| --- | --- | --- |
| UAP `bbc7e98da96dd7d77db42f65da7afae84a86f799`, [PR #500](https://github.com/777genius/universal-agent-plugins/pull/500) | `121120190376` | All 15, not only the two inline PR comments |
| Agent Teams `ede65c9ab9f46155eef861559c81bc38d2cbfc0f`, [PR #989](https://github.com/777genius/agent-teams-ai/pull/989) | `121128225094` | All 3: alerts 577, 578 and 580 |

**Review conclusion:** no concrete exploitable vulnerability requiring a
production change was identified in these 18 reported locations under the
documented personal-local deployment model. The per-location reasoning below is
the security disposition; it is not a successful scanner result or merge approval.

**Both historical aggregate security checks failed.** Successful analysis jobs do not
clear their findings. Formal repository security disposition remains outstanding:
the available alert-detail API required authentication, the connector exposed no
alert-review operation, and the browser did not provide an authenticated security
session. Do not dismiss findings merely to obtain green checks, rename values to
evade a heuristic, suppress queries, disable the gate or bypass protections.

Public check annotations were read from these reproducible endpoints:

- [UAP annotations](https://api.github.com/repos/777genius/universal-agent-plugins/check-runs/121120190376/annotations?per_page=100)
- [App annotations](https://api.github.com/repos/777genius/agent-teams-ai/check-runs/121128225094/annotations?per_page=100)

Both returned the source paths and sink lines used below. Public code-scanning
alert-detail requests returned HTTP 401; exact SARIF edge traces were therefore
unavailable through that route. The reviewer instead traced the management input,
callers, validators and sinks in the pinned source. No credentials or access
controls were bypassed. The annotation endpoints and pinned source are sufficient
to reproduce this review without treating an analysis-job success as gate success.

## Authority and trust boundaries

- Go management binds an ephemeral loopback listener. Every API route requires
  the private bearer capability and exact Host/Origin policy through
  [loopbackapi.Guard](../../cli/internal/loopbackapi/http.go). JSON decoding is
  bounded and rejects unknown fields and trailing documents.
- The [HTTP request contract](../../cli/internal/agentpluginscli/management/apitypes.gen.go)
  does not expose arbitrary installer `PackageRoot`, `TempRoot`, native executable
  or command arguments. Roots and executables belong to trusted startup input on
  stdin, then pass [canonical context capture](../../cli/internal/agentpluginscli/management_context.go).
  HTTP local authoring uses captured root IDs and validated relative paths.
- Authentication does not make package content trusted. Source, package-tree,
  receipt and ownership validators remain necessary and are traced below.
- Agent Teams' sidecar is a personal-local browser/network admission boundary:
  raw socket loopback, exact Host/Origin and bounded JSON. Those headers do not
  authenticate an OS account or local process. Its Go bearer stays in trusted main
  composition. Standalone viewing has no mutation owner and refuses management;
  remote/multiuser mutation deployment requires separate authenticated authority.
- The review does not claim isolation from hostile same-user processes racing
  private files. Git authentication helpers supplied by the user remain trusted
  native configuration. Explicit HTTPS acquisition is not an internet-facing
  proxy and does not impose a private-address denylist.

## All 15 UAP annotations

Line numbers refer to the pinned UAP source above. Test-source keys are listed
after the table; they identify inspected regressions, not tests rerun here.

| ID | Reported sink | Input-to-guard trace and disposition | Test sources |
| --- | --- | --- | --- |
| U01 | [management_local_skills_apply.go](../../cli/internal/agentpluginscli/management_local_skills_apply.go):79, `Lstat` | Restore ID passes `localSkillSHA`: exactly 64 lowercase hex characters. Only then is it joined beneath the fixed private receipt directory; `RequireContainedChild` rejects escape and existing symlink components. **Validated-path false positive.** | T1, T2 |
| U02 | [management_local_skills_apply.go](../../cli/internal/agentpluginscli/management_local_skills_apply.go):89, `ReadFile` | Same validated path; preceding check requires a regular file no larger than 65,536 bytes. Strict JSON, operation/backup identity and captured root/Skill matching follow. Save uses a `0700` directory and atomic `0600` file. **Validated private receipt read.** | T1, T2 |
| U03 | [source_direct.go](../../cli/internal/agentpluginscli/source_direct.go):28, `Stat` | Source text reaches the local-archive classifier. [managementAdd](../../cli/internal/agentpluginscli/management_prepare.go) refuses explicit local plugin paths/archives with `SOURCE_NOT_SUPPORTED`; the probe neither reads contents nor acquires/writes a local package. **Classification probe, not arbitrary local-file acquisition.** | T5; management refusal is directly visible in source |
| U04 | [dirswap.go](../../install/integrationctl/adapters/dirswap/dirswap.go):314, `ReadFile` | `Manager.Load` validates one portable leaf ID before joining the captured journal root. Local restoration already applies the stricter 64-hex check. Decoded operation identity and receipt/ownership validation gate recovery. **Constrained private-journal lookup.** | T1, T2 |
| U05 | [pathpolicy.go](../../install/integrationctl/adapters/pathpolicy/pathpolicy.go):111, `Lstat` | This is inside `RequireContainedChild`: absolute/clean paths and strict lexical containment are checked first, then every existing component is checked for symlinks or nondirectory ancestors. **The containment validator itself is reported as a sink.** | T1 |
| U06 | [process/osrunner.go](../../install/integrationctl/adapters/process/osrunner.go):637, `exec.Command` | Acquisition prepends literal `git`; source coordinates remain separate arguments. Endpoint/ref/subpath validators reject helper/file schemes, unsafe refs and option-like hosts/users. Native adapters use startup-captured executables and constructed arguments; HTTP supplies no shell-command field. **Intentional constrained process boundary; no injection path found.** | T5, T6 |
| U07 | [packagedigest/snapshot.go](../../install/integrationctl/agentplugins/adapters/packagedigest/snapshot.go):87, `Mkdir` | Destination is a newly created private snapshot plus a validated relative tree entry. Inspection checks portable segments, reserved names, normalization, collisions and depth before publication. **Constrained snapshot directory creation.** | T3 |
| U08 | [packagedigest/snapshot.go](../../install/integrationctl/agentplugins/adapters/packagedigest/snapshot.go):92, `Symlink` | Entry path and link text are validated. `resolveInternalLink` checks the complete known tree and refuses escape, missing targets, cycles and excessive chains; source walking does not descend through symlinks. **Only internal package links are accepted.** | T3 |
| U09 | [packagedigest/snapshot.go](../../install/integrationctl/agentplugins/adapters/packagedigest/snapshot.go):152, `Lstat` | Root inspection requires a real directory and rejects a symlink root. Management supplies acquired/private roots, not a raw HTTP `PackageRoot`; the generic builder also serves the existing explicit local CLI capability. **Root-validation probe.** | T3, T4 |
| U10 | [packagedigest/snapshot.go](../../install/integrationctl/agentplugins/adapters/packagedigest/snapshot.go):356, `OpenFile` | Destination uses the validated inventory in the fresh private snapshot and exclusive creation. Copy checks inspected/opened source identity, regular-file type and size, with bounded reads. **Constrained exclusive snapshot creation.** | T3 |
| U11 | [packagedigest/snapshot.go](../../install/integrationctl/agentplugins/adapters/packagedigest/snapshot.go):403, `Chmod` | Sealing visits only the validated entries already created in that private snapshot and explicitly skips symlink entries. **No outside chmod path found.** | T3 |
| U12 | [sourceacquisition/acquirer.go](../../install/integrationctl/agentplugins/adapters/sourceacquisition/acquirer.go):360, `Lstat` | A normalized portable repository-relative subpath is joined to a new private checkout. Immutable Git coordinates are validated before acquisition; the reported check refuses symlink/nondirectory selected roots. **Validated checkout-subtree inspection.** | T5, T6 |
| U13 | [sourceacquisition/skill_web.go](../../install/integrationctl/agentplugins/adapters/sourceacquisition/skill_web.go):178, `client.Do` | Explicit publisher selection passes credential-free normalized HTTPS validation. File URLs are derived from that publisher/index directory and rechecked on pinned replay. TLS remains verified; cookies, automatic auth and redirects are not inherited. Bodies, file counts and time are bounded. **Intended local source-download capability, not an unrestricted remote proxy.** | T7 |
| U14 | [installer/package.go](../../install/integrationctl/agentplugins/installer/package.go):77, `Lstat` | Installer root validation precedes snapshot creation. Assessed executable entries must be clean relative paths without traversal, backslash, absolute or volume forms and must name regular files; complete-tree inspection follows. **Validated internal inventory check.** | T3, T4 |
| U15 | [installer/package.go](../../install/integrationctl/agentplugins/installer/package.go):414, `Lstat` | Acquired inventory repeats those relative-path/regular-file checks after source-authority validation. Resealing requires exact digest, executable inventory and acquisition identity. **Validated acquired-inventory check.** | T4 |

No production correction is required by these reported paths on the reviewed
evidence. This conclusion depends on the boundaries above; it is not a claim that
the same handlers are appropriate for arbitrary untrusted remote callers.

### Inspected regression sources

| Key | Source and relevant assertions |
| --- | --- |
| T1 | [pathpolicy_test.go](../../install/integrationctl/adapters/pathpolicy/pathpolicy_test.go): unsafe leaf IDs, strict-child containment and symlink ancestors. |
| T2 | [management_local_skills_test.go](../../cli/internal/agentpluginscli/management_local_skills_test.go): traversal/symlink rejection, changed backup refusal, interrupted directory swap, private undo retention. |
| T3 | [snapshot_test.go](../../install/integrationctl/agentplugins/adapters/packagedigest/snapshot_test.go) and [snapshot_posix_test.go](../../install/integrationctl/agentplugins/adapters/packagedigest/snapshot_posix_test.go): external links, reserved metadata, special files, Unicode/case hazards, source independence and size/count/depth limits. |
| T4 | [snapshot_request_test.go](../../install/integrationctl/agentplugins/installer/snapshot_request_test.go): invalid executable inventory and altered acquired provenance. |
| T5 | [source_test.go](../../install/integrationctl/agentplugins/sourcespec/source_test.go) and [git_option_injection_test.go](../../install/integrationctl/agentplugins/sourcespec/git_option_injection_test.go): unsafe URL/ref/path inputs, credential redaction and option-like Git/SSH endpoints. |
| T6 | [acquirer_test.go](../../install/integrationctl/agentplugins/adapters/sourceacquisition/acquirer_test.go), [git_transport_test.go](../../install/integrationctl/agentplugins/adapters/sourceacquisition/git_transport_test.go) and [git_policy_test.go](../../install/integrationctl/agentplugins/adapters/sourceacquisition/git_policy_test.go): exact immutable subtree acquisition, authentication/routing isolation and TLS/file-rewrite policy. |
| T7 | [skill_web_test.go](../../install/integrationctl/agentplugins/adapters/sourceacquisition/skill_web_test.go) and [web_skill_policy_test.go](../../install/integrationctl/agentplugins/domain/web_skill_policy_test.go): unsafe indexes before file fetch, publisher/file coordinates, redirects, bounds, encoding, cancellation and certificate refusal. |
| T8 | [loopbackapi/http_test.go](../../cli/internal/loopbackapi/http_test.go): missing bearer, wrong Host/Origin, unknown/trailing/oversized JSON. |

## All 3 Agent Teams annotations

App links below are pinned to the reviewed revision.

| Alert / sink | Trace and disposition |
| --- | --- |
| 577 — [uap-management-desktop.ts](https://github.com/777genius/agent-teams-ai/blob/ede65c9ab9f46155eef861559c81bc38d2cbfc0f/scripts/e2e/uap-management-desktop.ts):344 | `wait` evaluates harness-authored JavaScript selectors. Labels/selectors are JSON-encoded; raw Skill names originate from fixed `uap-ui-demo` and its fixed copy suffix. Source content, registry data and page text do not become evaluated expressions. **Controlled test-expression origin; false positive.** |
| 578 — [uap-management-desktop.ts](https://github.com/777genius/agent-teams-ai/blob/ede65c9ab9f46155eef861559c81bc38d2cbfc0f/scripts/e2e/uap-management-desktop.ts):368 | `click` consumes the same authored expressions, obtains a DOM coordinate and dispatches CDP input. Text entry uses `Input.insertText`. **Same false-positive reasoning, not a blanket exemption for tests.** |
| 580 — [uap-management-desktop-e2e.yml](https://github.com/777genius/agent-teams-ai/blob/ede65c9ab9f46155eef861559c81bc38d2cbfc0f/.github/workflows/uap-management-desktop-e2e.yml):69–71 | Source pin comes from tracked app provenance, not dispatch input; repository is fixed and staging checks exact SDK/source identity. Untrusted PR execution has PR scope; a default-branch dispatch reads default-branch metadata. **No demonstrated attacker-controlled revision in default-branch cache-write context.** Full rationale follows. |

### Alert 580: corrected cache-authority rationale

Removing configured dependency caches does **not** itself resolve this rule. The
[official query help](https://codeql.github.com/codeql-query-help/actions/actions-cache-poisoning-poisonable-step/)
describes malicious checked-out code using the runner cache token directly.
`contents: read` and `persist-credentials: false` do not independently eliminate
that capability. The review does not rely on them as a cache-security boundary.

The current workflow has no `workflow_dispatch` revision input. Its first checkout
uses the event's application revision; the only UAP revision is read from that
checkout's tracked [provenance.json](https://github.com/777genius/agent-teams-ai/blob/ede65c9ab9f46155eef861559c81bc38d2cbfc0f/vendor/uap/sdk/provenance.json).
The external repository is the fixed `777genius/universal-agent-plugins` literal,
and the [stager](https://github.com/777genius/agent-teams-ai/blob/ede65c9ab9f46155eef861559c81bc38d2cbfc0f/scripts/stage-uap-runtime.ts)
checks source identity against that same SDK pin. A low-trust PR can change its
metadata but runs in PR context; default-branch dispatch reads the default
branch's repository-controlled metadata. An immutable SHA proves identity, not
trust by itself: the pin's origin and event scope provide the missing distinction.

The official [ActionsSHACheckout model](https://github.com/github/codeql/blob/main/actions/ql/lib/codeql/actions/security/UntrustedCheckoutQuery.qll)
includes a heuristic for dynamic checkout reference fields containing `head`,
`sha` or `commit`, explaining its classification of `steps.uap.outputs.commit`
without establishing an attacker-controlled input in this workflow. Do not rename
that field or suppress the query to avoid the heuristic. No workflow security
code change is justified by the reviewed finding; formal security disposition
must retain the provenance/event-scope explanation and the failing-check status
until the repository accepts it through its normal process.

## Relationship to functional qualification

This document disposes only of the named security annotations. It does not claim
that current Electron interaction, packaging, project/worktree support or all
other review findings passed. Those gates and exact artifact receipts belong in
the [management implementation plan](management-sdk-and-agent-teams.md) and the
Agent Teams integration plan. No test was newly executed for this audit.
