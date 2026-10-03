# Shared native observer authority ports

Ownership: pure `install/integrationctl/opencodehost` prerequisite only. The AN
helper, SDK readers, installed candidate and OS image checks below are downstream
work, not implemented or qualified by this patch. There is no time policy T,
clock grant, business IPC eligibility boolean or second resolver in this package.

## Public Go port and trust boundary

`NativeObserverEvidence(version) (NativeObserverDescriptor, bool)` returns a
detached value snapshot of fixed native/source eligibility. The boolean means
only that a descriptor exists. Lookup does not grant observer support.

`BindNativeObserver(e VersionEvidence, actual NativeObserverTuple) Profile`
calls the existing `Resolve` and compares every tuple field with that one fixed
descriptor. It requires `e.Source == "host_runtime"`, `e.ProbeStatus == "ok"`
and a nonempty `e.ExecutableIdentity`. The returned process-local profile retains
the qualified tuple and opaque identity privately. It can be passed directly to
the existing `Select`. Its four observer support flags are presentation snapshots;
`Select` derives observer authority from the private binding. JSON roundtrips,
caller-created profiles and public support-map changes cannot create that binding.
Generic selector behavior and the generic `EvidenceID` remain unchanged.

The binder is explicitly trusted-caller composition, not authentication or live
attestation. A caller can spell any string in Go; this pure package cannot know
where it came from. The AN Go helper owner must construct these inputs from actual
OS identity, the existing shared explicit-target probe and independently verified
native reader/adapter ports. Never deserialize a config/event/IPC tuple into this
call, relabel a cached installer probe as `host_runtime`, or translate an incoming
`Supported`/`runtimeEligibility` boolean into authority. A serialized profile is
diagnostic output only. If JS needs helper output, the helper owner must separately
bind its authenticated owned invocation and immutable reader lifetime; flags alone
are insufficient. No such protocol is added here.

## Exact fields and descriptor IDs

All fields below participate in exact equality, including the four hash slots.
Descriptors have `NativeStatus = native_source_qualified` and exactly
`observer_completion`, `observer_question`, `observer_permission`,
`observer_terminal_error`; there is no generic or clock capability in this list.

| Tuple field | Fixed value or source |
|---|---|
| `Version` | Exactly `1.18.33`, `1.18.34`, or `2.0.21`; must equal probed live version |
| `ImageSHA256` | Official copied executable pin in the table below; actual OS-bound image must match |
| `GOOS`, `GOARCH` | `linux`, `amd64` |
| `Entry` | `official_native_serve_default_dual_autoload` |
| `Adapter` | `observer_v1` for the two V1 hosts, `observer_v2` for V2 |
| `ReaderContract` | V1 `v1_source_causal_sync_callback_tombstones`; V2 `v2_direct_asynciterable_data_sparse_error_end_checkpoint_after_prepare` |
| `ProvenanceBasis` | V1 `v1_assistant_completed_or_assistant_created_lower_bound`; V2 `v2_native_envelope_created` |
| `UpstreamCommit` | Exact upstream commit in the table below |
| `NativeAdapterSHA256` | `d2d75185eb6283d6f8a34d2019aa201ada9ec9ea9db578b4f2f28dd7d9d8d901` |
| `EvidenceHashes[0]` | Amendment `219b716d5ef855bd35e4c3b18132d49535a1e43e8fac0952bb78b963d4c63e76` |
| `EvidenceHashes[1]` | Live-host proof `5ad9f81ce36cf2e1b2b424a74ca53d991fe75c8af3f71a4654b6a563a74d89cb` |
| `EvidenceHashes[2]` | Official artifact manifest `ab4435c0c8c1e9dbfbc731a490e9a01042428cdd9e84e426f13e50cff333986a` |
| `EvidenceHashes[3]` | Current four discriminators `e9baee317279d5fc8c421becc6d5cbf7096f9fc0146f3b130b07b5ed7af057ed` |
| `EvidenceID` | `native-observer:<Version>:sha256:<NativeAdapterSHA256>` followed, in order, by `:sha256:<EvidenceHashes[i]>` for all four slots |

| Version | Official image SHA256 | Upstream commit |
|---|---|---|
| 1.18.33 | `0abbb7c32ab0294c0a7bfa2705f9ff0df5dce5ab721d1f00cccfe393f2a11427` | `51ef4be1d3c122f18fefb510dca8d778571f4f18` |
| 1.18.34 | `9ca0b9953d49997601655e54f846a3efa464f237e47c6f1b04716d0f2e64c4c2` | `aec0b9a6d8898f68f923aaf08b7306d931fd9d76` |
| 2.0.21 | `f916986543348d7953d8d43aa048516cdbc3f84f4d0dc9c0c5b9d1da3030cea7` | `8a8bd622a3d7dc29ccf30ec17f84e363ed95ed72` |

The current-four proof supplements the amendment's stale 1.18.34 completion/error
gaps with terminal retry/interrupt, scope, manual and AUTO native records. Its
report hashes, source pins and native trace bindings are preserved in the durable
`evidence-bindings.json`. Old documents and evidence remain unchanged. This does
not inherit permission support from 2.0.0 or qualify any other version or cell.

## Trusted helper inputs and shared call sequence

1. Establish the owned actual loader instance and root/location binding. Treat
   public `process.execPath` as a candidate private absolute target, never as proof
   by itself. The helper must verify absolute, resolvable, non-wrapper identity;
   kernel executing image, launched image and public target agree in SHA256 and
   device/inode; the loader belongs to the owned live process and remains alive
   across reads. Hashing the current disk path alone cannot attest mapped memory
   after replacement. Establish the cooperating-runtime boundary, reject observed
   replacement/change, and retire authority when identity becomes uncertain.
   Do not infer compiled identity from argv/PATH or demand Bun's optional standalone
   API on V1, where the supplied native proof observed it absent. All these checks
   still require actual installed helper proof; this package performs none of them.
