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
Locked phase permits no effect. Released phase invokes exactly the same writer
without acquiring flock. Only two known semantic witness changes are permitted:
disabled_mcp_servers becomes [uap-lock-witness], and its server.enabled becomes
literal false. Config owner/device/mode/singlelink and bounded held reads remain
mandatory. Every other original member remains exact; unknown membership or
foreign mutation refuses before config-body reading. No baseline is renewed.

The vendor disable may pretty-serialize its task-owned config: this qualification
does not claim foreign-byte preservation for vendor writes. Our future append
writer remains separately gated. First complete capture is measurement-only,
all original qualifications false and final workflow assertion stays negative.
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
