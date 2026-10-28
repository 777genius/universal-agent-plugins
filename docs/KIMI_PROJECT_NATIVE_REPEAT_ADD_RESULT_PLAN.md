# Kimi: exact repeated-add result contract

## Current genuine failure and source

Use published PR524 source `03d7c243129c98ecdc4cbb99bfe1a47536a556bf`,
tree `de7a5565545f67e26e7e3e101b843f728e3876b1`. Runtime40553154543,
component job122367952819, now passes the installed native observation.
It then fails at repeat-add-a because the fixture's shared array-only
`commandsRequireMutated` cannot read the actual successful direct result.

Native12759050559 outer SHA256
`01d59c89fe8124ec17d2ab08f5239e746e23d2c0db7876e34ba482885cef4ffb`
has34 closed ordinary members. Frontend12759450014 outer
`b8163211655fe5215607758975694968dd05bc67c24c3b26a014b599a7ec9aae`
has25. Both archives and CRCs are verified against current API receipts.
The actual repeat-add command exits0, reports success and returns data.result
with mutated=false, no_change=true and the held installation/binding identity.
This failed16.26s capture remains unqualified; later lifecycle stages were not
executed. The separate single-tool approval refusal is a separate slice.

Production add_multi.go145-154 sends an already installed single target to
runAddLoaded. add.go205 renders the direct newAddResultData on NoChange.
The fixture's first multi-target-style add and later repeated single-target
result are therefore distinct supported public shapes, not fallback aliases.

## Narrow change and authority

Change only Kimi's f.uap handling of the exact repeat-add-a/add/project/A stage,
plus one small private helper/contracts Go file and this accepted record.
Keep strict whole JSON parsing and nativeValidateInstallerJSON before the
stage-specific result check. Every other stage keeps the existing shared
commandsRequireMutated array contract. Do not change shared legacy helpers,
production output, command argv/cwd, progress text or installed state.

The private repeated-add helper requires the already held fixture recipe,
exact stage/add/A/project/want-mutated-false parameters, a direct data.result
object, no data.targets member, data.plugin equal to the held package name,
and exact boolean dry_run=false. Require mutated=false, no_change=true and
requires_confirmation=false with exact bool types. Bind installation_id to
the first independently validated receipt and plan.physical_artifact_id to
that same held native binding. Require the observed plan's client_id=kimi,
scope=project, status=ready, package_mode=native, activation=prepared,
authentication=not_checked, policy=allowed and verification=package_validated.
Require outcome.activation's prepared/not_checked/allowed/installation_verified
facts. No direct/group ambiguity or generic either-shape fallback is accepted.
The existing successful versioned-envelope check and strict JSON duplicate/
trailing-value checks remain before this branch; require command=add here.

The existing source, state, native-file, package, data, foreign-tree and
unchanged-operation checks still follow. Neither result metadata nor a new
installation ID may renew the original proof authority. Preserve the four
component qualification gates, the separate initial single-tool false flags,
native admission, original deadlines and the original900s Kimi suite.

## Meaningful checks and publication

Use independently stated direct-result fixtures based on the actual public
shape and ordinary inert held identities. Reject array/group and ambiguous
dual shapes, missing fields, wrong types, target/scope/package/installation/
physical identity, mutated=true, no_change=false, confirmation or dry-run.
Exercise the actual stage-selection seam so every other stage retains the
array-only contract and cannot enter the new repeated-add branch.

Replay the current held repeat-add stdout against its separately held first
add receipt in a temporary inert Go control, removed before final lint. Do
not commit raw captures or call a vendor/client/installer. Preserve all existing
proof, receipt, mutation, privacy and hostile-schema contracts.

Obtain root R1 and independent R2 on this exact record before implementation;
the peer clarification requiring physical binding and confirmation checks is
included above. Obtain independent exact-code review. Use the coordinated
Go/lint slot for focused Kimi/shared contracts, affected all-enabled lint,
unchanged whole-file architecture/size lint, gofmt and diff checks.
Keep the code commit separate from any approval diagnostic. Root alone
publishes on a fresh exact PR524 head. A new genuine full component run must
establish later lifecycle outcomes; this correction does not reclassify the
failed archive or qualify MCP calls, authentication, models or a release.

## Accepted implementation boundary

Root R1 and independent desktop R2 approved proposal SHA256
`c709b9d3390263eb602ac1781090b1c0b8f48811c10c0c6696ee8aeff2d38e62`;
the native CLI reviewer independently approved the same plan. Implementation
uses the proof's first held stable installation/physical identities. The new
private selector handles only label repeat-add-a and refuses mismatched scope,
argv or mutation intent; every other label reaches the unchanged shared array
check. No native command has been run locally. Focused inert validation,
actual-stdout replay, lint and exact-code reviews precede root publication.

Actual inert validation on 2026-10-27: focused
`^(TestKimiProject|TestKimiSingleTool|TestProjectNativeCLI)` passed in 0.392s.
A temporary inert Go control replayed the actual repeat-add stdout against the
separately captured first-add installation/physical identities in 0.009s;
the shared array-only parser still rejects that same direct result. The replay
file was removed before final lint and no raw capture is committed. All
configured linters on affected root lines versus 03d7c243 reported zero issues,
as did the unfiltered six-linter architecture/size gate on the complete root module. Reports
are `/tmp/uap-kimi-repeat-add-affected-lint.json` and
`/tmp/uap-kimi-repeat-add-whole-root-six-lint.json`. The first lint invocation lacked
Go in PATH; a later whole-file invocation accidentally retained all configured
linters and reported unrelated legacy findings. Neither invocation is counted
as the required gates; the successful runs use the exact stated scope.

Desktop and native CLI reviewers gave independent code GOs. The only review
finding corrected a misleading comment from whole-schema validation to the
actual successful versioned-envelope check. Code semantics are unchanged from
the reviewed tree `ce7d293580faf433844c9b64f6b52456a04b3199`; this validation
record adds no native observations or qualification. Root alone publishes.
