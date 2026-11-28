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

## Current source qualification checkpoint — R37 deterministic prefetch failure fixture

**Current head:** `6b6a59142742f7f38455e6b9b33da04742e3c89d`; tree `9fabab41bb86cc682ece89c498d9f189811524d8`; parent `07231cd2cdf19829cc0139a1c5d7427286404a3e`; accepted main `4e68de5cbf579e28a8304487882e73a17e5f7302`.

The [previous Core run](https://github.com/777genius/universal-agent-plugins/actions/runs/46128583757/job/140928566204) exposed a scheduler-dependent observation in `TestGitRootPrefetchFailingSourceReturnsTheSerialError/tree_other/pc//plugins/pc`: an already admitted earlier root entered the fake source after its injected failure flag. The production prefetch admits work under its mutex; callback scheduling can occur later. Coverage did not run.

This commit changes only `cli/internal/agentpluginscli/deps_root_prefetch_test.go` (**72 additions / 2 deletions**). After measuring the serial prefix, the fixture waits for the maximal reachable prefetch window to enter the fake source before returning the injected error: five roots for beta and all six for pb/pc. The gate is context-aware and fails explicitly after ten seconds. All original serial-error, single failing read, no new root, bounded extra reads, serial-read coverage and cleanup assertions remain. Production admission/cancellation behavior is unchanged. Root and independent full-file/commit reviews are static evidence; hosted execution remains required.

The parent also retains the reviewed exact five-file [main #1146](https://github.com/777genius/universal-agent-plugins/pull/1146) fix for genuine Claude private-file creation. The original preflight observed `0644` instead of required `0600`; child-local `umask=077` now applies only to the genuine fixture and identity-checked removal observer. Strict custody, default probe behavior, limits and cleanup remain intact. Genuine preflight and all three new regressions must execute on this final source.

**Qualification pending for this exact head.** Require Core/Required lint and tests, the isolated repeated race regression, corrected macOS Terminal cases and path negatives, PM four PR hosts, Pi three hosts, six Claude/Codex collection lanes, full Windsurf/Codex cohorts and genuine Claude Marketing. Earlier-source successes, pilots, skips and queued/cancelled jobs do not qualify this head. Finish with ordinary expected-head squash and actual merged-main Required, PM six hosts, Pi three hosts, Marketing and every applicable push gate. No release, tag or npm publication.

Current runs: [Core](https://github.com/777genius/universal-agent-plugins/actions/runs/46132874600), [Required](https://github.com/777genius/universal-agent-plugins/actions/runs/46132874580), [Terminal](https://github.com/777genius/universal-agent-plugins/actions/runs/46132874572), [PM](https://github.com/777genius/universal-agent-plugins/actions/runs/46132874614), [Pi](https://github.com/777genius/universal-agent-plugins/actions/runs/46132874548), [Marketing](https://github.com/777genius/universal-agent-plugins/actions/runs/46132874622), [native clients](https://github.com/777genius/universal-agent-plugins/actions/runs/46132874610); every other lane is in [PR checks](https://github.com/777genius/universal-agent-plugins/pull/1130/checks). The isolated regression also verifies the exact source commit/tree and formatting before running the five affected prefetch tests twenty times with the race detector.

## Review and safeguards

Implementation and subsequent reviews covered original requested pins during saved/legacy repair, policy rechecks around lock bookkeeping and group callbacks, SDK error provenance, complete policy-file ancestry, JSON-meaningful empty extensions, original full commit URLs, and archive/Git source selection without weakening explicit source interpretation.

Windows machine-policy discovery uses protected machine registration and supported machine tokens, preserving configured absolute locations and failing closed on discovery errors. It does not depend on caller-supplied machine aliases. Positive macOS local-Skill fixtures resolve their temporary parent before constructing a valid source; path-hazard negatives and policy/removal assertions remain in place.

The five-file #1146 integration received two independent full-file reviews confirming exact main blobs, no feature overlap, ordered parents and the resulting tree. The subsequent one-file prefetch fixture received independent complete-file and commit review; no production change or assertion removal was found. Earlier external review coverage, resolved findings, source checkpoints and observed failures are preserved with their original limitations in the history below.

## Verification

### Current evidence

| Gate | Current source result |
| --- | --- |
| Core tests / coverage | Pending. |
| CLI and architecture lint | [PASS](https://github.com/777genius/universal-agent-plugins/actions/runs/46132874600/job/140941709318): eleven successful applicable steps, four conditional skips; both actual lint commands report zero issues. |
| Required main / hermetic staged package | Pending. |
| Linux/macOS/Windows contracts / Terminal fixtures | Pending. |
| Isolated repeated prefetch race regression | [Queued](https://github.com/777genius/universal-agent-plugins/actions/runs/46133308292); immutable source verification and five tests × twenty race-detector repetitions. No runtime result yet. |
| PM four PR hosts / Pi three hosts | Pending runtime and original-artifact qualification. |
| Six Claude/Codex collection lanes | Pending. |
| Full Windsurf and full Codex cohorts | Pending; pilot outcomes do not qualify the full cohorts. |
| Genuine Claude Marketing and other applicable gates | Pending. |
| Ordinary squash / actual merged-main qualification | Pending. |

The previous head `07231cd2` had raw-log-qualified staged package, Linux contracts and CLI lint passes, but Core failed. None is credited to this head. Previous Core decoded-log SHA-256: `1d144980b8eef4b91e644d2fc80c902c5f45764f51030424aa5ec1012fb72bb1`. The failure and the fixture correction remain part of the qualification record.

Six job types are conditionally unexecuted: manual released-CLI control, draft PM preflight, dispatch-only native SDK, two live-Skills jobs requiring their CI label, and Qoder requiring its hosted-runner label. They carry no PASS credit. Conditional steps inside completed jobs are checked separately.

Current CLI-lint decoded-log SHA-256: `779bf5168f2591436090b56e79908efee42668fa3ae17975b80c135aaed4ccb0`. The log identifies synthetic checkout `79acb56ae9a216d22aa9b276c36e677af11df996`, independently verified against final-source tree `9fabab41`.

### Required acceptance

The current checkpoint above identifies the only source being qualified. Require successful final-source Required/Core checks and every applicable platform/native gate, then inspect retained E2E reports, inventories and raw streams against the verified source tree. A successful job with a conditional supplement does not qualify that supplement.

Full package-manager acceptance now requires all 23 phases, 46 phase stream hashes, 140 mandatory Go roots and all six real collection commands; record the actual complete RUN/PASS inventory for each host (the audited historical Intel run had 463 nodes). Preserve source identity, complete mandatory execution, exact policy refusals, repeat/remove behavior and foreign-file ownership. The collection exercise explicitly does not establish native-runtime qualification or borrow a scanner receipt from a separate plugin lifecycle.

Require the full 30-source Windsurf/Firebase audit and dependent full Codex matrices, not only two-case pilots. Genuine Claude/Codex raw-Skill E2E also includes collection install/repeat/remove. Authentication, model-call and runtime claims remain limited to the actual retained observations. After ordinary expected-head squash merge, qualify actual merged-main push workflows, including six-host package-manager, three-host Pi, Required and all other applicable push gates. PR-only workflows retain their final premerge evidence.

### Retained history

The complete preceding public description, including all source-bound historical results, failed attempts, hashes, review limitations and expanded acceptance, is retained in [the qualification history](https://github.com/777genius/universal-agent-plugins/blob/d6568e5389109530c2ece5ee47d457be79e41364/.github/uap1130-evidence/pr-description-07231cd2.md). Its current-source section was a pending snapshot; use the current table above for subsequent outcomes. The [subsequent R36 failure checkpoint](https://github.com/777genius/universal-agent-plugins/blob/ec5f9bbe3269c7b93b46f652768c31b26551c2be/.github/uap1130-evidence/pr-description-07231cd2-failure.md) preserves that complete public description after the actual Core failure (decoded UTF-8 SHA-256 `ff5b1d00cfe0ff098e8be3ee26dc22ba872b30063868ea205ed4752f24e43620`). Historical da317 PM/Pi auxiliary audits remain bound to da317 and do not qualify this head.

## Release Impact

- [ ] No public contract change — this feature deliberately adds policy commands and schema 2. Existing commands, JSON/exit behavior and schema-1 policy compatibility are preserved.
- [x] Documentation updated: configuration/deployment guide, architecture, README and reviewed implementation plan.
- [x] Release-sensitive paths reviewed: CLI composition, SDK facade and installation lifecycle boundaries. This PR does not publish a release or tag.

## Boundaries

Preserve schema-1, public JSON/exit behavior and explicit dependency `--policy` compatibility. No runtime sandbox/control plane, second resolver or installer, feature bypass flag, release/tag, frozen VS Code changes, held #1114/#832 changes, or overlap with #1129 batch-outcome work.
