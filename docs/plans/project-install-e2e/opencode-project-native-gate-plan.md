# OpenCode: pinned current-source project native gate

2026-10-24. Plan only. The existing reviewed project adapter remains unchanged.
Two independent plan review rounds precede new fixture/selector/workflow code;
two independent code reviews precede publication that could execute a client.
This gate concerns source qualification, not a release or a native registry.

## Pinned source and controls

Keep the native matrix's existing OpenCode **1.18.34** pins and acquisition.
Official `anomalyco/opencode` tag `v1.18.34` points directly to commit
`aec0b9a6d8898f68f923aaf08b7306d931fd9d76`. Primary files inspected at that
immutable commit:

- [debug/config.ts](https://github.com/anomalyco/opencode/blob/aec0b9a6d8898f68f923aaf08b7306d931fd9d76/packages/opencode/src/cli/cmd/debug/config.ts): resolve effective config, redact, print JSON.
- [debug/skill.ts](https://github.com/anomalyco/opencode/blob/aec0b9a6d8898f68f923aaf08b7306d931fd9d76/packages/opencode/src/cli/cmd/debug/skill.ts): list actual Skill service inventory and locations.
- [effect-cmd.ts](https://github.com/anomalyco/opencode/blob/aec0b9a6d8898f68f923aaf08b7306d931fd9d76/packages/opencode/src/cli/effect-cmd.ts) and [bootstrap.ts](https://github.com/anomalyco/opencode/blob/aec0b9a6d8898f68f923aaf08b7306d931fd9d76/packages/opencode/src/project/bootstrap.ts): both commands load a real instance. Config, plugin and other vendor services initialize; these are not filesystem-pure reads.
- [config/config.ts](https://github.com/anomalyco/opencode/blob/aec0b9a6d8898f68f923aaf08b7306d931fd9d76/packages/opencode/src/config/config.ts) and [paths.ts](https://github.com/anomalyco/opencode/blob/aec0b9a6d8898f68f923aaf08b7306d931fd9d76/packages/opencode/src/config/paths.ts): user, upward project, `.opencode`, custom and inline sources merge. Config loading may create its own `.gitignore` and fork dependency bootstrap.
- [core/npm.ts](https://github.com/anomalyco/opencode/blob/aec0b9a6d8898f68f923aaf08b7306d931fd9d76/packages/core/src/npm.ts): vendor dependency installation uses Arborist with `ignoreScripts:true`; do not claim no download or no vendor cache effects.
- [plugin/index.ts](https://github.com/anomalyco/opencode/blob/aec0b9a6d8898f68f923aaf08b7306d931fd9d76/packages/opencode/src/plugin/index.ts), [runtime-flags.ts](https://github.com/anomalyco/opencode/blob/aec0b9a6d8898f68f923aaf08b7306d931fd9d76/packages/opencode/src/effect/runtime-flags.ts) and [index.ts](https://github.com/anomalyco/opencode/blob/aec0b9a6d8898f68f923aaf08b7306d931fd9d76/packages/opencode/src/index.ts): `OPENCODE_PURE=1` skips external configured plugins; it does not disable project config merging. Correct the misleading existing fixture comment without changing its behavior.
- [skill/index.ts](https://github.com/anomalyco/opencode/blob/aec0b9a6d8898f68f923aaf08b7306d931fd9d76/packages/opencode/src/skill/index.ts): native scans include global, project and compatible skill paths; duplicate names can replace a prior entry. A path-bearing winner is evidence; an empty result is not discovery.

The existing matrix provides verified CLI bytes, exact source build identity,
scanner provisioning and the genuine hosted admission. No native execution on
the task host, fake hosted variables, copied credentials, login, trust edit or
model request. Start with the existing macOS-arm64/Windows-amd64 source workflow;
no new Linux isolation claim or runner substitution is introduced by this plan.

Use one explicit fresh original HOME/XDG authority outside sibling A/B and an
ordinary user working directory. Create a real fresh Git repository boundary in
each A/B and the user working directory, and a fourth for the short JSON case.
All are canonical temporary directories outside the checked-out UAP source and
outside each other. Use only fixed bounded `git -c core.hooksPath=/dev/null init`
with the fresh HOME/Git configuration; no hooks, remote or Git config import.
Assert each native cwd resolves to its own expected worktree before observation,
so native upward discovery cannot inherit the parent checkout or sibling. Every UAP/native child shares that authority.
Never use a project as HOME or `OPENCODE_CONFIG_DIR`. Preserve the explicit
platform environment, `OPENCODE_PURE=1`, and source-supported literal controls
`OPENCODE_DISABLE_AUTOUPDATE=1`, `OPENCODE_DISABLE_MODELS_FETCH=1`,
`OPENCODE_DISABLE_DEFAULT_PLUGINS=1`, `OPENCODE_DISABLE_LSP_DOWNLOAD=1`.
These restrict this observed CLI lane; they do not qualify default-plugin,
model, LSP or unrestricted session behavior. No configured JS/TS plugin, hook,
remote skill URL, legacy TOML migration input, well-known auth source or account
file is supplied. `OPENCODE_CONFIG`, `OPENCODE_CONFIG_CONTENT`,
`OPENCODE_CONFIG_DIR` and `OPENCODE_DISABLE_PROJECT_CONFIG` are absent in the
positive lane. A later isolated inline-precedence observation is explicit.
Vendor bootstrap may create its own cache/config metadata; retain that fact and
separate it from exact UAP ownership/foreign preservation assertions.

## Fixture and supported contract

Add `TestAgentpluginsOpenCodeNativeProjectComponentsLifecycle` behind its own
`AGENTPLUGINS_OPENCODE_PROJECT_NATIVE_E2E=1`, then the unchanged real disposable
host guard before any acquisition/client operation. Keep all existing required
OpenCode user lifecycle, tool-collision, runtime and command tests in the source
selector; no existing test is renamed, weakened or dropped.

Use actual public CLI install/update/repair/remove with `--scope project` and
explicit canonical `--project-root`, original user authority, exact native
executable and no saved dependency intent where the existing native fixture uses
local source. Separate installer envelope/state/file receipts from native JSON.
The adapter's public lifecycle stays Prepared + installation verified. Native
observations in this test do not promote persisted UAP activation.

Use root `opencode.jsonc` with preserved foreign JSONC comments/settings for the
main A/B lifecycle, and `.opencode/skills/<logical-name>` as the actual skills
surface. Managed package/data receipts remain under the selected `.uap` root.
A short separate JSON fixture proves root `opencode.json` selection; it need
not duplicate every lifecycle step. Never invent a v2 schema or silently change
the pinned version/dialect when a native command fails.

Fixture packages contain only standard inert skills/portable commands and
remote loopback MCP declarations pointing at a parent-owned passive HTTP
recorder on an ephemeral `127.0.0.1` port, with no local stdio
server, JS/TS plugin, hook or payload executable. The passive recorder implements
no MCP protocol or authentication: it counts accepted TCP connections and HTTP
requests, and returns HTTP 503. Record bounded connection/request counters and a
bounded method/path metadata sample (no request bodies/credentials), with bounded
server read/header/write/idle deadlines. Both aggregate counters across all child
operations and their deltas for each positive config/skill probe must remain
zero. Any accepted connection or request fails that stage and is retained as
attempted connection evidence, never as a handshake. Close it under the fixture
deadline. Zero counters only describe this live recorder endpoint; they do not
prove the absence of unrelated network traffic.
The MCP declarations exercise
exact effective config selection, not connection or initialization. Do not run
`mcp list`: the existing helper explicitly attempts live connections. Actual MCP
handshake/tool qualification remains a separate gate and is marked not evaluated
in this fixture, even if a configured entry is enabled.

Positive ownership phase: original user has a differently named owned/control
skill and foreign settings; A/B receive the same project plugin with identical
logical skill/server names. With no conflicting user name, fresh native A/B
config+skill lists must identify their exact selected project paths/bodies and
MCP marker URLs, omit the sibling's exact path/marker, and retain the user
control. Native effective config is not ownership authority for deletion.

Then separately exercise the identical plugin name and identical skill/server
names at user+A+B, preserving all three receipts and files. Retain actual
same-name native winners by path/body from fresh user/A/B processes. If native
names collapse or a higher-priority source wins, record precedence/isolation as
blocked or unqualified, rather than demanding three simultaneously loaded
copies or overwriting user settings. This stage is separate from the positively
proved unique-marker project discovery stage.

## Finite lifecycle and observation stages

Use at most 30 direct native observation processes and 30 public UAP commands
per complete fixture, with an independent six-minute fixture deadline inside
the existing matrix's 15-minute test budget. UAP's own pinned host preparation
may invoke additional bounded version probes; do not misreport the direct-probe
ceiling as a measured total of all descendants. Setup uses at most four fixed
Git commands, with an overall 60 captured-child ceiling across these classes.
All new subprocess classes share bounded capture and the same aggregate artifact
budget, including UAP and setup failure output.
A fixed command list permits only `--version`, `debug config`, and `debug skill`.
No general shell, command taken from output, arbitrary path lookup, MCP list,
run/prompt/session, plugin install, trust, account or updater command is allowed.

1. Measure exact version/binary identity and record source/build provenance.
2. Install positive user/A/B fixtures through actual UAP; fresh A/B lists must
   prove exact project config/skills and user control, with raw path-bearing JSON.
3. Repeat add/unchanged update and intact repair; assert mutation flags/state
   stability and retained Prepared guidance without treating listing as activation.
4. Change skill body and portable command invocation selection/MCP marker URL;
   update A. Actual UAP owned bytes and fresh A native lists must advance while
   original user and B owned files/state/foreign settings remain exact.
5. Delete only the exact recorded A skill directory, run repair from the recorded
   immutable package pin, and require the original selected path/body again.
6. Remove A, repeat removal through its supported public result, and observe
   its owned native skill and MCP key absent. User/B/foreign settings and retained
   caller-owned preexisting plugin-data markers survive. These remote-only fixtures
   do not create a new stdio data receipt; its complete lifecycle remains a
   separately passed composed contract. Verify actual absence as well as native lists.
7. A separate same-name user+A+B phase records exact winners and conservative
   precedence boundaries. Add one fixed inline override of only a nonsecret MCP
   marker in a fresh process; observe its winner, clear the override, and verify
   ordinary restart returns to the file-selected value. Never rewrite owned
   project bytes to manufacture the override or adopt an observed foreign row.
8. The short root-JSON case installs/observes/removes exact project components
   and records its format independently from the main JSONC journey.

The count is a ceiling, not an invitation to add probes; retain an explicit
stage/command table in code and fail before a 31st direct observation or public
installer command. If the full table
cannot fit the cap/deadline together with required user tests, reduce repeated
native reads while keeping ownership/lifecycle assertions, or review a concrete
separate gate. Do not increase global native timeouts silently.

## Capture, interpretation and artifact contract

Do not copy the unbounded old `CombinedOutput` helpers into the new fixture.
Reuse the already reviewed bounded diagnostic capture primitive behind a
client-neutral test wrapper if necessary, parameterizing limits without changing
existing caller behavior. It applies to EVERY new child, including public UAP
operations and Git setup, not only direct native observations. Require fixed
executable/argv/cwd/environment, empty stdin, separate live 128 KiB stdout/stderr
limits, <=10 seconds for direct native/Git and <=30 seconds for public UAP, each
also capped by remaining fixture time, bounded pipe drain/WaitDelay and child
termination/reap. Never reuse old unbounded f.uap/f.native/CombinedOutput paths
in the new fixture. A failure,
deadline, overflow, nonzero exit, dirty warning or unrecognized JSON is recorded
and fails the relevant stage. No failed output becomes an empty successful list.
Any helper extraction is behavior-preserving and independently code-reviewed.

Only in the isolated same-name precedence phase, a recognized exact duplicate-
skill warning may be retained as a blocked/unqualified precedence observation
with its valid path-bearing winner. Do not strip or globally ignore warnings.
Validate the known name and every reported source against exact fixture paths;
if the warning format is unknown, retain it as a blocked/failed observation.
Mandatory positive phases continue to fail on dirty warnings, malformed output,
nonzero exit or truncation. No failure becomes an empty or successful discovery.

Decode one whole JSON document with duplicate/case-ambiguous keys, trailing
bytes, nonfinite values and malformed required field types refused. The new
project skill parser compares native locations literally/physically against the
selected known roots; it does not clean an arbitrary reported traversal into
an owned path. Verify required names and bodies; built-in/unrelated rows do not
count as managed discovery. Config expectations use exact known MCP entries,
source markers and transport fields, not a substring that can appear in logs.

Keep separate attempted/passed/blocked/not-evaluated stages. Mandatory passed
stages cover actual UAP Prepared file lifecycle and the positive unique-marker
project discovery/config source. Same-name precedence and inline shadowing
retain their measured limits. MCP initialization/tool use, authentication,
model use, trust policy, managed enterprise/ancestor sources, IDE and default plugins are
explicitly not evaluated. No stdout or filesystem fallback can upgrade them.

Retain only bounded flat JSON/log evidence, exact stdout/stderr lengths/hashes,
source/native/installer identities and foreign/owned state snapshots. No native
cache, database, binary, auth data or node_modules is uploaded. The aggregate
<=32 MiB/128 flat-file upload envelope includes ALL public-UAP/native/Git streams,
state snapshots, metadata and failure diagnostics. At most 60 captured child
commands with 128 KiB/stream contribute at most 15 MiB; reserve at most 120 stream
files and 8 metadata/snapshot files within the 128 file cap. The remaining bounded
budget covers summaries/snapshots. Enforce the cumulative limit while recording
and again at admission; stop further probes on exhaustion. Never admit only the
new native logs while leaving UAP/snapshot output outside the budget. A
refused envelope produces only a bounded false-qualification failure record;
raw refused trees are not the always-upload path. Reuse an already reviewed
admission helper if its actual boundary matches, otherwise review the narrow
new envelope code before publication. Preserve original errors on cleanup or
artifact refusal; fixture cleanup has an independent finite deadline.

## Selection and verification

Add OpenCode only to `PROJECT_TESTS` and its exact source-only selector. Released,
draft/prepared installer selection must continue to refuse project fixtures.
Assert the union with all four existing OpenCode required tests, no skipped or
omitted project test, and no source-scope option enabled by an environment alias.
Enable `--project-components` for OpenCode in the existing source workflow only
after two code GOs. Do not alter acquisition pins, hosted admission, permission
scope or released-native verification.

Before publication, inert contracts cover strict native parsers, actual stage
sets, scoped fixture environment/command table, unsupported source selection,
old user test union, capture/deadline/overflow, artifact upload admission and
foreign state preservation. Compile the fixture and run affected Python matrix
contracts plus syntax/diff checks. No native process runs locally. Genuine
artifacts are then downloaded, hashed and independently interpreted; their exact
source/version/platform and remaining boundaries enter the final client matrix.


## R1 corrections and MCP source boundary

Both independent R1 critics requested the all-child capture/aggregate budget and
known duplicate-warning distinction; root additionally required explicit Git
boundaries and a concrete enabled-MCP bootstrap trace. Those requirements are
incorporated above, without changing the adapter or promoting native lifecycle.

At exact upstream commit `aec0b9a6`, [mcp/index.ts](https://github.com/anomalyco/opencode/blob/aec0b9a6d8898f68f923aaf08b7306d931fd9d76/packages/opencode/src/mcp/index.ts)
creates a shared MCP service whose per-instance state is registered through
`InstanceState.make`. This is a service/cache allocation, not a per-server
transport/client construction. [instance-state.ts](https://github.com/anomalyco/opencode/blob/aec0b9a6d8898f68f923aaf08b7306d931fd9d76/packages/opencode/src/effect/instance-state.ts)
constructs a ScopedCache with a deferred lookup; only `InstanceState.get` invokes
the initializer. MCP's initializer enumerates enabled entries and creates the
actual Client/transports, so operations such as `MCP.status/clients/tools/connect`
can cause connections. `createClient`/`client.connect` are in those invoked
paths, rather than the shared service constructor.

The fixed debug config handler only calls Config.Service.get; fixed debug skill
only calls Skill.Service.all. InstanceBootstrap eagerly invokes Config.get and
Plugin.init plus its listed LSP/share/format/VCS/snapshot/project services, not
MCP.status/clients/tools/connect. The declared controls disable default plugins
and external plugin execution, and the fixture supplies no plugin, hook or skill
URL with another execution route. This is a static trace supporting the narrow
observation route, not a universal no-side-effects or no-network claim. The
passive loopback recorder adds actual attempted-connection evidence for the
only configured MCP URLs. If an accepted connection or request occurs despite this trace, keep that
failure and investigate before changing assertions or invoking another command.
No successful MCP initialization, tool use or network isolation is inferred.
