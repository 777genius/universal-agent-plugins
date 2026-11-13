# Grok session-only measurement inside a fresh loopback namespace

Original candidate status: API-authored during the outage; tests and native execution
were then UNEXECUTED. Later evidence is recorded below. Root owns publication.
This implements the separately reviewed session-only plan, not plugin enablement.
Source design is exact b79c2ac0; reconciliation preserves fbb3a1cb (including b526d963) and its
ordinary Devin workflow job.

## Exact scope and authority

Seven paths only: existing Grok ACP Python collector/test, existing fixed Go
controller/test, new cohesive loopback_linux.go, existing client automation
workflow, and this plan. Root approved approximately830 changed code/test/CI lines for the
concrete root-kernel/provider/wire negatives, absolute provider deadlines and
source-bound chronological transcript replay regressions. No generic executor/action bus,
production adapter, pin, shared process runner or public status schema changes.

Metadata mode omits mode exactly as before: old argv, request/result schemas,
41-or-43-name bootstrap policy, UUID continuity and original readout remain unchanged.
Only explicit --session-only authors mode=session-only-v1, restricted to scope A.
It uses a separate nonce/root/artifact and remains measurement-only.

The fixed privileged constant runs only after existing unshare --net. It verifies
Linux little-endian amd64 ABI, UID/EUID/GID/EGID0, a different actual net namespace
from the root controller's captured namespace, and exactly lo. Through a40-byte
ifreq it checks IFF_LOOPBACK with SIOCGIFFLAGS8913, sets ONLY oldflags|IFF_UP using
SIOCSIFFLAGS8914, rechecks namespace/interface/flags, closes the socket and
execve's the SAME already-verified setpriv/env/collector suffix. All publisher,
provider and fixture code runs after original UID/GID drop, empty groups, zero
caps and NNP. No host setns, route, interface, sysctl, extra tool or arbitrary
root code/path is introduced. Existing strict cgroup/pidfd family cleanup and
original30-second exchange/35-second total/150-second frontend limits remain.

## Native wire and provider

Four unchanged version/help probes admit pinned1.0.70 syntax. ACP sends exactly
initialize#1, session/new#2 with selected A cwd, empty mcpServers and public
_meta.modelId=uap-local-driver, then _x.ai/plugins/list#3 with the EXACT native
returned sessionId. No synthetic returned ID, prompt, plugin action, trust grant,
MCP/tool invocation or authentication handshake is sent. An unknown client
callback/notification refuses without answering.

Supplementary official source2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8:
agent/mvp_agent/session_setup.rs:388–436 selects the public model;918–921 returns
NewSessionResponse with sessionId, optional models/configOptions/_meta.
Its _meta assembly includes currentWorkingDirectory; validate it if present,
but do NOT require a field absent from the actual pinned binary. No invented
echoed model requirement. A bounded actual sessionId is the correlation authority.
Supplementary public source is not qualification of the pinned binary.

The same file:286–362 defines15 snake_case setup phases and x.ai/session/setup.
Only wire _x.ai/session/setup with exact method/phase/sessionId fields is accepted,
<=32 total and <=2 observations/phase; every nonnull ID joins the returned ID.
Existing empty _x.ai/mcp/servers_updated remains allowed. Unknown phases/methods
and nonempty MCP catalogs refuse; actual binary differences stay negative.

A nonce-derived fixed port is known BEFORE Baseline0. The fresh independently
authored public local-model configuration uses the existing noauth recipe,
telemetry/title_refresh false, and ONLY the inert localhost API token. Never
copy User config/auth or set HTTP proxy controls. After drop, the collector proves
lo UP and starts one provider thread INSIDE that namespace. It accepts at most12
connections with8KiB/32-line headers and5-second per-connection timeout clamped
to the original budget. Only GET /v1/models fixed model metadata and GET / fixed
404 are allowed. Other routes/methods, declared bodies/chunking, bad Host or
duplicate headers fail. No headers, request bodies, authentication values or
arbitrary error strings enter provider facts. Thread close/join precedes result;
an unreturned provider construction or incomplete capture/join emits no result.

