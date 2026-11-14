# OpenCode project repair: preserve the selected local directory revision

The remaining missing-skill repair stage has a source-backed expectation
mismatch. It deliberately advances a mutable local source after installation,
then expects the public CLI to recover the old package automatically. That stage
has not yet been reached by hosted execution; this finding comes from the public
source path and its existing contracts, not a newly observed native failure.

`AcquireLocal` in `sourceacquisition/acquirer.go` records an absolute canonical
directory with an empty source kind. `repairSource` in CLI `lifecycle.go` returns
that directory. `repair_multi.go`'s revision selection/resolution therefore takes
`loadPackageFor` and reacquires the current local directory. `RepairGroup` and
`repairPackageRevisionMatches` require its exact applied package digest. This is
deliberate refusal, covered by
`TestChatGPTUnsupportedRepairRendersStructuredRecoveryWithoutMutation` in
`cli_test.go`: a changed direct local source is non-reproducible and must fail
without changing the managed state. Immutable Git/archive and saved dependency
repair have separate source and pin contracts; this local fixture cannot claim
their acquisition behavior.

## Exact test-only correction

After deleting the exact recorded native guide directory, leave the current
local source at the already applied A-two revision. Remove only the deliberate
`UNINSTALLED-SOURCE-ADVANCE` append and the read used solely for that mutation.
Replace its explanatory comment and the stage reason with truthful unchanged
local-source wording. Preserve the missing owned directory, the same public
repair invocation, actual mutation requirement, Prepared binding/digests and
fresh native observation using the installed managed package's A-two bytes.

Do not change production acquisition or repair, the changed-source refusal
contract, any result reader, source update, command table, subprocess count,
timeout, cap, passive recorder or later lifecycle stage. The fixed 43-command
hosted path remains unchanged. Source-only expectation repair does not qualify
native repair until the final hosted stage passes.

Two independent plan rounds precede the narrow source diff; two independent
frozen code reviews precede publication. Run affected pure project contracts and
compile plus configured affected-source lint, with coordinated disk/build usage.
Preserve the already reviewed intact-repair assertions and all existing public
CLI tests. No local vendor execution or speculative product change is needed.
