# Installer reliability and console UX: implementation plan

Status: **implemented; independent review fixes and final cross-platform
qualification in progress**. Actual PR checks and retained evidence, rather
than this plan's estimates, determine merge readiness.
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
8. One machine-readable result across declared-install repair and convergence,
   retaining already committed repair effects when a later phase fails.

### Excluded

- Deferred MCP runtime probe, Skill-reference analysis, setup/secret wizard.
- Native client policy/approval changes, new clients, frozen VS Code.
- Acquisition-policy weakening, TLS bypass, ambient credentials or paid models.
- A visual redesign, browser mobile-tab polish or unrelated website changes.
- Public release/tag/npm publication; merge is the authorized delivery boundary.
- Reimplementation of ownership diagnosis, retry or progress already in main.

## 2. Evidence and overlap

Exploratory scratch tests on the unchanged base identified four candidate
regressions. Their temporary source and captured output were not retained in
the repository, so those baseline-failure claims are **not independently
verified by repository evidence** and are not acceptance proof.

This PR instead carries reproducible production-composed controls:
`TestPlainSkillsDoesNotAutomaticallyChooseBrowser`,
`TestSkillsReviewUsesAlreadySelectedVisibleStream`, and
`TestSkillsNoDetectedTargetsHasActionableDiagnosis` in
`cli/internal/agentpluginscli/skills_console_regression_test.go`, plus
`TestSkillsRepeatedPrepublicationFailureReportsEveryBinding` in
`cli/internal/agentpluginscli/install_outcome_truth_test.go`. Run them with the
repository Go toolchain using `go test -count=1 ./cli/internal/agentpluginscli
-run '^(TestPlainSkillsDoesNotAutomaticallyChooseBrowser|TestSkillsReviewUsesAlreadySelectedVisibleStream|TestSkillsNoDetectedTargetsHasActionableDiagnosis|TestSkillsRepeatedPrepublicationFailureReportsEveryBinding)$'`.
Current-commit results and public-process evidence determine acceptance.

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
- An observed BindingUnchanged stays unchanged if a later unrelated binding
  fails. Native readiness facts alone are not a completed transaction outcome.
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
- Normalize at one private item boundary shared by report/log/JSON. Failed
  bindings with completed native facts use existing failed status; typed
  cancellation remains its cause/code, not a new status vocabulary.
- Preserve errors.Is/As, public JSON shape and exit conventions. Correcting an
  incorrect status value is a bug fix; it must have an explicit regression.

### Diagnostics without a writable log

Use one small internal diagnostic result containing saved path and write error.
A logging failure never changes installation effects or the primary outcome.

When recording is disabled or unavailable, retain useful existing classified
Kind/Summary/Next facts and fixed generated wording. If attempted recording
fails, say that technical details could not be saved, optionally classifying the
log failure through existing fixed permission/disk/read-only wording. Do not
render raw error chains, provider detail, PathError.Path, URLs or logErr.Error(),
even after heuristic sanitization. Unknown causes retain safe generic recovery
guidance; original causes survive through Unwrap, not the terminal Error string.
Disabled recording must not be described as a failed write. Never advise
opening a nonexistent file. Private log redaction remains unchanged; no second
sanitizer is added. Preserve the typed primary cause when rendering also fails.

Integrate the same behavior into Skills, dependency install/update/repair,
group and single-install results. Recovery and unconfirmed process cleanup
continue to prevent every retry in the affected namespace.

## 5. Console choices, review and consent

### One visible interaction stream

- --plain disables automatic browser selection; explicit --ui keeps precedence.
- Numeric catalog, validation messages, target choice explanations, full review,
  oversized-review fallback, changed-retry review and confirmation acknowledge
  use the already selected visible interaction stream.
- Resolve and carry the existing App/prompter/visible writer through catalog,
  targets, review and retry review; do not discard a locally resolved host.
  Preserve injected prompters and numeric fallback, without a writer registry.
