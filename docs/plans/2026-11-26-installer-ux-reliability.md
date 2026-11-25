# Installer reliability and console UX: implementation plan

Status: **implementation plan; production changes and final E2E pending**.
Owner authorization: 26 November 2026 (Europe/Kiev).
Base: `ce789138a393bffd7e6d001c0e4f8aec05e47773`.
Delivery: one Draft PR, immediate publication of each cohesive commit, repeated
independent review and fixes, required E2E, ordinary merge and merged-main check.
The MCP probe / Skill-resource plan in #1114 remains deferred.

## 1. Outcome and scope

The user can see what will be installed and where, make an informed choice,
understand every failed or unattempted binding, and recover without accidental
extra installation. The same contract must hold in rich/plain terminals,
redirected stdout, large catalogs and browser-to-terminal fallback.

This work extends the existing installer, client adapters, prompt contracts,
reporting and terminal harness. It does not introduce another installer, state
store, retry engine, terminal framework, diagnostic command or permission model.

### Included

1. Truthful per-agent outcomes for unchanged actions, early refusals, mixed
   outcomes and cancellation after native mutation.
2. Useful failure diagnostics when writing the technical log fails.
3. Consistent interaction output and explicit presentation-mode handling.
4. Available and unavailable agent explanations before selection.
5. Fully browsable large plain Skills catalogs, retained selection basket,
   bounded explanations for omitted candidates and complete review identity.
6. Atomic browser-to-terminal handoff that cannot cancel or duplicate accepted
   installation work, with bounded opener execution.
7. Public-process Skills installation and terminal qualification using existing
   fixtures, production assets, real pipes/PTYS and durable-state oracles.

### Excluded

- Deferred MCP runtime probe, Skill-reference analysis, setup/secret wizard.
- Native client policy/approval changes, new clients, frozen VS Code.
- Acquisition-policy weakening, TLS bypass, ambient credentials or paid models.
- A visual redesign, browser mobile-tab polish or unrelated website changes.
- Public release/tag/npm publication; merge is the authorized delivery boundary.
- Reimplementation of ownership diagnosis, retry or progress already in main.

## 2. Evidence and overlap

Four diagnostic tests fail on unchanged main for the intended behavior:
`TestUXAuditPlainSkillsDoesNotAutomaticallyChooseBrowser`,
`TestUXAuditSkillsReviewUsesAlreadySelectedVisibleStream`,
`TestUXAuditSkillsNoDetectedTargetsHasActionableDiagnosis`, and
`TestUXAuditRepeatedSkillReportsPrepublicationFailure`.
They were executed with repository Go 1.26.11.

The suspected HTTPS retry-content vulnerability was **withdrawn** after real
fixture checks: a fresh retry re-reviews a changed destination, while retained
installation identity rejects changed revision through the existing invariant.
Preserve those passing controls; do not add a new authority mechanism.

Additional inspected gaps:
- Failed binding with native OutcomeCompleted can be rendered completed.
- Failed logging can remove the last useful explanation.
- Numeric catalog exposes only its first 50 entries without paging.
- Unavailable candidates and target reasons disappear from terminal flows.
- SourceMoved details and distinguishing long path suffixes can be lost.
- A browser operation can be admitted while terminal-fallback consent is open.

Already merged: #1104 diagnosis; #1017 result/native setup clarity; #1086 target
choice explanations. Active adjacent owners: #1087 retained/ownership diagnosis,
#1071 Cursor approval, #1023 native surfaces, #1093 public-binary/npm lifecycle,
#1078 live native audit. Exact candidate Skills source files were compared with
#1087/#1071/#1093 heads and are identical to main. Keep those owners' policies,
audit files and regression guarantees intact. Recheck current main before merge.

## 3. Estimate and quality target

Added/substantially changed physical lines, excluding dependencies, generated
assets, lockfile churn and unchanged fixtures. These are estimates, not reasons
to duplicate code or tests.

| Work | Production / integration LOC | Tests / E2E LOC |
| --- | ---: | ---: |
| Outcome and diagnostic correctness | 450–750 | 950–1,400 |
| Console selection and review usability | 450–700 | 750–1,100 |
| Safe browser-to-terminal handoff | 180–300 | 350–550 |
| Public-process qualification and existing CI wiring | 60–100 | 1,600–2,200 |
| Total | 1,140–1,850 | 3,650–5,250 |

Plan, testing documentation and necessary help: 310–750 lines.
Total estimate: **5,100–7,850 lines**, working target about 6,500.
Usefulness 9/10; complexity 6/10; estimate confidence 7/10.
Target reliability after mandatory evidence: 8/10, not a current qualification.

