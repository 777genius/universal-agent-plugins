# Restored Kimi and Devin assembly integration

Source checkpoint: a9dd44022252a0ee96c82d6840b4e9e880e5c606, published with the identical tree at 48333d69312c5133f135c7eded4b2a0517a3d10c. The published Kimi and Devin client heads retain their two independent implementation review records.

Kimi merged mechanically. The subsequent Devin merge had exactly two manual conflicts: nativeconfig project dispatch now retains both independent Kimi and Devin branches, and supportedCodec retains both codec IDs with all earlier entries. Domain declarations and compatibility goldens merged automatically. Direct byte comparison found no differences in the Kimi/Windsurf client directories or their public CLI lifecycle/fixture files against the corresponding published client heads.

Fresh Go 1.26.10 selected nativeconfig Kimi/Devin tests passed 0.043s; full domain passed 0.053s. Full planner ran and FAILED on two legacy scope expectations: TestPreparationRequiresSupportedNativeUserTarget accepts the newly qualified Kiro project trait, and TestPlannerFailsClosedForUndetectedClientAndUnsupportedScope requests Cursor project scope without a canonical ProjectRoot. A separate reviewed correction must determine and retain the meaningful negative boundary, not simply remove assertions. These are not counted as planner passes.

The known inherited Codex TOML dependency-closure issue remains an independent required shared correction; no guard snapshot is widened. Combined native runtime and final all-client feature qualification remain pending. Two manual-integration reviews are requested at this exact source.

## Fresh combined public CLI execution

At documentation-only successor 6401a953 (production unchanged from a9dd4402), the actual command `go test -p=2 ./cli/cmd/agentplugins -run 'Test.*Project' -count=1 -timeout=5m` passed in **97.217s**, using verified Go 1.26.10 and the production composed CLI tests on Linux. This includes the assembled project source/lifecycle/ownership contracts; it is not native vendor-client execution, a Windows/macOS pass, or a claim that the full planner/architecture gates are green. Both previously recorded planner failures remain required follow-ups.
