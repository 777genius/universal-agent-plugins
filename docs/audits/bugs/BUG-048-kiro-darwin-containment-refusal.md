# BUG-048 — Kiro Darwin host containment refusal misclassified as generic install failure

## Reproduced source evidence

[Live Kiro 3.0.0 Plugin JSON ZIP #13373676493](https://github.com/777genius/universal-agent-plugins/actions/runs/41934442984/artifacts/13373676493), on exact PR source HEAD `c45fd70ed4239a618d3e96d874e84b384eef2971`, contains 30 distinct native-source cases: 1 real lifecycle PASS, 5 installed/native-unqualified, 1 unsupported distribution, and 23 **nonzero** native MCP preflight refusals. The 23 refusals all report `manual_activation_required: automatic native Kiro MCP lifecycle containment preflight failed: safe duplex process containment unavailable on Darwin; manual activation required`. The observed client binary is genuine pinned Kiro CLI 3.0.0, with a reviewed Homebrew SHA-256 receipt.

The actual refusal is enforced in `install/integrationctl/agentplugins/adapters/process/osrunner_tree_darwin.go` via Kiro's existing `DuplexCapability` preflight. It prevents unsafe automatic child-process execution. No install registration or foreign-file mutation is recorded in these refused cases.

The current E2E harness `scripts/live-install-audit.py` recognizes `host-containment-unavailable` only when the diagnostic contains `cgroup` (Linux). On macOS it instead emits generic `install-failed`, erroneously suggesting a broken source or installer. **Both are non-PASS statuses.** Neither is a reason to disable the safety guard.

## Minimal plan and implementation

1. Reuse the existing fail-closed `is_cli_refusal` (nonzero integer exit). Recognize the exact established `safe duplex process containment unavailable` phrase in addition to the previous `cgroup` condition, always requiring `manual_activation_required`.
2. Add a no-network end-to-end harness regression for the actual Kiro error, old Linux cgroup case, and negative controls missing either signal or the nonzero exit. Require no installed identity, no residue and no classification as PASS.
3. Change only the evidence classifier and focused test; do not alter native Kiro adapters, process supervision, auto-activation policy, passwords or pinned Kiro archive.
4. Verify focused Python tests, both required and live E2E matrices on one final SHA. The queued Kiro Skills ZIP from the previous SHA is historical once this patch lands. Never promote documented unavailable/unsupported cases to PASS.

## Two critical reviews

- **Security/scope:** an unavailable containment capability means automatic native MCP execution is unsafe. Reporting an explicit host limitation is truthful only when the native CLI actually refused and emitted the exact known message. Do not infer success, bypass cgroup/duplex custody, or classify unrelated errors as expected.
- **Correctness/evidence:** preserve the independent source, repeat-add, removal, state and foreign-neighbor oracles. This patch changes **classification only**, not the binary, source distribution or outcome, and does not establish active paid-model use.

## Status

Reproduced from an original current-commit ZIP and source-verified Darwin guard. Regression/source correction prepared for PR #698; final-HEAD CI and independent review qualification remain pending.