## Effects, evidence and honest outcomes

This one-session measurement does NOT adopt unknown session persistence, SQLite,
logs or trust files into the metadata41-or-43 envelope. After strict family settlement,
the parent takes a bounded metadata-only delta and checks original authored
entries. Only the bounded original-config diagnostic below reads a postimage. Returned real-session inventory may
suppress untrusted Project plugins; do not impose the old unknown-session full
registry behavior, fabricate identity or grant trust to force their presence.

All existing qualification flags stay false. collection_complete stays false
for this initial measurement even if the three protocol exchanges finish; an
optional measurement_complete fact distinguishes captured wire from admission.
Its workflow final assertion remains negative. A separate independently reviewed
finite effects envelope is required before any later admission or enable route.

Existing held publisher, ZIP/source/stream hashes, strict cleanup receipt,
independent workflow admission and step-success upload fences are reused. No
second ZIP reader: session readout branches only AFTER the existing bounded
EOCD/member/CRC admission. It emits only fixed mode/reason, bools and counts;
no native IDs, paths, URLs, headers, names, tokens or raw errors. Diagnostic
refusal never revokes an already admitted original ZIP and never extends budget.

The explicit session job needs the SAMEHEAD successful grok-acp-inventory job.
A skipped dependency, queued job or measurement artifact is not native/product
PASS. First run unit tests BEFORE any native command through the existing CI.
Required CI-only validation: Python full contracts including actual fixed-
bootstrap gate tests/injected lo-only kernel calls and inert local provider;
Go fixed-command/result contracts, formatting, compile and existing containment
contracts; workflow YAML parsing, then genuine hosted metadata followed by
session measurement and independent original-artifact review. No local test or
native result is claimed by this API-only candidate.

Source-only review corrected test wire escapes to actual LF/CRLF and made
readout require final LF and refuse every frame after terminal inventory, matching
the live collector. Missing-LF and late-notification archive negatives are authored
but remain UNEXECUTED together with all candidate execution checks.

## Separate native config-lock qualification checkpoint

Source-only candidate, all new tests/AST/format/compile checks UNEXECUTED.
No append writer, nativeconfig modification, plugin action or product activation.
The held pure Grok preparer is not consumed by this lock-only checkpoint.

An explicit lock-qualification-v1 research mode uses fixed scopeA and exactly two
phases, locked-writer then released-writer, on one authored fixture. Metadata and
session request schemas/argv remain unchanged. Same original150s frontend budget,
30s exchange and35s strict cleanup per family; no outer deadline renewal.
Each family validates actual pinned version, mcp --help advertising disable,
and exact mcp disable --help Usage BEFORE attempting fixed argv:
grok mcp disable uap-lock-witness. Different help fails without a writer.
Supplementary2bdd source is not authority for binary1.0.70 lock compatibility.

All profiles are fresh task-owned inputs. Config has both plugin arrays, empty
disabled_mcp_servers, and one explicitly enabled inert witness. Its command is
only a task-owned tripwire script; any execution creates an unexpected member
and refuses. No configured MCP is launched intentionally; no session/model,
vendor credentials, trust grant, plugin enable or original User data is used.

Before launching the locked family, the Go supervisor opens the existing empty
0600 regular single-link native .config-init.lock through descriptor-relative
nofollow parents anchored to its original input directory. It binds caller
UID/GID, device, directory identities and mounts, then holds exclusive flock
through positive strict family cleanup and the final authority check. Only
deferred supervisor cleanup closes the FD. No config content is read or written.
Help now runs under this lock: if it needs the lock, measurement refuses without
fallback. The native writer still runs only after all live help/version gates.
After help, the unprivileged collector probes a separate FD: the locked phase
must receive EAGAIN/EWOULDBLOCK; the released phase must acquire and immediately
unlock it. Other errors or the wrong state refuse. Collector FD teardown cannot
release the supervisor lock. Failed family cleanup grants no lifetime or raw
capture proof. Only after proved cleanup may the parent take the authoritative
original-baseline metadata ledger.
Locked phase permits no change to original members except the existing bounded
request control. The separately measured task-born startup envelope below is the
only new membership admitted. Released phase invokes exactly the same writer
without acquiring flock. Only two known semantic witness changes are permitted:
disabled_mcp_servers becomes [uap-lock-witness], and its server.enabled becomes
literal false. Config owner/device/mode/singlelink and bounded held reads remain
mandatory. Every other original member remains exact; unknown membership or
foreign mutation refuses before config-body reading. No baseline is renewed.

