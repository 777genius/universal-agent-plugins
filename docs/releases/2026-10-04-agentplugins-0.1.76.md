# Agentplugins 0.1.76 release preparation

Owner requested a fresh release on 2026-10-04. The planned current public CLI
version is 0.1.76, distributed as native Agentplugins binaries and the
`universal-agent-plugins` npm facade. Follow [the current release policy](../RELEASE.md)
and [the executable runbook](../agentplugins-release.md). This preparation
record is not evidence that the release has already been published.

## Candidate and checks

The source baseline is `ab0b14af96b922d379ebf7be3182599ecbd3122e`.
Its parent `a9b9258634e5a4696b0d37c675b88bc15b9cd345` passed Required
(including required tests, generated artifacts and vet), CodeQL, Govulncheck,
and both Polyglot Smoke platforms. Its Landing run
[37223740184](https://github.com/777genius/universal-agent-plugins/actions/runs/37223740184)
passed all 225 browser scenarios on attempt 2 after the initial tooltip-focus
failure; no test assertion, retry setting or timeout was changed.
The baseline's only additional change is the owner's README image.

The final candidate is the merged commit of this preparation PR on current main.
Record its exact required CI and security checks, associated PR-head dependency
review, independent technical review and release run receipts before promotion.
Qualify the non-public draft on all six native targets, then reuse the exact
verified-draft run/attempt for promotion without rebuilding approved binaries.
Publish npm from the exact public tag, verify provenance and isolated lifecycle,
and synchronize the Homebrew tap from the same immutable manifest.

The reserved `agentplugins-v0.1.75` tag remains at the earlier parent. Its
non-public qualification run
[37227700639](https://github.com/777genius/universal-agent-plugins/actions/runs/37227700639)
was cancelled after main advanced. It must not be moved or presented as a
published release.

## Change scope and limits

The patch includes installer physical-profile authority and recovery fixes,
OpenCode host and observer lifecycle hardening, notification/hook SDK primitives,
terminal cancellation/input handoff, community catalog expiry fixes and security
dependency updates. New authoring command behavior is not added by this patch.
Preserve the current support boundaries; installer platform proof is not blanket
native-client, SDK or installed Agent Notifications qualification.

No retired plugin-kit-ai CLI channel, PyPI wrapper, future roadmap phase or
legacy capability removal is part of this release. Existing source and historical
artifacts stay preserved.

macOS assets are not signed with Apple Developer ID or notarized by Apple.
GitHub artifact attestations and SHA-256 checksums establish provenance and
integrity, not Gatekeeper acceptance of quarantined browser downloads.
Native platform proof exercises the existing CLI launcher on both macOS
architectures and the isolated install lifecycle on macOS arm64. It does not
prove every external client integration or browser-download experience.

Native downloads must retain `THIRD_PARTY_NOTICES.txt` with the binary and include
it when redistributing.

Related work: [Agent Notifications #105](https://github.com/777genius/agent-notifications/issues/105)
and [Registry #328](https://github.com/777genius/universal-agent-plugins-registry/issues/328).
