# BUG-015 — Source proof confuses PR HEAD with test checkout merge SHA

## Reproduced from actual uploaded ZIPs, 2026-11-04

[Main native run 41753370858](https://github.com/777genius/universal-agent-plugins/actions/runs/41753370858) has a 30-case Codex plugin artifact whose `github_sha` is `119109d2328581acfef0fc9858543a062b9bf235`, while the GitHub pull request's actual head when triggered was `8316562d88392142cce8df4bf8dd8b6fa31177c8`. The remaining-client ChatGPT artifact records the same merge-ref SHA. This is normal GitHub Actions `pull_request` checkout behavior, **not** a false CLI test, but ambiguous JSON provenance can mislead a final-head audit.

## Minimal plan

1. Preserve existing `github_sha` as the exact runner checkout SHA. Add a separately named `pr_head_sha`, populated only from the trusted GitHub Actions `pull_request.head.sha` context, to distinguish original branch HEAD from tested PR merge tree.
2. Inject that value through workflow-level non-secret environment for both matrix workflows (pilot, full and remaining). On manual dispatch with no PR, record null, never invent a head.
3. Add lightweight unit/static regression for both workflow injections and SHA format validation. No modification to installation behavior, client-env allowlist or secret propagation.
4. Check resulting JSON ZIP against the run's `head_sha`; require final-head checks and fresh review before merge.

## Critique A — data integrity

A GitHub Actions merge checkout incorporates the base branch and should not be mislabeled as the source branch. Retain both identities; neither proves a case PASS without lifecycle checks.

## Critique B — isolation and scope

The PR HEAD is not a secret and is only needed in the parent harness. Existing child environment allowlists must continue dropping GitHub workflow variables and tokens. No CI workaround, no special dispatch success state and no security-policy change.

**Status:** separate checkout/PR HEAD fields and workflow injection pushed with regression; final-head ZIP verification and independent review still pending.
