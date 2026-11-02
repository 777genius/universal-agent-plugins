# Kimi single-tool: diagnose only the two unobserved approval-poll refusals

PRE-CODE proposal. No implementation until two independent GPT-6.1 high/xhigh
plan reviews. This is independent of the approved production intact-repair fix
in /tmp/uap-additional-kimi-scoped-selection-20261027; preserve that work and its
native byte-stability assertion. No VS Code work or release. Read AGENTS.md,
docs/PROJECT_INSTALL_OWNER_SCOPE.md, docs/AUTHORING_CAPABILITY_PRESERVATION.md
and docs/plans/project-install-e2e/completion-plan.md.

## Exact observed boundary

Published source b2bb2b7f85b5461420f932478b197905e6fc5ad9,
tree b12354b2565a05d14037556f8493b149d7d36ea2. Genuine run40569825577,
single-tool job122438517585, failed after12.939s. Native artifact12768461663:
7895B, outer0500d6d9cafb34246f71cfd557db0fca848125d0a4d120cffbefdebe764c4ab3,
inner171da150010ee68b6aa8af632863447ad411c3bcca09f48d9d1d22a6e7ddcd29,
23 closed members. Frontend12768386913:3518B,25members,
outer66746161fca39fda9cdf71bfbe07060e2a1b85b98be4296500bd3fb6143d44da.
Exact-source admission, clean source/tree/build and24frontend stream joins
pass. Root audit /tmp/uap-additional-qoder-kimicall-fresh-ka537i9_/audit.json
SHA2569d48d60ff30934eda04e370893e74e130ed9d1843289b3d660c1e14cbbec02cf.

Public add succeeds Prepared. Actual probe events contain initialize and
tools/list, no tools/call, and one provider request is retained. The collector
emits only its closed generic failure: stage=approvals,phase=validate,
exception_kind=validation,observer_entered=true,service_recorded=true.
Cleanup reports group_reaped and port_closed, no forced kill, SIGTERM sent,
no shutdown acknowledgment. Private service/HTTP streams remain quarantined.
No approval_refusal optional facts or python-api.json were exported. All five,
runtime-complete, authentication and real-model flags remain false.

The source already projects finite facts for strict single-row ValueError via
kimi_approval_facts.Refusal. It deliberately leaves ResponseSchemaRefusal and
its separately reviewed graceful branch unchanged. The current approval-poll
loop can also raise a generic ValueError when more than one pending item is
returned, or when all20polls finish without a selected approval. These two
refusals carry no finite explanation today. The raw result does not establish
which occurred. Elapsed time is not proof of polling exhaustion. Do not infer
an actual approval shape, request count, native turn ID, missing permission,
tool execution, timeout or binary/source defect from this failed envelope.

## Narrow implementation

Add a separate typed ValueError subclass for exactly these existing two
polling refusal sites (including the duplicate post-shape multiple-items
check, if retained). Construct it using only a fixed reason and caller-owned
poll ordinal; pass no pending row, value, exception text, session, ID, path,
body, credential or arbitrary native text.

Finite public object, provisionally:
- topic: single-approval-poll
- status: refused
- reason: multiple-pending OR no-pending-after-polls
- polls: exact integer1..20; no-pending-after-polls requires exactly20.

A dedicated strict validator rejects unknown/missing fields, nonexact builtin
types (including bool-as-int), unknown reasons, impossible counts and arbitrary
strings. It returns an independent copied object. Keep the existing single-row
Refusal/project/validate API and its facts unchanged; use a separate method/type
in the existing small diagnostic module or a focused small sibling if lint/size
requires it. No acceptance branch or native permission is introduced.

_KimiFailureDiagnostic may retain this separately validated optional object only
for collector=single-tool,stage=approvals,phase=validate when the original observer
actually raises the new typed refusal. Preserve first retained cause; do not let
cleanup errors or a later unrelated phase acquire/replace it. Emit it only under
those same closed conditions, as an optional approval_poll_refusal field in the
existing <=2048byte failure line. Invalid internal optional facts fall back to
the existing generic failure; never replace or swallow the original exception.

The observer still fails at exactly the same place. It does not catch the new
refusal as a graceful ResponseSchemaRefusal, call approve-once, fetch snapshot/
history, call shutdown, start another process or expose quarantined raw data.
The existing ordinary supervisor determines actual failure cleanup and the
collector exit remains nonzero. This slice authorizes no new approval response,
remembered/blanket permission, native request, route, schema acceptance, polling
iteration, sleep, work/service/total budget, token, source/executable pin, hosted
guard, profile write, cache allowance, provider response or qualification.

The exact native runtime failed-only ZIP schema/readers should need no change:
the diagnostic remains inside the existing stderr member. Verify that existing
closed admission still accepts original bytes unchanged. If an additional
schema consumer actually requires a change, identify its exact source contract
and return for review before expanding the plan.

## Meaningful inert controls

Use existing KimiAttemptContracts/CaptureContext/CaptureService and the actual
observer with the real ordinary supervisor around its existing inert Python
child. No Kimi, installer, provider model, scanner or managed launcher runs
locally; never fabricate hosted eligibility.

1. Return two pending approvals on the first poll, including private canaries.
   Require the same failed observer/quarantine behavior and nonzero failure,
   finite multiple-pending/polls1 facts, no approve-once/snapshot/history, no
   marker/tool call and no new native permission. Unknown row bodies stay absent
   from the serialized diagnostic. Its whole line remains<=2048bytes.
2. Return an empty pending items list for all20polls. Exercise the exact existing
   loop and its real bounds, require no-pending-after-polls/polls20, unchanged
   failure/cleanup and no follow-on action. Do not increase sleeps or deadlines;
   an inert sleep seam may only remove wall-clock delay while asserting20existing
   sleep/poll operations, never alter production timing.
3. A single mismatched row still uses the existing single-row refusal facts;
   response-shape failures retain their original dedicated graceful path. No
   poll facts appear for successful single-row selection, unrelated stages,
   component collector, malformed internal objects or another exception class.
   Preserve all existing identity/permission/privacy/supervisor tests.
4. Strict object boundary tests reject bool counts,0/21/negative/float counts,
   wrong/missing/extra fields, arbitrary/private reasons, wrong topic/status,
   impossible exhausted count, nested/hostile objects and corrupted optional
   facts. Verify returned maps do not alias held facts and original refusal
   survives diagnostic failure. Check byte limit with existing optional fields.
5. Re-admit the unchanged genuine b2bb failed artifact through the exact new
   source reader and compare all bodies/summary. It remains failed/incomplete/
   allfalse and does not acquire a fabricated reason retroactively.

## Integration and publication

Before coding, obtain two exact plan GOs and commit the accepted plan. Use an
isolated branch from agreed current Kimi source; no edits to the original
continuation checkouts. Prefer merging this reviewed disjoint Python slice with
the independently reviewed production scoped-selection fix before publication,
so one genuine hosted run can observe both independent boundaries. Reconcile
concurrent remote movement; never force, overwrite or substitute commits.

Root coordinates actual serial focused and retained Python contracts/AST and
whitespace checks, and exact code review by two independent critics. The other
Kimi author owns the current Go/lint slot; no concurrent test-process suite
while that slot is active. Root alone publishes and updates PR524 after fresh
expected-head checks. A fresh genuine hosted capture and independent raw audit
must establish which refusal occurred, if either. It cannot turn the old failed
run into a pass or establish a full lifecycle, successful tool call, auth/model
qualification, main merge or release.
