# Grok session-only measurement inside a fresh loopback namespace

Status: API-authored review candidate; tests, AST, Go formatting/compilation and
native execution are UNEXECUTED because the workspace execution service is
unavailable. No branch ref is changed by this candidate. Root owns publication.
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
entries. It reads NO newly created bodies. Returned real-session inventory may
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
