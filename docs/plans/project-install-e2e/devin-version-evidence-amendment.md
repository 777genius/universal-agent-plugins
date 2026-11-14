# Narrow host-version evidence propagation for Devin project gating

Pre-code amendment, 2026-10-23, cursor_clients. This supplements the existing
Windsurf/Devin client plan's requirement for an explicit newer harness/version;
it does not introduce another native probe or installation engine.

## Observed composition gap

The existing explicit-target detector can return version evidence. CLI saved
sync and no-save facade currently pass only client ID/config/executable into
installer.Request. Engine.detectedClient reconstructs a domain client without
Version. Thus a correct version-gated project adapter cannot distinguish the
qualified current Devin artifact through the public installer, even when the
host already observed its version. The public SDK's Discover intentionally
never executes a found file, and that read-only contract must remain intact.

## Proposed narrow change

1. Add an optional operational `ClientVersion string` to installer.Request,
   excluded from diagnostic JSON. It is host-observed metadata paired with
   the already explicit ClientExecutable; it is never package-authored or
   read from portable plugin/dependency manifests. Copy it into the reconstructed
   domain.DetectedClient.Version. Request's immutable prepared snapshot already
   copies strings; no mutable evidence object, process runner or cache is added.
2. Pass selected.Version from the same selected DetectedClient alongside its
   executable in the existing no-save facade and saved dependency request
   builders. Preserve all existing version-probe behavior and environments;
   do not run a new subprocess from an adapter or public SDK Discover/Prepare.
   SDK hosts can supply their own already observed evidence; absent evidence
   stays unknown and cannot qualify this project lane.
3. Keep group/sibling/version/public capability behavior unchanged. The current
   SDK group boundary is Claude/Codex only, so no speculative group expansion
   or new version fields for unused group targets are needed. Selected Devin
   install/update/repair requests receive fresh request-scoped evidence; exact
   owned removal remains version-independent and never launches native cleanup.
4. Within the Windsurf adapter, admit only the selected primary executable whose
   basename is devin/devin.exe and normalized exact known3000.11.205 evidence.
   Current detected user ConfigRoot stays intact as the original authority.
   Keep existing detection ordering/channel selection unchanged. If a legacy
   Windsurf primary wins a coexistence environment, reject the automatic
   project lane with a precise explanation; do not infer current Devin merely
   from an editor directory or an unrelated devin surface. An embedding SDK
   can explicitly select its observed Devin executable/version pair.
5. This metadata permits only the reviewed configuration-file lane. It is not
   binary provenance, native discovery, trust, model/account authentication,
   plugin/session precedence or MCP runtime evidence. Preserve prepared+installed
   outcomes and project attestation refusal. Actual pinned native qualification
   must still record independently acquired binary hashes and genuine outputs.

## Project-only SDK admission alignment

Further static tracing found that a current-only Devin installation correctly
selects no legacy Cascade user ConfigRoot. Installer.detectedClient currently
rejects an empty user profile even for a validated project engine, although
NewTargetContext already permits that absence. Do not invent a profile or
change legacy detection ordering/user mutation to get around this mismatch.

Allow an empty ClientConfigRoot only when the engine/request is already in
validated project scope. A nonempty root still must be clean and absolute;
user scope keeps the existing required-root contract. Adapters continue checking
exact original user-context equality (both empty is legitimate), declared
project admission, real selected project/native root, artifact and receipts.
Project writers must never substitute the absent user root as native authority.
This changes no native path, profile resolver, authority token or fallback.

Add composed public SDK/CLI coverage for the selected current Devin with an
absent original legacy profile, plus user-scope empty-root refusal and nonempty
invalid project-root refusal. Existing scoped clients' stricter adapter policies
remain in force. Exact removal still uses recorded project authority, without
requiring a running/version-qualified current harness.

## Verification and publication

Two independent plan/fix GO reports must precede implementation. Add meaningful
composed tests proving exact known Devin evidence reaches both public CLI request
paths and the public SDK, while missing/older/other-harness evidence refuses
before native effects. Prove a source manifest cannot inject the host field.
Retain existing user/other-client behavior and public SDK Discover's no-execution
contract. Test current dedicated files, legacy migration refusal, local shadowing,
A/B/user isolation, exact ownership, immutable selected-data validation, lifecycle
and conservative status under the original client plan. Freeze a separate client
commit for two independent implementation reviews; root owns publication.
