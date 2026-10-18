# Removal safety: what `remove` deletes, exactly

The installer's removal guarantee is narrow on purpose: **`remove` deletes
exactly the files the CLI added, and never anything else.** This document
describes the rules that enforce that guarantee and the end-to-end evidence
that they hold for every supported client.

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
4. **Removal is idempotent and accounted.** A repeated `remove` of an already
   removed installation is refused rather than silently accepted. E2E runs
   record the exact removal inventory — every deleted path plus its ownership
   classification — as artifacts.
5. **Manual clients remove exactly nothing.** For clients without a supported
   silent uninstall path (for example ChatGPT plugins, or Copilot/Grok plugins
   removed through their own CLIs), `remove` fails closed until you confirm the
   external uninstall with `--external-uninstalled`, and then changes exactly
   zero files on disk.

## Evidence

The exact-removal accounting runs as end-to-end tests: a full file inventory
(path to SHA-256) is taken before and after each real removal, every deleted
path must be plugin-owned, no plugin-named file may survive, and foreign
neighbors must be byte-identical. The diffs are recorded as artifacts.

| Client | What `remove` deletes | Verified in |
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

These suites live in `repotests/agentplugins_*_native_e2e_test.go`,
`repotests/integrations_lifecycle_e2e_test.go`, and
`install/integrationctl/agentplugins/clients/*/removal_inventory_test.go` (and
the Kiro/ChatGPT lifecycle tests). The native suites run against real pinned
client versions on macOS and Windows CI runners and emit
`remove-inventory-diff.json` artifacts you can inspect after every run.

## What this does not cover

- External uninstalls performed by a client's own CLI (Copilot, Grok) are
  verified as delegated and accounted for; the client tool itself performs its
  part of the removal.
- `repair` reconstructs only absent managed objects. It never "fixes" a foreign
  object that occupies a managed key — that is a refusal, by design.
