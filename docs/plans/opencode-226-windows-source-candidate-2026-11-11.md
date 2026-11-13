# OpenCode 2.2.6 Windows source candidate — non-authorizing

Status: Draft evidence staging, 2026-11-11. Companion to Agent Notifications #643 / issue #642 and merged UAP #823.

## Scope

- Official Windows x64 OpenCode 2.2.6 executable SHA-256: `98cb0aa854c7c015cf6febafa7afb872c359bfe5a823a526f3b4fd347a9e9677`, from genuine Windows 2022 source and installed-denial testing in [AN run #42995343360](https://github.com/777genius/agent-notifications/actions/runs/42995343360) on exact HEAD `a7ccc01e925f7a2af01178d55b3657c926fef659`; upstream tag `dc2ffcf8bf26f9e669b6aa22b95f50e12be15e06`.
- Add a single literal `serve` image to `nativeObserverCandidates()`, using the existing `native_source_pending` status; extend existing fail-closed tests to cover Windows alongside Linux and macOS.
- No observer qualification, independent reader evidence, clock row, granted delivery, version-range inheritance, unrelated install behavior, release or deployment. In particular `EvidenceID` and all `EvidenceHashes` remain empty.

## Acceptance / review

- Existing Go `-race` candidate-contract CI and full applicable CI on exact HEAD pass.
- `BindNativeObserver` rejects even a forged held-process identity; `Select` does not grant 2.2.6 observer capabilities. Generic Skills/MCP remain available.
- This staging does **not** satisfy AN #643 merge criteria: independent Q/R/T/ABI/provenance/clock evidence, positive installed consented desktop/webhook delivery and revocation/replay/restart negatives remain outstanding. Do not publish draft v1.51.3.

## Assessment

Scope ~4 code/test lines, one platform candidate. Complexity 2/10, regression risk 2/10, confidence in fail-closed behavior 9/10 after exact-HEAD CI. Avoid feature flags, new APIs and any old-version evidence reuse.
