# Codex and Claude installation convenience

Status: implemented; three plan-review rounds complete. Qualification results
are tracked in [draft PR #1135](https://github.com/777genius/universal-agent-plugins/pull/1135).
Base: `942716eff44e787d5627e9f0ffbd2c2b83e57839` (`main`, 2026-11-27).
Delivery: one draft PR; push every commit immediately. No release or merge is
part of this draft-only request.

## Goal and evidence

Make ordinary plugin and Skill installation convenient and reliable for Codex
and Claude Code while preserving UAP's Agent Plugins model, dependency resolver,
immutable identities, saved environments and native adapter boundaries.

Current source already supports two-target plugin/Skill installation, Git/SSH
and HTTPS Skills, local single Skills, project scope, saved target assignments,
locks, updates, repair, partial removal and rollback. This change extends those
paths rather than replacing them. Existing native plugin and Skill lifecycle
tests remain acceptance evidence; writing files alone is not native discovery.

Primary comparison sources, inspected 2026-11-27:

- [APM install](https://microsoft.github.io/apm/reference/cli/install/): manifest
  bootstrap and effective target selection.
- [APM packages](https://microsoft.github.io/apm/consumer/install-packages/):
  collection selection and saved Skill choices.
- [APM native plugins](https://microsoft.github.io/apm/consumer/install-plugins/):
  native registration and documented Codex/Claude limits.
- [ToolHive plugins](https://github.com/stacklok/toolhive/blob/main/docs/arch/14-plugins-system.md):
  native layouts, lock/rollback and a remaining manual Codex install step.
- [ToolHive Skills](https://github.com/stacklok/toolhive/blob/main/docs/arch/12-skills-system.md):
  scoped installation, pinned synchronization and drift repair.

These sources inform concrete scenarios, not a claim of universal feature
parity. UAP keeps its dependency graph and portable plugin format; this PR does
not introduce ToolHive's service/runtime or APM's instruction compiler.

## Confirmed gaps and user outcomes

1. `skills list ./skills` currently routes to Git discovery; `skills add` accepts
   only one local Skill directory. A local collection should have read-only
   discovery and selection using the same names, diagnostics and review as Git.
2. Installing a complete collection noninteractively currently requires listing
   every Skill name. An explicit `--all` should select a complete validated
   catalog without a separate command or hidden selection expansion.

Examples to qualify:

```sh
agentplugins skills list ./skills --format json
agentplugins skills add ./skills --skill review,testing --target codex,claude --yes
agentplugins skills add ./skills --all --target codex,claude --yes
agentplugins skills add owner/repository --all --target codex,claude --yes
agentplugins skills add https://publisher.example --all --target codex,claude --yes
```

The existing single-directory command stays compatible. Collections with no
explicit selection use the terminal picker when interactive and return the
existing selection-required error otherwise. `--yes` only confirms a selection.
Saved requirements continue to identify each selected Skill independently;
later updates do not automatically enroll newly added siblings.

## Scope and architecture

- Add a bounded local filesystem discovery adapter in `adapters/skillsource`.
  Reuse `ScanPolicy`, `Catalog`, `Candidate` and `conformance.ParseSkill`.
  Filesystem enumeration differs from Git; no second parser or source resolver.
- Reuse CLI selection, diagnostics, terminal review and `SkillsBatchRequest`.
  Local selected children become existing `SkillsSelection.LocalPath` entries.
  `AcquireLocal`, normalization, `PrepareSkills` and saved installation remain
  the authorities for content, security, locks, publication and recovery.
- Add `--all` to `skills add`, with one shared selection rule. Require a complete
  nonempty catalog with no invalid/unavailable or ambiguous entries. Refuse
  `--skill`, `--path` (including empty), or `--ui` together with `--all` before
  acquisition. An intrinsic Git URL scope stays within that scope; an exact
  Skill source selects that Skill only. `--all` suppresses browser auto-open
  and counts as an explicit selection, while target selection and confirmation
  retain their normal rules. Do not reinterpret
  terminal picker's existing `all` navigation command.
- Local listing reports a mutable local source without inventing a Git commit.
  Discovery is read-only and never executes scripts or initializes Git.
- Presence of a root `SKILL.md`, including an invalid or symlinked one, selects
  the existing single-directory route. Never fall back to descendants after a
  root document fails validation. A collection scan otherwise includes nested
  Skills, as Git does; local `--path` selects exactly one child directory.
- Extract small data-oriented selection helpers from the current terminal
  wrappers. Do not construct a fake Git session with missing fields. HTTPS
  all-selection uses the existing `ListHTTPSkills` capability and carries its
  index expectation into prepared selections. Existing acquisition resolves and
  verifies tree identity; the catalog does not invent a tree digest.
- Preserve explicit target behavior and project/user scope defaults. Do not
  add preference storage, client-ID branches, schema changes or migrations.

Out of scope: native vendor-format import, new clients, frozen VS Code,
enterprise policy (#1130), held readiness/MCP probes (#1114), Codex environment
forwarding (#832), native audit changes owned by #1133, release publishing,
new daemons/services, automatic trust/auth changes, and browser UI expansion.
No useful legacy implementation, tests or design material may be removed.

## Safety and compatibility guarantees

- No source content is executed during discovery or installation.
- Local traversal is bounded by entries, metadata/document/aggregate bytes,
  depth, candidates, deadline and cancellation. Enumerate with bounded ReadDir
  batches and count non-Skill entries before descent. Read at most limit + 1
  bytes from regular documents; never open resources during discovery.
- Pin the local directory for descriptor-relative discovery; reject observed
  root/child/document symlinks and check selected-path ancestry. Reuse existing
  atomicfile regular-read admission for nonblocking opens. Rooted reads prevent
  escaping the opened tree. Do not claim arbitrary pre-open ancestor replacement
  or every filesystem's concurrent-write semantics are solved by Lstat checks.
- Name ambiguity, invalid metadata, incomplete all-selection or unsafe paths
  fail before persisted intent and native profile changes. A valid explicitly
  selected Skill can coexist with an unrelated invalid sibling.
- Discovery bytes are not installation authority. Existing immutable acquisition
  revalidates selected content before review and publication; repeat install keeps
  saved pins, and `update` is required to accept changed local source bytes.
  Selected directories removed, renamed or replaced with a different Skill name
  must fail; same-name byte changes before preparation are visible in the final
  immutable review and use existing security checks. No post-scan membership
  expansion is allowed. Changes after preparation use existing stale-plan checks.
- Existing foreign-file, native settings and project isolation protections stay
  intact. Failed preparation cannot leave a partial batch installed.
- `--all` snapshots only the selected source scope. It never grants permission to
  fetch or install additional siblings on later update or repair.
- Public JSON and exit semantics stay compatible; add no false native-ready
  claim. Local revisions may be empty; catalog digest is not a Git identity.

## Implementation and size budget

| Part | Production LOC estimate | Test LOC estimate | Complexity / 10 | Reliability target / 10 | Confidence / 10 |
| --- | ---: | ---: | ---: | ---: | ---: |
| Bounded local catalog adapter | 180-280 | 180-300 | 5 | 9 | 8 |
| CLI local collection/list integration | 100-180 | 200-350 | 4 | 9 | 8 |
| Shared explicit all-selection | 80-140 | 120-220 | 3 | 9 | 8 |
| Public CLI E2E and usage evidence | 0-40 | 150-250 | 4 | 9 | 8 |
| Total | 420-650 | 650-1120 | 5 | 9 | 8 |

Estimates are not permission to grow the scope. Keep new functions/files within
the existing limits and do not widen lint exemptions. If reuse requires a large
catalog abstraction or a second state engine, reduce the design first.

## Verification

1. Adapter tests: deterministic catalog; invalid/duplicate names; incomplete
   budgets; cancellation; root/ancestor/child/document symlinks and nonregular
   files; metadata-only access. Real temporary filesystem, no parser mocks.
2. Composed CLI E2E: two local Skills into Codex and Claude in one batch; listing
   and dry-run do not publish; repeated install keeps pins; changing one source
   requires update; repair uses retained snapshots; locked cold restore verifies
   bytes; target-specific removal preserves the other client; foreign data stays.
3. Complete catalog selection: Git, HTTPS and local; incompatible flags;
   incomplete/empty/invalid/ambiguous catalogs; review refusal/cancellation; no
   automatic enrollment of new siblings; selected-source scope remains exact.
4. Existing whole-plugin and Skill lifecycle regressions, rollback, architecture,
   `make test-core` and `make lint`. Run production CLI subprocess E2E and existing
   cross-platform harness entries relevant to changed code. Broaden only for a
   concrete remaining risk or a required gate.
5. Genuine native evidence is separate. Existing hosted qualification requires a
   real disposable eligible runner and pinned native clients. Do not fabricate
   hosted environment markers or count skipped/draft-only jobs as passes. Draft
   PRs skip heavy native jobs; do not wait for a job that is not scheduled. State
   the native verification limit explicitly if it cannot run for this draft.

Use isolated profiles and temporary directories. Never mutate the user's real
Codex/Claude profiles. Preserve other agents' checkouts and test caches.

## Review rounds and completion

- Round 1, code/competitor audit: existing lifecycle is already substantial;
  focus on local collections and explicit complete selection. Discarded an
  unconfirmed alias/no-save concern after checking Cobra flag ownership.
- Round 2, independent code and safety critics: clarified root-document
  precedence, real enumeration bounds, rooted reads, selection/content timing,
  exact flag conflicts, HTTPS expectations and data-oriented selector reuse.
  Raised the production estimate floor to 420 LOC. No new installer/framework.
- Round 3, revised plan/safety critique: both independent critics approved the
  bounded design. No installer, state engine, preference store or browser work.
- Implementation review round 1: corrected recursive enumeration budget
  propagation and directory admission; preserved re-add of an existing saved
  single Skill after its source/document disappears; rejected explicitly empty
  `--skill` together with `--all` before acquisition. Added regressions.
- Implementation review round 2: both independent reviewers found no remaining
  material defect after the fixes. Added real TLS two-Skill/two-client coverage
  and a changed-index refusal checked through the existing typed stale error.
- Final harness review: preserved the restricted-host archive suite by keeping
  its six local/HTTPS collection cases separate from the two full-suite Git
  cases. Required execution and explicit exclusion inventories agree with the
  command selection. A new archive/full regression passes, and independent
  source review found no remaining material defect in this correction.
- Focused verification: local collection Codex/Claude lifecycle, local single
  Skill compatibility, strict all-selection and HTTPS collection tests pass.
  The production executable collection fixture also passes using Gemini and
  the real scanner; that initial direct run predates the final review changes
  and does not qualify the final revision or native Codex/Claude runtime.
  All 39 Python package-manager harness tests pass after the archive correction.
- Final qualification record: PR #1135 records the tested revision, actual
  lint/core/package-manager results and environment limits. The portable runner
  captures source and binary identities for its full or explicit archive suite;
  an archive result always retains `full_qualification: false`. Queued, skipped,
  interrupted or unexecuted checks are not passing evidence.

Done for this request means implemented bounded changes, pushed draft PR,
recorded passing local checks and E2E with honest native/OS evidence, and no
unresolved material code-review defect. Keep the PR draft as requested.