- Keep JSON, noninteractive results and dry-run data on their existing stream.
- Interactive dry-run plan data also retains stdout. Pass interaction intent
  or writer from the caller instead of globally redirecting all review/error
  helpers. Test the stream matrix, including empty-catalog diagnostics.
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
- Keep 50 entries/page. Next/previous navigate the current view; selected-only
  uses the same canonical candidates and basket. Empty selected view can return
  to all candidates. Invalid navigation never changes selection or reads ahead.
- Bound candidate omission summaries and reuse skillDiagnosticGuidance.
  Include incomplete-scan guidance and exact-path narrowing where applicable.
- Preserve SourceMoved detail, full sanitized destination/source paths and
  existing reasons in review. Navigation labels may truncate, authority may not.
- Preserve distinguishing path suffixes in rich navigation without changing IDs,
  selection validation or existing display/normalization budgets.

## 6. Browser-to-terminal handoff

Use the current server mutex and session lifetime. Atomically decide whether an
unused session can be closed to future admission. While preparation is in
flight, refuse takeover. A prepared but unaccepted review may be disposed when
handoff wins. Any accepted operation, including an already finished operation,
blocks terminal takeover so fallback cannot start another install. Never cancel
accepted work because a fallback prompt was answered later. A check followed
by unguarded Close is insufficient; never wait for shutdown under the mutex.

Bound the OS opener invocation, honor cancellation, check output failures and
use a small private launcher seam for deterministic tests. Preserve session
capability secrecy and existing HTTP/JSON schemas. Tests must prove at most one
accepted execution path when apply and fallback race.

Protect every automatic fallback/error exit after browser exposure, including
diagnostic write and prompt failures, from unconditional deferred Close. The
opener timeout belongs to a child context for the opener only, never the picker
session. Explicit command-context cancellation still cancels the session.
Cover late requests after handoff, preparation failure during handoff and opener
timeout after operation admission as well as the ordinary apply race.

## 7. End-to-end evidence

Extend scripts/terminal-ui and its existing Fixture, Session, state hashes and
evidence format; do not introduce another test runner. Existing native_skills.py
and native_browser.py already qualify rich public Skills and browser journeys;
reuse their applicable parsing/progress/oracles. New cases address deterministic
failures, no-op/retry, plain/redirect/paging and platform gaps. Exact commands
and assertions are versioned alongside the implementation.

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
not arbitrary local folders. A deterministic Linux HTTPS publisher uses a valid
local CA through standard SSL_CERT_FILE with TLS verification enabled. Production
source policy accepts HTTPS loopback endpoints; prove the real composition early.
Go macOS/Windows trust does not consume SSL_CERT_FILE like Linux does. Those
platforms use a pinned public own-repository GitHub Skill, full commit in the
source selector and exact path, isolated profiles and native protocol fixtures.
The pinned base uses tests/fixtures/uap_apm/skills/benchmark-note, a standard
inert Skill with scalar frontmatter. The earlier docs-only-review illustration
contains legacy mapping-valued compatibility and is not a valid raw-Skills fixture.
Explicit --ref is a branch/tag selector, not the full-SHA source syntax.

Raw GitHub acquisition strips GIT_* and user Git configuration; GIT_SSL_CAINFO
or insteadOf fixtures from native Git acquisition are not a valid substitute.
The HTTPS website route requires exact --skill and cannot exercise catalog or
automatic browser selection. Catalog/paging/plain-browser cases therefore need
a proven pinned GitHub route with no explicit-path flag that bypasses routing.
Public-source availability failures remain failures, never silent source swaps.
Label Go composition tests and actual-binary cases separately. Synthetic native
protocol responders qualify installer behavior, not vendor acceptance.

