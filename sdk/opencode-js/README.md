# OpenCode event observer — unpublished 0.3.0 candidate

Published 0.2.0 remains immutable. Root `createObserver({client, emit})` and
`createV2Observer({client: {get, context}, location: {directory, workspaceID?}, emit})`
retain their public factories, named types, `observe` and `dispose` behavior.
V2 `turnID` remains the observed user inbox/message ID. Compatibility inputs can
lack original native provenance; their facts remain without provenance and cannot
qualify strict AN admission. Existing native 2.0.0/2.0.21 compatibility fixtures
are retained unchanged and do not qualify strict provenance or reader continuity.

`/v1` exports only V1. `/v2` exports the structural owned-reader V2 API with
neutral declarations only. Root includes both published factories. Neither
entrypoint imports a host package at runtime. The optional V1 peer is
`>=1.18.33 <1.19.0`; development pins are V1 1.18.34, V2 2.0.21, TypeScript 7.0.2.
No publication, release tag or install-time host inference is performed.

V1 completion requires an observed user/final assistant, matching bounded native
history, `stop`, and idle. Retry/resolution requires a later final update. Published
error/overflow behavior remains compatible. Additive strict `runtimeEligibility`
requires actual prior-qualified running-host authority, matching root/directory,
and native provenance. Strict source errors require a unique active ordinary
assistant and its final matching permanent error; error/idle alone cannot select
later history. Summary, retry, ambiguity and interruption suppress attention.
A fresh ordinary assistant after automatic continuation requalifies work despite
`session.compacted` occurring after the continuation user.

Strict V1 question/permission authority is the explicit
`callbackAuthority: 'qualified_native_sync'` binding to the actual host-registered
await-free live event callback. Native close ingress and bounded tombstones fence
future handoffs. The observed unique ordinary assistant must precede Asked and
match the native message/call and assistant-created lower bound. The actual root
client has no question.list, permission.list or global.health; none are assumed.
Hydration cannot open a request. AN composes its owned live-runtime profile helper
before activating this observer; installation-cached/PATH/peer versions do not
prove running-host eligibility.

One V2 semantic reducer/core drives root push compatibility and `/v2` owned-reader
lifetimes. The strict reader consumes `.data` AsyncIterable envelopes, owns the
subscription AbortSignal, and consumes exact native durable sequence for all
public session events. Genuine sparse increments (including 14→16 compaction)
are allowed under the qualified direct native public stream contract. Exact
retained duplicates are idempotent; unknown reorder, same-seq contradiction,
source error/end and capacity uncertainty fence work. A forward seq jump alone
cannot detect loss. Session-assigned ambiguity suppresses that session; global
transport/ownership loss closes every job before serialized metadata/IPC.

Missing execution location defers scope/root proof to bounded native public
session metadata and any observed native session scope. Load location cannot
substitute for native event scope. Ports must prove directory/project and full
workspace binding from native scope or separately qualified graph routing;
public session.get stripping workspaceID is not proof of workspace absence.
Only projected parentID is true child ancestry; fork lineage is separate.
Strict terminal proof binds the observed run, selected ordinary assistant and
native terminal idle projection/outcome. Step failure/retry/compaction is silent.
The private run `turnID` is execution.started.id; public fact `turnID` is userID.

Strict questions require a legitimate `V2CheckpointPort` bound to public
`ctx.rpc.register` portable no-method events. It registers an owned random
namespace and closed nonce schema, projects the actual `rpc.<id>.checkpoint`
type, emits via registration.events.emit, and disposes the registration after
reader teardown (including late completion). There is no public pending-form
read and no ambient transport/auth, REST client, guessed endpoint or cooldown.
The same independent reader consumes an initial marker before future LIVE
forms; after all metadata, callback capacity waits and consumer preparation it consumes **one final
marker per job** within the remaining total 2s budget. Emit-return alone cannot
authorize. Missing binding reports `form_checkpoint_unavailable` and closes
questions. Pre-readiness/reconnected forms cannot reopen via hydration.

