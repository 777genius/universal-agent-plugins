# Saved dependency discoverability: explain, do not redesign

Date: 2026-10-31. Base: `4d1f9e7655fad92ed3a3bc99fa25c92aefa4278c`.
Status: proposed, review before code changes.

## Observed problem

The existing TUI already supports discovery, target selection, progress, reviewed
installation and recovery. Do **not** replace it with a wizard. The existing
`agentplugins deps status` prints internal fact tokens such as `not_synced`,
`manifest_missing`, `activation_prepared` and `store_missing`, while
`deps tree` shows exact locked packages without explaining that indentation
means transitive dependency and `(reference)` is the same shared node.
New users may therefore not know whether an installation is ready or where to
see and manage the saved package graph. Advanced users need those exact facts.

## Bounded change under review

1. Keep all commands, flags, scope defaults, shortcuts, TUI components,
   advanced diagnostics, JSON fields and exit codes **byte-for-byte unchanged**.
   No automatic update, deletion, selection, activation or trust decisions.
2. Improve **human** `deps status` with a short, non-destructive explanation
   of the recorded state and existing commands to inspect versions (`deps tree`),
   check possible updates (`outdated --all`) and apply an exact lock
   (`install --locked`). Where recovery is pending, reference the existing
   `deps recover` contract, never execute it. Warn that project scope
   needs the same explicit `--scope project` / `--project-root`.
   Do not claim runtime loading, OAuth or readiness from saved files alone.
3. Improve **human** `deps tree` only when useful: explain direct vs
   transitive indentation and shared `(reference)` and give a clear message
   for an empty lock. Keep exact versions, source labels, graph edges and
   recursive output. No artificial depth cutoff.
4. Add focused golden/behavior tests for empty environment, shared and
   transitive graph, project guidance, pending recovery and JSON equivalence.
   If exact-output compatibility concerns outweigh the benefit, prefer
   docs-only guidance and explain why. Avoid a second “deps list” implementation
   that duplicates `deps tree` and `deps status`.

## Critic checkpoints

- **First-time user:** Can I see what is installed and the safe next action?
  Do not make a new modal, jargon, or required steps.
- **Power user:** Are exact facts, lock graph, scoped flags and JSON untouched?
  No hidden options or destructive default.
- **Standards/interoperability:** Keep Agent Plugins 1.0 `plugin.json` and
  Agent Skills 1.0 `SKILL.md` untouched; `uap.toml` remains a UAP manager
  contract, not a requirement for portable plugins.
- **Safety:** No command assembled from untrusted paths/shell strings.
  No claim that “installed” means activated/authenticated or that an invalid
  lock is healthy.
- **Maintainability:** Modify existing renderers, not create a UI/DI framework
  or additional state store.

## Acceptance

Run focused CLI tests and ordinary `Required`/Core and platform-relevant CI.
Independently review new copy and precise behavior, fix findings, and merge only
after exact-head CI succeeds. No release, no VS Code change, no edits to other
agent-owned PRs.
