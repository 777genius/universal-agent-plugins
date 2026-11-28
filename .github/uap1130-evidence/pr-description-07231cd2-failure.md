## Summary

Teams need consistent installation rules across imperative installation, standalone Skills, declared dependencies, management and the installer facade, with understandable configuration and safe removal after policy tightening.

## What changes

- One shared domain evaluator, narrow installation-policy port, strict file adapter and operation-scoped change detection.
- Machine, user, project and additive policy layers; a lower layer cannot weaken another layer's refusal.
- Source/repository/host, original requested pin, target/scope, component and MCP restrictions.
- Admission before acquisition where source facts are known, before host preparation, across the selected dependency closure and before publication/apply.
- `policy init/validate/status/check`, additive candidate inspection, safe human/JSON diagnostics and a deployment guide.
- Ownership-safe removal, rollback and ordinary read-only validation remain available after policy tightening.

[Plan and review record](https://github.com/777genius/universal-agent-plugins/blob/feat/enterprise-policy-20261127/docs/plans/2026-11-27-enterprise-install-policy.md) · [Policy guide](https://github.com/777genius/universal-agent-plugins/blob/feat/enterprise-policy-20261127/docs/ENTERPRISE_POLICY.md)

Research: [APM enterprise policy](https://microsoft.github.io/apm/enterprise/apm-policy/), [ToolHive registry policy](https://docs.stacklok.com/platform/enterprise-manager/policies/registry), and [non-registry policy](https://docs.stacklok.com/platform/enterprise-manager/policies/non-registry-servers). The implementation reuses UAP's existing source, integrity, ownership and transaction authorities.

## Current source qualification checkpoint — R36 Claude private-file creation

**Current head:** `07231cd2cdf19829cc0139a1c5d7427286404a3e`; tree `9015760d53d052a9a499b1ed3ee852616b492f70`; ordered parents `da317646047e60d13929a16e67f38cf71710ec5d` and accepted main `4e68de5cbf579e28a8304487882e73a17e5f7302`.

Imports exactly the five Python files accepted in [main #1146](https://github.com/777genius/universal-agent-plugins/pull/1146), **89 additions / 4 deletions**. [Original genuine Claude preflight](https://github.com/777genius/universal-agent-plugins/actions/runs/46108454822/job/140866391686) failed with a new passive temporary file at `0644`, while custody requires `0600` (77 tests / one error). The fix requests child-local `umask=077` for the genuine fixture and identity-checked removal observer. Strict custody checks, default probe behavior, limits and cleanup remain intact. Three added inert regressions cover private creation, parent-mask preservation and invalid-mask refusal. Both independent full-file reviews confirmed no feature overlap and exact accepted-main bytes; no local runtime validation is claimed.

GitHub's actual synthetic checkout `113a0fd22ee3947198c7cb1b9e5f8370c4691f70` has this exact tree and parents `[4e68de5c, 07231cd2]`.

**Qualification pending for this exact head.** Require Core/Required lint and tests, corrected macOS Terminal cases and path negatives, PM four PR hosts, Pi three hosts, six Claude/Codex collection lanes, full Windsurf/Codex cohorts and genuine Claude Marketing. Earlier-source successes, pilots, skips and queued/cancelled jobs do not qualify this head. Finish with ordinary expected-head squash and actual merged-main Required, PM six hosts, Pi three hosts, Marketing and every applicable push gate. No release, tag or npm publication.

Current runs: [Core](https://github.com/777genius/universal-agent-plugins/actions/runs/46128583757), [Required](https://github.com/777genius/universal-agent-plugins/actions/runs/46128583727), [Terminal](https://github.com/777genius/universal-agent-plugins/actions/runs/46128583571), [Marketing](https://github.com/777genius/universal-agent-plugins/actions/runs/46128583387); all other lanes are in [PR checks](https://github.com/777genius/universal-agent-plugins/pull/1130/checks).

## Review and safeguards

Implementation and subsequent reviews covered original requested pins during saved/legacy repair, policy rechecks around lock bookkeeping and group callbacks, SDK error provenance, complete policy-file ancestry, JSON-meaningful empty extensions, original full commit URLs, and archive/Git source selection without weakening explicit source interpretation.

Windows machine-policy discovery uses protected machine registration and supported machine tokens, preserving configured absolute locations and failing closed on discovery errors. It does not depend on caller-supplied machine aliases. Positive macOS local-Skill fixtures resolve their temporary parent before constructing a valid source; path-hazard negatives and policy/removal assertions remain in place.

The current five-file #1146 integration received two independent full-file reviews. Both verified exact main blobs, no feature overlap, ordered commit parents and the complete resulting tree. Earlier external review coverage, resolved findings, source checkpoints and observed failures are preserved with their original limitations in the history below.

## Verification

### Current evidence

| Gate | Current source result |
| --- | --- |
| Hermetic staged package | [PASS](https://github.com/777genius/universal-agent-plugins/actions/runs/46128583727/job/140928566176): seven successful steps; one test, zero fail/skip/cancel/TODO; 264.218s; Node22.21.1. |
| Linux contracts | [PASS](https://github.com/777genius/universal-agent-plugins/actions/runs/46128583757/job/140928566146): eleven successful steps. Cursor/Devin verbose suites have 142 RUN/PASS nodes; the initial ownership/lifecycle suite has package-level output. |
| Core test | [FAIL](https://github.com/777genius/universal-agent-plugins/actions/runs/46128583757/job/140928566204): `TestGitRootPrefetchFailingSourceReturnsTheSerialError/tree_other/pc//plugins/pc` reported a root starting after the injected failure. Concurrency/fixture triage is in progress; coverage was skipped. |
| CLI lint | [PASS](https://github.com/777genius/universal-agent-plugins/actions/runs/46128583757/job/140928566257): eleven successful applicable steps, four conditional skips; changed-line and full size/architecture gates both report zero issues. |
| Required main / corrected Terminal macOS | Pending. |
| PM four PR hosts / Pi three hosts | Pending runtime and original-artifact qualification. |
| Six Claude/Codex collection lanes | Pending. |
| Full Windsurf and full Codex cohorts | Pending; pilot outcomes do not qualify the full cohorts. |
| Genuine Claude Marketing and other applicable gates | Pending. |
| Ordinary squash / actual merged-main qualification | Pending. |

Staged decoded-log SHA-256: `c1b47989a9dc7a1242e589451f5ceba4e1aa887ea19b8da4ed2c248fe81b3f22`. Linux-contract decoded-log SHA-256: `275baac86cb316040dc0517851f377c346691f4b9a983212a2062a845d857c03`. Both logs identify synthetic checkout `113a0fd2`, independently verified against tree `9015760d`.

Six jobs are conditionally unexecuted: manual released-CLI control, draft PM preflight, dispatch-only native SDK, two live-Skills jobs requiring their CI label, and Qoder requiring its hosted-runner label. They carry no PASS credit. Conditional steps inside completed jobs are checked separately.

### Required acceptance

The current checkpoint above identifies the only source being qualified. Require successful final-source Required/Core checks and every applicable platform/native gate, then inspect retained E2E reports, inventories and raw streams against the verified source tree. A successful job with a conditional supplement does not qualify that supplement.

Full package-manager acceptance now requires all 23 phases, 46 phase stream hashes, 140 mandatory Go roots and all six real collection commands; the expected raw inventory is 463 RUN/PASS nodes. Preserve source identity, complete mandatory execution, exact policy refusals, repeat/remove behavior and foreign-file ownership. The collection exercise explicitly does not establish native-runtime qualification or borrow a scanner receipt from a separate plugin lifecycle.

Require the full 30-source Windsurf/Firebase audit and dependent full Codex matrices, not only two-case pilots. Genuine Claude/Codex raw-Skill E2E also includes collection install/repeat/remove. Authentication, model-call and runtime claims remain limited to the actual retained observations. After ordinary expected-head squash merge, qualify actual merged-main push workflows, including six-host package-manager, three-host Pi, Required and all other applicable push gates. PR-only workflows retain their final premerge evidence.

Core failure decoded-log SHA-256: `1d144980b8eef4b91e644d2fc80c902c5f45764f51030424aa5ec1012fb72bb1`. CLI-lint decoded-log SHA-256: `1db438071c7f96860d2ce277414e86e4d768a41066edb918e21b27816c24f232`.

### Retained history

The complete preceding public description, including all source-bound historical results, failed attempts, hashes, review limitations and expanded acceptance, is retained in [the qualification history](https://github.com/777genius/universal-agent-plugins/blob/d6568e5389109530c2ece5ee47d457be79e41364/.github/uap1130-evidence/pr-description-07231cd2.md). Its current-source section was a pending snapshot; use the current table above for subsequent outcomes. Historical da317 PM/Pi auxiliary audits remain bound to da317 and do not qualify this head.

## Release Impact

- [ ] No public contract change — this feature deliberately adds policy commands and schema 2. Existing commands, JSON/exit behavior and schema-1 policy compatibility are preserved.
- [x] Documentation updated: configuration/deployment guide, architecture, README and reviewed implementation plan.
- [x] Release-sensitive paths reviewed: CLI composition, SDK facade and installation lifecycle boundaries. This PR does not publish a release or tag.

## Boundaries

Preserve schema-1, public JSON/exit behavior and explicit dependency `--policy` compatibility. No runtime sandbox/control plane, second resolver or installer, feature bypass flag, release/tag, frozen VS Code changes, held #1114/#832 changes, or overlap with #1129 batch-outcome work.
