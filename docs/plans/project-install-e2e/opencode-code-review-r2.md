# OpenCode project adapter independent implementation review R2

Reviewer: cursor_clients, 2026-10-23. Initial reviewed checkpoint
`f94906f42e788791e15d198127144ad8fe2c40f2` in `work/uap-project-opencode`,
including production adapter `79cc15cf`, qualification fixes `69f97713` and the
public composed saved lifecycle. Governing instructions, original two-GO
client plan and author implementation evidence were read.

## Initial decision: NO GO pending selected-data authority correction

`clients/opencode/project.go` ProjectActiveNative ignores its selected
data-path argument and derives MCP ownership from the old package's stored
OpenCodeProjection.DataRoot. Native projection validation only requires that
stored data path be absolute; it does not compare it with the independently
selected DataReceipt.Locator supplied by core. Thus a read-only reprojection
can silently acknowledge old-locator desired receipts when the selected data
authority differs. The corresponding Cursor adapter review found the same
class of issue. This is not a request to rewrite immutable package bytes.

Required correction: on the project branch, read and validate the existing
projection against selected native/config/package authority and compare the
stdio data locator with the selected data-path argument. A mismatch must
refuse or require refreshed projection. Add a correct-data positive case and
wrong-data refusal with unchanged immutable bytes. Author/core acknowledged
and is implementing this narrow correction; no reviewer-owned production
edit was made.

## Boundaries reviewed without another blocker so far

- Resolved project layout keeps root `opencode.json[c]` separate from
  `.opencode/skills`, preserves the original detected user ConfigRoot and
  validates explicit target context/native root through projection, registry,
  apply, verify and removal. Containment refuses native aliases.
- Native ownership validates project object IDs/names/exact paths. Skills
  remain leaf paths; MCP keys retain native slash/space semantics and are not
  converted into filenames. Unowned/disabled foreign MCP entries reserve their
  keys and cannot be adopted. JSONC preservation and exact receipt/conditional
  kernel behavior remain in use.
- Existing durable dialect transition schema retains binding scope and native
  root authority. Project pending/reconciliation selection requires an equal
  explicit ProjectRoot, rejects missing/different project contexts before
  mutation, and derives private skill transaction roots from the stored split
  layout. Current state and recorded old-state layouts must agree. Project
  committed state remains prepared plus installed verification.
- Activation preflight precedes generic attestation admission and direct
  activation reuse. Exact configured bytes do not become effective native
  config/discovery/permissions/MCP proof. User lifecycle and legacy user helper
  layouts remain separate.
- Production recorder constructions pass the selected ProjectRoot in CLI and
  installer service/observation/recovery. Namespace preflight derives the root
  from neutral validated context without importing a concrete client adapter.
- The public composed journey uses real TLS Git with an explicitly inert
  version-only TEST process. It covers saved A/B isolation, locked no-op,
  exact-old-pin repair after source advance, interrupted directory recovery,
  moving-source update, conservative status, retained data and exact/repeated
  removal. Those are fixture contracts; genuine native discovery/runtime and
  the generic Git/ZIP transfer proof remain separate evidence.

## Independent verification and fix follow-up

After coordinated capacity recovery, independently inspected frozen correction
`90fcc0af34008a4416758c0c9d83678e72323d5d`. Project active reprojection now
validates the scope-selected split native layout, exact active root and
contained package path, original config candidates and package root. A stdio
selection requires the caller's nonempty clean absolute data locator to equal
the immutable stored DataRoot. Mismatch returns the existing projection-refresh
error instead of rewriting or acknowledging old package bindings. Existing
user reprojection behavior remains intact.

The new regression supplies matching package/data authority and requires
identical desired objects. Empty/relative/different-project/different-receipt
data and a mismatched active package root refuse. It checks unchanged package,
native, user and project-B byte inventories after the read-only refusals.

Independent Go 1.26.10 execution with GOMAXPROCS=3, `-p=2 -count=1`:

- `./clients/opencode ./providers -run '^TestProjectOpenCode|^TestNativeTransition'`:
  client PASS, 0.964s; providers PASS, 0.143s. This includes the project durable
  crash/reopen boundaries, selected-context refusal and new data contract.
- Production public CLI `./cmd/agentplugins -run '^TestProjectOpenCodeComposedSavedLifecycle$'`:
  PASS, 3.526s.

The compiled crash/version helpers are explicitly inert TEST processes. No
genuine OpenCode process, authored MCP command, package payload, model call,
login, trust or account action was run by this review.

## Final decision: GO at corrected checkpoint 90fcc0af

The selected-data blocker is closed, with no remaining implementation blocker
found in the reviewed scope. GO for configured project components, scoped
durable recovery and the composed saved lifecycle. Effective native config,
real skills/MCP discovery and harmless runtime markers, precedence and fresh
native restart/removal remain genuine hosted evidence gates. The passing
fixture journey and this review do not establish those facts or release
readiness.
