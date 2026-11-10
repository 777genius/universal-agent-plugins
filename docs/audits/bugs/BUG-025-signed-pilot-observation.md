# BUG-025 — signed release mismatch lacks actionable provenance in pilot output

## Real reproduction

On exact branch HEAD `d5283e482c7a00d4b6366ee1c1d990a373d69a6c`, GitHub Actions [#41783837241](https://github.com/777genius/universal-agent-plugins/actions/runs/41783837241), Codex plugin pilot ran 33 Python regressions successfully but both real signed alias installs returned `incomplete-native-proof`. Because the first two-case pilot is a safety prerequisite for the full plugin matrix, all full installer jobs were skipped. Current logs reveal only status, not which immutable signed release source/revision/sequence/digests were observed. **This is a reproduced evidence-diagnostic gap; the source mismatch itself is not automatically a product bug.**

## Narrow plan

1. For signed aliases only, record strictly allowlisted fields from the actual `list` response: source selector, directory recorded distribution/revision/sequence, and exact audited client's recorded revision and SHA-256 digests. Do not retain auth, full configs or raw CLI output.
2. For any signed alias `incomplete-native-proof`, print these allowlisted fields together with the independent frozen Registry pin in the CI case status line. Preserve full JSON audit reports. All negative statuses remain negative.
3. Use new real pilot output to distinguish a benign selector-format difference, a genuinely drifted signed release or missing provenance. Fix only a reproduced mismatch without relaxing immutable source proof; rerun final-head CI and ZIP coverage.

## Critical review round 1 — secrets/safety

Log only public source identifiers and immutable hashes. No API keys, URLs with userinfo, full `list` JSON, secrets or ambient profiles.

## Critical review round 2 — truthfulness/scope

No PASS rule changes, no bootstrap bypass, no mutable-pin substitution. Diagnostics must not mark a new release eligible; final-head Required, both native workflows and independent reviews remain gating.

## Status

Plan recorded ahead of scoped logging fix. The current Codex plugin pilot failure remains unqualified until new provenance is inspected.