This is the accepted native snapshot plus synchronous consumer-ingress relevance
contract, not a source settlement lease. Settlement after the snapshot can race
an already eligible effect. After close reaches ingress no new spawn is allowed.
`V2NativePorts` attest actual root/execution/scope, exact ordinary assistant,
question tool/message/call and public permission.get/list presence. Methods
return direct Promise values. Public V2 metadata/RPC calls do not accept abort
options: bindings check signal before/after, while SDK lookup slots remain held
until actual settlement. Native source adaptation/qualification belongs to AN;
the structural ports do not claim the host supplies additional Context methods.

Bounds: 512 sessions, 256 end-to-end jobs, 16 actual-settlement lookup slots,
2s shared metadata/preparation/final-snapshot deadline, four active callbacks, 4096-byte
frames, 30s original ingress age. Three lifetime replacements use 250ms/1s/2s;
success never resets the budget and no replay authorizes old work. Cleanup
synchronously invalidates tokens, aborts the owned subscription, then disposes
registration. Callback promises must settle only after actual owned child close.

Optional `beforeEmit(event, preparation)` runs once after callback reservation and
before the final question marker/permission snapshot. Only literal `true` continues.
It shares the original metadata deadline and sixteen actual-settlement lookup
slots with native reads in both V1 and V2. It receives the same frozen fact as final
`emit`, original ingress/clock coordinates, and a bounded abort signal, without
`revalidate`. Use a private WeakMap keyed by the fact for prepared state. False,
throw, timeout or consumed close suppress delivery; abort-ignoring preparation
retains lookup, callback and job capacity until its actual promise settles.
Capacity waiting also expires at the original deadline. V1 never restarts its
native-read allowance for preparation; V2 establishes its allowance at job dequeue.

AN puts asynchronous clock/frame/registry preparation in `beforeEmit`. After the
SDK's final snapshot, `emit` checks relevance and starts its owned child synchronously
before its first await. Its later await lasts through actual child close. Repeated
`handoff.revalidate()` returns the memoized snapshot and cannot move the barrier
past additional consumer awaits. The preparation signal stays tied to job relevance
through final delivery; reserved resources must observe cancellation and actual close.

Private wire v1 provenance preserves V2 envelope.created/id or V1 assistant
completion/assistant_created_lower_bound and stable canonical native observation
keys. Strict V1 terminal errors additionally preserve the verified final assistant
as typed private `provenance.nativeMessageID`; opaque observation IDs are not identity
ports. Legacy missing provenance remains compatible and unqualified. A lower bound
is not request birth. Handoff retains original
`ingressMonotonicMs` and `clockID` from the explicit `clock: {id, now}` port through
all waits. The default `local-performance` coordinate is process-local, useful
only for local durations, and cannot be compared with Go/shared boot ticks.
AN owns clock/age qualification, registration incarnation, durable admission,
child registry/reaping, consent and outward effects. No native content is sent.
Old Go decoding tolerates additive fields; upgraded consumers must retain and
validate provenance before strict admission. No wire v2 is needed.

Run `npm run check` and `npm run check:packed` with the pinned bootstrap. The latter
installs a real tarball in external floor/current/V2-only consumers, compiles both
root API types and strict subpath types, exercises both factory behaviors, and
requires V2-only consumers to contain no V1 SDK. Parent supplies dependencies and
regenerates the stale baseline lockfile before npm ci can qualify this candidate.
Unit/packed checks and supplied native source facts do not prove installed-host
E2E. Exact native adapter compilation, BOTH-version unanswered questions,
reply/cancel during metadata, reconnect/reload, root/child/fork/scope, duplicate
instances, clocks and all five packaged platform cells remain parent qualification.
Program mandatory question/E2E requirements are unchanged; no full-support claim.
