# BUG-014 — Native CLI bootstrap result missing from full-matrix evidence

## Reproduction (2026-11-04)

[Full native CI on PR HEAD `8316562d88392142cce8df4bf8dd8b6fa31177c8`](https://github.com/777genius/universal-agent-plugins/actions/runs/41753370858) runs the main nine-client matrix with an install/version probe step, but its later audit step does not receive that step's outcome. The evidence harness defaults an absent `AUDIT_VENDOR_OUTCOME` to success. If a native binary exists while its `--version` check fails or times out, subsequent per-source JSON can improperly claim PASS despite the failed bootstrap. This is a harness provenance problem, not an installer defect. CodeRabbit independently flagged the same counterexample in [review](https://github.com/777genius/universal-agent-plugins/pull/698#discussion_r4462505036).

The exact-head [Required run 41753370411](https://github.com/777genius/universal-agent-plugins/actions/runs/41753370411) also reproduces an unrelated check-removed-contract documentation false-positive: its scanner matches a historical literal API identifier inside the BUG-011 explanatory text. The product check itself correctly remains strict.

## Minimal plan (before code)

1. Name the main matrix native-bootstrap step and pass its actual GitHub Actions `outcome` into the source-evidence step, exactly like the remaining-client matrix. Keep the existing negative-evidence behavior if bootstrap fails, but refuse PASS on the failed native bootstrap. Do not hide failed jobs.
2. Extend the existing workflow guard unit regression to require this binding in both matrices. Do not alter the runtime's client-version or signed-release rules and do not change the default for independent published/pilot steps which do not use this matrix.
3. Rewrite only the historical documentation line that triggered the removed-contract scanner, keeping product scanning intact. Repair the audit ledger's split Markdown defect table and record BUG-013/014.
4. Run the actual Python guard and all applicable fresh-head CI/E2E; inspect JSON artifacts. No merge without final-head qualification and independent review.

## Critique round A — security and proof

A native binary's presence or `--version` output cannot replace a successful bootstrap step. The source harness needs a workflow-verified outcome, not a user-supplied successful default in the main matrix. Even on failure, retain source-by-source negative rows; never count them PASS. Do not loosen vendor SHA review, cgroup or LintAI policies.

## Critique round B — regression and scope

Use exactly the existing GitHub Actions `steps.<id>.outcome` binding in both workflow matrices, and a static test that fails if that link disappears. Preserve actual failed native job conclusions. The removed-API guard continues checking every existing product location; only paraphrase the historical bug document. Formatting changes must not influence classification.

**Status:** reproduced, plan recorded; fixes and final-head evidence pending.
