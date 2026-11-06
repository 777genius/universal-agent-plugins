# BUG-009 — Signed Directory identity could pass without immutable release proof

## Reproduction / evidence (2026-11-04, source plus real historical list JSON)

`scripts/live-install-audit.py::matching_installations` checks `directory.product_id` for a signed alias (or `directory.recorded_distribution` for explicit `777genius/chrome-devtools-bridge`) but does not validate that the matching client's `package_revision` contains the recorded signed distribution/release sequence, resolved Git revision, or SHA-256 tree/manifest digests. A malformed synthetic `list` containing the correct alias, verified client, and directory product name but **no revision metadata** would be accepted by this matching function. That could falsely establish part of an E2E PASS if native file changes and cleanup also matched.

Original live [Codex plugin artifact from run 41727674760](https://github.com/777genius/universal-agent-plugins/actions/runs/41727674760) shows real signed aliases include the required fields: `directory.recorded_revision`, `directory.recorded_release_sequence`, `directory.recorded_distribution`, and client `package_revision` with `resolved_revision`, `distribution_id`, `release_sequence`, `tree_digest`, `manifest_digest`. All 20 signed entries inspectable in that artifact's shortened stdout matched these fields; one very long response cannot be reconstructed from the 1,800-character tail (the live harness itself separately parses full JSON).

This is an **audit proof weakness**, not evidence that the production Directory has emitted a mismatched release.

## Minimal correction

1. Add one small helper that checks the selected client's signed release metadata against the directory's *recorded* distribution, revision and sequence, plus valid nonempty SHA-256 digests. Require this for signed aliases; keep Git-pinned external packages on their existing exact commit path.
2. Add a subprocess-free regression with an exact-format historical signed `list` skeleton: accept a complete match; reject missing/invalid digests, wrong revision, distribution ID, sequence, or wrong target. Preserve `prepared-lifecycle-passed` and `manual-activation-required` distinctions.
3. Verify the final-head real 30-plugin matrix and artifact identity. If a vendor omits these fields legitimately, classify as incomplete evidence and investigate; never silently count PASS.

## Plan critique

- **Integrity critic:** Product name alone is not the selected signed artifact. The E2E proof must bind recorded distribution to the exact resolved immutable revision and cryptographic digests.
- **Scope critic:** Validate metadata already present in real CLI `list`; do not introduce a new registry API, third-party signer, trust bypass, or a production data-model migration.

**State:** exact signed-release binding and verified-client regressions pushed in PR #698; final-head native artifact inspection and CI pending.
