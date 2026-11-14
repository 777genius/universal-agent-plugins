# Claude failure-listing diagnostic plan — independent R1

Reviewed documentation-only frozen `423d912ff56c033cbae455786b6176a64394cfac`
in `work/uap-project-claude` on 2026-10-23. **GO** for this test-only diagnostic
supplement, after both independent plan GO reports are committed. No Claude
process, acquisition or native operation was executed during review.

Independently traced the existing project fixture. Its installer run can return
an error before the later skills/listedPath probes, so the existing failure
artifact can lack the raw native project listing needed to assess a decoder
refusal. The unchanged real disposable-host guard precedes fixture creation.
The fixture already owns the verified client path, original explicit profile
environment and existing A/B project directories, and its evidence writer hashes
captured JSON/log files. The plan uses those existing authorities rather than
guessing an install ID, root, profile or native shape.

The proposed first-error-only capture is at most two fixed read-only
`plugin list --json` calls, A then B, with the same genuine verified binary and
fresh profile. Successful installer paths incur no diagnostic calls. No retry,
mutation, guessed plugin details, trust/account/model/MCP action, parser change,
new acquisition or guard alteration is introduced. The original installer error
and failed lifecycle stage remain failed; diagnostic failure is recorded
separately. The foreign-byte check still runs after collection.

The stream design is appropriately bounded while running: distinct 1MiB writers
for stdout/stderr, cancellation on overflow, independent ten-second contexts,
closed stdin, no shell and at-most-250ms WaitDelay for inherited pipes. Preserve
the available capped raw bytes even for errors, malformed output or overflow,
plus bounded exact argv/cwd, exit/error/deadline/limit/elapsed and hash/count
metadata. This does not promise OS isolation or general descendant sandboxing.
Inert tests must exercise exact caps, cancellation, separate-stream accounting,
first-failure call selection and bounded metadata without executing Claude or
spoofing hosted admission.

No blocking plan issue found. Both implementation reviews still precede root's
exact hosted replay, and both reviewers must interpret the resulting genuine
listing before any production decoder/scope amendment. Neither collected JSON
nor passing existing user suites may relabel the failed project lane, runtime,
trust, another platform, all-client E2E or release qualification.
