# Release Checklist

Use this checklist for post-`v1.0.0` hardening releases and any beta surface that is still shipping outside the stable contract boundary.

## Required Before Tagging

- `make release-gate` green
- `make release-gate` includes `test-required`, `vet`, and `generated-check`
- `make version-sync-check` green
- `make release-rehearsal` may be used as the canonical deterministic local rehearsal shortcut
- `make test-install-compat` green
- `make test-polyglot-smoke` green when stable Node/Python local-runtime, local bundle-install, remote bundle-fetch, or GitHub bundle-publish claims, shell beta claims, launcher logic, doctor/bootstrap/export behavior, or runtime bundle contract changed
- `release-preflight` green for the planned stable tag and required downstream channels
- `dependency-review` green on the candidate branch
- `govulncheck` green on maintained Go modules
- `codeql` green or explicitly triaged on the candidate branch
- generated-config/runtime-contract drift evidence recorded when changes affect `generate`, scaffolded target files, target contracts, or runtime docs
- generated artifacts in sync
- root GitHub Release asset publish result recorded
- support matrix matches shipped claims
- OpenCode and Cursor parity audit refreshed when their documented surfaces or generated contracts changed
- `@opencode-ai/plugin` scaffold pin re-checked against the latest stable npm release when OpenCode scaffolds, examples, or live smoke fixtures changed
- changelog updated
- support/status/release docs updated if contract changed
- candidate commit SHA recorded
- when the public Go SDK consumption contract changed:
  - `plugin-kit-ai-vX.Y.Z` and `sdk/vX.Y.Z` tags planned from the same commit
  - clean-module `go list -m github.com/777genius/plugin-kit-ai/sdk@vX.Y.Z` recorded
  - clean-module `go get github.com/777genius/plugin-kit-ai/sdk@vX.Y.Z` recorded

## Extended / Live Recording

- `polyglot-smoke` workflow result recorded when stable Node/Python local-runtime, local bundle-install, remote bundle-fetch, or GitHub bundle-publish claims, shell beta claims, launcher logic, or Windows runtime resolution changed
- generated-config/runtime-contract drift result recorded when Claude/Codex config wiring, generated target files, or target contract metadata changed
- Homebrew tap update result recorded when the `plugin-kit-ai` CLI install path changed
- npm publish result recorded when the `plugin-kit-ai` CLI npm channel changed
- PyPI publish result recorded when the `plugin-kit-ai` CLI Python channel changed
- npm runtime-package publish result recorded when the `plugin-kit-ai-runtime` npm authoring package changed
- npm runtime-package postpublish registry smoke result recorded when the `plugin-kit-ai-runtime` npm authoring package changed
- optional live npm runtime-package install result recorded when the `plugin-kit-ai-runtime` npm authoring package changed
- PyPI runtime-package publish result recorded when the `plugin-kit-ai-runtime` PyPI authoring package changed
- PyPI runtime-package postpublish registry smoke result recorded when the `plugin-kit-ai-runtime` PyPI authoring package changed
- optional live PyPI runtime-package install result recorded when the `plugin-kit-ai-runtime` PyPI authoring package changed
- when the Python CLI channel uses Trusted Publishing, the PyPI trusted publisher matches `777genius/plugin-kit-ai`, `.github/workflows/pypi-publish.yml`, and environment `pypi`
- `extended` workflow result recorded
- `live` workflow result recorded, or an explicit waiver is noted in release notes
- `release-preflight` workflow result recorded
- `release-assets` workflow result recorded
- release artifact attestation result recorded
- any skipped real-CLI smoke reason is written down
- waiver justification explicitly states why the failure is outside plugin-kit-ai contract scope
- release notes use the same evidence fields as the release playbook

## Beta-Breaking Changes

- beta change note written when beta user code, scaffold output, readiness semantics, or bundle contents change
- deprecation or removal called out in docs/changelog
- stable-candidate set impact reviewed
- [V0_9_AUDIT.md](./V0_9_AUDIT.md) updated when the declared `v1` candidate set changes
- [INTERPRETED_STABLE_SUBSET_AUDIT.md](./INTERPRETED_STABLE_SUBSET_AUDIT.md) updated when the promoted Node/Python local-runtime or bundle-handoff subset changes

## Rehearsal Completion

- each candidate surface is marked `stable-approved`, `stays-beta`, or `blocked`
- no core stable-set surface remains `blocked`
- release notes draft exists
- rehearsal worksheet exists
- known limitations are written down

## `v0.9` Freeze Check

- no new public-beta surfaces added unless required to finish the declared `v1` set
- remaining work limited to bug fixes, docs, e2e hardening, release tightening, and reviewed post-`v1` promotion work
