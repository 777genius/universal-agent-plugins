# Website Skill implementation evidence

2026-10-24. Extension to PR #473, source-only and unreleased. This evidence does
not qualify native agent execution or an authoring release.

## Lifecycle checks

`TestWebsiteSkillsSavedLifecycleAndColdRefusal` exercises the CLI command path
against a disposable TLS publisher and isolated client roots, with a raw
acquirer injected into the harness. It does not exercise the production
composition. `TestProductionSourceAcquirerAcquiresHTTPSkills` in
`cli/cmd/agentplugins` covers that: it resolves and restores a website Skill
through the acquirer that `composeAgentpluginsApp` returns. Only the native
client runner and security scanner use existing test fixtures.
The extended scenario passed locally in 7.290 seconds:

- Select and install one scoped website Skill, including actual nested resource
  bytes, intent and immutable lock.
- Repeat locked sync and export the agreement.
- Cold import from recorded file coordinates while the discovery index is
  unavailable, verifying the restored nested resource.
- Lose the store, introduce managed drift and run the public repair command
  without rediscovery.
- Explicitly update to changed publisher content while retaining source identity.
- Refuse historical repair and cold import when old file bytes have changed,
  preserving existing state/lock and publishing no new installation on failure.

After the final portability and architecture corrections, root ran the focused
CLI regression with `GOTOOLCHAIN=go1.26.10`: PASS in 12.250 seconds.

```sh
go test ./cli/internal/agentpluginscli -run 'TestSkillsBatch|TestDependencyStagingSeparatesSkill|TestSkillRootRequiresRaw|TestSourceSkills|TestWebsite' -count=1
```

Final adapter package run passed in 25.787 seconds. Environment, manifest
decoder/editor, domain, installer and state-v2 suites passed. The full usecase
suite encountered `TestPublicS1GeminiFailedFirstRetriesCompleteDesiredWithoutAdoptingForeignBytes`
panicking at `public_native_retry_test.go:168` on local macOS. Root reproduced
the exact focused test with the CI Go 1.26.10 toolchain both on this change and
on a separate clean checkout of pre-extension PR head
`7aa99d5a42469e54c15ca7cf9f988bbebf9eef4b`: both panic at the same line because
the fixture has no installation. This existing local failure is not a green
full-suite result or a newly introduced website-source regression.

Root's final Go 1.26.10 focused domain/acquisition/installer website tests passed
(0.468s / 1.131s / 1.187s). Pinned golangci-lint 2.13.2 and Go 1.26.10 report
zero issues for both CLI and agentplugins modules, both changed-line checks against
`origin/main` and whole-file size/complexity/import-boundary checks. The original
oversized acquirer helper was extracted without relaxing lint configuration.

Read-only independent review covered source authority, exact replay, per-client
repair and portable paths. Review and implementation checks found cache
pin-fingerprint omissions, Unicode file/directory aliasing and mixed-case GitHub routing; fixes and narrow
regressions are included. Final review confirmed both fresh and verified
acquisition reject Unicode collisions before file downloads. Installation does
not execute publisher scripts.

## Real publisher acquisition

A temporary read-only harness called the production acquisition adapter and
normalizer over normal public HTTPS. Receipt captured at
2026-10-24T12:42:06.797255Z:

| Publisher / Skill | Result |
| --- | --- |
| `https://uizze.com` / `ui-radar` | Complete listed tree acquired and normalized: 1 file, 4,739 bytes. Raw digest `sha256:bee69e0f63060bbf45e1ff2179b8a907c37e40706e0ac42af6ecc2e27662e150`; normalized digest `sha256:7651a2d497a8546b81826e939cd8606aa239b6651149d997eddae4d21c39ce5f`. |
| `https://open.feishu.cn` / `lark-attendance` | Preferred HTML response correctly fell back to legacy index. Complete listed tree acquired: 1 file, 1,620 bytes; raw digest `sha256:d98d2796275d09b298f1a5ee879a98c6fab0b0ab59e0fa0ac8fd58ebd3d6b333`. Existing strict normalization rejected its metadata with `metadata values must be strings`. This is an observed compatibility limit, not a successful installation. |

Uizze index hash:
`f76981128062950aa13cbafd9ae9e1d824cf0d90f1c59628261adfcf20b960a3`.
Feishu legacy index hash:
`a800fb7803dd9295a249a3db097100c4c7b0ae320cd1deb126bc25b1c66855eb`.
No real client profile or native agent was opened by this acquisition check.

## Delivery boundary

Existing PR #473 and its commits remain the delivery vehicle. This extension
does not merge the PR, publish a CLI package, change the landing command to an
unsupported published `latest`, or claim every catalog entry is compatible.
Discovery artifact format 0.2 and redirects remain explicitly unsupported.

## Completion pass

2026-10-24, on PR #473 head `19f12263` before this pass. Source-only and
unreleased; no merge, installer publication or release was performed.

Defects found by the lifecycle checks and fixed:

- `update` of a website Skill verified the previous pin's bytes first. With an
  empty local store and changed publisher bytes it failed with `skill file
  original-byte SHA256 does not match its locked checksum`. Update no longer
  requires those old bytes; `runSavedEnvironment` still verifies the whole
  candidate, including unchanged pins, before publication. This applies to every
  saved update, not only website Skills.
