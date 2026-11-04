# BUG-012 — Audit workflow trusts unverified vendor bootstrap

## Reproduction and independent review

[Strix review finding on PR #698](https://github.com/777genius/universal-agent-plugins/pull/698) reported CWE-494 in `.github/workflows/live-install-remaining-clients.yml`: unpinned `curl` scripts were executed in a runner, mutable .deb downloads received root-level install scripts, and `$HOME/.local/bin` preceded runner tools in the evidence PATH. This is a source-backed integrity problem, not proof of an unrelated UAP installer defect. The review thread remains open until independently rechecked.

## Scoped plan

1. Refuse running vendor-provided code until the **exact downloaded bytes** match a repository-reviewed SHA-256. Never treat a computed hash of an unreviewed live download as a security pin. Explicitly report missing digests as vendor-bootstrap blockers; do not turn them into PASS.
2. Append vendor bins *after* runner-controlled system tool paths. For vendor `.deb`, extract unprivileged instead of running root maintainer scripts. Pass the bootstrap step's observed outcome to the case runner; even if an unrelated vendor-named executable is preinstalled on the runner, a failed digest/provenance check must emit 30 `client-unavailable` rows and zero native PASS claims.
3. Download but never execute unreviewed vendor bytes; record their observed SHA-256 in `vendor-proof/observed-untrusted.sha256` so independent reviewers can inspect the candidate. Use `shasum -a 256` on macOS if GNU `sha256sum` is absent. Preserve negative-path artifact and guard-test the workflow. Only add real immutable SHA values after independent source review and successful real downloads; until then five vendor lanes cannot qualify.

## Two critical reviews

- **Supply-chain critic:** Simply recording `sha256sum` after download does not verify trust. An unknown digest must remain an external infrastructure/security blocker, not a path to execution or PASS. A review of provenance and actual pinned bytes is still missing.
- **Native compatibility critic:** Non-root extraction of a .deb is not equivalent to an installed desktop client. Verify an executable and its actual version before permitting source-level PASS; if the wrapper does not work from extraction, report unavailable. Do not restore `sudo apt` for green CI.

**State:** fail-closed remediation committed; official immutable artifact digests, vendor-version proof, independent Strix clearance, and final-head 30-source artifacts remain pending. No merge authorized.
