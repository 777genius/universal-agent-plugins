# Codex and Claude installation convenience

Status: draft plan; implementation starts after the review rounds below.
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
  incompatible explicit selection flags before acquisition. Do not reinterpret
  terminal picker's existing `all` navigation command.
- Local listing reports a mutable local source without inventing a Git commit.
  Discovery is read-only and never executes scripts or initializes Git.
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
  depth, candidates, deadline and cancellation. No FIFO/device reads or symlink
  traversal; root and selected-path ancestry must be checked.
- Name ambiguity, invalid metadata, incomplete all-selection or unsafe paths
  fail before persisted intent and native profile changes. A valid explicitly
  selected Skill can coexist with an unrelated invalid sibling.
- Discovery bytes are not installation authority. Existing immutable acquisition
  revalidates selected content before review and publication; repeat install keeps
  saved pins, and `update` is required to accept changed local source bytes.
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
| Total | 360-640 | 650-1120 | 5 | 9 | 8 |

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
- Round 2, independent plan critique: pending.
- Round 3, revised plan/safety critique: pending.
- Implementation review: pending; address material findings before final tests.

Done for this request means implemented bounded changes, pushed draft PR,
recorded passing local checks and E2E with honest native/OS evidence, and no
unresolved material code-review defect. Keep the PR draft as requested.
