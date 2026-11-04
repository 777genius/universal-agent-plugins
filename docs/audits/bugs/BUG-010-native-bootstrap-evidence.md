# BUG-010 — Native CLI bootstrap failure drops the 30-source evidence report

## Reproduction (source-reviewed GitHub Actions workflow)

In `.github/workflows/live-install-audit.yml`'s `real-native-cli-matrix`, a failing `npm install --global` or native-client `--version` step makes the job fail, so subsequent `go build` and `python3 scripts/live-install-audit.py` are skipped under GitHub Actions' implicit `success()` step guard. The artifact uploader has `if: always()`, but `full-proof/` does not exist. Consequently, **none of the intended 30 per-source outcomes are recorded** when one of the nine client binary installations fails. The existing remaining-six workflow has a vendor bootstrap `continue-on-error` path and records explicit `client-unavailable` instead. This is a missing audit-evidence path, not proof of a UAP product defect.

## Minimal plan

1. In this one workflow/job, make `Build PR source CLI` and `Attempt all 30` eligible to execute after a failed native-bootstrap step, provided the job was not cancelled (`if: ${{ !cancelled() }}`). Leave the bootstrap step itself **failing**, not `continue-on-error`: the job remains red and cannot satisfy merge gating.
2. With no genuine client executable, the real audit's existing `EXECUTABLE` preflight already writes one explicit `client-unavailable` outcome for each of the 30 selected sources and exits nonzero; do not fake a native binary or treat the output as PASS. Preserve the current upload's `if: always()`. If Go checkout/build itself fails, the job stays failed; do not synthesize a successful CLI execution.
3. Keep the two-source Codex native pilot as a separate gate; do not remove existing CI cost controls or alter product installation behavior. Check actual failure-path artifacts on a real failing runner and exact-head CI before calling complete.

## Plan critique

- **False evidence critic:** An uploaded empty or absent artifact is not a source-by-source test. Recover an explicit 30-row `client-unavailable` result without hiding the bootstrap's red status.
- **Infrastructure critic:** Avoid `continue-on-error` on the critical vendor bootstrap and avoid starting work on a canceled job. This is a two-line workflow-only control-flow fix, not a new retry framework.

**State:** minimal two-step workflow correction committed; real failed-bootstrap artifact and final-head CI verification still pending. The parent `needs: native-pilot` remains a separate gate: matrix jobs are not launched if the pilot fails.