The vendor disable may pretty-serialize its task-owned config: this qualification
does not claim foreign-byte preservation for vendor writes. Our future append
writer remains separately gated. A complete lock capture remains measurement-only
with collection_complete and all product qualification flags false. Its process
exit succeeds only for the explicit lock mode after both semantic/ledger phases,
the final source-clean check, positive cleanup, and no captured error. Session
measurement keeps its existing negative exit; its measurement marker never grants
lock success. This changes no native command, write authority or product admission.
Closed held-publication/ZIP/source/stream authority is reused, with finite phase
artifact names; no new diagnostic archive executor or public CLI option exists.

The lock diagnostic reuses the existing ZIP/EOCD/CRC boundary after mandatory
publication admission. It reconstructs only in-memory stream/RPC receipt fields,
reuses validate_receipt and lock_help, and emits finite phase booleans/counts.
Missing phases stay missing; source/stream/schema/cleanup failures or the SAME
original180s caller deadline yield fixed readout-refused without revoking the
original ZIP. No raw errors, argv, IDs, paths or names are projected. All new
positive/negative archive and real-publication contracts remain UNEXECUTED.

## Measured agent_build phase and selected config diagnostic

Original session run43453065814/job132000359819/artifact14058338742 comes from
9cb6341f42ae833f08f8445db6a14db6b7df93d7, tree02c03ada4906196db11544a40a205075cb54271e.
Replaying the actual stream reproduces refusal at frame10, phase agent_build;
the earlier nine frames pass. No session/new result or inventory was returned.
Public2bdd session_setup.rs blob061460db05fa95afe7d0fa8aba726c34a7bbec5d lacks that
phase. Only this measured literal is added; public-source equivalence is not claimed.
Closed fields,32 notifications, two per phase, actual session ID joins and terminal
ordering remain enforced. Replaying a partial original does not complete it or
retroactively make its stored RPC equal the newly parsed notification sequence.

Only the original config.toml changed:405 to458 bytes via atomic replacement,
same0600/singlelink/device/owner; body unread. accepted=false is intentional.
protected_originals_unchanged=false accurately reports the replacement. Public
marketplace migration and the52-byte ACP marker suggest a cause, without proof.

The next normal session attempt can capture only home/.grok/config.toml after
both validated family-cleanup receipts. Existing before snapshots remain the
authority: root/home/.grok identities and all other original entries must match.
Require regular0600, singlelink, original device/UID/GID and at most16KiB before
reading. Reuse Devin read_bootstrap_file's descriptor-relative nofollow reader,
then require equality of the complete after metadata map. No new reader or
whole-profile snapshot is introduced; session/cache/log bodies stay unread.

Optional selected_config_capture stays inside the existing a.ledger.json: closed
schema1, original identity/bytes/SHA joined to the shared snapshot and unchanged
nonce-based authored recipe, then post identity/bytes/SHA/base64. Failure retains
null with no partial body. The existing readout validates these bounded joins,
accepts historical records without this field, and prints no new raw information.
ZIP names, wire schemas, commands, provider policy and deadlines stay unchanged.
Metadata accepted=false, computed original-preservation, collection_complete=false,
all qualification flags and the final workflow fence remain unchanged.

Inert contracts cover phase/order/bounds, real pre-read hooks, atomic postimages,
midstream/whole-metadata changes and malformed optional records. Native validation
remains pending; no enablement, recovery or Project activation is claimed.


## Actual da680106 lock capture and task-born startup envelope

