# BUG-017 — fail-closed audit evidence integrity (2026-11-04)

## Reproduced cause

The exact PR HEAD `5e79cf1a2d34a900f7fb5a542b39d134c8b29057` still has five observable oracle weaknesses, confirmed against the source following the independent [CodeRabbit review](https://github.com/777genius/universal-agent-plugins/pull/698#pullrequestreview-5762784607):

1. `snapshot()` silently ignores new native symlink entries and unreadable files, so removal residue may be omitted even when an installation touched them.
2. `locked_skill_is_pinned()` accepts incomplete/non-hex `sha256:` lock values.
3. Direct Git source matching uses a repository *substring*, allowing an unrelated repository path containing the pinned owner/repository to satisfy source identity.
4. Foreign-file sentinels are read with `Path.read_text()`, which follows replacement symlinks; their contents alone cannot prove that the original non-symlink file survives.
5. A credential-bearing `HTTP_PROXY`, `HTTPS_PROXY` or `ALL_PROXY` URL may be inherited intact by the child despite the explicit environment allowlist.

These are **audit harness evidence/security defects**, not verified product installation bugs. None of the historic PASS labels is rewritten into a new final-head PASS.

## Minimal implementation plan

- Preserve symlink/unreadable markers from native snapshots without traversing symlink directories; fail closed on residual markers after removal, while retaining existing normal-file behavior.
- Require exact lowercase 64-hex SHA-256 values for both locked Skill digests; compare exact canonical owner/repository for direct Git.
- Verify that each foreign sentinel remains an ordinary non-symlink file before reading. Keep CLI installation/removal contract and isolation unchanged.
- Strip proxy URL userinfo from the narrow runtime allowlist, preserving noncredential proxy settings; do not inherit secrets or create new feature flags.
- Add focused regressions for each counterexample in `scripts/test_live_install_audit_env.py`. Run `python3 -m unittest scripts/test_live_install_audit_env.py` and applicable CI/E2E on the new HEAD; store **only exact-head** evidence.

## Critical review round 1 (source/security)

Threat model: an installer may create a symlink under its native profile or replace a neighboring file with a symlink. A file's digest/content is not proof of file type. Avoid reading targets of symlinks and ensure a missing/ambiguous residual cannot be classified as removed. Use only the disposable case roots; never weaken cgroup, LintAI or source policy.

## Critical review round 2 (false-positive/false-negative and scope)

Preserve normal native-file hashes and the existing signed-release proof. Do not count unsupported/failed bootstrap or unrelated config changes as native activation. Tests must demonstrate normal regular-file retention and explicit refusals for malformed digests, forked sources, symlink sentinels and credential-bearing proxy URLs. Any new CI failures require analysis before merge; reviewed negative cases remain non-PASS.

## Status

Plan and source counterexamples were recorded **before** implementation. Narrow harness changes and regressions are **pushed**; follow-up critical pass additionally kept synthetic unreadable os.walk markers fail-closed. Final-head Python/CI E2E and independent CodeRabbit/Strix clearance remain **unqualified**, so no merge.
