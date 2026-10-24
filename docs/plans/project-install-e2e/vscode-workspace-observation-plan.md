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

## Reviewed observer extension

Add a small repository-authored CommonJS extension and test entry with no runtime
dependencies, providers, commands, tools, hooks or network calls. It declares
untrusted-workspace support only so observation can run without granting trust.
The test entry exports the documented async test runner shape, reads workspace
folders/trust, the two metadata APIs and resource-scoped read-only configuration
inspection. It writes one bounded JSON result and returns. It never registers
skills/MCP definitions, creates an MCP gateway, starts a server, invokes a prompt,
modifies settings, installs extensions, accepts dialogs or reads resource bodies
outside the exact inert marker set.

Allow at most 128 native MCP/skill rows, 512KiB result JSON, bounded strings and
portable expected local URIs. Record actual API shapes before semantic assertions;
unknown shapes, duplicate JSON keys, missing fields or output overflow fail.
Subscribe/poll to allow normal asynchronous discovery for at most 15 seconds;
record the final raw inventory, including empty/blocked results. Record the
read-only effective trust, skills-enabled and MCP-access setting values; do not
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
setup, recording its digest. Its sole task is the finite sequence below, not an
arbitrary command/JSON execution endpoint. No production process-runner change.

The hosted controller uses sudo only for this fixed audited helper so existing
strict containment can create task-owned cgroups. The helper itself executes no
publisher code while privileged: construct literal unshare/setpriv/env-i plus
the existing same-process nonroot/network/capability verifier before the ELF.
Root controller assets/control paths are private and cannot be replaced by the
nonroot publisher; validate all derived paths and original IDs before launch.
Task data/results are untrusted data, never executable controller inputs. Failure
to obtain the required cgroup/CLONE_INTO_CGROUP or namespace capability stops
before workbench code; no ordinary-runner/process-group/no-sandbox fallback.

Use already installed absolute system Xvfb and xauth binaries only; do not apt
install them. Record ordinary executable identities/version. Missing tools fail
before publisher GUI launch. A private fixed task display, no TCP listener and
a random task-only Xauthority cookie provide the test display. Cookie contents
are not logged/uploaded and do not authenticate any product/user account. Existing
display/lock collision fails; do not attach to another display. Run Xvfb under
the same strict contained lifecycle and original nonroot identity. No shell
wrapper or inherited DISPLAY/XAUTHORITY is used.

Run shipped ELF directly as an editor (without ELECTRON_RUN_AS_NODE), explicit
fresh user-data/extensions dirs, --new-window, exact extensionDevelopmentPath,
extensionTestsPath, the one proposal-enabled extension ID and one selected local
folder. The exact argument spelling is in pinned argv.ts; no default chat call.
Never pass --disable-workspace-trust, --no-sandbox, --disable-setuid-sandbox or
change kernel/AppArmor policy. If the real Electron sandbox cannot initialize,
retain its bounded diagnostics and stop. No second archive/runtime fallback.

Each GUI observer has a 45-second bound. The strict duplex callback ends only
after the bounded current result is validated or an error/deadline occurs;
planned shutdown then terminates the kernel-contained process tree. Record that
planned shutdown separately, never as a natural exit-zero result. Existing
primitive cleanup failures remain failures. A total independent eight-minute
watchdog, shorter inner deadlines, bounded stdout/stderr and independent five-
second filesystem cleanup/upload admission remain. The helper must own signal/
timeout cleanup; an outer kill alone is not a successful containment proof. Set
its total/signal contexts before starting any child, keep inner deadlines shorter
than the outer watchdog, and emit only bounded controller summaries on stdout.
A killed controller or unproved cgroup cleanup forces collection incomplete; do
not convert VM disposal or a missing process handle into successful cleanup.

## Inert fixtures and finite observation table

Prepare only A/B folders with no Git hooks/code, fresh owned home/profile data,
text-only skills with disable-model-invocation:true, and remote loopback port-9
MCP markers (no stdio commands, credentials, OAuth or external endpoint). Fixtures
and extension sources are immutable during each observation. Native cache/log/
database writes are isolated from authored baselines and never uploaded wholesale.
Fresh profile preferences disable updater, telemetry, extension auto-update and
welcome prompts; they keep workspace trust enabled and ungranted. No agent,
discovery, autostart or user security policy is silently enabled to create proof.

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
proposal/extension limits; no forbidden commands/security bypass; path/output/
identity/freshness caps; native-shape/empty/duplicate failures; callback lifetime
and contained planned-shutdown causality; source mutation refusal; bounded clean
failure envelope; and exact source pins. Reuse existing process containment tests,
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
