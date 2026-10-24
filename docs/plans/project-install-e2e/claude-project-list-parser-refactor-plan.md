# Claude project native-list parser extraction plan

## R1: concrete failure and bounded scope

The genuine [architecture job121008405952 in run40139625683](https://github.com/777genius/universal-agent-plugins/actions/runs/40139625683/job/121008405952) checked merge34c85ffb (428bdf5d49e63b78bc4783df6b766c87c8e678ca into09b7650d34eb1a7876fabfd26fa2be89afe15cdf). Its raw log reports all three existing limits for clients/claude/project_observation.go: observeClaudeProjectList has72 statements (limit40), cognitive complexity57 (limit25), cyclomatic complexity43 (limit20). The complete decoded job log was fetched read-only; no local vendor process ran. The current function still has the exact reported shape.

This is an adapter-local behavior-preserving extraction. Keep the existing private entry point, return type, state enum, native strings and every admission/refusal predicate. Do not edit lifecycle, ownership, filesystem observation, trust, recovery, generic JSON decoding, user listing, commands, harness selectors, workflows, linter configuration or budgets. Existing successful hosted source428 is evidence for the pre-extraction behavior; it is not evidence that a later extraction has already executed natively.

## Proposed extraction

Keep all helpers private to the Claude adapter and specific to this exact JSON format. A small projectListScan accumulator will contain the existing result, selected/other/suppressed/foreign booleans and seen-identity map; a projectListQuery value will contain only name/active/project/user, so long repeated string parameter lists are avoided. Neither type is a public abstraction or a new generic parsing framework.

1. Extract query validation with the exact current size/name/absolute-clean-path/direct-parent predicates. Extract bounded unique whole-JSON decoding into a helper that retains UseNumber, shared.DecodeUniqueJSONValue, the trailing-token EOF requirement and the top-level array assertion.
2. Keep the entry loop in observeClaudeProjectList; reject non-object rows. Delegate each object to the scan's record method. Preserve id string/nonempty validation first, exact sentinel handling before ordinary metadata, the duplicate sentinel refusal and untrusted-note assignment. Ordinary-row validation remains before filtering an unrelated id, so unrelated malformed records still invalidate the whole listing.
3. Extract ordinary row validation (scope/enabled/path types, exact absolute path, known scope and validProjectListMetadata) into a private typed row helper. Preserve id+NUL+scope+NUL+path duplicate identity detection for every ordinary row before the same-name filter. Do not reject unrelated known local/managed rows merely because same-name skills-dir candidates permit only user/project origins.
4. Extract matching-candidate processing into its own method. Keep user direct-parent and one-user-winner rules; project direct-parent, exact active path and one-selected rules; selected-row nonempty errors/notes refusal; enabled/disabled classification; and same-name local/managed refusal. A foreign same-name project row remains foreign even if disabled and is not treated as absent.
5. Extract final aggregation with the exact existing order: selected plus any suppression/user/foreign is unknown; otherwise foreign wins, then suppression (retaining otherSource), then same-name user, otherwise the selected/absent result. Return a fresh zero observation on every refusal, rather than leaking partially accumulated untrusted/otherSource flags. Preserve the behavior of valid unknown suppression notes: suppressed without manufactured trust evidence.

No helper should itself exceed current statement/complexity limits. Split only at these existing validation/state boundaries; do not move predicates into opaque generic expression combinators or introduce linter suppressions. The existing helper metadata and suppression validation functions need no semantic change.

## Verification and exact acceptance

Retain all existing TestProjectObservationSeparatesBlockedDiscoveryFromIdentity, TestProjectSuppressionRejectsMalformedNearMisses, TestProjectListingRequiresExactScopePathAndNoLoadErrors and TestProjectListingRejectsSymlinkTraversalSpelling assertions. Add one bounded adapter-level full-observation table for current mixed-row boundaries that currently assert only state: suppression+user in both orders preserves untrusted/otherSource; foreign+suppression/user permutations preserve the exact current foreign result and flags; selected combined with any competing source always returns the completely zero observation; duplicate identities and two different same-name user paths refuse; malformed unrelated rows and unrelated known local/managed rows retain their current behavior. These examples pin public observer truth without testing private helper layout.

Run the complete small Claude adapter package inert test suite with -p=1 after coordinating the shared build slot. Then run pinned golangci-lint2.13.2 only over ./clients/claude with concurrency1, GOMEMLIMIT512MiB and a90s timeout, using the exact committed pre-extraction revision as --new-from-rev. Verify the whole modified source remains within the existing file-length guard. Do not run broad module lint or native commands locally.

Acceptance is identical adapter outcomes for the existing and added boundary tables plus no changed-source lint findings. There is no new positive native-discovery claim. Genuine native runtime checks may run through the normal hosted PR gate after publication; any later result must identify its exact source and preserve the same Prepared/blocked limits. Obtain two independent plan rounds from Root and APM before production edits, then freeze the source and obtain both code reviews before publication.

## Risks retained

Extraction can accidentally defer validation until after the unrelated-id filter, silently accept duplicate unrelated rows, merge selected with a competing source, clear foreign flags, treat suppression as ownership or leak partial observations after invalid input. The named boundaries and full-result tests explicitly target these risks. No observer identity, persisted state, file authority, recovery eligibility or trust is changed by this plan.

## R2: explicit preserved ordering and full-result facts

Both Root and APM independently gave R1 PLAN GO on document SHA256ef752f5627ce59107730f21b6d77850bde395e8a8d8468c29b569fd8dd020088 after reading the complete current classifier. Their clarifications are binding for this extraction:

- The exact suppression discriminator is handled before the ordinary row decoder. Ordinary metadata validation must happen before any notes/errors `[]any` access and before skipping an unrelated id. Keep the same boundedProjectStrings validation and projectSuppression schema; selected-row typed access therefore relies on the same prior validation, not an unchecked assertion introduced by a helper.
- Final foreign aggregation retains any previously recorded untrusted flag but does not set otherSource, even if a same-name user row was seen. Suppressed aggregation retains both the exact note-derived untrusted flag and the same-name user-derived otherSource. OtherSource-only is true only for the user-winning or suppressed+user results. This asymmetry is the existing return value and will not be normalized.
- Every rejection returns projectListObservation{}, including after a valid suppression or selected row. The accumulator's partially recorded flags must never escape a refusal path. Mixed-row table comparisons will compare the complete struct, not only state, in both row orders.

The added table will explicitly include known and opaque suppression+user; foreign+known suppression; foreign+user; foreign+suppression+user; selected+suppression/user/foreign; duplicate unrelated identities; two different same-name user paths; malformed unrelated metadata; and valid unrelated local/managed rows. Each case's expected value comes from the existing contract described above. Existing path/scope/sentinel/duplicate-key/trailing-input tests remain unchanged. No new native command, trust, file or state behavior is requested.

The exact pre-extraction architecture log has also been preserved locally at /dev/shm/uap-project-runtime-5e2d9d9f7f5b/claude-project-parser-121008405952.log (75319bytes, SHA2565df5e916efd4868771a390b6c6841bf38f10947b09b39b506366af0e1dc0cabf). Its three reported metrics remain the acceptance problem; no suppression or budget adjustment is permitted. Production edits remain blocked until Root and APM independently approve this frozen R2.

## R2 independent approval receipts

- **Root (`/root`) — final PLAN GO** on frozen R2 document SHA2561bafb19627ad09ffc884bfe778a8f1228c7fc85d078558e8d969ae2ed4d9ce31 after reading the complete current function and both plan rounds. Typed metadata validation, foreign/suppressed flags and completely zero refusal ordering are explicit. Authorized only the mechanical adapter-local extraction and reviewed regression/lint checks.
- **APM (`/root/apm_vercel_scope`) — final independent PLAN GO** on the same R2 document SHA256. Read the full R2 and source; confirmed metadata validation before typed slices/unrelated filtering, sentinel-first/all-row duplicates, foreign untrusted-without-otherSource asymmetry, suppressed/user flags and completely zero refusal. Full-observation permutations, unchanged boundary regressions and narrow lint satisfy the behavior-preserving scope. No source edits, build or vendor execution formed part of this plan review.

Both approvals preceded production edits. Source still requires two independent frozen code reviews before publication.

## Frozen implementation and focused validation

The adapter now separates query admission, unique whole-array decoding, ordinary metadata validation, one-row accumulation, matching-candidate classification and final aggregation. Every original predicate remains at the same semantic boundary. New full-struct mixed-row cases exercise rotations/reversals (all permutations for two/three rows), including foreign's retained untrusted/no-otherSource asymmetry and zero flags after any rejection. No lifecycle/user-parser/trust/recovery/ownership/selector source changed.

Using Go1.26.10, the exact new full-observation table first passed0.017s against original parser source1bc5b898d8085e707fb405ed0c8f84ca2454157f via a read-only Go source overlay. The final complete current Claude adapter package passed0.024s. Its first ordinary root run hit two pre-existing mode000 permission expectations because this workspace is uid0 with DAC bypass; rerunning only the test subprocess under `setpriv --bounding-set=-dac_override,-dac_read_search` exercised those assertions successfully. No tests were skipped or relaxed and no file permissions/product code were changed to accommodate the environment.

Pinned golangci-lint2.13.2 over only ./clients/claude with concurrency1, GOMEMLIMIT512MiB/GOGC50/90s and --new-from-rev=1bc5b898d8085e707fb405ed0c8f84ca2454157f passed0issues. One initial inert test-loop intrange finding was corrected by ranging over the same slice indices; the final whole-package test and lint runs include that correction. The modified source is below the unchanged500-line guard. Raw focused logs remain at /dev/shm/uap-project-runtime-5e2d9d9f7f5b/claude-parser-{before-tests,final-tests,final-lint}.log. No native, MCP, model, trust, auth or broad lint execution occurred. Two independent frozen-source code approvals remain required before publication.