## 4. Outcome and diagnostic correctness

### Preserve actual execution facts

- Build Skills result items from exact observed binding outcomes first.
- For reviewed actions not reached because required checks failed, preserve
  the classified apply-level cause, including planned unchanged actions.
- A verified successful unchanged action remains unchanged. Do not erase an
  already successful binding when a later independent binding fails.
- BindingFailed cannot become completed because its native result carries
  OutcomeCompleted. Preserve native facts, manual actions and mutation evidence
  separately; partial batch status must remain truthful.
- Early refusal with reviewed actions must leave enough per-agent information
  to explain what did not run.
- Report normalization must not count failed/canceled/not_attempted items as
  successful merely because optional failure metadata is absent.
- Preserve errors.Is/As, public JSON shape and exit conventions. Correcting an
  incorrect status value is a bug fix; it must have an explicit regression.

### Diagnostics without a writable log

Use one small internal diagnostic result containing saved path and write error.
A logging failure never changes installation effects or the primary outcome.

When recording is disabled or unavailable, retain a useful classified cause.
If attempted recording fails, say that the diagnostic file could not be saved.
Never advise opening a nonexistent file. Reuse existing redaction and safe-text
policies; do not print an arbitrary raw error chain. Bound diagnostic detail,
remove terminal control effects and prevent credentials/URLs with secrets from
appearing. Preserve the typed primary cause when rendering also fails.

Integrate the same behavior into Skills, dependency install/update/repair,
group and single-install results. Recovery and unconfirmed process cleanup
continue to prevent every retry in the affected namespace.

## 5. Console choices, review and consent

### One visible interaction stream

- --plain disables automatic browser selection; explicit --ui keeps precedence.
- Numeric catalog, validation messages, target choice explanations, full review,
  oversized-review fallback, changed-retry review and confirmation acknowledge
  use the already selected visible interaction stream.
- Keep JSON, noninteractive results and dry-run data on their existing stream.
- A short/failed review write stops before consent/apply. Reuse planWriter and
  the current PromptFactory/reviewWriter/promptio contracts.
- Preserve bounded, non-prefetching reads and fresh confirmation input. No
  custom raw-mode management or goroutine input reader.

### Targets and catalogs

- Project existing safe target Reason into the existing Skipped contract.
- Empty eligible-target list gets actionable grouped reasons before invoking
  a selector. Unavailable entries never become selectable.
- Plain catalog gets next/previous pages and selected-only view. Existing
  filter and canonical basket remain the source of selection identity.
- Query changes reset the page; basket survives filters/pages; page numbers
  refer to displayed entries only. Cancel/EOF never means acceptance.
- Bound candidate omission summaries and reuse skillDiagnosticGuidance.
  Include incomplete-scan guidance and exact-path narrowing where applicable.
- Preserve SourceMoved detail, full sanitized destination/source paths and
  existing reasons in review. Navigation labels may truncate, authority may not.
- Preserve distinguishing path suffixes in rich navigation without changing IDs,
  selection validation or existing display/normalization budgets.

## 6. Browser-to-terminal handoff

Use the current server mutex and session lifetime. Atomically decide whether an
unused session can be closed to future admission. If preparation or an accepted
operation has started, refuse terminal takeover and continue observing that
browser operation. Never cancel accepted work because a fallback prompt was
answered later. A check followed by unguarded Close is insufficient.

Bound the OS opener invocation, honor cancellation, check output failures and
use a small private launcher seam for deterministic tests. Preserve session
capability secrecy and existing HTTP/JSON schemas. Tests must prove at most one
accepted execution path when apply and fallback race.

## 7. End-to-end evidence

Extend scripts/terminal-ui and its existing Fixture, Session, state hashes and
evidence format; do not introduce another test runner. Exact commands and
assertions are versioned alongside the implementation.

