# VS Code 1.143.1: shipped-workbench workspace observation

2026-10-24. Plan only. The genuine help/version artifact at source `0a34182a`
is [preserved with provenance](../../evidence/project-scope-research/vscode-1.143.1-linux-amd64-40132182435/README.md).
Root and the implementing worker independently verified the outer/inner ZIPs,
all seven text members/six process streams, exact version/publisher commit and
namespace/privilege proof. Both admit only the next observation plan. Obtain
two independent plan-review rounds, then two frozen code reviews before any new
native publication. No VS Code adapter is included here.

## Required gate and observation meaning

The original all-client plan, section 14, requires genuine workspace evidence;
help/version does **not** close that gate. Actual help labels --add-mcp as a user
profile writer and supplies no read-only workspace inventory command. Do not use
that writer, a Copilot CLI listing, the old NewLocal TEST tuples or a synthetic
extension-host shim as a substitute for the shipped workbench.

This next collector opens only inert task-owned single-folder workspaces in the
actual pinned shipped editor, with one reviewed test extension. It observes the
editor's configured MCP definitions and parsed skill metadata without a chat
request or server start. It does not establish effective Local/Copilot session
selection, trust acceptance, MCP initialization, skill execution or model use.
Workspace trust remains enabled/ungranted; require the expected selected local
folder and actual isTrusted=false. Pinned workspaceTrust.ts was checked: disabled
trust would report true; no extension-development exception was found in its
calculation. An unexpected true value fails this untrusted-stage contract rather
than becoming trust evidence. A trust or policy block remains explicit negative
evidence rather than a reason to bypass it.
Even a successful inventory leaves native_project_qualified false. After actual
artifact interpretation, a separately reviewed concrete adapter plan is still
required. The original stronger after-trust/runtime proof remains unqualified.

## Exact pinned source and limited observer APIs

Pin the same publisher version 1.143.1 / commit
`e646868227c3e3a9e3f24277c8c7220be42c0be7`, archive length/digest and required
ELF/CLI/product hashes. Reuse the previously reviewed acquisition, fixed-header
streaming extractor, CRC/TAR closure, descriptor containment, required-file
checks, budget, held-root cleanup and bounded upload admission. No npm, VSIX,
marketplace, new package dependency or publisher update is fetched or run.

The immutable primary source exposes two **proposed** read-only APIs suitable
only for this pinned research extension:

- [mcpServerDefinitions.d.ts](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/src/vscode-dts/vscode.proposed.mcpServerDefinitions.d.ts): `vscode.lm.mcpServerDefinitions`, synchronized from all editor collections by [mainThreadMcp.ts](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/src/vs/workbench/api/browser/mainThreadMcp.ts). Subscription/publication reads definitions; launching/gateway APIs are separate and forbidden.
- [chatPromptFiles.d.ts](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/src/vscode-dts/vscode.proposed.chatPromptFiles.d.ts): `vscode.chat.getSkills(token)`, dispatched through [mainThreadChatAgents2.ts](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/src/vs/workbench/api/browser/mainThreadChatAgents2.ts) to the actual prompt service's parsed skill inventory. It does not request model invocation.

[extHost.api.impl.ts](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/src/vs/workbench/api/common/extHost.api.impl.ts)
requires the exact proposals `mcpServerDefinitions` and `chatPromptFiles`. Enable
only those for the one `uap-research.workspace-observer` test extension using
its manifest and the exact --enable-proposed-api ID; record this research-only
instrumentation. Do not enable an agent harness, alter product user settings or
claim a stable public inventory API. Missing/denied APIs fail that stage; no
private command, monkeypatch, service injection or API substitution fallback.

[MCP conversion](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/src/vs/workbench/api/common/extHostTypeConverters.ts)
returns labels and resolved HTTP/stdio configuration, but drops collection/source
IDs. Record that limit: exact unique marker URL attribution is an inference from
the sealed fixtures, not a native-emitted config-file path. Skill metadata does
include URI and source. Preserve all native rows/duplicates; never collapse by
name before checking attribution. The source's workspace/personal skill priority
is a hypothesis until an actual same-name observation supports it.

