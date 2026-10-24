# Claude grouped Prepared update correction plan

## Scope and frozen evidence

This plan concerns result classification for an unchanged, verified project delivery that remains Prepared after a fresh native observation. It does not change native discovery, activation completeness, ownership admission, transaction authority, or trust policy. No production source has been changed for this plan.

The genuine current-source run [40131383052](https://github.com/777genius/universal-agent-plugins/actions/runs/40131383052) checked out commit `75cece1488cdab0674b4817ea7a30ea95f7aad06`, tree `c302aa9bc97127b89504c9e6de049abfcf5ef580`, and executed pinned Claude Code 2.1.263 on darwin-arm64 and windows-amd64. Both jobs passed the existing user commands, installation, and runtime lifecycle tests. Both failed `TestAgentpluginsClaudeNativeProjectPreparedLifecycle` at the unchanged update assertion:

```
unchanged prepared update: mutated = true, want false
```

Each raw update result contains `mutated:true` together with `no_change:true`; each repeated add result contains `mutated:false,no_change:true`. Both outcomes remain Prepared + installation_verified with the trust blocker. The fixture aborted before checking its post-update state hash, so this artifact alone does not prove the state remained byte-identical after update. Later lifecycle stages were not evaluated. Project discovery remains blocked; MCP initialization and model use were not evaluated.

Raw project-stage files, original test transcripts, and independently checked ZIP/member hashes are in:

`docs/evidence/project-scope-native/claude-2.1.263-prepared-update-failure-40131383052/`

Darwin artifact 12575658957: 382183 bytes, SHA256 `0359fc9c1dc6bf694a3fa1c89bbe585636a21c799eba5b0ea576d0bc6369ede4`.
Windows artifact 12575786665: 396353 bytes, SHA256 `a944c0b0f27fe96c2eb32700830e6ec11b04ec3048cec3def9201f8c9479f60b`.
Both ZIP archives and every original project artifact hash were checked after connector download. No vendor process was executed locally.

The newer source run 40135056328 at `4a1634f4906453eac0b87eddbcc54246226dcfdd` was queued when inspected. No run was dispatched or retried.

## Exact cause and boundaries

The ordinary repeated-add path in `usecase/lifecycle.go` calls `verifiedPreparedObservationUnchanged`. That helper already requires project scope; non-native-effect lifecycle; an available actual verifier; materialized exact-path/digest delivery; saved and fresh Prepared/Installed states with equal authentication and policy; no activation/authentication attestation; no native effect or newly returned native objects; and exact recorded delivery native objects.

The grouped update path in `usecase/group_activate.go` derives and verifies the existing delivery through `activeNativeDelivery`, then reobserves it through the ordinary activator. Claude is intentionally a CLI-backed lifecycle whose Prepared registration is incomplete. `nonNativeRegistrationComplete` therefore returns false. After successful unchanged Prepared observation, `activateOneGroupTarget` unconditionally clears its local `target.noChange` whenever `nativeComplete` is false. `assignActivationOutcome` derives `Mutated=true` from that cleared flag, although its preflight `NoChange=true` flag survives if no lifecycle field changed.

This diagnosis does not rely on treating complete=false as an error or promoting it to complete=true. The root's separately reviewed default prepared-file observer work addresses early ownership admission and does not correct this grouped result branch.

## Proposed minimal implementation

1. Return the actual delivery used for activation from private `activateGroupDelivery` to its sole caller, alongside the unchanged outcome/nativeComplete/error results. This keeps the boolean's native completeness meaning intact.
2. After fresh activation succeeds, preserve a preflight no-change target only when its existing managed binding and the returned actual delivery/outcome pass the same `verifiedPreparedObservationUnchanged` helper used by repeated add. Do not infer this from Prepared alone. Keep ordinary noncomplete-effect classification unchanged for every request that does not pass this predicate.
3. Keep persisted lifecycle/receipt change detection independent. Any real lifecycle change still makes the result mutated. An actual package transaction still starts from target.noChange=false and cannot become unchanged through this exception. Native committed/uncertain effects, newly returned native objects, policy/authentication drift, attestation, missing digest, or changed owned bytes cannot pass the predicate.
4. Synchronize a target's final NoChange flag with the existing invalidation branch so an effect-bearing successful incomplete activation cannot emit mutated=true/no_change=true. OR each final target Mutated flag into GroupResult.Mutated, because a native effect can occur while recorded receipts and lifecycle fields remain identical. GroupResult has no NoChange field: retain its existing wire schema; require the target NoChange=false on actual mutation. Preserve existing projection-failure no-change semantics: a failure before effects can retain its original no-change classification with the recorded lifecycle and both mutation flags false.
5. Do not change `activeNativeDelivery`, `nonNativeRegistrationComplete`, ownership, identity, recovery eligibility, native attempt creation, transaction staging/commit, or read-only versus acting adapter selection. Do not change the strict native harness assertion or its existing required tests.

No new generic framework, public API, adapter-specific shared branch, or production injection hook is proposed.

## Regression acceptance and finite validation

- Add a focused inert usecase group regression with a real materialized project binding and a fresh successful Prepared observation: repeated grouped update reports group and target Mutated=false, target NoChange=true; exact state bytes, receipts, and owned tree remain unchanged; verifier is called again; activation remains Prepared/Installed.
- Cover the actual saved CLI single-target batch update route used by the native harness with an inert Claude runner returning the exact known suppression sentinel. Check repeated add and update, exact state hash/owned bytes/user+A/B preservation, fresh native-list calls, and no discovery promotion. This is an inert regression, not replacement native evidence.
- Include changed source bytes while remaining Prepared: update must mutate real owned bytes and report Mutated=true, NoChange=false. Include a successful incomplete outcome carrying NativeEffectCommitted while the persisted receipts and lifecycle facts remain identical: BOTH GroupResult.Mutated and Targets[].Mutated must be true, and Targets[].NoChange must be false. Include changed lifecycle/policy and explicit authentication/activation attestation negatives so the exception cannot mask real changes; the existing authentication-preservation rules remain intact. The shared predicate must still require saved/fresh normalized authentication and policy equality. Preserve existing pre-activation projection-failure test and strict user-scope no-change tests.
- Run only new focused group/CLI tests and existing relevant idempotent/projection-failure/Claude Prepared tests with shared Go1.26.10, -p=1. No broad suite, vendor process, authentication, trust change, or local hosted guard bypass.
- Freeze implementation for independent root and peer code review after both independent R1 and R2 plan reviews. Publish only after both code GOs.
- Reuse the next existing exact-source hosted native run if its source includes the reviewed correction, or let the reviewed normal PR workflow run on publication. Do not blindly dispatch or retry the already known failing run. Retain original failures and require the unmodified Prepared lifecycle gate to pass before declaring this issue closed.

## Remaining unrelated bootstrap investigation

The Linux bootstrap cancellation failure is being traced read-only. No timeout/assertion has been relaxed and no production fix is proposed by this Claude plan.

## Independent plan review record

R1 reviewed exact `6876e8f3` before production edits:

- Root: architecture GO after plan and grouped-branch trace; require final group/target mutation aggregation, retain actual wire fields, and keep projection failure classification.
- apm_vercel_scope: architecture GO after tracing the helper, grouped activation, CLI route, and native evidence; require same-recorded-facts NativeEffectCommitted negative at both group and target levels.

The R2 proposal incorporates both requirements above. No production source, test selector, strict native assertion, or native completeness semantics have changed.

## SDK group mutation propagation amendment — R2

Root's real public SDK Cline+Codex two-target repair regression exposed a separate inherited reporting omission while the default observer work was being tested. In installer/group_apply.go, finishGroup receives a core GroupResult whose real managed directory restoration has Mutated=true, but never transfers that flag to installer.Result. Its inherited Prepared/Installed bindings continue to have hostHandoffPending, so the SDK honestly retains OutcomeIncomplete with the existing reason committed binding is not activated; it incorrectly reports Mutated=false even after the owned files were restored. This amendment changes only mutation reporting, not that Prepared handoff contract.

Proposed bounded correction after both independent R1 architecture GOs; R2 review is required before editing SDK code:

1. At the start of finishGroup, accumulate result.Mutated OR got.Mutated before state reload, activation-error, pending-handoff, or normal-success return branches. This preserves effects already recorded by host reconciliation and actual core mutation even if later inspection fails. Do not infer native activation or mutation from Prepared alone.
2. Keep OutcomeIncomplete, its native handoff reason, activation/verifier facts, recovery decisions, and all hostHandoffPending behavior unchanged. There is no new automatic completion of Prepared bindings and no attestation/discovery promotion. Gate only finishGroup's final unchanged classification on !result.Mutated, so a stale no-change preview cannot override an actual observed mutation. Keep its existing pending/native handoff checks. Do not alter the unrelated early unchanged Apply branch.
3. Focused SDK tests must pin: actual managed repair with Prepared/Installed remains Incomplete but Mutated=true; unchanged read-only core result does not manufacture mutation; existing input result.Mutated=true is not cleared; committed effects remain visible on activation/pending/reload-error exits even if those exits keep their existing Outcome classification. Include the stale prepared.plan.NoChange=true plus actual core/prior host mutation case: SDK must not claim NoChange or OutcomeUnchanged after a real observed effect. A truly read-only result with a no-change preview still returns unchanged. Reuse the root's composed real two-target repair regression when integrated and add a narrow installer finishGroup unit table for the reporting paths as needed. The earlier core group classification negatives remain mandatory independently.

No SDK production code was edited for this amendment. Independent review and a frozen code checkpoint are required before publication.

SDK amendment R1 exact a30408aa received independent architecture GOs from Root and apm_vercel_scope. Root required verified mutation preservation even when state reload/Outcome classification fails; APM required that finishGroup's final unchanged branch cannot contradict an actual core/prior host effect. R2 incorporates those requirements without changing the early applyGroup branch or Prepared handoff semantics.

Core grouped Prepared R2 exact cd99183b independently received PLAN GO from Root and apm_vercel_scope before source edits. SDK reporting amendment R2 exact ad6478e8 independently received PLAN GO from both reviewers before its two production-line changes. Original failing native assertions remain unchanged.

## Local implementation checkpoint validation

The approved correction now reuses the exact activation delivery in grouped classification, retains nativeComplete=false for blocked registration, synchronizes target NoChange after invalidation, and accumulates final target mutation into the group. The SDK amendment accumulates the core/prior mutation before every finishGroup exit and gates only that function's final unchanged result. The existing Prepared handoff remains Incomplete with its original reason when pending. The existing delivery equality predicate was extracted unchanged into a small private helper to satisfy the repository complexity gate; no condition was removed or widened.

Focused inert Go1.26.10 tests, sequential with -p=1:

- CLI project regression plus existing user idempotent update tests: PASS0.226s. Actual project namespace composition, exact suppression fixture, repeated add/update with fresh listing and exact state/owned bytes, changed real skill bytes remaining Prepared, and original user-native/user-state/A/B foreign preservation.
- Usecase grouped Prepared table plus existing single-resume authentication, Kimi unchanged and pre-activation projection-failure regressions: PASS0.231s. The same-recorded-facts committed effect writes an actual inert native marker while receipts/lifecycle/state bytes remain exact; both group/target mutation flags must report it. Policy/authentication/attested outcome negatives are covered.
- SDK reporting table, existing group attempt/cancellation classification and project Claude Prepared lifecycle: PASS2.148s. Pending handoff/error/read-failure effects remain visible; stale no-change previews cannot override actual mutation; existing handoff facts remain Prepared/Installed.

These tests execute no vendor, native plugin, MCP server, authentication or model process. The unchanged genuine hosted Prepared lifecycle assertion still needs a successful exact corrected-source run before closure. Bootstrap remains a separately approved plan with no production edit in this checkpoint.

### Frozen code review correction

Root's independent review of2cbfecc8 identified a known-effect reporting edge when nativeComplete=true: an adapter that unexpectedly reports NativeEffectCommitted during VerifyOnly must still report the actual effect, even with identical saved facts. Final target mutation therefore includes NativeEffectCommitted independently of projection completeness; the same final flag clears target NoChange and accumulates into group mutation. This preserves effect truth without promoting discovery or changing adapter authority. A new inert Active/Installed binding case proves complete=true/VerifyOnly=true, real marker write, unchanged state/receipts, and both mutation flags true. Focused six-case group table plus existing Kimi/projection-failure tests PASS0.299s. No test assertion or native gate was weakened.

## Independent frozen-source code review receipts

Both reviewers assessed exact source commit `20920de589227ffd34ece978ac52add365ceac4b` (tree `39a42178feabf4a9589085f8c67f9bd4501c477c`) after the earlier `2cbfecc8` correction. This record changes documentation only; the reviewed core/SDK production source remains identical.

- `/root`: CODE GO. Reviewed all three production files, CLI/core/SDK regressions and final committed-effect delta. Confirmed exact actual delivery reuse and unchanged strict Prepared predicate; nativeComplete/native attempts/ownership/Prepared handoff remain separate. Final known committed effect is reported independently of complete/VerifyOnly, clears target NoChange and aggregates group mutation. SDK mutation propagation/final unchanged guard remain narrow. Reviewed the owner's focused passing test outputs; no independent vendor run was claimed.
- `/root/apm_vercel_scope`: CODE GO. Independently read the complete production, CLI, core and SDK source and the final delta. Confirmed complete=true committed-effect marker case pins group/target mutation and target NoChange; Prepared predicate, actual delivery, native completeness and uncertainty fences remain intact. Confirmed SDK entry OR and final mutation guard. No new build or vendor process was run by this reviewer.

Publication may carry the exact reviewed production source plus this receipt and already reviewed plans/evidence. Genuine exact corrected-source hosted Prepared lifecycle qualification remains required; neither code review nor the inert local tests certifies native project loading, MCP initialization, or model use.
