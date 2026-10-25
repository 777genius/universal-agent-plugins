# Executor recovery: reviewed root and Claude source

The temporary executor was replaced before the last local completion commits
were published. Published PR499 remained at46b4678ea66d7804311f98270b70ebd3cceb928a.
This checkpoint distinguishes exact source recovery from newly reconstructed code.
It is not final all-client qualification or release evidence.

## Reconstructed shared source

Frozen recovered commit4e83f9a29d1ad01beec02a1762f5e3e7b71185fc has
tree78a4b7fae6c18415d60d1b9aca20f0fe5cee6035, parent46b4678.
The recovered R2 removal plan has exact SHA256
6a0a2de5771aadeec396518593e994c5fa084883371deeaf443bb7b2999c98a8,
matching the independently reviewed plan before the reset.

The source restores the positive project-only grouped-removal admission,
runner-free early ownership observation at six existing removal fences,
independent observation packets, failure-result/picker predicate extractions,
meaningful single/group mutation regressions, and mechanical lint corrections.
It preserves original user guards, exact identity checks, saved state/journal
comparison, and honest uncertainty for changes after the last observation.
The grouped lifecycle fixture reopens its engine before grouped cleanup.

Fresh independent source reviews at the exact recovered commit:

- /root/skillshare_scope: CODE GO. Read all changed production and fixture source;
  confirmed strict Managed/ReceiptReconciled/digest/positive-Absent observation,
  false discovery flags, independent packets, unchanged historical guard and
  error precedence, and foreign/absent/replay/change-after-Prepare regressions.
- /root/apm_vercel_scope: CODE GO. Independently reviewed the same frozen source.
  Confirmed all six fences, the explicit supported-project exception, unchanged
  late-attempt uncertainty, exact pure extractions and mechanical lint corrections.

Neither critic claimed duplicate test runs or native execution.

## Fresh checks on the restored shared source

Using verified Go1.26.10 and golangci-lint2.13.2:

- Installer focused removal/group/handoff/result lifecycle regressions:
  PASS18.765s; focused Grok selected-source collision regression: PASS0.056s.
- Changed-source lint against46b4678 for installer, Cline and Grok:0 issues.
- Whole installer revive/funlen/gocyclo/gocognit/dupl/depguard:0 issues.
- Changed-source public/internal CLI lint against46b4678:0 issues.
- Authored diff whitespace check:PASS.

The focused test selector was:
Test(ProjectRemoval|ProjectGroupRemoval|ProjectPreparedGroupPublicLifecycleAndHandoff|
ProjectHandoff|ProjectFileLifecycleWithoutLiveObserver|FinishGroupFailure|
ProjectGrokSelectedSkillNameCollidesAcrossNativeSources).
These Linux results do not qualify actual macOS/Windows execution.

## Exact Claude source recovery

Recovered commit7bf45c74b19ca2054bedfb23937d5d84923f8d45, parent46b4678,
has exact tree003ef4979bdb76035008ecadb470998e1f260450. It matches the
previously independently reviewed six-file Claude lint correction tree.
Root and APM had approved that source; no new behavior is inferred from the
recreated commit identity. Integration is a disjoint cherry-pick.

This changes the project-list fixture result to a named struct, lowercases
error prefixes, fixes capacity/copy naming, and removes only three newly
unreachable private historical fixture helpers. Existing native gates remain.
The earlier exact source checks (Claude package0.036s, prepared contract0.031s,
affected lint0) remain historical attributed checks, not new executor runs.

## Remaining program gates

Current hosted46b4678 evidence still has eight expected planner golden deltas,
static lint corrections, observed platform fixture defects, and the separate
OpenCode/Windows native harness corrections. Qoder, Cline current-source native
evidence, and VS Code workspace observation/adapter completion remain distinct
lanes. Final assembled source must pass the required checks and independent
review before merge. This checkpoint does not authorize a release.
