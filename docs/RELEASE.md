# Release And Quality Gate Policy

This document defines the expected test lanes and release ladder for the current post-`v1.0.0` repository.

## Test Lanes

- `required`: deterministic local tests that must stay green on every change. This includes unit tests, integration tests, and repository guard tests that do not require live external CLIs or network access.
- `polyglot-smoke`: deterministic cross-platform launcher and executable-ABI smoke for `go`, `python`, `node`, and `shell`, including Windows `.cmd` behavior, path-with-spaces coverage, generated Claude/Codex config canaries, `generate --check` drift protection for runtime-affecting artifacts, stable Node/Python doctor/bootstrap/export/bundle-install/bundle-fetch/bundle-publish claims, official `plugin-kit-ai` bootstrap/setup path evidence (Homebrew formula generation, the `public-beta` npm wrapper contract, the `public-beta` PyPI/pipx wrapper contract, the `plugin-kit-ai-runtime` npm/PyPI authoring packages, `scripts/install.sh`, `setup-plugin-kit-ai@v1`, generated bundle-release workflow), shell beta claims, and repo-local bootstrap failure paths such as broken `.venv`, missing built Node output, and non-executable shell targets.
- `live`: may also record macOS Homebrew install evidence, npm install/npx evidence, pipx install/run evidence for the released `plugin-kit-ai` CLI, plus manual npm/PyPI install evidence for the shared `plugin-kit-ai-runtime` authoring packages when those channels changed.
- `runtime-package-registry-smoke`: exact-version postpublish registry install/import smoke for `plugin-kit-ai-runtime` on npm and PyPI after the downstream authoring-package publish workflows succeed.
- `dependency-review`: pull-request gate for newly introduced vulnerable runtime dependencies before they merge.
- `govulncheck`: deterministic Go reachability scan across maintained modules using the latest stable `govulncheck`.
- `codeql`: repository code scanning lane for Go and JavaScript/TypeScript.
- `extended`: subprocess smoke and platform-CLI tests that may depend on locally installed tools or opt-in environment variables, but should still stay narrowly scoped and finish quickly.
- `nightly/live`: real network or externally authenticated scenarios, including live install compatibility checks and live-model sanity runs.
- `generated-sync`: deterministic generated-artifact drift check used by release gates and rehearsal, but kept separate from the default `required` lane.
- `version-sync-check`: deterministic pinned-version contract check for Go SDK and shared runtime package references across scaffolds, examples, docs, and release-facing tests.

`extended` should prefer one external-CLI smoke class per `go test` invocation. This avoids mixed-process hangs from combining multiple real CLI harnesses in a single test process.

Current workflow mapping:

- `ci.yml`: blocking `required` lane
- `polyglot-smoke.yml`: deterministic Ubuntu/Windows `polyglot-smoke` lane
- `release-preflight.yml`: manual release prerequisite check for tag format, metadata hygiene, and downstream publish secrets/vars
- `dependency-review.yml`: pull-request dependency gate for vulnerable runtime additions
- `govulncheck.yml`: reachability-based Go vulnerability scan across maintained modules
- `codeql.yml`: code scanning for Go and JavaScript/TypeScript
- `extended.yml`: manual `extended` lane with artifact upload
- `live.yml`: manual live lane with artifact upload
- `runtime-package-registry-smoke.yml`: automatic postpublish registry verification for the shared runtime helper packages, plus manual rerun by tag

Current local maintainer shortcuts:

- `make release-gate`: `test-required -> vet -> generated-check`
- `make release-rehearsal`: `release-gate -> test-install-compat -> test-polyglot-smoke`
- `make version-sync-check`: validates pinned Go SDK and shared runtime package references against `scripts/version-contract.env`

## Branch And Flow Policy

- `main` remains the canonical releasable branch.
- Release rehearsal must be performed on one fixed candidate commit SHA.
- If any repo-tracked change lands after rehearsal evidence is recorded, rehearsal evidence must be refreshed for the new candidate commit.
- Promotion decisions come from the audit ledger, not from branch naming or tag naming alone.
- A rehearsal tag or notes artifact may be created before `v1.0`, but it does not imply promotion until the audit ledger is complete.
- Branch protection intent in the current repo policy:
  - `required`: blocking on normal PR flow
  - `polyglot-smoke`: separate deterministic lane required for runtime/ABI/bootstrap-affecting changes and for release rehearsal
  - `extended` and `live`: manual evidence lanes, not blocking by default

## Release Playbook

Use this exact order for stable or beta release work:

1. checkout the candidate commit
2. run `make release-gate`
3. run `make test-install-compat`
4. run `make test-polyglot-smoke`
5. review `docs/V0_9_AUDIT.md` and any post-`v1` promotion ledger that applies, including `docs/INTERPRETED_STABLE_SUBSET_AUDIT.md` for the Node/Python local-runtime subset
   and the official CLI bootstrap/setup path when the release changes community setup ergonomics
6. run or record `extended`
7. run or record `live`
8. record waivers for skipped external smoke if needed
9. draft release notes from the release-notes template
10. update each candidate row to `stable-approved`, `stays-beta`, or `blocked`
11. run `release-preflight.yml` against the planned stable tag and required downstream channels
12. cut the rehearsal or release tag only after the audit ledger is complete and preflight is green
13. publish root GitHub Release assets from the finalized stable tag through `release-assets.yml`

