# Codex Prepared native configuration: unchanged replay result

2026-10-25. At source `d6809d5438654ccdb47bd8d241b96b44e91515c8`, tree
`7ba2bf288af06d6be36f65c708e913688a26567a`, native run `40187422616`, Windows
job `121151065083`, fails both project suites on repeat add. Public JSON reports
`mutated:false`, Prepared and installation verified, but omits `no_change`.
The exact artifact is `12596773186`, 133,705 bytes, SHA-256
`4d2dbe5f4eda4b84e6ed0a0af11586da419fec0dc312f36720c788910de16da4`.
Its complete runner membership/digests match that source. Initial trusted direct
MCP assertions passed with zero captured Responses requests; replay and removal
qualification remain incomplete. Neither native assertion will be weakened.

## Cause and bounded correction

`Service.resume` currently classifies unchanged Active registrations or unchanged
Prepared observations for clients without native lifecycle. Codex deliberately
retains Prepared after exact project native configuration verification, so its
successful verify-only replay falls outside both classes.

Extend the existing Prepared-observation classifier with a separate branch only
for project `EffectiveTraitsForScope(...).LifecycleKind == LifecycleNativeConfig`.
Keep its current nonnative Prepared branch unchanged. The additional branch must
require the exact same Prepared/Installed lifecycle, authentication and policy;
materialized binding; exact unchanged delivery and proven owned receipt set;
explicit `NativeEffectUnchanged`; exact outcome/client/delivery native receipt
equality; no unresolved native attempt/intent; and no input completion or output
activation/authentication attestation. Retain exact project scope/identity and
available verifier requirements. Unknown, missing, committed or uncertain effect,
changed/missing/foreign ownership, or any contradictory outcome cannot qualify.

Carry the existing `activeNativeDelivery` completion proof into this classifier
from its two existing callers. The native-config branch requires that proof and
a nonempty receipt set; the nonnative branch ignores it and retains its guards.

This reports the completed file verification as an unchanged operation. It does
not make the lifecycle converged, grant trust, promote Prepared, or qualify MCP,
authentication or model behavior. Every repeat retains the existing fresh
verify-only activation path and ownership observation. Change no native fixture,
CLI envelope adapter, registration decision, transaction or activation call.

## Verification and publication

Two independent plan rounds and frozen implementation reviews precede publication.
Strengthen the existing composed Codex project installer fixture to assert
`NoChange` on repeat add and unchanged update with exact state/native file bytes,
preserved user/B files and Prepared binding. Add focused inert classifier
contracts for exact unchanged ownership and mismatched delivery/outcome, committed
effect, unresolved attempt/intent, wrong scope/lifecycle and attestation refusal.
Run affected installer/usecase tests and configured changed-source lint. The
unchanged genuine macOS/Windows native suites at the next assembled exact head
remain the qualification gate; the historical partial stage is not full E2E.
