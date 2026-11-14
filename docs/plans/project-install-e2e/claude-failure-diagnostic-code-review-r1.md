# Claude bounded failure-listing diagnostics — independent code R1

Reviewed frozen clean `0b85d4a293beee1be05733cfc75dd4fd9afd260a` in
`work/uap-project-claude`, after the plan and both GO reports were committed at
`c53e030e`. **GO** for the diagnostic-only checkpoint. Reviewer did not author
the implementation and executed no Claude/native process or acquisition.

The production parser, Claude adapter, opt-in and real disposable-host guard
are unchanged. The four-file diff adds test-only capture/metadata/contracts,
first-error fixture wiring and truthful evidence. Successful installer runs
take the original path. The fixture's latch permits only the first returned
installer error to trigger the fixed two A/B listing observations. It uses the
already verified genuine client and fresh explicit original profile environment,
then retains the original result/error and existing foreign-byte check. The
lifecycle marks a stage failed before running its installer, and the original
installer stdout/stderr are already recorded before this additional capture;
diagnostics do not mark it passed or retry it.

Each capture has its own ten-second context, empty stdin/no shell, separate
stdout/stderr writers capped live at 1MiB, cancellation on overflow and 250ms
WaitDelay for inherited pipes. Writer state is mutex-protected, never appends
beyond its cap, and returns an independent snapshot. Direct-child kill/pipe
bounds are described accurately without a descendant-sandbox claim. Exit -1
for failure to start, real process exit when available, context deadline,
per-stream overflow, elapsed time and bounded error remain explicit.

The finite helper copies the fixture environment per child, uses only
`plugin list --json` from exact A/B cwd, and preserves separate capped raw logs
and byte/SHA256 metadata. No unknown listing is parsed into ownership, active
status or a success result; every diagnostic record and aggregate qualification
remain false. Fixed artifact paths are inside the existing evidence root, whose
ordinary writer hashes the files for upload. No mutation, guessed plugin ID,
trust/account/model/MCP action, new acquisition or format relaxation appears.

Independent inert verification:
`go test -p=2 ./repotests -run '^TestClaudeProjectDiagnostic' -count=1` passed
both contracts in **0.009s**, without opting in to a native test. They cover
exact boundary/overflow cancellation, bounded later writes and snapshot copies,
fixed two-call selection, independent env/streams, failure metadata/hashes and
bounded error text. `git diff --check` passed. Guard and production adapter bytes
are unchanged across the reviewed diff. The author's exact lint pass remains
separate recorded evidence.

No blocking implementation issue found. Root may publish/replay this exact
diagnostic checkpoint only after the other independent implementation GO.
Actual raw hosted output and both reviewers' interpretation are still required
before a separate evidence-based production decoder amendment. This GO does not
qualify the failed project lane, native discovery/runtime, trust, another OS,
all-client E2E or a release.