- Cancellation while reading a Skill file body returned `skill download was
  incomplete` instead of `context.Canceled`. Body-read cancellation now reports
  the context error, as the request path already did.

Checks added or extended:

- `TestWebsiteSkillsSavedLifecycleAndColdRefusal`: locked replay with an empty
  store and changed publisher bytes is refused by checksum, leaving state, lock
  and the installed resource unchanged. A deleted historical resource fails cold
  import with HTTP 404 and publishes no profile or lock.
- `TestWebSkillCancellationDuringFileBodyLeavesNoTree`: cancellation inside the
  body read returns `context.Canceled` and removes the download workspace.
- `TestWebSkillRefusesUntrustedCertificateByDefault`: the default client refuses
  a self-signed publisher certificate.
- `TestWebSkillSourceFactsBoundPinnedFileCount`: 256 pinned files are accepted
  and 257 are refused.

Commands and results (`GOTOOLCHAIN=go1.26.10`):

- `cd cli && go test ./internal/agentpluginscli -run TestWebsiteSkillsSavedLifecycleAndColdRefusal -count=1`: PASS.
- `cd install/integrationctl/agentplugins && go test ./adapters/sourceacquisition -run TestWebSkill -count=5`: PASS.
- `cd cli && go test ./internal/agentpluginscli ./cmd/agentplugins/... -count=1`: PASS (`agentpluginscli` 332.5 s, `cmd/agentplugins` 67.0 s).
- `cd install/integrationctl/agentplugins && go test ./... -count=1`: FAIL in `adapters/clientdetect`, `providers`, `usecase` and `vscodelocalhooks`. The same 50 test failures reproduce on the unmodified base `19f12263` in a temporary worktree. `usecase` also panics in `TestPublicS1GeminiFailedFirstRetriesCompleteDesiredWithoutAdoptingForeignBytes` on this host. The cause of the base failures was not investigated. `cursorhooks` `TestLiteralArgvThroughJSONAndTargetShell` failed once in the full run and passed 3 of 3 reruns; it passed on the base.
- `golangci-lint` v2.13.2 with `--new-from-merge-base=origin/main` on the changed `cli` and `install/integrationctl/agentplugins` packages: 0 issues. The whole-file gate `--enable-only=revive,funlen,gocyclo,gocognit,dupl,depguard` on the same packages: 0 issues.

The real-publisher receipt above predates this pass. Live acquisition was not
re-run after these changes; the changed code does not alter the success path of
acquisition.

## Independent review fix pass

An independent pre-merge review of 4c3667ed returned REQUEST_CHANGES. Fixes:

- Blocker: the production source acquirer did not expose HTTPS Skill methods, so
  website installs failed in the real binary. The wrapper now forwards them, and
  `TestProductionSourceAcquirerAcquiresHTTPSkills` covers the composed acquirer.
- Git option injection: SCP-style hosts and users starting with `-` are refused,
  and Git receives `--` before every remote endpoint in the direct transport.
  `TestNormalizeGitURLRejectsLeadingDashInSCPHostOrUser` covers it.
- Website URL is normalized once, so the default alias and saved identity match.
- One HTTP client per acquisition reuses connections; the per-request timeout
  stays at 30 s and the whole acquisition is bounded at 10 min.
- Files are written through `os.Root` with `O_EXCL`.
- The certificate test asserts `tls.CertificateVerificationError`, not a message.
- The Skill-name rule has one implementation, `domain.ValidSkillName`.

Commands and results (`GOTOOLCHAIN=go1.26.10`):

- `install/integrationctl/agentplugins`: `go test ./domain ./environment ./installer ./sourcespec ./adapters/sourceacquisition ./adapters/dependencymanifest ./adapters/statev2 ./adapters/skillsource ./adapters/environmentstore -count=1`: PASS.
- `cli`: `go test ./cmd/agentplugins ./internal/agentpluginscli -count=1`: PASS (`agentpluginscli` 293.6 s).
- `golangci-lint` v2.13.2 with `--new-from-merge-base=origin/main` on the touched packages in both modules: 0 issues. The whole-file gate `--enable-only=revive,funlen,gocyclo,gocognit,dupl,depguard` on the same packages: 0 issues.

A focused re-review of `cb3e89a5` returned APPROVE. Its four P3 notes are fixed in the next commit:

- `ssh://` hosts and users starting with a dash are refused, not only the SCP form.
- `conformance` delegates the Skill-name check to `domain.ValidSkillName`.
- The `ValidSkillName` comment states the Unicode behaviour.
- The default alias normalizes the publisher URL, covered by `TestWebsiteDefaultAliasUsesCanonicalPublisherURL`.

Final commands (`GOTOOLCHAIN=go1.26.10`):

- `install/integrationctl/agentplugins`: `go test ./adapters/sourceacquisition ./adapters/skillsource ./adapters/dependencymanifest ./adapters/statev2 ./adapters/environmentstore ./conformance ./domain ./environment ./installer ./sourcespec -count=1`: PASS.
- `cli`: `go test ./cmd/agentplugins ./internal/agentpluginscli -count=1`: PASS (`agentpluginscli` 288.3 s).
- Lint and whole-file gate on the touched packages in both modules: 0 issues.