| User scenario | Required assertion |
| --- | --- |
| Fresh multiple-agent Skills install | Reviewed selection, delivered bytes, saved intent/lock, receipts and native inventory agree |
| Successful unchanged repeat | No native mutation, durable bytes unchanged, truthful current observation |
| Unchanged repeat with pre-apply failure | Non-success agent result, safe cause, log/fallback and correct retry eligibility |
| Partial apply and declined retry | Healthy agent stays installed; failed agent explained; no extra effects |
| Partial apply and accepted retry | Only eligible failed agents retried; success not replayed |
| Recovery / cleanup refusal | No retry prompt/command; original typed failure and state retained |
| Redirected stdout | Catalog and complete review visible before consent; result/JSON streams preserved |
| Plain with bundled browser assets | Automatic browser not launched; explicit --ui remains intentional |
| Empty/incomplete/large catalog | Reasons, pages, stable basket and cancellation work |
| No eligible target | Reasons visible; selector/confirmation/apply not entered |
| Declined review / EOF / Ctrl+C | Correct exit and actual durable state; same terminal descriptor usable |
| Failed review output | No apply; stream error retained |
| JSON partial / unchanged / cancellation | One envelope, correct per-item state, no interactive prompt |
| Browser/fallback race | Single admission and accepted operation preserved |

Actual public binary must include production picker assets. A development build
with Bundled()==false cannot prove the plain/browser-routing fix. Build with
the existing npm ci + bundle:go sequence (TypeScript 7.0.2 or newer), then test
the actual resulting CLI hash. Never hand-edit generated assets.

Public source constraints must be proven first. Skills sources are GitHub/HTTPS,
not arbitrary local folders. For deterministic Unix HTTPS fixtures, use a valid
local CA through standard SSL_CERT_FILE only if actual production acquisition
accepts the endpoint; TLS verification stays enabled. No policy test bypass.
If this does not work, use existing pinned public source or verified HTTPS Git
fixture mechanisms. Windows trust/ConPTY needs its own supported proof; label
production-composed Go tests and actual-binary cases separately. Do not claim
unexecuted Windows source coverage.

Keep actual command, SHA, binary hash, fixture classification, exit, raw output,
state/native snapshots and terminal restoration evidence. Skipped/unsupported,
queued and cancelled are not PASS. Use private disposable profiles and synthetic
protocol responders, with no real secrets or model turns.

## 8. Work ownership and sequence

1. Publish this plan in the new Draft PR; run two independent plan critique
   rounds and incorporate findings before production edits.
2. Prove public Skills source fixture and bundled-binary test feasibility.
3. Implement outcome/diagnostic changes and focused regressions; publish.
4. Implement console choices/review changes and safe fallback; publish.
5. Integrate public-process cases, docs and CI in the same PR; publish.
6. Run independent architecture/authority review and user-scenario/E2E review.
   Fix all critical findings and repeat affected evidence on the latest HEAD.
7. Run required checks on the final revision, mark Ready at the appropriate
   point for draft-gated workflows, obtain actual independent approval, merge
   normally with expected-head protection and verify actual merged main.

Reliability owns skills_batch.go, apply_outcome_status.go, install_report.go,
install_failure.go, skills_retry.go and new diagnostic helpers/tests.
Console owns skills.go, skills_terminal.go, skills_search_prompt.go,
skills_review_output.go, prompt/skills.go, terminalprompts/skills.go and the
skillspicker server handoff. Shared call-site changes are coordinated explicitly.
Qualification owns new terminal Skills fixture/cases/oracles and narrowly scoped
existing workflow wiring. Root owns integration, publication and final gates.

All new Go files/functions obey the existing size/complexity/import limits;
legacy exemptions may only shrink. Reuse existing contracts instead of adding
interfaces without a real boundary. Generated files remain generated.

## 9. Verification and review gates

Focused tests precede broad gates. Heavy local builds run sequentially using the
exact repository toolchain and existing caches. Required make lint and
make test-core remain required. Run affected compatibility/native/rollback and
cross-platform qualification prescribed by CONTRIBUTING; preserve unrelated gates.

Terminal UI and Skills picker workflows skip Draft PRs: a skipped draft run
does not qualify implementation. Once code is ready for full CI, use the normal
Ready transition and inspect actual final-HEAD results. Source-binary evidence
does not claim a published npm/release already contains the change.

Subagent review is useful code review, not a GitHub APPROVED from an independent
reviewer. Do not self-approve or bypass repository branch rules. Handle actual
review comments, revalidate changed code and use ordinary merge.

## 10. Review log and acceptance

Plan critique rounds: pending.

- [ ] Two plan critique rounds incorporated.
- [ ] Exact source-fixture / bundled-binary feasibility proven.
- [ ] Confirmed outcome and console regressions fixed.
- [ ] Diagnostics remain useful when log recording is unavailable.
- [ ] Large catalogs and fallback races covered.
- [ ] Public-process / terminal evidence complete for declared platforms.
- [ ] Required final-HEAD gates passed; review findings resolved.
- [ ] Independent review requirement satisfied.
- [ ] Ordinary merge and actual merged-main verification complete.
