# Skills picker selection: preserve behavior while passing the size gate — R1

Hosted job 121008406005 at source428bdf5d reports cyclomatic complexity24
for skillsAddOptions.run (limit20). Changed-lines correctness lint passes;
this separate full-file architecture gate must also pass without exclusions,
new budgets or assertion weakening.

Extract only the existing browser-selection boolean into a small private method
on skillsAddOptions. Pass the already computed interactive flag, options, exact
path flag and whether the resolved source has an exact path. Preserve the exact
operator grouping: explicit UI OR interactive automatic selection with no noUI,
no dry run, no skill names, no explicit path, no exact source path and no target.
The method has no I/O and makes no decisions about scope or filesystem authority.

Keep validation, app.forOptions, target parsing, project context admission,
context cancellation, website dispatch, source path validation and source
acquisition in the same order. Keep the existing browser fallback and terminal
flow unchanged. No user-visible text, flags, request schema, lifecycle or SDK
behavior changes. Existing actual project Skills CLI/picker regressions and the
narrow CLI correctness plus full-file size gate provide verification; do not add
an implementation-mirroring boolean table for this mechanical extraction.

Two independent plan rounds precede this small production change, followed by
independent code reviews. No production changes for this slice have been made.

## R2 — independent R1 findings incorporated

APM and Skillshare independently gave R1 GO exactc7d13ed5 after reading the
actual skillsAddOptions.run. Both confirmed that extracting only the original
boolean preserves operator grouping and all prior admission/acquisition order.
The private helper receives already computed values; it performs no context
checks, source resolution or I/O. Keep explicit UI as the unchanged left-hand
OR branch. No browser/terminal fallback, format, option or scope changes, and
no linter exclusion/budget adjustment. The same existing actual scoped CLI and
picker tests plus the full-file size gate will verify the extraction. Final
independent R2 reviews precede the source edit.
