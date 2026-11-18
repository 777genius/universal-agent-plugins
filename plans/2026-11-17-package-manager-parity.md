# UAP package-manager parity — scoped implementation plan

Date: 2026-11-17. Source baseline: main \`b632a66a828d526a3ac797a98c9adeaf6b705518\`.
Status: implementation plan; **not** a claim of released functionality.

## Verified baseline

- A saved \`agentplugins add\` already creates \`uap.toml\` and \`uap.lock\` if absent. \`agentplugins deps init\` exists. Bare \`agentplugins install\` selects Project only when a regular \`uap.toml\` exists in the exact current directory. \`add\`, \`update\`, \`repair\` and \`remove\` default to User unless explicitly scoped.
- Authoring commands exist behind explicit build-time release gates (\`commands.IsRelease\`, \`release_root.go\`, \`authoringcli\`). Current native v0.1.110 installer does **not** ship \`author\`. The present authoring surface does not include new standardized bundle \`pack\` / publication workflow; legacy \`plugin-kit-ai\` is separate.
- Range selector implementation: \`environment/semver.go\` (61 lines), exact stable and \`^MAJOR.MINOR[.PATCH]\` only; the existing graph resolver, locks, digest and ownership state are reused.
- \`[dev-dependencies]\` is rejected today. Graph uses typed \`Requirements.Dependencies\` and resolves producer dependencies separately from project roots.

## Goals / acceptance

1. **README / discovery:** make the existing released feature set findable with first-party links, and describe source-only authoring explicitly, without promoting source capabilities to released status.
2. **Semver:** use vetted \`github.com/Masterminds/semver/v3@v3.5.0\` for exact/range checking. Preserve existing exact and caret behavior and lock stability; support comparable registry constraints (\`^ ~ >= > <= < = * x ||\`, ranges). Retain strict validation of published package versions and safe handling of prereleases. Keep the resolver's existing candidate ordering and fail-closed policy.
3. **Development dependencies:** accept root \`[dev-dependencies]\` locally, include them when resolving the selected development environment, but **never propagate a dependency's development-only requirements to consumers**. Preserve direct normal dependency/target semantics and avoid losing comments or changing foreign manifests. Lock fingerprint and frozen replay must detect the declaration set; no silently orphaned owned files.
4. **Project UX:** document and test present exact-CWD auto-selection; no implicit Project install for a bare \`add\` without a manifest and no User/Project fallback.
5. **Authoring release:** separate gated PR. Reuse the existing release root instead of simply flipping the build-time flag. Qualify complete published-binary CLI and distribution on supported OSs before changing README statements to \`released\`. \`pack\` / marketplace publication remain separate unless real workflows and proof exist.

## Scope boundaries

**In:** registry version selectors; root-only development dependency declarations; existing graph/lock/install/manifest editor integration; explicit documentation and E2E for those journeys; public authoring release verification in its own PR.

**Out:** rewrite of the dependency graph, new registry protocol, npm execution, arbitrary Git-tag range resolution (separate, bounded follow-up if a concrete APM parity test demands it), generic central enterprise policies, raw \`plugin.yaml\` authoring, silent project selection changes, generalized package publication without a complete package format/provenance contract.

Existing \`uap.toml\` v1 / \`uap.lock\` v1 must retain compatibility. If dev-only classification cannot be preserved safely by v1's lock contract, introduce a narrow backwards-compatible optional field rather than a second lock format.

## Delivery order

### PR A — manifest + semver + dev dependencies + README

1. Audit existing root, package and lock paths; look for overlapping PRs and preserve ownership and tests.
2. Add pinned Masterminds semver dependency in the owning Go module and verified checksums. Single adapter function replaces bespoke caret arithmetic; maintain strict canonical version input (no implicit v-prefix/coercion).
3. Decode \`[dev-dependencies]\` into an explicit typed boundary. Root's dev direct requirements participate in selected environment; package-local dev requirements do not enter transitive resolution. Conflict on reused alias/source must fail before file mutation.
4. Update lock intent/compatibility checks, manifest editing and saved lifecycle with minimal changes. Do not change agent targets or source acquisition behavior.
5. Add table tests against frozen node-semver fixtures and cross-scope root/package/locked-restore E2E. Include no-op, shared dependency, stale lock, transitive dev exclusion, undo/remove and partial failure cases.
6. Add short README feature navigation with precise source vs released status.
7. Push commits immediately to the isolated branch, open Draft PR, inspect CI and fixes, merge after E2E and independent review.

### PR B — authoring release qualification (separate branch)

1. Inventory earlier authoring release branches and explicit release blockers. Do not overlap a concurrently owned authoring branch.
2. Run the already existing release-root and native integration tests against an exact source revision.
3. Validate one real packaged native executable per OS/architecture and npm/Homebrew channel where applicable: \`author --help\`, \`init\`, \`validate\`, negative cases, installer separation, binary distribution and published checksum.
4. Ship the author command only once the full release gate is proven. Document it prominently with a linked authoring guide. Do not advertise \`pack\` or publish as shipped until separately completed.

### UX comparison

Use the same scenario cards for APM v0.36.0 and UAP v0.1.110 / candidate: cold package add, no-manifest first add, project scope, exact locked restore, partial multi-agent failure, authoring creation. Record exact commands, exit code, resulting files, elapsed wall time and user prompts in disposable HOME and project roots. Without a runnable matched binary, mark a scenario **not run**, not passing. No invented participants.

## Three critical review rounds (before coding)

**R1 — API/semantic compatibility.** Correction: the claim that UAP lacks initialization is false; preserve saved-add auto creation. Also, authoring exists in source but not in v0.1.110. Directly enabling a build gate bypasses release qualification; isolate authoring to PR B.

**R2 — security/state review.** Correction: never auto-change a bare \`add\` from User to Project. Validate selectors before network/actions; enforce explicit agent scopes, preserve exact old lock pins; reject missing manifests, unexpected dev graphs and conflicting alias declarations before applying changes. No data loss on failed parse.

**R3 — Clean Architecture and minimalism.** Correction: use mature semver library, not own parser. Keep package-local dev exclusion at the manifest boundary, and existing graph resolver as sole dependency resolver. Avoid a new parallel manifest, new publishing framework or sweeping CLI rewrite. Keep PRs independently reviewable.

## Estimates (changed/added LOC, including focused tests)

| Deliverable | Range | Complexity / 10 | Confidence / 10 |
| --- | ---: | ---: | ---: |
| Semver library/constraints and regression tests | 250–600 | 4 | 7 |
| Root dev-only requirements + graph/lock/saved lifecycle tests | 550–1,200 | 7 | 5 |
| README + project discovery links and scoped docs | 40–120 | 2 | 9 |
| Authoring release **qualification**, separate PR | 100–350 if gates already green; substantially more if defects | 7 | 5 |
| Matched UX E2E protocol/results | 150–300 | 5 | 6 |

## Merge gates

- Frozen comparison fixtures for supported range syntax and prerelease edge cases.
- Go test / lint and specific E2E for dependency workflows on current main + relevant platforms; no skipped/queued job reported as success.
- No change to User/Project default without a separate approved contract.
- No authoring release promise unless exact packaged distribution and installer coexistence pass.
- Re-review diff after test fixes; never merge on pending checks.

## Progress log

- 2026-11-17: 3 review rounds above incorporated; scope frozen before first code change.
