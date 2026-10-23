# Cursor project adapter independent implementation review R1

Reviewer: codex_claude, 2026-10-23.
Reviewed adapter commit `524b808b401d29ebe5f932dc7ce0f5258746f31d`
in `work/uap-project-cursor`. The shared ProjectComponents helper already has
separate independent implementation reviews; this review concentrates on the
final Cursor adapter, production composition, projection and lifecycle.

## Decision: GO for the configured project component implementation

No correctness blocker found in the reviewed adapter diff. This decision does
not qualify Cursor native discovery, an MCP handshake or tool result, fresh
native-session behavior, plugin installation, genuine Stop, or release readiness.
The implementation and evidence correctly keep those observations pending.

## Reviewed boundaries

- Project scope selects exact `<project>/.cursor` native authority while the
  detected user ConfigRoot remains distinct. Planning, activation, inspection,
  binding validation and removal revalidate the selected project/native root;
  mismatching scope, original user root and copied project context refuse.
  Prepared package state remains under the selected project's `.uap` namespace.
- Project policy is declared through core's scoped native-config lifecycle
  traits. Existing user preparation and its CLI-registry traits remain intact;
  project mutations do not invoke a user CLI or redirect HOME/ConfigRoot.
- Selected skills, including existing rendered command skills, receive exact
  directory ownership. MCP projection goes through the existing exact-entry
  kernel. The adapter strips only the shared managed-package receipt before
  handing objects to the native component helper; unknown ownership kinds are
  retained for closed refusal. Removal never treats a shared config as owned.
- Stdio projection uses the trusted managed launcher with absolute package,
  persistent-data and cwd bindings. Active reprojection reads already-bound
  staged bytes without executing or mutating them. Remote Cursor native config
  uses the documented URL/headers shape. The official MCP documentation permits
  the URL-only remote shape for HTTP/SSE; no native execution claim follows.
- Automatic file success is downgraded from shared activation success to
  `prepared` plus `installation_verified`, with discovery, aliases, enablement,
  approvals, policy and MCP initialization explicitly unobserved. Generic
  `ActivationComplete` is rejected by preflight before provider shortcut
  admission and also by direct adapter reuse, including prepared selections.
- Provider contracts cover unchanged verification, update, missing-same-pin
  repair, withdrawn MCP, repeated remove, persistent data, malformed JSON,
  foreign disabled/member/skill collisions, writer failure rollback, contained
  alias refusal, and user/project-A/project-B byte-and-mode isolation. These are
  loader/planner/stager/provider fixture contracts; they are not a claim that
  native Cursor was run or that a hosted all-command journey has completed.

## Independent verification

Using the shared Go 1.26 toolchain and bounded existing caches:

- Complete `clients/cursor` package: PASS, 0.141s.
- `providers -run 'TestProjectCursor|TestCursor' -count=1`: PASS, 0.168s;
  this selected all newly added composed Cursor project contracts. The matching
  pattern selected no client-package tests, so the complete client package was
  run separately as recorded above.

Inspected the production activation preflight/attestation order, shared MCP
projection and nativeconfig neutral projection, scoped domain declaration,
existing user projector and all four new provider fixtures. Consulted current
official `https://cursor.com/docs/mcp`, which documents project/global config
locations, URL/headers remote entries and native enablement/approval boundaries.
No Cursor process, model task, package payload, trust action, login, configured
MCP command or launcher was executed by this review.

## Remaining evidence boundary

The authored fixture proof and pinned official help do not establish runtime
discovery or initialization. A genuine pinned Cursor session in the approved
disposable-host workflow remains the separate gate for native observations.
Public reporting must keep this adapter's configured/prepared status and those
pending native observations distinct. A separate independent R2 review is still
required for the final adapter checkpoint.

## Final R1 selected-data correction follow-up: GO at f55dc462

Independently reviewed the narrow corrective commit `f55dc462` after the root
identified the analogous stale selected-data authority gap. Project active
reprojection now validates each selected stdio entry against the exact managed
launcher path, selected active package, selected data receipt and resolved cwd
anchor. Both native PLUGIN_ROOT/PLUGIN_DATA values and the launcher's six-field
package/data/cwd protocol prefix must agree; missing, changed or stale values
return ErrProjectionRefreshRequired. This is read-only validation of already
digest-verified package bytes. Existing user projection and remote-only native
entries retain their prior contract.

The existing composed stdio fixture now refuses an empty locator and a changed
selected receipt, while comparing the complete immutable active-package
inventory after each refusal. It still accepts the exact selected authority.
Independent `providers -run '^TestProjectCursor' -count=1` **PASS 0.065s**
(Go1.26.10/GOMAXPROCS3/-p2). The same pattern selected no client tests, which is
recorded rather than presented as new full-client execution; the earlier full
client run remains recorded above. No blocker remains in this correction.
Neither review execution launched Cursor, its launcher or an MCP process, and
the genuine hosted native boundary is unchanged. The independent R2 author
has separately recorded GO for the same corrective checkpoint.
