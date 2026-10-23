# OpenCode project adapter independent implementation review R1

Reviewer: codex_claude, 2026-10-23. Reviewed cumulative leaf
`d9f3f757..f94906f42e788791e15d198127144ad8fe2c40f2` in
`work/uap-project-opencode`. Reviewer did not author this implementation.

## Decision: interim NO GO pending selected-data authority correction

One confirmed blocker remains in active native reprojection: the
`ProjectActiveNative` data-path argument is discarded, while the native object
builder reads the stored projection's DataRoot. Validation checks that the
stored root is absolute but does not compare it with the currently selected
data authority. An intact, digest-valid package therefore cannot establish
agreement with a different data root supplied by the owning lifecycle. Root/R2
already identified this independently; the author is preparing the correction.

The fix must compare against the selected data authority without rewriting
digest-covered package bytes, preserve existing user behavior where applicable,
and reject mismatches with no package/native effects. Final R1 closure will
review the frozen fix and independently execute its negative regression.

## Other reviewed boundaries: no additional blocker found

- Project root config candidates are exact `opencode.json`/`opencode.jsonc`,
  while native skills resolve separately below `.opencode/skills`. Prepared
  packages and persistent data remain in selected project `.uap` authority.
  Original detected user ConfigRoot stays distinct; neither package paths nor
  ambient cwd select project scope.
- The internal layout travels through projection, native namespace checks,
  inspect, apply, verification and remove. Project objects require exact IDs,
  names, absolute clean paths and containment. MCP logical keys retain their
  exact slash/space spelling instead of becoming filenames. Foreign same-name
  disabled entries and aliases retain closed refusal.
- The scoped domain declaration uses core's native-config lifecycle. Existing
  exact-entry kernel ownership, conditional writes, exclusive skill swaps and
  receipts remain the mutation machinery; no second configuration engine or
  alternative plugin registry lifecycle is added.
- Durable dialect transition discovery/recovery derives split native layouts
  from persisted binding scope and root. Every production recorder constructor
  now receives selected ProjectRoot. Missing/mismatching selected context
  refuses before reconciliation. Old/target state, object receipts, skill
  intents and journal root checks remain bound to the same selected layout;
  user-only helper wrappers preserve their original layout.
- Successful project configuration reports prepared plus installed, with
  effective policy, native discovery, permissions and MCP initialization
  unobserved. Project preflight rejects ActivationComplete before provider
  attestation shortcuts and on direct adapter dispatch. Native prepared status
  is also retained through durable transition target-state publication.
- The production public saved CLI fixture uses real TLS Git acquisition,
  standard loading, immutable store, planner, staging, installer and native
  transactions. Its clearly named inert TEST executable handles only version
  contract probing. The fixture is not represented as real OpenCode discovery.
- The saved journey checks A/B installs, exact unchanged locked state, old-pin
  repair after moving source, interrupted directory recovery, update with
  retained source/binding/data authority, prepared status, exact removal and
  repeat-removal no-effects. User/B inventories, foreign disabled settings,
  comments, skills and retained persistent data are checked separately.

## Independent verification at the reviewed checkpoint

Go 1.26.10, GOMAXPROCS=3, `-p=2 -count=1 -timeout=3m`, selection
`^Test(ProjectOpenCode|NativeTransition)`:

- OpenCode client PASS **0.479s**;
- providers PASS **0.040s**;
- production public CLI PASS **3.544s**.

These include the authored scoped contracts, durable transition tests and
composed saved project journey. They do not cover the missing selected-data
negative until the correction lands. No genuine OpenCode client, model,
configured MCP server, trust action or account operation was executed.

Genuine pinned hosted effective-config, skill discovery, marker handshake/tool
result and restart observations remain separate pending native gates. File
receipts and the inert version probe cannot supply those observations.

## Final R1 closure: GO at 90fcc0af

Independently reviewed corrective commit
`90fcc0af34008a4416758c0c9d83678e72323d5d`. The active-project entrypoint now checks cancellation, validates
scope/layout and exact selected active root, reads the existing immutable
projection, and requires its exact package root. For projected stdio servers,
the selected data locator must be absolute, already clean and exactly equal to
the stored DataRoot. An empty, relative, other-project or changed receipt
returns ErrProjectionRefreshRequired without rewriting package bytes. Legacy
user reprojection retains its earlier behavior. Remote-only/skill-only plans do
not invent a data requirement.

The added regression checks the accepted exact projection against the original
objects; exercises empty, relative, project-B and another selected data root;
refuses another active package authority; and compares complete project,
user and project-B inventories after the read-only calls. Independently reran
`^TestProjectOpenCode` with Go1.26.10/GOMAXPROCS3/-p2/count1: client **PASS
0.135s**, providers **PASS 0.030s**. The selected-data blocker is closed and no
additional blocker was found. The genuine hosted native boundary stated above
is unchanged.
