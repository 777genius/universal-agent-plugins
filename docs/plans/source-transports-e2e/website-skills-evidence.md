# Website Skill implementation evidence

2026-10-24. Extension to PR #473, source-only and unreleased. This evidence does
not qualify native agent execution or an authoring release.

## Lifecycle checks

`TestWebsiteSkillsSavedLifecycleAndColdRefusal` exercises the actual public CLI
composition against a disposable TLS publisher and isolated client roots.
Only the native client runner and security scanner use existing test fixtures.
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