### Exact nonprivileged executable modes for the GUI slice

The help collector's extractor intentionally creates every regular file 0600 and
only enables its top-level code ELF. Do not silently reuse that mode policy for
a GUI, restore arbitrary TAR modes or turn on setuid/setgid bits. After the exact
archive length/digest and streaming extraction succeed, the workspace collector
permits mode 0555 on **exactly** these two regular, single-link, bounded ELF paths:

- VSCode-linux-x64/code;
- VSCode-linux-x64/resources/app/node_modules.asar.unpacked/@vscode/ripgrep-universal/bin/linux-x64/rg.

Check no-follow parent/leaf identity, ELF magic, individual length/hash and exact
archive-derived source before sealing; record both individual digests. All other
publisher files are read-only ordinary data, and root-owned directories are
non-publisher-writable. The archive's privileged/other executable mode metadata
cannot authorize another executable. A missing/mis-typed required path or any
additional required executable stops the stage for review, without exploratory
native probes or permission widening. Keep the old help collector's default
materialization behavior unchanged.

The exact pinned [packager](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/build/gulpfile.vscode.ts)
unpacks the ripgrep-universal bin tree; [platform filtering](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/build/lib/copilot.ts)
keeps linux-x64, and [rgDiskPath](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/src/vs/base/node/ripgrep.ts)
rewrites node_modules.asar to node_modules.asar.unpacked. That is source support
for the fixed search-helper path, not a runtime success claim. Pass the fixed
--disable-crash-reporter flag: [mainImpl.ts](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/src/mainImpl.ts)
configures reporting only on its explicit directory/runtime-preference path;
there is no need to enable an unobserved crashpad executable. Never enable
chrome-sandbox setuid mode or disable/replace the real Electron sandbox. A real
sandbox/helper failure remains a failed observation stage. Inert tests cover
exact allowlist, missing/non-ELF/link/alias refusal, read-only sealed modes and
all privileged/unlisted executable metadata being ignored/refused.

## Reviewed observer extension

Add a small repository-authored CommonJS extension and test entry with no runtime
dependencies, providers, commands, tools, hooks or network calls. It declares
untrusted-workspace support only so observation can run without granting trust.
The test entry exports the documented async test runner shape, reads workspace
folders/trust, the two metadata APIs and resource-scoped read-only configuration
inspection. It writes one bounded final JSON result, then keeps the test-runner promise
pending until the controller requests planned kernel shutdown. Returning normally
could cause the extension-test leader to exit before the strict shutdown boundary,
so a final file is not permission to return/exit. It never registers
skills/MCP definitions, creates an MCP gateway, starts a server, invokes a prompt,
modifies settings, installs extensions, accepts dialogs or reads resource bodies
outside the exact inert marker set.

Allow at most 128 native MCP/skill rows, 512 KiB result JSON, bounded strings and
portable expected local URIs. Record actual API shapes before semantic assertions;
unknown shapes, duplicate JSON keys, missing fields or output overflow fail.
Subscribe/poll to allow normal asynchronous discovery for at most 15 seconds;
record the final raw inventory, including empty/blocked results. Record the
read-only effective trust, skills-enabled, MCP-access and MCP-autostart setting values; do not
copy the entire settings/profile. Empty markers with untrusted context are a
blocked attribution stage; do not invent a causal trust explanation without
explicit native evidence. Parent validates
an ordinary single-link stable result file under the exact per-probe root before
reading. A result from another probe/root or before the current process starts
is refused. The source extension/fixtures are hash- and type-checked before and
after every native process; output cannot authorize another native command.


The result schema records publisher version/appRoot/appHost, extension identity/
mode/proposals, exactly one selected folder URI and trust boolean, read-only
policy values, and bounded native arrays. MCP rows contain actual label/version
and HTTP URI or stdio fields as exposed by the API (no invented origin path).
Skill rows contain actual URI/name/description/source/sessionTypes and invocation
flags. Serialize data only, not API functions/providers; bound every string and
record unknown/unsupported shapes as a failed contract. No returned field is
used as a path to execute or a source of controller arguments.

