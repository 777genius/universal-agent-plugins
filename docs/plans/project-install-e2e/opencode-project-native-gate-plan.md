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
ordinary user working directory. Every UAP/native child shares that authority.
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
remote loopback MCP declarations (`http://127.0.0.1:9/...`), with no local stdio
server, JS/TS plugin, hook or payload executable. The MCP declarations exercise
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
ceiling as a measured total of all descendants.
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
Reuse the already reviewed bounded native diagnostic capture primitive behind a
client-neutral test wrapper if necessary: fixed executable/argv/cwd/environment,
empty stdin, separate live 512 KiB stdout/stderr limits, <=10-second per-process
context, bounded pipe drain/WaitDelay and child termination/reap. A failure,
deadline, overflow, nonzero exit, dirty warning or unrecognized JSON is recorded
and fails the relevant stage. No failed output becomes an empty successful list.
Any helper extraction is behavior-preserving and independently code-reviewed.

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
model use, trust policy, managed enterprise sources, IDE and default plugins are
explicitly not evaluated. No stdout or filesystem fallback can upgrade them.

Retain only bounded flat JSON/log evidence, exact stdout/stderr lengths/hashes,
source/native/installer identities and foreign/owned state snapshots. No native
cache, database, binary, auth data or node_modules is uploaded. With <=30 probes
at 512 KiB per stream plus bounded summaries, admit the new fixture to <=32 MiB
and at most 128 flat files before publication to the upload source. A
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