The original run43468549490/job132057540236 failed the final measurement gate.
Artifact14064428474 is the5350-byte outer archive with SHA-256
ca5ca085d7017d7d41986197ef855c7d156f2c07dcd4ec546863da6660d945cc;
its sole research.zip is36817 bytes, SHA-256
8bd7506918cfddcd427d4f3e075f5daa96b68071e0cd0ec380108c1613a5cd9d.
Both CRCs, exact eight inner members, source/nonce joins and all four native
stream byte counts/hashes were independently checked. The source is da680106,
not the later8d008cdb supervisor-held lock implementation. This original cannot
qualify the corrected lock lifetime through strict family cleanup.

Version and both help probes exited0. The actual locked native writer exited1
with the bounded timeout waiting for its config-init lock. Receipt semantic
completion, observed lock ownership and both cleanup facts were true. The
original authored302-byte0600 config and every other original member remained
unchanged. Exactly31 new members appeared: docs, docs/user-guide and logs
subdirectories; the27 already pinned public user-guide documents; and one
693-byte0644 logs/unified.jsonl. The old exact-membership guard correctly refused
before reading config, so no released-writer phase ran. All product qualification
flags remained false. No output from the unknown log was used as authority.

This follow-up permits only that exact task-born31-member envelope. It reuses
the existing27 literal document length/SHA-256 pins, requires original root
owner/group/device, directory0755, regular single-link file0644, exact document
sizes and a128-KiB bound for the sole log. It admits all metadata and preserves
all original entries before any new body read. Descriptor-held nofollow reads
then check only those27 public documents against their literal hashes and the
existing bounded config semantics. No log body is read. Parent, leaf and root
identities are rejoined and the complete metadata inventory must remain stable.

The first fully accepted capture freezes the exact created identities separately
from Baseline0. The released phase requires that same closed membership, exact
document/directory metadata and unchanged log inode/mode/owner/device; only the
new task-owned log may change its bounded contents. No original member or prior
log is adopted, no source baseline is renewed, and any extra name, alias, owner
change, rotation, replaced parent or document mutation refuses. The config still
must show precisely both witness-disable semantic changes after lock release.
This fixture exception grants no recovery or write authority over a real User
profile and does not consume the held prospective plugin preparer.

Local verification on the reconciled follow-up: all71 Grok ACP Python contracts
pass, including actual new-file creation, both phases, unchanged Baseline0,
wrong pinned content, unknown membership, original-log refusal, aliases, file
bounds, ownership and post-creation replacement before body reads. The previously
published original remains a valid closed negative readout. Native qualification
of this follow-up and released-writer behavior still require new hosted evidence;
no native or product PASS is claimed here.


## Actual session parser cutpoint: measured agent_build phase

Original run43468549490/job132057540198 also reached its measurement assertion
without qualification. Artifact14066155399 is10494 bytes, SHA-256
1a9aa7a036363775826e7cb6b8126234dd3832cfa5636a16d1567250532696d4;
the closed nine-member research.zip is52283 bytes, SHA-256
5d2fb30e399276928e1464c0d05c75b0c4f5902190f3deb48e3949ed556fe6a3.
Both CRCs, exact da680106/treeeba42 source and all five stream hashes/lengths
were independently checked. Native1.0.70 initializes and sends nine setup/MCP
notifications. The final actual frame is `_x.ai/session/setup` with the literal
phase `agent_build` and the same nonnull session ID as persistence_init and
spawn_session_actor. The older supplementary2bdd source has15 enum phases and
does not contain this phase; the measured pinned binary is the authority here.

The old finite parser rejected that exact ninth frame and stopped ACP with
protocol-refused. Its provider had only one models GET and one root GET, zero
forbidden requests, and completed settlement; both strict cleanup facts were
true. No session/new or plugin-inventory reply was received. Metadata-only
measurement found48 additions, zero removals and one original config change;
session SQLite/WAL/SHM and temporary files remain unadmitted. No newly created
private body was read. All existing qualification flags remain false.

