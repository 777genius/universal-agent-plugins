# Current Devin project adapter — independent implementation R2

## Frozen initial review

Reviewer codex_claude, 2026-10-23. Initial target `132b8c38226f87a39724637cf5ac11b7e0978ea9` in work/uap-project-windsurf. **Not yet GO:** the accepted R1 native-comments seam and the bounded direct-binding client discriminator finding below require closure at a corrected frozen source SHA. Reviewer has not edited this worktree or executed a vendor/native process.

## Findings

1. R1 independently identified documented comments in current native Devin configuration. The selected target CodecDevin and native main/local policy decode currently reject them as strict portable JSON. Exact vendor3000.11.205 docs confirm line/inline/block comment grammar. The separate docs-only comments correction plan `e09d06d372575b0ad8edc0bf5ffa5f9ecc030e0f` has this reviewer's R2 GO in research/devin-native-comments-plan-r2.md; retain Gemini grammar/errors, all legacy/portable strictness, duplicate/trailing refusal and original comment bytes. No parser source changed by this reviewer.

2. `validateDevinProjectBinding` validates scope/root/artifact/path/objects but omits `binding.ClientID`. An otherwise identical direct binding tagged for another client is therefore passed to ProjectComponents.Inspect as ownership. That helper deliberately checks component receipt mechanics, not client identity, so the native record can be reported RegistryExpected despite contradictory owner identity. Require exact existing product ID `windsurf`, leaving user behavior unchanged. Update the valid copied-binding fixture to include its real ClientID and add a wrong-client inspector case. Higher production composition already fences IDs; this correction makes the direct project ownership boundary agree with its stated contract.

## Static coverage completed

Traced the current selected primary basename/version gate, original user-context equality including legitimate both-empty current-only profile, exact project/native root and `.uap/managed/clients/windsurf/<artifact>` binding, selected-delivery/path fences, shared host-version propagation and selected-versus-sibling exclusion, project-only native lifecycle/backend policy, new Devin transport codec versus preserved Cascade codec, skill/MCP object receipts, no-follow paths, immutable selected stdio package/data/cwd/environment reuse, direct/composed attestation refusal, and version-independent exact recorded removal. No additional blocker found in those boundaries.

Project policy refuses legacy main-config MCP migration and same-name local override rather than modifying it; unrelated imported/user/ancestor/aliases/cloud/team/session effects remain unobserved. Project materialization uses the reviewed component transaction/native kernel with existing rollback and committed-cleanup receipt semantics. It never calls default-local native remove or registers a user plugin. Existing detection/channel ordering and legacy user paths/lifecycle remain preserved.

Inspected meaningful fixture coverage: production CLI saved TLS Git/ZIP add/locked-no-op/read/repair/update/export/cold-import/remove plus no-save, public SDK A/B stdio/data/exact repair/restart/remove and missing/wrong-version/empty-profile/non-injection admission, selected-only KnownTargets correction with exact sibling facts, direct/composed root/artifact/delivery/receipt refusal, foreign disabled/stale/symlink preservation, visible-write rollback and committed-cleanup verification. These are configuration/ownership fixtures with inert host/security/launcher boundaries, not genuine native loaded/runtime proof. Ten actual bounded version/help observations and pins remain pre-code evidence only.

Independent affected test results and final correction closure will be appended at the next frozen checkpoint. Native discovery/trust/permissions/authentication/MCP initialization/runtime/Desktop/release and all-client completion remain unqualified by this review.

## Final correction and component-base closure

**GO** at frozen clean `d69091b3328a1b626d9475643e8e97a8bf040b4a`.
Both initial findings are closed. The native-comments amendment and both plan
GO reports were committed ate8f638ed before corrective4f5d9558 source. Its new
Devin branch reuses the existing comments-only AST admission and retains Gemini
error labels/explicit JSONC behavior. Duplicate keys and trailing commas refuse
in the target codec and the ephemeral policy normalizer. Original foreign
comments/entries remain in the mutation AST. Legacy Cascade and portable strict
JSON retain their previous grammar. Commented main/local migration and same-name
shadowing policy has meaningful preserved-byte success/refusal coverage.

The direct project validator now requires literal binding.ClientID `windsurf`.
Its valid binding includes that discriminator, and copied-client bindings are
checked by both direct validation and native inspection. User-scope validator
behavior remains unchanged.

Independently examined all three manual d39e40ef component-base merge conflicts:
Codex TOML dispatch precedes the Gemini/new-Devin comments parser, projectServer
retains both Copilot and Devin dispatches, and supportedCodec retains every
baseline member plus Devin. All foreign client directories are byte-identical
to d39e40ef; the domain diff is only the Windsurf project wrapper. All four
compatibility JSON goldens preserve every non-Scopes field. The first static
comparison used lowercase `scopes` and correctly failed; rerunning with the
actual `Scopes` field confirmed the narrow change. No source changed in review.

Independent full nativeconfig tests passed3.471s at this frozen target, including
Codex/Copilot/Gemini/Devin codec routes and existing kernel ownership/rollback
checks; the preserved Windsurf client suite passed0.006s. A subsequent independent
project provider/SDK/public CLI selection could not execute its intended
assertions because package snapshot creation under actual /tmp failed ENOSPC.
Those are recorded resource failures, not passes or semantic regressions. Parent
explicitly requested no duplicate broad builds under the shared-space shortage.
Prior authored actual composed/provider/public SDK/CLI successes recorded above
remain evidence for unchanged paths, and the focused native codec execution plus
static correction/merge review support this source GO. No new vendor/native
process ran, and current effective native discovery/runtime remains pending.