Required release artifacts:

- candidate commit SHA
- required lane result
- vet result
- generated-artifact sync result
- version-sync-check result
- install compatibility matrix result
- polyglot smoke result
- generated-config/runtime-contract drift result
- extended result
- live result or waiver
- release preflight result
- root GitHub Release asset publish result
- release artifact attestation result
- updated audit ledger
- updated post-`v1` promotion ledger when the release changes the interpreted stable subset
- Homebrew tap update result or explicit manual-fallback note when the CLI install path changed
- npm publish result and optional live npm smoke result when the npm CLI channel changed
- PyPI publish result and optional live pipx smoke result when the Python CLI channel changed
- npm runtime-package publish result when the Node/TypeScript authoring helper package changed
- npm runtime-package postpublish registry smoke result when the Node/TypeScript authoring helper package changed
- optional live npm runtime-package install smoke result when the Node/TypeScript authoring helper package changed
- PyPI runtime-package publish result when the Python authoring helper package changed
- PyPI runtime-package postpublish registry smoke result when the Python authoring helper package changed
- optional live PyPI runtime-package install smoke result when the Python authoring helper package changed
- when the Python CLI channel changed and uses Trusted Publishing, the PyPI-side publisher must match:
  - owner/repo: `777genius/plugin-kit-ai`
  - workflow: `.github/workflows/pypi-publish.yml`
  - environment: `pypi`
- Go SDK module proxy evidence when the Go SDK public consumption contract changed:
  - `go list -m github.com/777genius/plugin-kit-ai/sdk@vX.Y.Z`
  - `go get github.com/777genius/plugin-kit-ai/sdk@vX.Y.Z`
- release notes draft

No stable tag should be cut without one completed rehearsal cycle using this playbook.
When a release changes the public Go SDK consumption contract, cut the
`plugin-kit-ai` release tag and the SDK submodule tag from the same commit:

- plugin-kit-ai tag: `plugin-kit-ai-vX.Y.Z`
- SDK submodule tag: `sdk/vX.Y.Z`

GitHub Release assets stay on the product-prefixed tag. The SDK is published
through the Go module proxy path via the `sdk/vX.Y.Z` tag, not through separate
tarballs. Historical `vX.Y.Z` tags remain valid compatibility references and
must not be moved or deleted.
The legacy v1 GitHub Release assets are published through
`.github/workflows/release-assets.yml`, which runs GoReleaser from the selected
stable tag and uploads the `plugin-kit-ai_*` archives plus `checksums.txt`.
Downstream `.github/workflows/homebrew-tap.yml`, `.github/workflows/npm-publish.yml`, `.github/workflows/pypi-publish.yml`, `.github/workflows/npm-runtime-publish.yml`, and `.github/workflows/pypi-runtime-publish.yml` follow successful `Release Assets` completion and resolve the exact stable tag from that commit; `.github/workflows/runtime-package-registry-smoke.yml` then verifies the published `plugin-kit-ai-runtime` channels from npm/PyPI by that exact version. Manual `workflow_dispatch` remains the fallback when a maintainer needs to rerun a channel by tag.
When a published stable release should update `777genius/homebrew-plugin-kit-ai`, `.github/workflows/homebrew-tap.yml` is the automatic path. If `HOMEBREW_TAP_TOKEN` or tap permissions are unavailable, the release notes must record the failure and the maintainer must run `TAG=<tag> HOMEBREW_TAP_TOKEN=<token> ./scripts/update-homebrew-tap.sh`.

## Shipping Gate For New Stable Functionality

No event or public contract claim should be treated as shipped unless all of the following exist:

- descriptor definition
- runtime wiring
- public registrar API
- scaffold support
- validate rules
- generated support-matrix row
- unit coverage
- integration coverage
- contract or golden coverage
- smoke e2e coverage

## Release Ladder

- `dev`: normal mainline delivery with the current mix of stable and beta surfaces
- `beta`: feature-complete enough for targeted external validation
- `rc`: release-candidate stabilization; only bug fixes, docs, beta change notes, hardening work, and explicitly reviewed post-`v1` community beta additions
- `stable`: reserved for `v1.0` and later major-compatible releases

See also [RELEASE_CHECKLIST.md](./RELEASE_CHECKLIST.md) for pre-tag execution steps.

## Waiver Policy

Waivers are allowed only for failures outside the plugin-kit-ai contract boundary:

- external Claude/Codex runtime-health failures before hook execution
- live/network failures in external systems that do not indicate a repo regression

Waivers are not allowed for:

- repo-controlled test failures
- deterministic required-lane failures
- scaffold/validate/runtime contract regressions
- generated Claude/Codex config contract regressions
- smoke failures that show plugin-kit-ai misbehavior after the hook path should have executed

Every waiver must record:

- date
- candidate commit SHA
- affected lane
- exact skipped or failed surface
- reason
- why it is outside plugin-kit-ai contract scope
- maintainer sign-off in release notes or rehearsal notes

## `v0.9` Freeze Criteria

After `v0.9`, only these change classes are expected:

- bug fixes
- docs corrections
- beta change notes
- quality-gate hardening
- e2e stabilization
- release process tightening

New public-beta surface should not be added after the freeze unless it is required to complete the declared `v1` stable set.
