# Retained-data project follow-up correction — implementation evidence

2026-10-23. The separately reviewed plan and both independent GO reports were
committed at `b485a24a` before source changes. This is an output correction for
the genuine Codex project-removal artifact's missing scope/root, with no change
to removal, confirmation, retained-data ownership or native client behavior.

The shared CLI renderer retains the exact existing user command. In project
scope it appends explicit project scope and the selected canonical App root,
using the existing argument-quoting convention. Both producers use it:
imperative grouped removal's JSON/human retained_data_action and the human
saved-remove hint. Saved rendering now receives its selected App and continues
using that App's environment ID for alias/target selection. Portable formats,
receipt fields and user output do not gain project roots.

The new public composition regression exercises imperative JSON, imperative
human and saved human removal separately. Each installs user/A/B with retained
data markers, with A's root containing spaces, an apostrophe and a literal
dollar sign. An inert POSIX `set`/`printf` step captures the emitted command's
actual word boundaries; it never executes agentplugins or purge through a
shell. The captured argv must exactly retain the installation ID, project scope
and root. Feeding those argv to the public CLI from an unrelated cwd purges
only A's owned retained data/receipt. Complete user/home and B snapshots also
cover other native/state/data bytes and shared immutable cache preservation.
The presentation proof requires sh and makes no universal Windows-shell claim.

The initial test compilation hit actual shared RAM ENOSPC before execution;
root reclaimed only old reproducible task-cache entries and the bounded rerun
completed. The first executed fixture attempt used a nonexistent remove --yes
flag; the test was corrected to the existing explicit-target command contract
without changing production confirmation or guards. All three public routes
then passed in **2.046s**. The final rerun after test-only lint corrections passed
in **1.987s**.

The selected existing user/grouped-removal, saved-removal and ownership-checked
purge regressions passed in **0.075s**. Existing path-free user JSON and human
wording assertions remain unchanged. Exact pinned golangci-lint **2.13.2**
changed-code and whole-file size/complexity/architecture gates both report
**0 issues**. Python/native/client execution is unnecessary for this correction;
no vendor process, trust/auth action, model or MCP payload was run.

Both independent implementation reviews are **GO** at frozen source
`33004e06cf5d65f112efad0455cc793bc0a56f39`, with complete reports committed beside
this note. R1 independently replayed all three public routes in 2.004s; R2 did
so in 2.130s and also passed affected existing user/saved/removal tests. Both
verified exact selected App context and unchanged user sentence bytes. No
production correction was required after review. Root owns feature-stack
publication. No native, all-client E2E or release qualification is added by
this correction.
