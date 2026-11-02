# Removal safety: what `remove` deletes, exactly

Removal is limited to objects whose recorded ownership UAP can verify. It
preserves foreign files, unrelated configuration and externally edited managed
content that cannot safely be removed. Plugin data and verified source caches
can be retained intentionally. This document describes the ownership rules
and the scope of the existing removal tests.

## The rules

1. **Ownership is recorded, not guessed.** Every installed object (skill
   directory, MCP entry, config section, plugin artifact) is recorded with its
   kind, logical name, path, and a digest of its managed content. Removal only
   operates on objects whose recorded ownership the installer can prove.
2. **Foreign files are untouchable.** Files and config entries that live next to
   installed ones but are not recorded as managed are never deleted or edited.
   End-to-end checks verify them byte-identical after removal.
3. **Collisions fail closed.** If another tool has taken over a managed logical
   key (for example a foreign MCP server entry that claims the name the CLI
   installed), `update` and `repair` refuse before committing anything and
   explain how to resolve the conflict. The CLI never adopts or overwrites an
   entry it cannot prove it owns. After a refused operation the client config is
   verified unchanged, byte for byte.
4. **Removal has an explicit result.** A repeated `remove` of an already
   removed installation can be refused rather than silently accepted. E2E runs
   record the exact removal inventory — every deleted path plus its ownership
   classification — as artifacts.
5. **External uninstall stays explicit.** Where a client requires its own
   uninstall, UAP reports that action before settling the managed binding.
   Use `--external-uninstalled` only after completing the reported action.
   This is an attestation for that reviewed removal, not permission to delete
   arbitrary client files. Native cleanup and UAP receipt cleanup are distinct.

## Evidence

The referenced suites exercise removal accounting: a full file inventory
(path to SHA-256) is taken before and after each real removal, every deleted
path must be plugin-owned, stale owned native projections must be absent within
the tested footprint, and foreign
neighbors must be byte-identical. The diffs are recorded as artifacts.

| Client | Recorded fixture behavior | Test category |
| --- | --- | --- |
| Claude Code | managed plugin files + purge marker (8 paths in the reference run) | native e2e, real pinned client (macOS + Windows CI) |
| Codex | managed plugin files + plugin cache/registry entry (14 paths; CLI scratch churn is classified separately) | native e2e, real pinned client (macOS + Windows CI) |
| OpenCode | managed installer state + installed skill (5 paths) | native e2e, real pinned client (macOS + Windows CI) |
| Cursor | managed workspace config entries only | integrations lifecycle e2e |
| Kiro | managed objects only (1 path in the reference run) | disposable lifecycle e2e |
| Gemini | managed skill directory + managed MCP entry only | native removal e2e |
| Cline, Windsurf | managed skill/MCP objects only; foreign skill, foreign MCP entry, sentinels byte-identical | native removal e2e |
| VS Code | its owned profile entry only (local adapter); CLI adapter removes exactly nothing on disk | removal e2e, both adapters |
| Kimi | exactly the registry record; plugin artifacts byte-identical | removal e2e |
| ChatGPT, Copilot, Grok | exactly nothing on disk (manual/external uninstall contract) | removal e2e, fail-closed refusal verified |

This table is a map to existing fixture coverage, not a claim that every row
was rerun for the latest release or in project scope. Path counts describe
the reference fixture, not a fixed contract for arbitrary packages. Qoder and
newer adapters must be evaluated against their own current tests and evidence;
an absent row is not native runtime qualification.

These suites live in `repotests/agentplugins_*_native_e2e_test.go`,
`repotests/integrations_lifecycle_e2e_test.go`, and
`install/integrationctl/agentplugins/clients/*/removal_inventory_test.go` (and
the Kiro/ChatGPT lifecycle tests). Native suites can run against pinned client versions on macOS and Windows
CI runners and emit `remove-inventory-diff.json`. To claim a pass, identify the
successful run and retained artifact for the selected release, client, platform
and scope. Deterministic adapter tests do not substitute for native execution.

## What this does not cover

- External uninstalls performed by a client's own CLI (Copilot, Grok) are
  delegated to the client and accounted for by the applicable contract; the client tool performs its
  part of the removal.
- `repair` reconstructs only absent managed objects. It never "fixes" a foreign
  object that occupies a managed key — that is a refusal, by design.