## Hosted-only workbench execution and actual tree containment

Keep the real unchanged hosted admission before any output/acquisition/process,
exact HEAD verification, contents-read checkout, no persisted credentials and
explicit clean child environments. Use the same actual original nonroot IDs,
network namespace separation, loopback-only interfaces, zero capabilities,
empty groups and NNP proof immediately before each publisher exec. No vendor
process runs locally, and no hosted guard or namespace proof is spoofed.

The full GUI can spawn detached descendants. The existing Python process-group
helper alone is insufficient for this new boundary. Reuse the repository's strict
`process.OS.RunDuplexWithPlannedShutdown` in a dedicated fixed-purpose research
Go helper. Its Linux path requires atomic CLONE_INTO_CGROUP creation and kernel
cgroup teardown; do not add a second process manager or loosen that primitive.
Build the helper from exact reviewed source with the existing pinned Go workflow
setup, recording its digest. Build is nonroot, has a 60-second deadline and live
1 MiB limits on each output stream, and may use only the already pinned module
set. A build failure remains bounded evidence; no on-the-fly package setup.
Its sole task is the finite sequence below, not an
arbitrary command/JSON execution endpoint. No production process-runner change.

The hosted controller uses sudo only for this fixed audited helper so existing
strict containment can create task-owned cgroups. The helper itself executes no
publisher code while privileged: construct literal unshare/setpriv/env-i plus
the existing same-process nonroot/network/capability verifier before the ELF.
Before native work, the fixed helper requires actual/effective root controller
identity and validates the original nonzero real/effective UID/GID against the
trusted sudo-populated SUDO_UID/SUDO_GID and the frontend's pre-sudo measurement;
arbitrary command-line IDs alone cannot authorize a different identity. The
frontend itself must have matching nonroot real/effective IDs. The helper seals
its own executable, publisher tree, observer source and fixture executable parents
into task-only root-owned, non-publisher-writable directories, verifies the exact
reviewed hashes/types, and checks them before/after every run. Root-private
control paths are mode 0700. Only declared HOME/profile/result directories are
writable by the original identity; untrusted results cannot replace executables.
Validate every derived path before launch.
Task data/results are untrusted data, never executable controller inputs. Failure
to obtain the required cgroup/CLONE_INTO_CGROUP or namespace capability stops
before workbench code; no ordinary-runner/process-group/no-sandbox fallback.

Use only the already installed absolute system Xvfb and xauth executables; do
not apt install them. Record ordinary root-owned executable identities/version.
Missing tools fail before publisher GUI launch. Use the literal task display :93,
Xvfb -nolisten tcp, and one random task-only Xauthority cookie. Before launch,
lstat the exact /tmp/.X93-lock and /tmp/.X11-unix/X93 and require both absent;
a collision fails without querying or connecting to that display. Xvfb uses the
same strict contained lifecycle and original nonroot identity. Its same-process
verifier records its PID before exec. Within ten seconds, readiness requires the
new ordinary lock to contain that exact PID and the new Unix socket to have the
expected UID and stable identity. A preexisting/replaced socket or wrong PID/UID
fails before any GUI process; no attach-or-retry display search.

Create the authority file with the fixed xauth -f <exact-file> source - command,
feeding only the reviewed add-display operation plus cookie on its explicit
stdin. Cookie bytes never appear in recorded argv or controller summaries. Give
the native identity read-only access through a root-owned nonwritable auth parent
and file; keep it separate from mode 0700 controller assets. Exclude the file from
upload and redact any occurrence in bounded xauth diagnostics, explicitly marking
that stream as redacted. This is task display isolation, not product/account
authentication. No inherited DISPLAY/XAUTHORITY or new vendor shell wrapper is
used. The reused strict primitive does perform its existing fixed inert shell
loop capability probe; that established probe is not an editor invocation.

