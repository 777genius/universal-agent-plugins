# Grok hosted research supplement independent plan review R2

Reviewer: cursor_clients, 2026-10-23. Reviewed pre-code plan
`2b49b18710066820ea88ab439d219710830f6bd1` in work/uap-project-grok.
The original client plan and its two GO reviews are not reopened.

## Initial decision: HOLD for two bounded-collection clarifications

1. The nineteen native observations at thirty seconds each, plus the ninety
   second acquisition budget, exceed the ten-minute outer workflow timeout.
   Specify an independent total collector deadline/budget with time left for
   failure-summary creation and artifact upload, or increase the outer timeout
   with an explicit bounded margin. A hard job cancellation cannot itself
   guarantee the promised post-admission summary/upload.
2. The new complete native-created file inventory needs explicit resource and
   filesystem limits: maximum entry count, per-file bytes and aggregate bytes;
   no-follow inspection; symlink/special metadata recorded without following
   links or reading special files; overflow is failed collection. Otherwise
   this additional evidence path could bypass the reviewed acquisition and
   stdout/stderr bounds through unbounded reads.

Both findings were sent to the author before implementation. The remaining
plan is appropriately bounded in authority: unchanged real hosted guard before
effects, exact measured official archive/binary pins honestly distinguished
from a vendor digest, fresh credential-free profile and independent project
Git boundaries, text-only plugin/skill candidates, list/inspect observations
without native registry mutation, shared Qoder helper preservation, strict
failure accounting, and project_source_qualified:false until genuine populated
outputs are independently interpreted. No native source identity, discovery,
precedence or stronger project runtime proof is inferred from valid JSON.

## Final decision: GO at `9314feef8056812465552f3f76ebd4e6a19b4617`

Both initial findings are closed in the frozen plan before code. The collector
now has an independent hard eight-minute wall deadline covering acquisition,
fixtures, processes and inventories, backed by a Linux interval timer and
monotonic checks. Probe deadlines use the smaller remaining budget. Cleanup
is bounded, and the timer is disabled for a bounded failure summary, leaving
margin before the ten-minute outer timeout/upload. Budget exhaustion is a
truthful incomplete collection failure, rather than an obligation to schedule
all nineteen probes beyond the budget.

Combined inventory admission now limits entries, depth, individual file and
aggregate content bytes. It uses no-follow inspection and regular-file opens,
type/identity and before/after metadata rechecks, bounded symlink/special
metadata without following or reading special files, and refuses multiply
linked regular content. Overflow, volatility or missing baseline fails the
collection and cannot become partial success. The implementation review must
verify these bounds remain enforced during walks/reads and failure paths.

This GO approves only the supplemental research plumbing described above.
Commit both independent plan/fix GO reports before implementation. Genuine
populated native output interpretation and the original project's stronger
E2E requirements remain subsequent work.
