# Release and quality-gate policy

`agentplugins` is the sole public CLI. Its executable release contract is the
[Agentplugins stable release runbook](./agentplugins-release.md). This document
does not authorize a version, create a release, or replace the exact-version
owner approval required by that runbook.

## Current release workflows

- `.github/workflows/agentplugins-release.yml` builds and attests the six native
  `agentplugins` binaries, stages a non-public draft, runs platform proof, and
  promotes only when `publish_release=true` after all dependencies succeed.
- `.github/workflows/agentplugins-npm-publish.yml` is a separate manual trusted
  publisher for the `universal-agent-plugins` npm facade. Its prepare job is
  read-only; its publish job uses the protected `npm-agentplugins` environment;
  its final job verifies the exact public package.
- `.github/workflows/npm-runtime-publish.yml` and
  `.github/workflows/pypi-runtime-publish.yml` are manual-only publishers for
  the intentionally retained `plugin-kit-ai-runtime` helper libraries. These
  packages are implementation dependencies, not public CLI installation paths.
- `.github/workflows/runtime-package-registry-smoke.yml` verifies those helper
  packages after a helper publisher succeeds or by explicit manual dispatch.

Tag-keyed concurrency for both `agentplugins` workflows never cancels an
in-progress release. The npm workflow is dispatch-only. A failed, cancelled, or
skipped prerequisite must leave every mutating downstream job unreachable.

## Quality gates

- `required`: deterministic unit, integration, and repository contract tests.
- `polyglot-smoke`: retained cross-platform implementation and runtime-helper
  coverage. Historical CLI cases in this lane do not define a current product
  or release channel.
- `dependency-review`, `govulncheck`, and `codeql`: dependency and source
  security gates.
- `extended` and `live`: manually collected evidence. Historical package checks
  are regression evidence only, never current installation guidance.
- `generated-sync` and `version-sync-check`: generated-file and shared
  dependency drift checks.

Local maintainer shortcuts remain:

- `make release-gate`: `test-required -> vet -> generated-check`
- `make release-rehearsal`: `release-gate -> test-install-compat -> test-polyglot-smoke`
- `make version-sync-check`: validates pinned Go SDK and runtime-helper references

## Agentplugins release evidence

For an owner-approved exact version:

1. Fix one candidate commit on `main` and record the required CI, dependency
   review, CodeQL, vulnerability, generated-file, and applicable platform proof.
2. Create the approved `agentplugins-vX.Y.Z` tag at that exact commit.
3. Follow `docs/agentplugins-release.md` to qualify the non-public GitHub draft.
4. Promote only through the same workflow with explicit owner authorization.
5. Run the separate agentplugins npm workflow against the exact public tag and
   retain its prepare, protected publish, and public-readback evidence.
6. Record the native and npm artifact identities, attestations, checksums,
   workflow run IDs, commit SHA, approvals, and any truthful limitation.

Do not reuse or move a stable tag, overwrite a published package version, use an
unpinned `latest` binary, or treat a draft as a public release.

## Retired plugin-kit-ai CLI publication

No new `plugin-kit-ai` CLI GitHub, npm, PyPI, Homebrew, or native release is supported. Do not run a release preflight for it and do not offer a pipx install
journey. Already published artifacts and release records remain immutable
history. The last workflow implementations are preserved, non-executable, in
[`docs/history/plugin-kit-ai-release-workflows/`](./history/plugin-kit-ai-release-workflows/README.md).

The retained CLI implementation, package sources, tests, SDK, runtime helpers,
and design documents remain source history under the capability-preservation
contract. Their presence does not create a second supported engine or authorize
restoring a publisher.

## Waivers

Waivers apply only to failures outside the current product boundary, such as an
external client or network failure before repository-controlled behavior runs.
They never cover repository-controlled test failures, release identity or
attestation failures, unsafe mutation reachability, or missing exact-version
evidence. Record the date, candidate SHA, affected lane, exact failure, reason,
boundary analysis, and maintainer sign-off.