Run shipped ELF directly as an editor (without ELECTRON_RUN_AS_NODE), explicit
fresh user-data/extensions dirs, --new-window, exact extensionDevelopmentPath,
extensionTestsPath, the one proposal-enabled extension ID and one selected local
folder. The exact argument spelling is in pinned argv.ts; no default chat call.
Never pass --disable-workspace-trust, --no-sandbox, --disable-setuid-sandbox or
change kernel/AppArmor policy. If the real Electron sandbox cannot initialize,
retain its bounded diagnostics and stop. No second archive/runtime fallback.

### One nested lifetime owner and exact cleanup reserve

The helper installs its signal/overall contexts before creating any child. One
outer RunDuplexWithPlannedShutdown call owns Xvfb. Its callback starts the bounded
Xvfb diagnostic drain, proves readiness, then runs all four inner GUI strict
calls sequentially. Each GUI exchange watches only its current result, validates
it, and returns nil solely to request planned kernel shutdown; it returns a
non-nil cause on missing/invalid result, overflow or deadline. It never returns
nil just because the publisher exited. Join each inner strict call and its
bounded drain before starting another. Only after all inner trees are reaped
may the callback return and request Xvfb teardown. A failed stage stops the
remaining table, then cleans the current GUI tree before the display tree.

Use separate work cancellation and display-lifetime contexts: cancellation of
a GUI/API work deadline signals the current inner exchange first, and does not
simultaneously kill the outer Xvfb owner while that inner tree is being reaped.
An external signal requests the same ordered shutdown. The outer context is a
later independent safety deadline, not the parent canceled work context.

Reuse the exact process primitive, not copied kernel code. At the current source,
Linux creates an exact task cgroup beneath its own unified cgroup2 membership;
required events/kill/procs files and an open cgroup FD precede atomic
CLONE_INTO_CGROUP/Setpgid launch. Each invocation first runs the existing strict
capability probe. Planned teardown allows a one-second pidfd stop/causality wait,
then cgroup.kill and up to one second for leader exit plus populated=0; exceptional
leader-kill fallback can add one second. Sole Wait/reap has a one-second bound,
WaitDelay is100 ms, and containment close owns only its created cgroup. The
ordinary two-second duplex exit grace applies to the nonplanned natural-exit
path, not as a substitute for planned teardown. Reserve ten seconds per strict
scope including its established preflight/cleanup/fallback windows and report
any failure from the primitive. These are bounded userspace operations; an
uninterruptible kernel syscall is not promised an impossible hard bound.

The existing strict runner retains only a bounded32 KiB diagnostic stderr buffer
and does not export a successful stderr stream. Therefore, immediately before
exec, the same-process nonroot verifier redirects stderr to stdout for Xvfb and
GUI children. Each exchange drains one explicitly **merged** diagnostic stream
with a live1 MiB retained limit; overflow cancels work and cannot become a nil
exchange result. Keep result JSON separate (512 KiB). Join drains after the
primitive closes their pipes, bounded by the enclosing cleanup reserve. Record
merged byte counts/hashes, overflow and planned stop; never invent separate
stderr=0 or a natural successful exit. Planned teardown can truncate a final log
tail, which is stated separately from the validated final API JSON boundary.
Pre-exec verifier failures retain the primitive's bounded diagnostic error.

All absolute offsets below start at the frontend's monotonic eight-minute
budget before acquisition/build. Check remaining budget before each phase; do
not start another inner GUI if less than 65 seconds of work budget remain. Each
GUI has a 45-second work bound including its 15-second API discovery wait; Xvfb
readiness is ten seconds. At 420 seconds stop/cancel work, allowing up to ten
seconds to finish inner teardown and another ten for outer Xvfb teardown. The
root helper then gets an independent five-second held-root cleanup window for
its root-owned assets and returns its bounded summary by 450 seconds. Its normal
return deadline precedes the frontend watchdog: at 460 seconds the watchdog sends
TERM, permits signal cleanup until 470 seconds, then may kill the helper only as a failed,
unproved-cleanup outcome. Reserve the last ten seconds for frontend-owned
filesystem cleanup (five) and upload admission (five). Partial collection is
allowed if earlier acquisition/build consumed the work window; deadlines are
not silently extended to finish the table. No new child starts during cleanup.

