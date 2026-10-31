# Canonical fixture seed roots — recovery plan

Recover the former fixture-only contract (lost source818dcb90/final1fa8b098,
prior plan GO946b/code GOb287 recorded in retained handoff). Old passes are not
proof of restored bytes. Obtain two fresh independent plan GOs before edits,
then two fresh code reviews and affected tests. No production code changes.

Actual macOS /var versus /private/var and Windows short RUNNER~1 spellings can
name the same existing test directory. Production composition canonicalizes the
selected root, so raw t.TempDir seeds produce false exact receipt/path assertion
failures. Canonicalize the EXISTING fixture-created temporary root first through
filepath.EvalSymlinks and then construct all expected children/compose the app.
Fail if resolution fails; never clean/rewrite native output, production receipts,
actual state, caller path spelling or assertions after observation.

Add one CLI projectJourneyTemp(t) seed helper beside the shared project fixture.
Use it for shared journey home/A/B, transport/source/git-home/certificate seed
roots, Cursor/Kiro journey seeds, explicit project composition fixture roots,
OpenCode inert probe executable root and lifecycle transfer exchange roots.
Existing alias-specific tests keep their deliberately authored aliases. Providers
already have canonical projectGeminiTemp; use it for remaining OpenCode base/
source seeds rather than add a second helper. Leave unrelated fixtures alone.
Kimi/Devin/Cline/Grok restoration must use canonical seeds on their own leaves;
root coordinates equivalent followups only after those leaves join assembly.

This is fixture setup only. Preserve all exact path, scope, digest, receipt,
original-user/B isolation, readonly, drift, rollback and removal assertions.
No fake native proof, mock guard, production path normalization, trust/model/
auth operation, new dependency, platform skip or loosened expectation follows.

Verify the helper through affected project composition/lifecycle fixtures using
real UAP/TLSGit transactions and inert host boundaries. Existing helper ensures
physical roots locally; macOS/Windows actual execution remains separately
attributed to hosted CI, never claimed from Linux. Check authored whitespace
and relevant lint, then immediately checkpoint internal
feat/project-fixture-canonical-roots through expected-head exact-tree publication.
No extra PR or main/release merge.
