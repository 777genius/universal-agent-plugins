# Hosted project fixture portability correction — R1

This is a fixture-only follow-up to the twice-reviewed canonical seed plan.
Actual source46b4678 hosted Core fast run40145167029 failed the macOS project
contracts job121024834276 and Windows job121024834278. Terminal jobs121024833733
and121024833729 repeated the Claude/Grok fixture failures. Linux project contracts
passed. These are observed fixture defects; Linux cannot qualify the corrected
macOS/Windows execution.

## Scope and observed defects

1. Codex provider fixture uses os.MkdirTemp without resolving the existing seed;
   Kimi/Kiro SDK lifecycle tests use raw t.TempDir. Canonical guards correctly
   refuse /var versus /private/var and RUNNER~1 aliases. Resolve each existing
   positive seed using filepath.EvalSymlinks before constructing children and
   expected authority, as in the accepted canonical fixture plan. Failure is fatal.
   Preserve deliberately authored alias negatives and every exact result assertion.
   Claude's equivalent list/SDK/CLI fixture follow-up belongs to the Claude lane.
2. Grok inspect JSON test interpolates unescaped Windows paths in strings.Replace.
   The intended invalid candidates remain unchanged valid JSON. Marshal only
   fixture path string literals, replace the exact projectRoot field, assert the
   replacement occurred, and retain each noncanonical/traversal/type/duplicate
   negative. Do not normalize actual client output or modify the strict parser.
3. Grok entry-budget fixture uses 129 adjacent Unicode characters as names.
   Case-insensitive filesystems collapse some pairs, so fewer than129 entries
   exist. Use unique lowercase ASCII indexed names and verify129 entries before
   invoking the unchanged bounded scanner. Keep the128-entry production budget.
4. Grok public CLI fixture uses /inert/... as its injected executable identity.
   It is not an absolute Windows path. Bind a clean absolute inert path underneath
   the existing canonical fixture seed in detector, plan and exact runner assertion.
   Do not execute a vendor client or change the production executable guard.
5. Windows post-read Grok recheck fixture tries renaming an ancestor while its
   descendant file is open; hosted Windows reports Access is denied. Keep the
   existing before-leaf anchored-open deterministic replacement test unchanged.
   For the recheck test, open the canonical original leaf, assert a successful
   stableProjectPath for that exact path, then supply a separately created
   ancestor symlink spelling pointing to the same still-open leaf. Verify
   os.SameFile independently and require stableProjectPath(alias, file, info)
   refusal. This proves same-object ancestor-alias refusal by the recheck; it
   does not claim the hosted filesystem allowed a post-open ancestor rename.
   No handle is closed before the check and no production share flags change.

## Boundaries and acceptance

Only the finite fixture files above change. No production adapter, path policy,
native parser, platform skip, linter exclusion, workflow selector, retry budget,
assertion relaxation or runtime claim changes. Existing symlink prerequisite
handling stays visible; positive and negative contracts remain separate.

Run focused Codex provider and Kimi/Kiro SDK lifecycle tests, Grok strict/budget
tests and composed TLS Git lifecycle, relevant changed-line and architecture lint.
Compile the Windows-only test and obtain actual hosted Windows/macOS passes after
integration. A compile is not execution. Record exact reviewed source and commands.
Two independent plan rounds precede these new Grok fixture edits; frozen code is
reviewed independently before integration and publication to the working PR.
