# BUG-021 — final-head security review: false native PASS and isolation gaps

## Independent evidence (Strix, 2026-11-04)

[PR #698 review](https://github.com/777genius/universal-agent-plugins/pull/698#pullrequestreview-5763204337) identified five audit-harness qualification defects on `897fa830241ee25d021308ce319d107e55b12923`:

1. Missing `AUDIT_VENDOR_OUTCOME` is treated as successful bootstrap. Both Codex pilot and published-CLI jobs omit it.
2. Download caches/logs and unbounded substring matches, including unrelated URLs, can be called native plugin installation proof.
3. `snapshot()` maps *all* files above 4 MiB to `too-large`; repeat-add can rewrite them without changing the snapshot.
4. A future case's native-profile or temp directory can be preplanted as a symlink and reach outside the disposable case before sentinel writes.
5. Skill removal excludes symlinked/unreadable Skill entries and never examines native registration residue.

These are harness proof defects, **not confirmed failures in UAP production installers**. Previous PASS counts must not be carried forward.

## Minimal plan

1. Make bootstrap success explicit, bind actual native and published Codex install step outcomes; keep failure rows for every source.
2. Exclude passive caches/logs from *positive* attribution; use exact package identity in path/structured native data rather than arbitrary substring matching. Ambiguous large or generic config is not PASS.
3. Stream SHA-256 over all regular native files without the 4 MiB `too-large` digest sentinel; retain symlink/unreadable markers.
4. Require newly-created case directory, verify resolved native roots and temp within the disposable case **before** writing unmanaged sentinels or invoking native clients. Do not weaken macOS `/private/var` path resolution or Windows profile isolation.
5. Require targeted Skill subtree and native registration removal after successful `remove`, including links and errors. Do not delete foreign files.
6. Add focused counterexample tests for all five, run Python unittest / Required and both live E2E workflows on the **new** HEAD and inspect original per-client JSON artifacts. Keep external blockers separate.

## Independent critical review — pass 1, safety

A previously planted symlink must never touch a foreign profile. A logged cache or arbitrary URL must never prove native activation; installed-but-unqualified remains distinct. Missing bootstrap evidence is always fail-closed. Do not modify cgroup, LintAI, vendor hashes, or environment credential boundaries.

## Independent critical review — pass 2, scope and completeness

Keep source/lock verification and managed native state semantics unchanged. Do not turn ambiguous native paths into a guaranteed PASS. Don't require removal of unrelated neighbor Skills or passive caches. Never interpret green unit tests alone as full live 30x30 qualification; rerun latest-head Actions and independent CodeRabbit/Strix review.

## Status

Plan recorded before implementation. Narrow harness, pilot/published bootstrap-binding and counterexample regression changes pushed to PR #698. Required and live E2E checks are running on a newer head; independent CodeRabbit/Strix clearance, full source-by-source ZIP review, and merge qualification remain pending.