The follow-up adds only the literal measured agent_build phase to the closed
enum. It retains notification counts, exact wire keys, bounded IDs, chronological
replay and final returned-session identity joins. A new inert regression fails
on the old parser and passes after the one-value change. Root also replayed the
actual captured prefix: nine notifications are accepted, while session and
inventory responses remain absent and only the planned session/new request is
sent. The old detached RPC recorded only eight accepted notifications, so its
original negative diagnostic is not rewritten or promoted by this parser fix.
The next genuine run must establish whether session setup proceeds further;
this change admits no session effects, prompt, trust, enable or tool action.


Reconciliation note: external2c19d826 already supplied the measured agent_build
phase plus its bounded selected-original-config diagnostic and stronger phase
regressions. Those exact changes are preserved; the duplicate local phase test
was not replayed. The reconciled runtime delta adds only the separately reviewed
lock-startup envelope, and preserves both the selected-config capture call and
the lock-created identity argument in their respective modes. Both independent
original da captures above remain attributed to their actual historical source.

## Measured no-MCP initialization notification (2026-11-13)

Original source `b397682f3600a75207237a72a617b45b2737be17`, tree
`cf823249a6d6f413e15d3d628db3db54f6d966f3`,
[run 43495420970, session job 132172862340](https://github.com/777genius/universal-agent-plugins/actions/runs/43495420970/job/132172862340)
produced artifact `14080385857`: outer ZIP 11300 bytes,
SHA-256 `18243d45163f931e2b909f676a0b230e5499ce1654a167ab8bbf02d94e472c14`;
inner `research.zip` 54987 bytes,
SHA-256 `57473d7afafd6aadc04cbe25c7dc1e51abe00e38c17e32818303e79c72368813`.
Both CRC checks, all five stream joins and the original source/receipt join were
independently verified. Both cleanup facts were true. The ACP command ended
with exit -9 and `protocol-refused`; no session or inventory reply was received.

Replay located the first refusal at frame 12: `_x.ai/mcp_initialized`, with
exact parameters `sessionId`, `mcpToolCount: 0`, `elapsedMs: 0`. Its nonnull
session ID matches all four preceding nonnull setup IDs. The 143-byte frame's
SHA-256 is `a1b6e208ca78d06c715de73be1fb9b1a3dbfecd94806938e28ebb51b73f94709`.
The correction admits only this closed notification after initialize and before
terminal inventory, once per session. Its tool count must be strict integer
zero; elapsed time must also be strict integer zero, as in the original native
frame. A process deadline does not establish other notification values. The
bounded printable ASCII ID must join prior
and subsequent setup IDs and the actual `session/new` result. Unknown methods,
keys, callbacks, duplicate notices, booleans, negative values and foreign IDs
still refuse. Existing frame, notification, phase and process bounds remain.

Supplementary public source at `xai-org/grok-build` commit
`2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8`, blob
`f4d6e17fcdcd0cb7694fc2d7818f516a17c9f76f`, emits literal zero/zero in
`finish_init_without_handshakes`. Its general handshake path has variable
elapsed time and is not admitted by this empty-MCP measurement. A separate
negative contract checks the actual response identity in both orders without
earlier setup IDs masking that join. Historical detached readout stays refused.

The initial correction at `9d201619` added three inert parser contracts that
reproduce the old `notification-refused`; all 77 existing-file contracts passed.
The stricter zero/zero composition with the lock exit correction passes all 124
Grok contracts. The actual
12-frame original prefix now parses with 11 notifications, no session/inventory
reply and only the planned `session/new` request. Its historical detached RPC
contains only 10 notifications, so original readout still refuses at
`session-wire-join`. Original archives and qualifications are unchanged; the
fresh native run must establish any subsequent progress. No effect envelope,
native command, prompt, trust, enablement or collection-complete rule changes.

The separate lock original from the same source and run, job `132172862093`,
artifact `14079492925` (10101 bytes, SHA-256
`ce01526399da4e503ecfb9cec0895f3415a134f1b6239780351b790da590fda3`), also
passed independent outer/inner CRC, eight stream and two receipt joins. Its
unaltered reader reports both held and released phases as measurement-only:
the native writer refuses under the held lock, then succeeds after release;
31 exact startup members and the immutable original baseline remain joined.
This confirms the measured lock behavior, without enabling Project activation,
accepting session effects or supplying the still-separate recovery proof.
