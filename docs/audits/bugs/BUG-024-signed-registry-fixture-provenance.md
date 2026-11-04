# BUG-024 — signed alias proof is not bound to the frozen Registry inventory

## Reproduction

Strix independent review on 2026-11-04 reproduced a false-PASS case: `matching_installations()` accepts a signed alias when a forged `list` node repeats the expected package name and supplies internally consistent `directory.recorded_revision`, distribution, sequence and client SHA-256, **even if `source` is absent or refers to another repository**. The frozen `registry_inventory_commit` in the fixture was never compared. This is an audit harness proof defect; not a demonstrated compromise of the Registry or production installer.

## Immutable upstream input

`777genius/universal-agent-plugins-registry@0627020e5546354d111926c18d768bacae989890:registry/directory.json` was fetched independently from GitHub. It has 21 selected signed aliases and active distribution entries with repository/path, sequence, tree and manifest SHA-256. Six active releases use `package_source.revision=null` in this frozen source: chrome-devtools-bridge, cloudflare, firebase, firecrawl, marketing-kit and playwright. **Null is not an immutable revision**; do not invent a 40-hex value or award PASS for these six until the actual recorded revision can be independently verified.

## Minimal plan

1. Pin expected signed distribution ID, source repository/path, active sequence, full revision if available, and exact tree and manifest digests for the 21 selected signed aliases, sourced only from the frozen Registry commit. Preserve the 9 existing explicit 40-hex Git sources.
2. Validate that every signed alias has a pin; if the frozen release lacks a concrete revision, preserve the source but mark its native proof unqualified instead of accepting whatever the live Registry advertises.
3. Require the verified `list` installation's source, signed recorded distribution and revision, sequence and both digests to match the pinned expectation. Reject absent/forged source or changed release without comparing to `list`'s own values only.
4. Regress a forged same-name/signed-looking release with missing or wrong source, wrong digest, and happy exact immutable known release. Rerun focused unittest, Required and both live 15-adapter workflows; inspect source-by-source JSON artifacts on final HEAD.

## Reproduced new pilot failure and source-selector contract

Exact HEAD `d5283e482c7a00d4b6366ee1c1d990a373d69a6c` ran 33/33 Python regressions successfully, but both real Codex plugin pilot cases returned `incomplete-native-proof`. Static inspection of the **actual CLI serializer** `cli/internal/agentpluginscli/read.go:586-590,643-682` confirms `publicInstallationView.Source` is **`publicRepositoryPath(installation.Source.Repository, installation.Source.PackageSubpath)`**, i.e. `repository/package/path`, **not** `directory.recorded_distribution`. Our initial frozen-pin guard erroneously compared these different authority fields and caused the pilot failure. The correction must compare reported `source` to the frozen repository+path, and independently compare `directory.recorded_distribution` to the pinned distribution. This does not relax immutable revision or digest checks. Regress both legitimate repository/path source and forged/missing sources before any new PASS.

## Critical review round 1 — supply chain

Self-consistency of data returned by a live Registry is not independent provenance. No mutable tag, unsigned release or unresolved revision may be promoted to PASS. Do not weaken BUG-009's fail-closed checks.

## Critical review round 2 — scope

Only the E2E proof layer and frozen fixture change. No installer or Registry policy change, feature flags, forced source updates, LintAI/cgroup bypass or foreign profile mutation. A missing immutable release is an evidence blocker, not a product regression.

## Status

Minimal source-selector correction and exact-source regression pushed in commits `7bd2b0e` and `2246108`. The CLI exposes `repository/subpath` as `list.source`, while `directory.recorded_distribution` is separately matched to the frozen distribution; distribution-only and absent/forged source are rejected. Public diagnostic evidence now preserves safe multi-component source paths. **Final-head Python, CI, complete live ZIP evidence, and CodeRabbit/Strix re-review remain pending; do not count historical pilots or merge.**