2. Use existing `ProbeOpenCodeTarget` on that exact verified target, preserving
   its bounded output, isolated cwd and allowlisted environment. Keep the helper
   and nested version process in the single owned child registry (weight two),
   retain slots through actual inner close/reap, and abort startup on unresolved
   children. The helper must bind the resulting version to that same live identity;
   a successful probe of a mutable path or installation cache is insufficient.
   V2 `ctx.app.version` must independently agree with the shared probe. V1 has no
   public client health/version API. Unknown/default metadata stays unverified.
3. Construct `VersionEvidence{Version, Source:"host_runtime", ProbeStatus:"ok",
   ExecutableIdentity:<private opaque verified live fingerprint>}` in trusted Go.
   The helper keeps its full actual live identity and reader lifetime separately
   for revalidation; the fingerprint is not a public path or a serialized grant.
4. Look up the exact descriptor. Independently establish actual image/platform/
   entry plus the reader, provenance and native adapter contracts below. Prepare
   the consumer while business effects remain disabled; V2's initial same-reader
   consumed checkpoint follows that preparation. Keep the verified reader epoch
   tied to this runtime identity. Use the
   pinned evidence/source values as contract identifiers, not evidence generated
   by the current runtime. Fill `actual` from verified observations and the
   enforced fixed contracts. Copying the lookup tuple without these checks cannot
   authorize a live runtime.
5. Call `BindNativeObserver(e, actual)`, then `Select(profile, requirements)` with
   adapter `observer_v1` or `observer_v2`, `local_plugin_dual` and each required
   notification capability. The mandatory product selection requires all four.
   Only a nil error with a nonempty artifact/adapter is a selection. Generic skill,
   MCP and placement decisions continue through their existing independent paths.
6. Keep this result within the one verified runtime/reader epoch. Revoke it on
   identity/scope/reader change or uncertainty; reinstalling bytes or reusing an
   old profile cannot revive it. Selection and separately qualified clock/admission
   gates must pass before the prepared adapter enables business effects. This
   patch neither starts an observer nor grants business IPC or clock qualification.

## Native reader and provenance obligations

V1: host-provided root `input.client.session.get/messages` are genuine ports;
`question.list`, `permission.list` and `global.health` do not exist. The qualified
reader is the source-causal hook callback with an await-free synchronous ingress
prefix, future-live Asked tokens only and bounded close-before-ask tombstones.
Replied/Rejected immediately invalidate exact session/message/call/request tokens.
Hydration/reload cannot reopen old requests. Source cache deletion may precede
close publication, so this is consumer-ingress relevance, not a settlement lease.
Completion provenance is native ordinary `assistant.time.completed`; question,
permission and permanent error use native `assistant.time.created` with the
`assistant_created_lower_bound` label. Receive time and tool start are invalid
substitutes. Require unique current nonsummary ordinary assistant/root and final
matching permanent error; idle/error alone, retry, abort and compaction are silent.
AUTO's new continuation user before compacted cannot be blocked by a sticky flag.

V2: use the actual `ctx.event.subscribe({signal})` direct AsyncIterable with native
`data`, sparse relative durable sequence and real end/error/overflow fences.
Never require last+1 adjacency or apply this descriptor to external/SSE/replay
readers. Real `ctx.session.get/context`, `ctx.permission.get/list` and
`ctx.rpc.register` return direct values. No form-list, execution-get, message-get
or session-log port may be invented. Prepare the independent consumer and owned
RPC registration first, then emit and await one actual initial checkpoint consumed
by that same reader before future-live form eligibility. Emit completion alone is
insufficient. Each eligible question's post-metadata checkpoint and immediate
pre-effect token rechecks follow the accepted native adapter contract. This is
checkpoint plus consumer-ingress relevance, not a source settlement lease.
Execution.started.id is the run key; terminal.id is distinct. Bind actual assistant,
tool/request and terminal identities, native envelope.created provenance and real
root scope. Missing execution.location needs observed scope or bounded native
metadata; load directory is not observed scope. Fork lineage is not parentID.
Compaction, step.failed and execution.interrupted are silent; execution.failed
needs the matching final ordinary assistant. Metadata/reader/admission bounds and
clock checks remain the downstream owners' work.

## Fail-closed states and remaining qualification

Plain `Resolve` and descriptor lookup leave all four exact-version observer flags
literally `unverified`. Absent/wrong tuple or missing trusted version/identity
returns the ordinary unresolved profile; exact hosts yield `unverified_capability`
from observer `Select`, with empty usable artifact/adapter. Unknown/incompatible
families can yield `no_adapter`; preexisting unsupported generic semantics remain.
Malformed requirements yield `invalid_requirement`. Serialized supported flags
yield no observer authority. Unknown patch/minor/major, prerelease/build, platform,
entry/wrapper, image, reader, basis, adapter, source or evidence hash never inherit
a descriptor. Generic Supported never substitutes for observer qualification.

Still unproved: installed helper OS/live-image checks and normal/abort inner reap;
real candidate enforcement of both native reader contracts; four positive facts
and stale/race/compaction/root/duplicate negatives through the installed dual
bundle; independent native-age/Go-JS clock calibration and admission; generic
installed skills/MCP; all five packaged platform cells with both hosts. Supplied
Linux amd64 native serve evidence and ordinary pure Go tests prove no full E2E,
product support, TUI/desktop/wrapper support or executable release qualification.