Root-created control/sealed trees are cleaned by their root helper owner using
the existing anchored, descriptor-relative bounded cleanup pattern; the
nonroot frontend cannot claim to remove root-owned descendants. Frontend cleanup
handles only its own held task root. A killed controller, cleanup timeout or
unproved cgroup emptiness forces collection incomplete. Never convert VM
disposal or a missing process handle into successful cleanup.

### Every-child capture and upload admission

GUI/Xvfb diagnostics are merged as above. The helper build has separate live1 MiB
stdout/stderr limits and 60 seconds; fixed system identity/version and xauth setup
commands have five seconds and 64 KiB per stream. Controller stdout/stderr each
have 64 KiB, and no child command or cleanup failure uses an unbounded fallback.
Every child is in a literal finite command table; no executable/argument comes
from metadata. All retained raw/redacted streams, four <=512 KiB API results,
bounded identity/cgroup/cleanup records and failure summaries share an enforced
<=32 MiB/64 flat-file envelope. Stop on cumulative exhaustion. The previously
reviewed five-second separate ZIP_STORED admission and independent upload gate
remain the only upload path; rejected raw trees yield a bounded false-claim
failure ZIP, never an always-upload of raw profiles or caches.

## Inert fixtures and finite observation table

Prepare only A/B folders with no Git hooks/code, fresh owned home/profile data,
text-only skills with disable-model-invocation:true, and remote loopback port-9
MCP markers (no stdio commands, credentials, OAuth or external endpoint). Fixtures
and extension sources are immutable during each observation. Native cache/log/
database writes are isolated from authored baselines and never uploaded wholesale.
Fresh profile preferences disable updater, telemetry, extension auto-update and
welcome prompts; they keep workspace trust enabled and ungranted. All four runs
use the **same** original task HOME outside A/B; only their explicit user-data
profiles are fresh clones of the declared inert baseline, not of another run's
mutable state or the actual user's profile.

Set chat.mcp.autostart="never" explicitly in that baseline and record its actual
effective value. This is a restrictive research setting, not a claim about the
editor's default startup or enabled session behavior. At the exact pinned source,
[mcp.contribution.ts](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/src/vs/workbench/contrib/mcp/browser/mcp.contribution.ts)
registers editor discovery and services. [mcpService.ts](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/src/vs/workbench/contrib/mcp/common/mcpService.ts)
constructs definition/enablement/cache observers; its autostart method checks
Never and returns before collection activation, candidate enumeration or
startServerAndWaitForLiveTools. [mcpManagement.ts](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/src/vs/platform/mcp/common/mcpManagement.ts)
defines that exact setting/value. [mcpRegistry.ts](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/src/vs/workbench/contrib/mcp/common/mcpRegistry.ts)
separately gates workspace connection resolution on trust; the observer never
calls that start path or accepts trust. Preserve metadata discovery without
enabling an agent, harness or autostart. The absence of a port 9 server is **not**
proof of zero connection attempts. This collector does not instrument accepted
TCP or prove universal absence of attempts; MCP connection/handshake/runtime
remain not evaluated regardless of returned metadata.

Use one user skill at the documented personal root and user-profile User/mcp.json;
A/B each have .github/skills and .vscode/mcp.json with unique marker names/URLs.
Also include one same-name user/A/B skill with distinct inert descriptions to
observe actual path precedence, without inferring that three copies are active.
Do not create portable .mcp.json, chat.pluginLocations, copied personal settings,
Local TEST registrations or installed external extensions.

Exactly four shipped-workbench observations are allowed, each a fresh process
and fresh profile cloned only from the declared inert baseline:

