# Qoder project lint extraction plan

## R1: bounded mechanical changes

Frozen source: `96117cd43064e50a4541387d5dc9a3aeddbdadeb`. Actual changed-source lint reports exactly three Qoder findings: `verifyProjectView` cognitive complexity 29 above 25; a De Morgan simplification in the selected-skill predicate; and an unused string result from `projectPlanRoot`.

1. Extract the complete existing closed-directory path-set loop and selected-skill regular-file loop into a private helper receiving the same active path, view path, and selected-name map. Call it at the exact existing point after containment validation and before manifest reads. Preserve every iteration, containment/type check, error, and return order; no new permissive fallback or changed set membership.
2. Rewrite only the selected-skill boolean with equivalent De Morgan grouping. Unsupported entries still skip before it; supported native skills and projected command skills remain the exact admitted set.
3. Rename the private validator `projectPlanRoot` to `validateProjectPlan`, returning only error. All callers currently discard its string result. Preserve internal root computation, validation ordering and final RequireContainedChild call; update every caller without moving native/project dispatch.

No public API, lifecycle, ownership, schema, runner, native qualification, output contract or test expectation changes. Existing project Qoder provider/SDK/actual CLI and Prepared-file regressions remain meaningful gates; rerun only affected Qoder contracts plus changed-source lint and focused vet/Windows compile checks. No vendor execution.

This plan requires two independent R1 reviews, then a final R2 freeze and two reviews before source edits; source publication still requires two independent code approvals.

## R2: reviewed ordering and admission precision

Root and Claude independently approved R1 at `016060d3`. Preserve the existing map iteration (no newly promised order), each first error in the moved loops, and their call position after view containment but before bounded manifest reading and manifest-key allowlist validation. Preserve the outer full-bundle digest before and after view verification. The equivalent predicate admits exactly `Skill && (Native || (Projected && Command))` after the existing Unsupported skip. Search all private validator references and update all six callers; every currently ignored root result disappears without reordering any checks or dispatch. Existing exact negative contracts, provider/SDK/CLI journey and narrow lint remain the gates. Both final plan reviews are required before mechanical source edits.

## Implementation and independent code reviews

The complete Qoder/Prepared-file implementation was frozen at `96117cd43064e50a4541387d5dc9a3aeddbdadeb`. Root independently reviewed the full source/tests and confirmed its final ownership-packet alias correction; Skillshare independently reviewed the full adapter/core/provider/SDK/CLI slice and returned GO for that source plus the mechanical `da0948cb5eb6eb8fa15db7cd0e9de7c2b1ec96c8` delta. Claude independently reviewed the complete six-file mechanical delta under both approved plan rounds and returned GO for exactly `da0948cb`. These are source reviews, not native/runtime qualification.

Exact `da0948cb` verification: meaningful PreparedProjectFile/ProjectQoder tests passed in usecase 0.055s, providers 0.319s and installer 2.381s. The complete existing Qoder adapter suite passed in 15.186s; internal architecture tests passed in 0.660s and conformance in 0.413s. Focused four-package go vet exited zero. Configured golangci-lint 2.13.2, changed-source baseline `49a5549d7912bfe34ef97d4bf6db8254c45f090d`, concurrency 1, 512 MiB heap target and 90-second deadline reported zero issues for Qoder/usecase/providers/installer. No exclusion or budget was changed. The actual saved TLS-Git CLI journey passed at the preceding complete `96117cd4` source in 5.966s; the later private extraction does not change its dispatch or assertions.

Windows AMD64 compile-only checks passed on exact `da0948cb`: Qoder test binary 7,671,808 bytes SHA256 `719d6d53d5ffaecf5e25f8a058dbdf1c7c0233bca1757fedd4e8063f0cf28071`; SDK test binary 18,800,640 bytes SHA256 `b31ad020d0f8ddf77f79187c7023ece4614ae7047edcee1ca0587b77048853f8`. These owned completed binaries were removed after hashing. No Windows test execution, Qoder vendor process, MCP initialization, model use or automatic native activation is claimed. The only genuine native Qoder observations remain the preserved non-Git/different-version global-cache research evidence.
