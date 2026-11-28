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

## Current source qualification checkpoint — R39 repeated prefetch fixture

**Current HEAD:** `fccd72199a1eedc12e3828cf2fdc2f10a2f418f7` ([commit](https://github.com/777genius/universal-agent-plugins/commit/fccd72199a1eedc12e3828cf2fdc2f10a2f418f7)). Only change since R38 `c93d3b98`: a test-only, 3-line substitution in `mixedRootsJourney` using existing `directGitTestRunner{}` for real disposable local Git acquisition, avoiding production process-supervisor scheduling overhead within the prefetch race fixture. This applies the independently reviewed [candidate](https://github.com/777genius/universal-agent-plugins/commit/fe7c6951ebd27260d6adacd63b2b4ff2e09e5901); production runner, protections, test assertions and time budgets are unchanged. The prior exact-source 20x `-race` run [timed out](https://github.com/777genius/universal-agent-plugins/actions/runs/46137655740) before completing the stress repetitions; it was not a PASS.

**Fresh PR workflows:** [Core](https://github.com/777genius/universal-agent-plugins/actions/runs/46139633102), [Required](https://github.com/777genius/universal-agent-plugins/actions/runs/46139633008), [Terminal](https://github.com/777genius/universal-agent-plugins/actions/runs/46139632959), [Package Manager](https://github.com/777genius/universal-agent-plugins/actions/runs/46139633158), [Pi](https://github.com/777genius/universal-agent-plugins/actions/runs/46139633114), [Marketing](https://github.com/777genius/universal-agent-plugins/actions/runs/46139632998). All applicable E2E work must be qualified on this head; initial checks were queued/pending, with no completed source PASS asserted. Previous-head cancellations and skips are not PASS. **Isolated 20x race regression: PASS on exact current PR HEAD.** [Run 46139910373 / job 140963332729](https://github.com/777genius/universal-agent-plugins/actions/runs/46139910373/job/140963332729) checked out `fccd72199a1eedc12e3828cf2fdc2f10a2f418f7` and verified tree `41d82c5eb527f34d16ec2e458194496012bc1c22`, fixture blob `2b5e3d7`, Go 1.26.11, CGO, workspace and formatting. The unchanged `-race -p=2 -count=20 -timeout=12m -json` command produced exactly 20 RUN and 20 PASS events for each of five parent tests and three nested error cases (**160/160**), zero FAIL/SKIP or race warnings, one package PASS in **12.679s**, and nine successful job steps. Decoded-log SHA-256: `5f2bc76445d254bb2654a625f6eead038610eb8575c0d130220962f686239e3c` (209,569 bytes). The earlier successful `fe7c6951` candidate remains separate diagnostic evidence. All Required/Core, cross-platform PM/Pi, Claude/Codex, Windsurf, Marketing and applicable native gates in the acceptance checklist remain outstanding until original logs and artifacts are audited. Then normal expected-head squash and actual merged-main qualification. No release, tag or publication.

### Current R39 evidence

| Gate | Exact current source result |
| --- | --- |
| Isolated repeated prefetch race regression | PASS: exact `fccd7219`, 20 repetitions, 160 RUN/PASS, no failure/skip/race warning. |
| Hermetic staged package | [PASS](https://github.com/777genius/universal-agent-plugins/actions/runs/46139633008/job/140962468717): all seven steps succeeded; one test/pass, zero fail/skip/cancel/TODO; 256.909s total; Node 22.21.1. |
| CLI lint and architecture | [PASS](https://github.com/777genius/universal-agent-plugins/actions/runs/46139633102/job/140962468891): both lint commands reported zero issues; 11 successful steps and four source-authorized conditional skips. |
| Linux contracts | [PASS](https://github.com/777genius/universal-agent-plugins/actions/runs/46139633102/job/140962468819): all 11 steps; 33 scope packages (eight selectors with no matching tests), Cursor 64/64 and Devin 78/78 RUN/PASS, zero failed/skipped test events. |
| Core / Required main / remaining platform contracts / Terminal | Pending original current-source results. |
| PM four hosts / Pi three hosts / native six / full Windsurf and Codex / Marketing | Pending runtime and retained-evidence qualification. |
| Ordinary squash / actual merged-main verification | Pending. |

Current staged decoded-log SHA-256: `acaa415f8fabc5641fa96058ed3af88bfe16c3bcea626c776069d90631bf22b3` (17,829 bytes). The original checkout is verified synthetic `8a1f7d8c87f467e98ca8bfb2ba57fc2a5a16a221`, whose tree equals the exact current source and whose ordered parents are `[ff6c507d, fccd7219]`.

Current CLI decoded-log SHA-256: `185e1302220b9615dd63bc027131b8014c88f2ed91aaa5a3a2cbf6f7da178df4` (86,854 bytes). Current Linux contracts decoded-log SHA-256: `e2dbc33f079e702dcb523b9bacf33f8088478f51db8146b5a485a8b7bcf60e4b` (66,247 bytes). Both original logs check out the same verified `8a1f7d8c` synthetic merge.

### Historical R38 checkpoint (previous source; not current E2E evidence)

## Historical source qualification checkpoint — R38 Copilot cold Project runtime custody

**Current head:** `c93d3b9803a3dd036ac6f05b3787a3b6b51200a0`; tree `d2b10c5ef27715cf5fe8bcea02ee0b7ab33492e6`; ordered parents `6b6a59142742f7f38455e6b9b33da04742e3c89d` and accepted main `ff6c507ddcefd23d41d2f26a6fd0b545ab7b6022`.

Integrates the accepted [main #1147](https://github.com/777genius/universal-agent-plugins/pull/1147) correction for a demonstrated genuine Copilot E2E failure. The [original job](https://github.com/777genius/universal-agent-plugins/actions/runs/46108454822/job/140866391659) provisioned UAP 0.1.116 and Copilot 1.0.105, passed 290 preflight tests with one unrelated explicit Claude-availability skip, and passed add/repeat/update/repair. Cold Project preservation then rejected the newly created pinned Playwright `.bin/playwright-mcp` symlink. Cold refusal, later scopes, cleanup and fresh E were not qualified. Current R37 still had the exact pre-fix runner, so this integration addresses a reproduced blocker.

Exactly **four files, 232 additions / 15 deletions** change. Three files are byte-for-byte accepted-main blobs. The Copilot test file adds exactly the accepted 158 lines while preserving every preexisting branch line, including its equivalent diagnostic-alternative ordering. All 17 prior tests remain; four new methods require complete-image preservation, 16 authority/namespace negatives, two borrowed-document image refusals and seven baseline mutation refusals.

The exception is restricted to the complete cold closure receipts and exact Playwright installation/physical path, signed source/package pins, one nonretained Copilot Project binding, data receipt and owner marker. State and owner bytes must match their hashes in the same protected snapshot. Exact relative links, pinned lock, ordinary targets and captured link identity remain checked; existing changes/deletions refuse first. Default Claude behavior, its private-child `077` setting, public CLI consent and ownership behavior remain intact. Root and independent source/candidate reviews are complete; runtime qualification is pending.

The parent retains the one-file deterministic prefetch fixture and #1146 genuine-Claude private-file correction. Prefetch waits for the maximal reachable window before returning the injected error, retaining every original assertion; the final source must pass the repeated race regression. Genuine Claude must execute the private-file controls and actual lifecycle.

**Qualification blocked for this exact head by the isolated repeated race regression.** Require Core/Required lint and tests, the isolated repeated race regression, corrected macOS Terminal cases and path negatives, PM four PR hosts, Pi three hosts, six Claude/Codex collection lanes, full Windsurf/Codex cohorts, genuine Claude and corrected Copilot Marketing. Earlier-source successes, pilots, skips and queued/cancelled jobs do not qualify this head. Finish with ordinary expected-head squash and actual merged-main Required, PM six hosts, Pi three hosts, Marketing and every applicable push gate. No release, tag or npm publication.

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
| Isolated repeated prefetch race regression | [FAILURE](https://github.com/777genius/universal-agent-plugins/actions/runs/46137655740/job/140956474531): exact current source on the repository-configured Core Linux runner. Beta fixture gate timed out three times; package stopped after 720.037s. Twenty repetitions were not completed. |
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

### Observed prefetch stress failure and isolated candidate

The exact current source passed checkout/tree, Go 1.26.11, CGO, workspace and formatting checks. The unchanged five-test `-race -count=20` command then completed two full cycles with failures and entered a third before the 12-minute package timeout. The failing `skills/beta` case could not observe the fifth callback within the fixture's ten-second budget; admission of that callback requires a previous real Git acquisition and consumer progress. The final stack was in serial acquisition through the process supervisor. No race warning was observed, but this incomplete run provides no stress PASS.

A [three-line candidate](https://github.com/777genius/universal-agent-plugins/commit/fe7c6951ebd27260d6adacd63b2b4ff2e09e5901) reuses the already-existing `directGitTestRunner` for this strictly local scheduling fixture. It retains genuine Git, local source mapping, production acquisition/snapshot checks, the maximal-window gate, every assertion and every timeout. Root and independent review verified the exact +3/-0 change and the runner wiring. The PR ref remains `c93d3b98` until the exact candidate passes isolated prevalidation.

Failed stress decoded UTF-8 log SHA-256: `e78022cc072de42ad39227610f3a7c726215be924870c84e1eb7ac816785950b` (128,632 bytes). Old hosted stress invocations remain separate and carry no execution credit.

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
