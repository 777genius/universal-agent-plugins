# Agentplugins release checklist

Use only for an owner-approved `agentplugins` version. The full procedure is in
[`agentplugins-release.md`](./agentplugins-release.md).

## Before tagging

- [ ] candidate is the exact current `main` commit from a merged pull request
- [ ] exact `agentplugins-vX.Y.Z` owner approval is recorded
- [ ] required CI, dependency review, CodeQL, and `govulncheck` evidence is green
- [ ] release-bound Directory bootstrap and production-binary negative test pass
- [ ] generated artifacts and version contracts are in sync
- [ ] applicable platform and native-client evidence is recorded without
      overstating its scope
- [ ] release notes name the candidate SHA and exact evidence

## GitHub release

- [ ] dispatch `agentplugins-release.yml` from the exact tagged `main` commit
      with `publish_release=false`
- [ ] approve the protected draft staging job
- [ ] all six builds depend on validation
- [ ] draft staging depends on validation and every build
- [ ] all six read-only platform proofs consume the same-run frozen artifact
- [ ] `verified-draft.json` is retained and matches the live draft
- [ ] no failed, cancelled, or skipped prerequisite reaches promotion
- [ ] obtain separate owner authorization before rerunning with
      `publish_release=true`
- [ ] verify the public asset set, checksums, manifest, notices, attestations,
      tag, and commit after promotion

## npm facade

- [ ] dispatch `agentplugins-npm-publish.yml` manually from the exact public tag
- [ ] prepare verifies the public release and uploads one immutable tarball
- [ ] publish consumes that artifact in the protected `npm-agentplugins`
      environment using OIDC, with no npm token fallback
- [ ] public verification depends on both prepare and publish and reconciles
      metadata, integrity, signatures, provenance, and isolated lifecycle
- [ ] no failed, cancelled, or skipped prerequisite reaches a mutating step

## Retained helper packages

- [ ] if an intentionally retained `plugin-kit-ai-runtime` helper changed,
      separately approve and manually dispatch only its applicable helper
      publisher and registry smoke
- [ ] do not present a runtime helper as a CLI installation channel

## Retired surface

- [ ] no executable workflow under `.github/workflows` publishes or preflights a
      `plugin-kit-ai` CLI GitHub, npm, PyPI, Homebrew, or native release
- [ ] no release note instructs users to install that retired CLI with pipx
- [ ] historical workflows and useful source remain preserved outside the
      executable workflow directory
