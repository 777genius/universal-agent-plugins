# Release and quality-gate policy

`agentplugins` is the sole public CLI. Its executable release contract is the
[Agentplugins stable release runbook](./agentplugins-release.md). This document
does not itself authorize publication or create a release. An explicit owner
request to make or publish a release authorizes choosing and publishing the next
suitable unused version within the same product and channels, subject to explicit
constraints and the runbook's gates. Report the chosen version; no repeated
approval is needed solely for version selection or skipping a cancelled,
never-public candidate. Preparation, review, and draft-only requests do not
authorize public publication.

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

For an owner-authorized release, after choosing and reporting the exact version:

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

## OpenCode event SDK

`universal-agent-plugins-opencode-events` is an independently versioned npm
library from `sdk/opencode-js`. Publishing the CLI does not publish this SDK.
The owner may authorize an SDK-only release without releasing CLI binaries or
runtime helpers. Preserve the published root factories when adding generation
specific `/v1` and `/v2` entrypoints.

For an owner-authorized SDK release:

1. Choose an unused version and merge its reviewed candidate into `main` through
   the required repository CI and security gates. Record the source commit and
   exact tested OpenCode generations/versions. From `sdk/opencode-js`, run
   `npm ci --ignore-scripts`, `npm run check`, and `npm run check:packed`.
   From `sdk`, run `go test -count=1 ./opencode`.
2. Qualify actual V1 and V2 host plugins against the packed SDK in disposable
   projects and isolated profiles. Record positive completion and unanswered
   question/permission behavior and cancellation/settlement fencing. Unit ports,
   type declarations and loader-only checks do not establish native behavior.
   Preserve the distinction between compatibility factories and strict native
   reader/provenance qualification. Product-specific delivery, native installers,
   and notification platform matrices remain separate product gates.
3. Create an immutable `opencode-events-vX.Y.Z` tag at that qualified `main`
   commit. Dispatch `opencode-sdk.yml` **from that tag** with `version=X.Y.Z`.
   Its Linux/macOS/Windows package contracts and Go decoder must all pass before
   the read-only release job packs and attests one tarball. This workflow does
   not publish to npm or mutate GitHub releases.
4. Download the `opencode-events-X.Y.Z` artifact from that exact run. Verify
   `source-commit.txt`, `pack.json`, SHA256 and the GitHub attestation against
   `.github/workflows/opencode-sdk.yml` and the tag's source digest. Stage the
   exact tarball and checksums in a non-public SDK GitHub release with
   `--latest=false`; never replace an existing release asset.
5. After all gates pass, publish **that tarball**, without repacking, using the
   maintainer's existing authorized npm session:
   `npm publish /absolute/path/to/package.tgz --registry=https://registry.npmjs.org/ --access public --tag latest`.
   Do not read or copy credentials. An authentication failure requires restoring
   the supported session, never bypassing npm's controls. If publication outcome
   is uncertain, inspect the exact version before attempting another mutation.
6. Read back npm name/version, exports, peer range, integrity and shasum from
   `https://registry.npmjs.org/`. Compare metadata to `package/package.json`
   inside the attested tarball and integrity/shasum to `pack.json`.
   Install that exact public registry version in clean V1 and
   V2-only consumers and verify imports, public types and behavior. Preserve the
   V2-only guarantee that the legacy V1 SDK is not installed. Promote the SDK
   GitHub draft with `--latest=false` only after public verification succeeds.
   Record tag, source SHA, workflow/review/native evidence and public artifact
   identities. Do not equate this SDK release with full qualification of a
   consuming notification product.

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
