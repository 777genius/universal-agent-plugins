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

## Current source qualification checkpoint — R38 Copilot cold Project runtime custody

**Current head:** `c93d3b9803a3dd036ac6f05b3787a3b6b51200a0`; tree `d2b10c5ef27715cf5fe8bcea02ee0b7ab33492e6`; ordered parents `6b6a59142742f7f38455e6b9b33da04742e3c89d` and accepted main `ff6c507ddcefd23d41d2f26a6fd0b545ab7b6022`.

Integrates the accepted [main #1147](https://github.com/777genius/universal-agent-plugins/pull/1147) correction for a demonstrated genuine Copilot E2E failure. The [original job](https://github.com/777genius/universal-agent-plugins/actions/runs/46108454822/job/140866391659) provisioned UAP 0.1.116 and Copilot 1.0.105, passed 290 preflight tests with one unrelated explicit Claude-availability skip, and passed add/repeat/update/repair. Cold Project preservation then rejected the newly created pinned Playwright `.bin/playwright-mcp` symlink. Cold refusal, later scopes, cleanup and fresh E were not qualified. Current R37 still had the exact pre-fix runner, so this integration addresses a reproduced blocker.

Exactly **four files, 232 additions / 15 deletions** change. Three files are byte-for-byte accepted-main blobs. The Copilot test file adds exactly the accepted 158 lines while preserving every preexisting branch line, including its equivalent diagnostic-alternative ordering. All 17 prior tests remain; four new methods require complete-image preservation, 16 authority/namespace negatives, two borrowed-document image refusals and seven baseline mutation refusals.

The exception is restricted to the complete cold closure receipts and exact Playwright installation/physical path, signed source/package pins, one nonretained Copilot Project binding, data receipt and owner marker. State and owner bytes must match their hashes in the same protected snapshot. Exact relative links, pinned lock, ordinary targets and captured link identity remain checked; existing changes/deletions refuse first. Default Claude behavior, its private-child `077` setting, public CLI consent and ownership behavior remain intact. Root and independent source/candidate reviews are complete; runtime qualification is pending.

The parent retains the one-file deterministic prefetch fixture and #1146 genuine-Claude private-file correction. Prefetch waits for the maximal reachable window before returning the injected error, retaining every original assertion; the final source must pass the repeated race regression. Genuine Claude must execute the private-file controls and actual lifecycle.

**Qualification pending for this exact head.** Require Core/Required lint and tests, the isolated repeated race regression, corrected macOS Terminal cases and path negatives, PM four PR hosts, Pi three hosts, six Claude/Codex collection lanes, full Windsurf/Codex cohorts, genuine Claude and corrected Copilot Marketing. Earlier-source successes, pilots, skips and queued/cancelled jobs do not qualify this head. Finish with ordinary expected-head squash and actual merged-main Required, PM six hosts, Pi three hosts, Marketing and every applicable push gate. No release, tag or npm publication.

Current runs: [Core](https://github.com/777genius/universal-agent-plugins/actions/runs/46134103517), [Required](https://github.com/777genius/universal-agent-plugins/actions/runs/46134103481), [Terminal](https://github.com/777genius/universal-agent-plugins/actions/runs/46134103464), [PM](https://github.com/777genius/universal-agent-plugins/actions/runs/46134103369), [Pi](https://github.com/777genius/universal-agent-plugins/actions/runs/46134103409), [Marketing](https://github.com/777genius/universal-agent-plugins/actions/runs/46134103396), [native clients](https://github.com/777genius/universal-agent-plugins/actions/runs/46134103452); other lanes are in [PR checks](https://github.com/777genius/universal-agent-plugins/pull/1130/checks). The [isolated regression](https://github.com/777genius/universal-agent-plugins/actions/runs/46134244976) verifies this exact commit/tree and formatting before running the five affected prefetch tests twenty times with the race detector.

GitHub's actual PR synthetic checkout `8fe1bcd5697132a3660b71a7b38badd2c7372c4e` independently verifies to this exact tree, with parents `[ff6c507d, c93d3b98]`.

## Review and safeguards

Implementation and subsequent reviews covered original requested pins during saved/legacy repair, policy rechecks around lock bookkeeping and group callbacks, SDK error provenance, complete policy-file ancestry, JSON-meaningful empty extensions, original full commit URLs, and archive/Git source selection without weakening explicit source interpretation.

Windows machine-policy discovery uses protected machine registration and supported machine tokens, preserving configured absolute locations and failing closed on discovery errors. It does not depend on caller-supplied machine aliases. Positive macOS local-Skill fixtures resolve their temporary parent before constructing a valid source; path-hazard negatives and policy/removal assertions remain in place.

The five-file #1146 integration received two independent full-file reviews confirming exact main blobs, no feature overlap, ordered parents and the resulting tree. The subsequent one-file prefetch fixture received independent complete-file and commit review; no production change or assertion removal was found. The four-file #1147 integration received independent ownership/default-behavior, original-failure and exact candidate/preservation review. Earlier external review coverage, resolved findings, source checkpoints and observed failures are preserved with their original limitations in the history below.

## Verification

### Current evidence

| Gate | Current source result |
| --- | --- |
| Core tests / coverage | [PASS](https://github.com/777genius/universal-agent-plugins/actions/runs/46134103517/job/140945461369): all ten steps succeeded. Original log has 78 passing primary packages and seven packages without test files; separate informational coverage has 76 passing packages. No cached results or failures in either phase. |
| CLI and architecture lint | [PASS](https://github.com/777genius/universal-agent-plugins/actions/runs/46134103517/job/140945461272): original log and 11 successful steps verified; both lint passes report zero issues. Four conditional Windows/legacy supplement steps were skipped and carry no execution credit. |
| Required main | Pending. |
| Hermetic staged package | [PASS](https://github.com/777genius/universal-agent-plugins/actions/runs/46134103481/job/140945459650): seven successful steps; one test/pass; zero failure/skip/cancel/TODO; 245.864s total; Node 22.21.1. |
| Linux contracts | [PASS](https://github.com/777genius/universal-agent-plugins/actions/runs/46134103517/job/140945461263): all 11 steps succeeded; scoped lifecycle group has 33 successful package results, including eight with no matching tests. Cursor has 64 paired RUN/PASS nodes; Devin/original-profile authority has 78. No recorded failure or skip. |
| macOS/Windows contracts / Terminal fixtures | Pending. |
| Isolated repeated prefetch race regression | [Queued](https://github.com/777genius/universal-agent-plugins/actions/runs/46134244976/job/140945893026) for exact current source; earlier queued invocation remains R37-only. |
| PM four PR hosts / Pi three hosts | Pending runtime and original-artifact qualification. |
| Six Claude/Codex collection lanes | Pending. |
| Full Windsurf and full Codex cohorts | Pending; pilot outcomes do not qualify the full cohorts. |
| Genuine Claude / corrected Copilot Marketing and other applicable gates | Pending. |
| Ordinary squash / actual merged-main qualification | Pending. |

R37 `6b6a5914` had raw-log-qualified CLI lint and staged-package passes, plus successful PM Intel job metadata/logs (original ZIP byte audit pending); these remain historical. R36 `07231cd2` had the scheduler-sensitive Core failure that prompted the preserved fixture correction. Original Copilot failure decoded-log SHA-256: `1f07d0a6161241c270800d7fbe3153bbe0ca41042387a5aa2bbc28d3c15ce0b9`. R37 staged decoded-log SHA-256: `e9207c19f0b2ce09f59f0d97df684dd90396298ff2e5109ccda0b9774f0bd22b`; R37 CLI lint: `779bf5168f2591436090b56e79908efee42668fa3ae17975b80c135aaed4ccb0`.

Six job types are conditionally unexecuted: manual released-CLI control, draft PM preflight, dispatch-only native SDK, two live-Skills jobs requiring their CI label, and Qoder requiring its hosted-runner label. They carry no PASS credit. Conditional steps inside completed jobs are checked separately.

Current Core decoded-log SHA-256: `d458b904534d504d424b766d891da9a40b8b5307c6a98902be9067490d6594d2` (50,589 bytes); Linux contracts: `1a8461a8c60d2b0b456d2aa14a9423059c30ba8ce7e7931613f8279b28b723a3` (66,246 bytes). Both identify verified synthetic checkout `8fe1bcd5`. Coverage is informational in the workflow; the actual original coverage output was independently checked for failures. The separate twenty-repeat race gate is still pending.

Current CLI lint decoded-log SHA-256: `a9b96544e8d3ff086b66e48c67c6524f1f3072a41de528fdd9194b2920d68c94` (86,716 bytes); exact verified synthetic checkout `8fe1bcd5` and main lint base `ff6c507d`.

Current staged-package decoded-log SHA-256: `2035b04d9c29a7e66f6f808365aecebcd8714a9805302a2eaa922fcb70a47e9c`; the original log identifies verified synthetic checkout `8fe1bcd5` and the exact current-source tree.

### Required acceptance

The current checkpoint above identifies the only source being qualified. Require successful final-source Required/Core checks and every applicable platform/native gate, then inspect retained E2E reports, inventories and raw streams against the verified source tree. A successful job with a conditional supplement does not qualify that supplement.

Full package-manager acceptance now requires all 23 phases, 46 phase stream hashes, 140 mandatory Go roots and all six real collection commands; record the actual complete RUN/PASS inventory for each host (the audited historical Intel run had 463 nodes). Preserve source identity, complete mandatory execution, exact policy refusals, repeat/remove behavior and foreign-file ownership. The collection exercise explicitly does not establish native-runtime qualification or borrow a scanner receipt from a separate plugin lifecycle.

Require the full 30-source Windsurf/Firebase audit and dependent full Codex matrices, not only two-case pilots. Genuine Claude/Codex raw-Skill E2E also includes collection install/repeat/remove. Authentication, model-call and runtime claims remain limited to the actual retained observations. After ordinary expected-head squash merge, qualify actual merged-main push workflows, including six-host package-manager, three-host Pi, Required and all other applicable push gates. PR-only workflows retain their final premerge evidence.

### Retained history

The complete preceding public description, including all source-bound historical results, failed attempts, hashes, review limitations and expanded acceptance, is retained in [the qualification history](https://github.com/777genius/universal-agent-plugins/blob/d6568e5389109530c2ece5ee47d457be79e41364/.github/uap1130-evidence/pr-description-07231cd2.md). Its current-source section was a pending snapshot; use the current table above for subsequent outcomes. The [subsequent R36 failure checkpoint](https://github.com/777genius/universal-agent-plugins/blob/ec5f9bbe3269c7b93b46f652768c31b26551c2be/.github/uap1130-evidence/pr-description-07231cd2-failure.md) preserves that complete public description after the actual Core failure (decoded UTF-8 SHA-256 `ff5b1d00cfe0ff098e8be3ee26dc22ba872b30063868ea205ed4752f24e43620`). The complete [R37 public checkpoint](https://github.com/777genius/universal-agent-plugins/blob/bb3ddffa27a47bbbb3b371956a40a6629d4b9a67/.github/uap1130-evidence/pr-description-6b6a5914.md) is also retained (decoded UTF-8 SHA-256 `66edec5265397947e3372b63b624b19987564958e7f7c6cb059f35981d9f9aab`). Historical da317 PM/Pi auxiliary audits remain bound to da317 and do not qualify this head.

## Release Impact

- [ ] No public contract change — this feature deliberately adds policy commands and schema 2. Existing commands, JSON/exit behavior and schema-1 policy compatibility are preserved.
- [x] Documentation updated: configuration/deployment guide, architecture, README and reviewed implementation plan.
- [x] Release-sensitive paths reviewed: CLI composition, SDK facade and installation lifecycle boundaries. This PR does not publish a release or tag.

## Boundaries

Preserve schema-1, public JSON/exit behavior and explicit dependency `--policy` compatibility. No runtime sandbox/control plane, second resolver or installer, feature bypass flag, release/tag, frozen VS Code changes, held #1114/#832 changes, or overlap with #1129 batch-outcome work.