| Evidence group | Linux | macOS | Windows |
| --- | --- | --- | --- |
| Actual apply, partial, unchanged failure, retry, JSON | Trusted local TLS publisher and pinned GitHub control | Pinned GitHub | Pinned GitHub |
| Actual plain/catalog routing with bundled assets | Pinned GitHub without routing-bypassing flags | Same | Same with ConPTY |
| Large generated or incomplete catalog | Production-composed command and real renderer | Same | Same |
| Review short/failed writes | Command writer fixture and actual pipe control | Same | Same |
| Browser handoff races | Synchronized existing server/bridge fixtures and bundled CLI control | Applicable opener behavior | Applicable opener behavior |
| Terminal restoration | Same-descriptor Unix probe | Same-descriptor Unix probe | Native console owner/reuse probe |

Require actual Windows Skills fresh/repeat/partial/retry and cancelled-review
cases; Go command-seam coverage is supplementary. Record product and source
fixture SHAs separately. Public acquisition gets a realistic bounded deadline,
separate from short interaction deadlines after the menu is ready. The public
source fixture exposes only required Git/runtime directories and removes the
existing test deny-network proxy settings, without inheriting host credentials.
Do not create a remote repository or enlarge product fixtures for generated
2,000-item catalog tests; use the labeled composed command boundary.

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

Plan critique round 1 incorporated: authoritative observed outcomes; typed-only
log fallback; carried visible writer and dry-run stream matrix; bounded paging;
atomic handoff across every browser exit; isolated opener timeout; actual source
policy, existing public Skills harnesses, Linux-only CA fixture and distinct
pinned-public-source macOS/Windows lanes.
Round 2: all three independent reviewers rechecked the revised plan and
reported PASS with no remaining blocking plan findings. This is plan review,
not production-code approval or E2E evidence.

Implementation review found and corrected two additional user-visible paths:
failed native next/manual instructions could leak provider text in human output,
and a refused interaction factory could leave a hidden numeric reader when both
output streams were redirected. The imperative `update --all` owner now also
records classified failures and preserves failed output writes. A further review
requires authoritative per-binding provenance when projecting healthy native
rows with missing optional identity in a mixed batch. These reviews are tracked
separately from actual CI and do not count unexecuted platform lanes as passing.

Actual declared-install regression execution also exposed two JSON documents
after a successful drift repair followed by convergence. The fix retains typed
repair presentation until the final result, keeps full convergence, and finalizes
once on a later early refusal. Failed or short output cannot cause a second
document. It changes neither the installer transaction nor the JSON field schema.

The subsequent repository review found three more result-path defects: saved
logs could change a failed binding's displayed status, an imperative batch
failure omitted earlier successful bindings, and a matched failed target could
lose aggregate recovery guidance. Their fixes retain original native/JSON facts
and add regressions at the actual reporting and staging boundaries. Qualification
controls now also reject saved-intent changes before consent, incorrect fresh
statuses, a digest from an unrelated lock entry and stale native cache without an
installed registration. Executed binary provenance and publisher cleanup are
checked independently of the caller's directory and shared executable path.

The recovered local executor builds the production picker assets and runs the
focused Go and Python controls. Its ordinary process-containment prerequisite
is unavailable (`/proc/.../task/.../children`), so Git/native cases remain
mandatory on the normal CI hosts. After evaluating the documented per-PR hosted
fallback, Linux qualification uses the repository's existing runner-pool policy,
including the new Skills job. No assertion or global runner setting is changed.

- [x] Two plan critique rounds incorporated.
- [x] Exact Linux HTTPS source and production picker bundle feasibility proven.
- [ ] Confirmed outcome and console regressions fixed.
- [ ] Diagnostics remain useful when log recording is unavailable.
- [ ] Large catalogs and fallback races covered.
- [ ] Public-process / terminal evidence complete for declared platforms.
- [ ] Actual Windows/macOS Skills source/process lanes executed, not replaced
  by plugin-only or composed-command evidence.
- [ ] Required final-HEAD gates passed; review findings resolved.
- [ ] Independent review requirement satisfied.
- [ ] Ordinary merge and actual merged-main verification complete.
