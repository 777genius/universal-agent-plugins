# Shared project component implementation independent review R1

Reviewer: sources_lead, 2026-10-23. Reviewed new shared-only implementation
`91738de1f0245d2ba00b991c6e00cfde07d17675` plus corrective
`5e987e979661cde95d178f7d2eba99cd6a7cafe1`, read-only for all shared helper
files. The reviewer is implementing the independent Codex consumer, not this
helper. The helper files in the Codex integration tree are byte-identical to
the reviewed shared commits; no reviewer helper edits were made.

## Decision

**R1 GO at corrective 5e987e97.** No distinct confirmed implementation blocker
remains. The other reviewer's portable-spelling finding is closed by the
corrective commit: padded logical names are rejected before the historical
leaf-ID validator can trim them, and source-relative components use the
existing portable NFC/UTF-8/reserved-name grammar.

## Independent component assessment

- The public boundary matches the twice-reviewed amendment: explicit project,
  skills and config roots, a caller-selected codec and declared ownership kinds;
  prepared server values and exact desired receipts; no payload loader,
  transport mapper, settings/trust engine or native process dispatch. Existing
  client implementations remain unchanged.
- Layout and object validation enforce exact clean paths, strict project
  containment with no-follow ancestor checks (including contained aliases),
  known kinds, exact kind:name IDs, unique identities and nonempty digests.
  Active package roots and portable skill source-relative directories are
  independently validated. Infrastructure package ownership is deliberately
  an adapter/core concern; adapters must remove only that known receipt before
  passing native component objects to this closed helper.
- Prepared server maps must exactly cover desired MCP names: both cardinality
  and lookup are checked. Caller-reported server values are reprojected through
  Kernel DesiredReceipt and compared to the staged digest/path before any native
  mutation. An extra, missing or mismatched prepared server cannot silently
  change the selected component set.
- Previous ownership is verified before mutation, allowing missing entries only
  for repair/removal. New same-name native objects collide. Missing same-pin
  skills are staged and recreated; equal receipts skip only an actually present
  real directory with the exact digest, after independently checking the active
  source bytes.
- Skill staging uses private same-filesystem directories, records their inode
  identity and exact contents, verifies staged bytes, exclusively moves old
  objects to backups and rechecks isolated digests. Installed desired skills and
  cancellation are checked before the single nativeconfig batch.
- Native MCP edits retain existing writer locks, complete-entry receipts,
  selected-path DesiredReceipt checks, conditional replacement, rollback and
  postwrite verification. Distinct skill/config roots and a different format
  such as Codex TOML are ordinary layout/codec inputs, without borrowing a
  Cursor path or JSON serializer.
- Failure rollback removes only newly installed directories that remain exact,
  restores exact backups without replacing a competing destination, and retains
  the recovery root when a destination, backup, private inode or unknown private
  entry changed. Safe observed failures may report unchanged; unknown MCP commit
  failures remain uncertain even after skill restoration.
- A successful batch or CommittedCleanupError remains authoritative. Cleanup
  checks the private identity, complete entry inventory and each remaining
  directory digest before deletion; cleanup degradation cannot roll back
  committed config or erase externally changed recovery bytes. No data or
  foreign native directory is adopted or removed.

## Independent decisive execution

At the corrected shared tree, `go test -p=2 -count=1 ./clients/shared -run
'^TestProjectComponent'` passed **0.081s**. The selected contracts exercise
complete add/no-op/missing-pin repair/update/repeat-remove, extra/missing server
maps, staged-source/relative-path/receipt tamper, native aliases, portable
spelling, MCP failure after skill swap with exact restoration, externally edited
install preservation with retained recovery, changed/unknown committed backup
cleanup refusal, and cancellation before mutation.

The independent in-progress Codex consumer additionally passed its production
loader/planner/renderer/stager/provider contracts **0.070s**, using actual
`.agents/skills` and `.codex/config.toml` roots with CodecCodex. This checks
helper interoperability with a distinct TOML config root, repeat verification,
missing-pin recreation, exact removal, foreign user/cache/project-B preservation
and conservative prepared status. The Codex adapter itself still requires its
own implementation reviews; that consumer result is not an adapter GO.

No package payload, native agent session, configured MCP command or model was
executed. Native product qualification and final integration CI remain separate.
No corrective request beyond the independently discovered portable-spelling
fix was necessary in this rotation.
