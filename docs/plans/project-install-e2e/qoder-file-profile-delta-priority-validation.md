# Qoder held-file detail priority: exact validation

## Reviewed source boundary

The published base is PR523 head
`faf2fef5ae4c094698b6818b4b6380f11b0bd351`, tree
`ed31f769953b661d46975078dfd5d29cb782c540`. Plan commit
`81bb84d11bd73e459ec16adbe57972c0dad1c1e4` preserves the exact independently
approved plan SHA256
`69031dd2cfaaf9fb72706f33c4e6783d22d6e7dddb0f0fe3e5fa56cd3793c2a2`.
Root gave R1 GO and CLI gave independent R2 GO before source edits.

Code commit `9b2f5466c3b4a4a401229c31cd343894e751922b` has reviewed tree
`1ca241e5d2218d7547136c83b08f9d1d97ac9e0c`. Only two code files changed:
the existing comparator has seven added lines and the new inert contract file
has 140 lines. Their SHA256 values are
`87f80a281af6f760c30f38d1e08847841785f829039c06c1fcfc3568d174588f`
and `5b8fbe1add6cf9f3db9166f54206f37749a93608f1f97aa6ab72dcf8e3ee40f0`.

CLI independently read the complete two-file delta and existing bounded
inventory/parser/fatal contracts, reconstructed the fixed expected key/content
hashes in Python, and gave exact-tree CODE GO without findings. Root separately
reviewed the comparator and approved the same tree. No source changes followed
these reviews; this record does not alter reviewed code.

## Actual verification

Focused inert `^(TestQoder|TestProjectNativeCLI)` contracts passed in 0.157
seconds with Go 1.26.10. The actual all-enabled affected-line root lint gate
against plan commit `81bb84d1` reported zero issues. The unchanged unfiltered
whole-root revive/funlen/gocyclo/gocognit/dupl/depguard gate also reported zero
issues. Gofmt and staged whitespace checks are clean.

Real-file controls cover two directory and five file additions with the
measured log/latest shape, exact ten-addition/one-change counts, three selected
details and eight omissions. Removed files and both file/directory replacement
directions retain file priority; all-directory, existing all-file and known-area
ordering remain strict. Independent literal hashes, reverse map insertion,
before/after pointer non-aliasing, private serialization and full actual profile
metadata frames are checked without adopting the unequal before/after maps.

The current genuine native artifact `12732771926`, from run `40495061122`,
has verified outer SHA256
`f52ca8dd0a55104125fee2c9e94bd90bcdbd0267fc8d6510adacd9cbc0aa254e`.
Its actual sixteen-member archive replays through current admission unchanged
as failed/incomplete, with all five qualifications false and eight omissions.
No omitted path, body or authentication state is inferred from that replay.

No vendor, package hook, public installer or positive hosted guard ran locally.
The diagnostic adds no read, field, permission, baseline renewal or preservation
exception. Root alone owns publication and fresh genuine capture review. The
complete original-profile mismatch remains fatal; lifecycle, MCP, account,
model, merge and release gates remain unchanged.