| Probe | Selected single folder | Authored fixture difference | Observation |
| --- | --- | --- | --- |
| A | A | A/B/user marker fixtures present | Actual folder/trust, editor MCP metadata and parsed skill paths. |
| B | B | Same immutable fixtures | A/B/user selection separation and same-name skill URI. |
| A-restart | A | Same immutable fixtures | Fresh-process inventory, with no claim about an existing session. |
| A-after-removal | A | Controller removes only its declared A marker files before launch | Absence of exact A markers; user and B authored files remain. This is fixture removal, not UAP ownership proof. |

Unexpected row structure, native failure, sandbox/privilege failure, mutation of
sealed sources, missing result or deadline fails collection and retains bounded
raw diagnostics. A collected empty/blocked inventory is recorded as negative,
not silently promoted to successful source selection. Never substitute hand-
parsed files for an unavailable native API or mix filesystem data into native rows.

## Verification, interpretation and conditional product design

Inert tests cover real guard refusal before effects; literal finite argv and
proposal/extension/autostart limits; no forbidden commands/security bypass;
trusted sudo identity and root-owned executable parents; cookie-free/redacted
records; preexisting display refusal, exact PID/socket readiness and ten-second
startup failure; fixed HOME/fresh baseline; path/output/identity/freshness caps;
native-shape/empty/duplicate/API-denied distinctions; pending extension lifetime;
GUI-before-display teardown order, cancellation and joined bounded drains;
all-child/aggregate overflow, build/xauth failure; source mutation refusal;
bounded clean failure envelope; and exact source pins. Reuse existing process containment tests,
including immediate setsid descendants and cancellation; do not copy their kernel
implementation into the collector. Validate build/vet/format/workflow and run
the relevant no-vendor suites. Obtain two frozen code GOs before publication.

Independently interpret actual metadata attribution and each source/trust limit.
This collector never sets native_project_qualified, Local-harness/runtime, UAP
ownership, MCP initialization or skill-execution flags true. Only an actual
shipped-workbench inventory stage can be passed here. If it cannot run safely,
the workspace gate stays blocked; no product adapter follows from static source.

After positive relevant observations and independent interpretation, freeze a
separate project adapter implementation plan. Its likely narrow design is a new
project-only VS Code constructor registered by project composition, exact
.vscode/mcp.json component receipts and documented project skill paths, shared
native-config mutation and Prepared lifecycle. The legacy New user/Copilot
backend and NewLocal1.140 TEST/profile/hook source stay untouched. Single local
folder only; remote/multi-root/Local-session effectiveness and trust are separate
limits. No adapter source or new scope capability is admitted by this plan.

## Independent R1 review receipt and R2 scope

Both critics reviewed immutable 01f55f6218427ce97e44d6e167507ad06e5cbcf2
(published equivalent 9c43f29fcd4e580672ef2adfd7f8cbc559de3c57), without native
execution. Root independently read the pinned proposed APIs, extension-host
gates/MainThread publication and trust calculation, and gave architecture GO
conditional on explicit nested display/GUI lifetime, readiness and cleanup
reserve, every-child caps and blocked-versus-denied classification. APM's
independent R1 architecture GO additionally required a workbench autostart
boundary, fixed original HOME, trusted sudo caller identity, root-owned
nonwritable controller/source parents and cookie-free recorded argv.

This R2 plan makes those conditions concrete, corrects final-test return into a
pending promise, and describes the existing runner's merged-diagnostic limit
honestly. An empty inventory or a known API response with blocked/unknown attribution
may complete safe collection with false attribution/qualification; explicit API
denial, unknown/malformed output shape, changed
trust or sandbox failure is a failed stage. No sandbox fallback, trust grant,
collector code or project adapter is authorized before the two final R2 GOs.

The root critic gave final R2 GO on5995254a after independently checking the
MCP autostart enum/service and trust gate. Before collector code, implementation
feasibility review found the extractor's intentionally non-executable file modes.
The explicit two-file nonprivileged-mode section above is a narrow R2 addendum
requiring both critics' final review at this new frozen checkpoint. It enables no
extra native probe, privileged publisher mode or project adapter.
