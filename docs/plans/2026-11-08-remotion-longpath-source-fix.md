# Windows Remotion long TEMP source-acquisition fix (Draft)

## Reproduced evidence
Diagnostic PR #780, run 42473053740 original JSON artifact 13613814696, published UAP 0.1.101 and immutable remotion-dev/skills commit `c7cb2fdcacd134fb2ccb9b7a44f9ffa44e467271`:
- Short runner TEMP: genuine Codex Skill installation PASSED.
- Default Windows-long TEMP: `SKILL_REJECTED`.
- Long project, short TEMP: acquired package tree digest differed from the expected pinned digest, **not** a success.
- Long TEMP, short project: `SKILL_REJECTED`.
- Manually pinned Git sparse checkouts with `core.longpaths=true/false` in **short** worktrees both passed; these do not prove long worktrees.

## Minimal source change
Use Git for Windows' repo-local `core.longpaths=true` only for the isolated temporary Git repository created by `sourceacquisition.Acquirer.acquireGitTransport`, before fetch/sparse-checkout/checkout. This is a path-handling setting, not a relaxation of source hash policy. Keep selected immutable Git SHA, `ls-tree` validation, digest comparison, symlink/junction refusals and foreign project state unchanged. No extra proxy or direct file download.

## Real E2E requirements
1. Build source CLI from exact PR commit on Windows 2025 and execute the pinned Remotion Skill through genuine Codex in **both** short and default-long Windows TEMP, Project A. Check add/repeat/lock/native discovery/remove and compare SHA, no foreign mutation. Verify `core.longpaths` setting on isolated Git repo; no global config modification.
2. Same script on pre-fix source demonstrates the long failure; fixed source must pass, not skip. Keep original JSON on failure. Include a distinct negative tampered fixture/expected digest; it must still fail.
3. Normal Go Source transports/Required/CodeQL, independent review, exact-head CI and post-merge acceptance. Do not label the old published 0.1.101 fixed by source changes: public qualification requires a separately verified new release.

## If still failing
Do not merge: inspect original Windows error/paths and focus only on ownership of the missing deep file. Avoid indiscriminate \\?\ prefixes and global path/registry changes.
