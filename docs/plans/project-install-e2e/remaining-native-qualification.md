# Remaining native project qualification

2026-10-25. Work in progress; source-only, not release qualification.
Baseline: `d5c16856cef67a1457704971d46765739141ab08` (project implementation
#499 and evidence cleanup #519). The owner requests completion of the remaining
agents' `--scope project` E2E with incremental commits/pushes. This plan does not
claim that an unexecuted scenario passed.

## Authority and review

Follow [owner scope](../../PROJECT_INSTALL_OWNER_SCOPE.md), including the narrow
VS Code project exception, original-user authority and existing transaction,
ownership and package-store architecture. Preserve legacy implementations and
tests. Keep published release availability separate from source qualification.

Workers and independent reviewers use the owner's GPT-6.1 Sol/xhigh selection.
Before implementation, review each slice in two independent rounds and resolve
findings. A client owner does not approve their own slice. Record review results
in the PR, with the exact reviewed commit/tree. Each client has a separate PR
stacked on this program branch. Product changes discovered by a spike receive a
specific amended plan and the same review gate before implementation.

Keep this as one consolidated plan. Do not add per-command logs, expanded
captures or separate repetitive review documents to Git. Raw evidence belongs
in bounded CI artifacts; retain concise source-bound conclusions and links.

## Required evidence

There are ten remaining client targets: `gemini`, `copilot`, `kiro`, `cursor`,
`cline`, `qoder`, `windsurf` (existing `devin` alias), `kimi`, `grok`, and `vscode`.
Claude/Codex/OpenCode existing native gates remain intact. OpenCode project MCP
is a separate optional follow-through after the requested remaining targets.

Every client receives public UAP project lifecycle coverage and genuine native
observation for its supported surface. Distinguish installed ownership, native
discovery, MCP initialize, tools/list, tools/call, authentication and model use.
Prepared is not globally promoted to Active because a trusted fixture passed.

Required lifecycle: empty baseline; install distinct A/B packages and an
independent original-user marker; unchanged reconciliation; move source/update
A; remove one exact owned component/repair from the recorded pin; fresh native
process or workbench restart; remove A and repeat removal. Require exact selected
source, revision/path/identity and correct absence after removal. Verify B,
foreign native content, original-user authority and retained plugin data.

Use the existing harmless native MCP probe and fresh nonce. A runtime-positive
claim requires native-driven initialize/tools/list/tool-call events, matched
native/probe session and returned result, selected package/data roots and cwd.
A connected label, package list or direct test call to the probe is insufficient.
ACP receives no inline MCP definitions; config injection cannot prove discovery.
Qoder's documented explicit session plugin selection and Cline's process-wide
MCP override are separately named activation modes, not cwd autodiscovery.

## Execution boundary and shared infrastructure

Native/vendor execution occurs only on genuinely disposable, supported
GitHub-hosted machines. Check actual platform, `GITHUB_ACTIONS`,
`RUNNER_ENVIRONMENT` and explicit native opt-in before the first child, including
Linux. Never synthesize host eligibility locally. Inert parser/protocol tests may
run locally; they do not qualify a vendor process.

The new VS Code trusted runtime starts on the already reviewed `ubuntu-22.04`
workspace host, preserving its ordinary-user, NNP, strict cgroup admission,
Xvfb and standard Electron sandbox contract. Record the actual image/version.
The earlier Ubuntu 24.04 failure is not an invitation to use `--no-sandbox`, a
setuid-helper workaround, privileged launch or another process primitive.
Unsupported admission fails; an image change needs its own reviewed amendment.
See [the retained host plan](vscode-workspace-host-plan.md).

Use a fresh original user profile plus separate real Git projects A/B and an
outside cwd. A project never becomes HOME. Explicitly allowlist child environment;
do not inherit credentials, proxies, account state, shell configuration or the
workflow GitHub token. Do not claim a network firewall that is not implemented.
Every new Cline child, including version/help, receives literal
`CLINE_NO_AUTO_UPDATE=1` through the preserved runtime-environment contract.
Refuse a pin where that documented control no longer applies. Use documented
updater suppression for other clients where available and verify immutable
runtime-file identities before/after observation; changed bytes cannot continue
under the old pin. Freeze Cline tool approval explicitly rather than inheriting
its CLI's auto-approval default; reject unexpected built-in tool requests.

Exact owned A/B trust can be authored through documented native configuration or
native UI/API before preservation baselines, following the existing
[trusted-fixture amendment](priority-trusted-native-plan.md). Record that setup
separately from observed trust. UAP itself never changes trust. No forged trust
database, blanket trust-disabling flag, authentication bypass or fabricated
vendor account is permitted.

An officially supported custom provider may point to the existing scripted
loopback fixture. Label it as a test provider; its own optional test credential
is not vendor OAuth evidence. Never inherit or publish real credentials. Native
local bearer tokens stay private and are redacted from artifact transcripts.
In particular, Kimi startup raw streams, generated token files and auth/provider
configuration stay in a private capture directory outside artifact admission.
Parse the native token privately, redact diagnostic streams before copying them
into the admitted evidence namespace, and upload only the closed evidence set.
An inert admission test must reject a known token canary in JSON, transcripts
or archive members. Do not retain bearer-bearing URLs as public source evidence.

Reuse `native_client_research.run_probe`, existing native fixture/capture and
process containment, source/provenance checks, the native MCP probe and existing
loopback-provider patterns. No second installer, generic RPC framework or
speculative client registry. Extend pinned acquisition only for concrete formats:
Node/support-file distributions and full Kiro companions, with bounded extraction
and regular-file/path checks. Do not execute package install scripts.

Root owns shared acquisition, source identity, process/evidence helpers, workflow
registration and support wording. Client owners own leaf collectors/parsers and
native lifecycle cases. Use source-only project jobs without weakening released
client evidence. Register clients and exact required tests explicitly as their
slices arrive; the final program cannot complete with a missing target or case.

Typical budgets: help/list 30 seconds, one session/tool observation 60 seconds,
one MiB per captured stream, six minutes per initial research client. Long-lived
native services have explicit total deadlines, owned child cleanup and shutdown
evidence. Fail on overflow, timeout, malformed results, unsafe artifacts or
missing required observations. Reuse existing closed artifact admission rather
than recursively uploading a native profile.

Research completion and qualification are distinct. Research can record an
explicitly planned native authentication/unsupported boundary, with raw bounded
evidence and all affected qualification flags false. A required runtime case
fails if blocked, skipped, not reached or missing evidence. Never convert an
arbitrary nonzero exit into an expected boundary or a successful qualification.

### Concrete phase and leaf contract

This two-round plan gate initially authorizes shared acquisition and bounded
research collectors, not an unobserved product change or guessed session
protocol. `acquire` requires `<client>.acquisition`; `observe` additionally
requires `<client>.help` and `<client>.scope-observation`. A research runtime
attempt is separately registered as `<client>.runtime-attempt`. Cursor/Grok
cannot enter `observe` until the reviewed immutable acquisition amendment.
Unknown flags/schema, malformed output and unrelated failures are failures,
not supported-boundary records. Only a measured explicit authentication or
unsupported-native-capability response can complete a planned boundary attempt.

The initial Kimi/Cline help-only leaves explicitly declare `help`, which selects
only acquisition and help. It cannot satisfy scope observation. Leaf
`AVAILABLE_PHASES` (or the unconditional acquisition-only declaration) registers
the exact initial phase in the shared research matrix; missing leaves stay
visible as outstanding program work. The initial shared runner never emits a
positive qualification flag, and accepts only explicit passed selected research
IDs. Structured observations may be a JSON mapping or list; they cannot set case
success by themselves. Raw private files are never enumerated for upload.

Gemini's reviewed `gemini.js` bundle uses the exact Node **24.21.0** Linux x64
runtime, acquired from
`https://nodejs.org/dist/v24.21.0/node-v24.21.0-linux-x64.tar.xz`, SHA256
`fd8e59d5a511510f6a298afb548f18c7d2b1be404d8b4a27d94fbe49f56cb2d6`.
Only the exact regular `node-v24.21.0-linux-x64/bin/node` member is materialized;
the archive is verified before extraction, version is checked before the client,
and runtime/support-file bytes are verified before and after every probe. No npm
installation or package lifecycle script is used. This does not authorize a
TypeScript observer or SDK toolchain, which retains its separate amendment gate.

All ten targets must be accounted for in the final matrix. A successful research
job does not satisfy any qualification ID. The required follow-through IDs for
an installation/discovery claim are `<client>.project-lifecycle` and
`<client>.native-restart-removal`; a native MCP claim additionally requires
`<client>.mcp-initialize`, `<client>.mcp-tools-list` and
`<client>.mcp-tools-call`. An outstanding boundary remains visible and unqualified,
never a substituted pass. After observation, freeze exact native mode/argv,
schemas, tool approval, source entrypoint and selected Go test names in a concise
amendment here; independently review it twice before coding that runtime slice.

Collectors are closed, client-specific leaves under
`scripts/project_native_collectors/`; their inert contracts belong in
`scripts/test_project_native_<client>.py`. Root owns shared entrypoint,
acquisition/host/process/artifact helpers and workflow registration. This fixes
the initial file ownership without creating a new plugin or RPC framework.

| Client ID | Owned collector | Subsequent native fixture prefix or controller | Mode to freeze after observation |
| --- | --- | --- | --- |
| `gemini` | `gemini.py` | `repotests/agentplugins_gemini_project_native_` | Installed settings, native session, supported Gemini loopback protocol and returned MCP tool result. |
| `copilot` | `copilot.py` | `repotests/agentplugins_copilot_project_native_` | Offline BYOK ACP/session with empty injected MCP list and exact named-tool permission response. |
| `kiro` | `kiro.py` | `repotests/agentplugins_kiro_project_native_` | Measured V2/V3/native ACP mode; config visibility and actual auth/startup response stay distinct. |
| `cursor` | `cursor.py` | `cli/cmd/agentplugins/project_cursor_native_` | Genuine list/list-tools source resolution, then separately admitted tool-execution mode. |
| `cline` | `cline.py` | `cli/cmd/agentplugins/project_cline_runtime_` | Real task using exact process MCP override, suppressed updater and explicit tool approval. |
| `qoder` | `qoder.py` | `cli/cmd/agentplugins/project_qoder_native_` | Explicit session plugin-dir, pinned runtime/SDK, exact initialized plugin/skill source and absent-path control. |
| `windsurf` | `windsurf.py` | `repotests/agentplugins_devin_project_native_` | Genuine Devin skills/MCP discovery; any auth-required tool tier stays separate. |
| `kimi` | `kimi.py` | `repotests/agentplugins_kimi_project_native_` | Native authenticated local API resolves installed entry by name/cwd; native prompt tool result is a separate case. |
| `grok` | `grok.py` | `repotests/agentplugins_grok_project_native_` | Observed plugin versus direct project path; actual source/trust/connection before any adapter change. |
| `vscode` | `vscode.py` | `install/integrationctl/research/vscodeprojectruntime/` and `scripts/vscode-project-runtime-observer/` | Trusted native development host; correlate gateway endpoint with exact definition metadata and nonce/package/data/cwd, not label alone. |

The Go prefixes above admit focused `_test.go` leaves only, preserving existing
fixtures. The initial shared/collector PRs cannot assert these later cases are
implemented merely because their research completed.

## Frozen candidates and initial observations

Pin every executable before execution. Cursor and current Grok start with an
acquisition-only stage: download immutable official bytes, bound size and record
digest/member inventory without executing them; review/freeze that pin before
the native observation stage. Existing pins are reused only for their own exact
version/platform. Measure main and companion binaries and any Node runtime.

| Client | Candidate | Initial genuine observation and follow-through |
| --- | --- | --- |
| Gemini | 0.65.0, existing official bundle pin | `skills list --all`, `mcp list`; explicit trusted/untrusted A/B; native connection events; documented loopback Gemini provider for a harmless tool call. |
| Copilot CLI | 1.0.103, existing Linux archive/executable pins | `help providers`, `skill list --json`, `mcp list/get --json`; documented exact `trustedFolders`; offline BYOK ACP/native session with empty MCP injection and narrow tool approval. |
| Kiro | 2.34.0, current official manifest | Actual help, `mcp list workspace/global`, `mcp status --name`; freeze observed engine and 2.34 trust semantics; bounded actual ACP session attempt. Config listing cannot stand in for startup if genuine auth is required. |
| Cursor | 2026.10.22-27dcf5f, digest pending acquisition | Exact launcher/Node distribution; actual help, `mcp list`, `mcp list-tools`; correlate native handshake; investigate supported authenticated/custom-provider tool path separately. Preserve legacy auth-skipping tests without borrowing their semantics. |
| Cline | 3.0.73, existing pin | Preserve metadata/source-selection gate. New real runtime fixture uses exact cwd plus `CLINE_MCP_SETTINGS_PATH`, actual installed MCP connection/tool evidence and ordinary-user-source negative control. Supported provider/auth behavior must be observed first. |
| Qoder | 1.1.78, existing pin | Verify `--plugin-dir` and actual session plugin/skill inventory using UAP's existing closed view; exact A/B revision and fresh-session absence. Pin any official SDK and force the selected vendor executable. MCP requires a separately reviewed closed-view extension after observation. |
| Devin Local | 3000.11.205, existing pin; target `windsurf` | Actual skills list/paths/show and MCP list/get across lifecycle; require native startup/tool events separately. Observe bounded auth boundary if no supported account-free tool surface exists. |
| Kimi Code | 2.3.2, official asset digest; retain observed 2.3.1 contract | Genuine `kimi web --no-open` local API: native bearer, own A/B workspaces/trust, skill catalog and MCP origin/test by name+cwd. Supported loopback provider can drive actual tool use. Qualify new bytes before narrowly admitting 2.3.2. |
| Grok Build | 1.0.70, digest pending acquisition | Actual inspect/skills/plugin/MCP help and doctor. Observe trusted bundled plugin MCP versus documented project `.grok/config.toml`. Implement only the observed path; no speculative fallback. Documented BYOK loopback may drive the tool call. |
| VS Code | 1.143.1, existing publisher pin | Preserve untrusted metadata observer. Separate real trusted development-host UI and `lm.startMcpGateway()` collector; initialize/list/call through the vendor gateway, exact installed server identity, restart/removal and cleanup. Label proposed-API/development-host scope. |

New observer source uses TypeScript (`.ts`); generated JavaScript is a build
artifact, not a new tracked `.mjs`/`.cjs` implementation. Preserve existing
observers and reuse repository tooling rather than introducing another stack.
Before implementing/executing an observer or SDK-backed path, a reviewed leaf
amendment freezes exact Node, compiler/bundler and SDK versions plus integrity
or a committed lock containing immutable resolved artifacts, the build command
and output location. Existing CJS observers do not supply a TypeScript build
contract by implication. No `npx`/latest resolution or package lifecycle scripts;
emitted JavaScript stays under the job's temporary artifact/build directory,
outside the checked-out source. One concrete shared TypeScript toolchain suffices.

### New immutable pin material already available

- Gemini bundle SHA256
  `bb5e7e879608bd9b3fe261930d3886fa56aacbf471da1655f2978eb35b705a73`,
  21,215,960 bytes, official `google-gemini/gemini-cli` release `v0.65.0`.
- Copilot archive SHA256
  `9aeb4430e9ac9523546db07b227c7aba179a4710a55f9ac29ad143b0f9e1b325`,
  129,018,878 bytes, official `github/copilot-cli` release `v1.0.103`.
  Executable SHA256
  `f19b3554a6b62605db3a26d5255ebe2a3ee39fcd6a357edb18301ea5037f6f0d`.
- Kiro [immutable 2.34.0 archive](https://prod.download.cli.kiro.dev/stable/2.34.0/kirocli-x86_64-linux.tar.xz)
  SHA256 `11dce9286cf7401079cf4eac02a3d5982daac16a346d880109ec7b4a0b941040`,
  132,018,592 bytes. Do not reuse 2.33.1 companion hashes.
- Kimi [2.3.2 release](https://github.com/MoonshotAI/kimi-code/releases/tag/%40moonshot-ai/kimi-code%402.3.2)
  Linux x64 archive SHA256
  `4715ccd1a591b64683929a14b26b3d07ac4c484e4da3881b22a2ccca56304042`,
  65,938,009 bytes; release source `b8dd3a134060c6e80ba4a1659335cd63b68483f2`.
- Cursor [immutable acquisition candidate](https://downloads.cursor.com/lab/2026.10.22-27dcf5f/linux/x64/agent-cli-package.tar.gz):
  no native execution until its measured archive and runtime pins are frozen.
- VS Code proposed gateway declaration is present at the already pinned publisher
  commit `e646868227c3e3a9e3f24277c8c7220be42c0be7`, declaration blob
  `81e75fe69d74e82bdfb4ca69d0e89b801e728118`.

## Primary references for the reviewed observations

- Gemini: [skills](https://geminicli.com/docs/cli/skills/),
  [MCP](https://geminicli.com/docs/tools/mcp-server/),
  [configuration](https://geminicli.com/docs/reference/configuration/).
- Copilot: [BYOK](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/use-byok-models),
  [ACP](https://docs.github.com/en/copilot/reference/copilot-cli-reference/acp-server).
- Kiro: [commands](https://kiro.dev/docs/reference/cli-commands/),
  [permissions](https://kiro.dev/docs/permissions/), [ACP](https://kiro.dev/docs/cli/acp/).
- Cursor: [CLI MCP](https://cursor.com/docs/cli/mcp),
  [permissions](https://cursor.com/docs/cli/reference/permissions).
- Cline: [CLI](https://docs.cline.bot/cli/cli-reference),
  [MCP](https://docs.cline.bot/mcp/mcp-overview).
- Qoder: [session plugins](https://docs.qoder.com/cli/sdk/plugins),
  [SDK MCP](https://docs.qoder.com/cli/sdk/mcp),
  [CLI](https://docs.qoder.com/cli/cli-reference).
- Devin: [commands](https://docs.devin.ai/cli/reference/commands),
  [MCP](https://docs.devin.ai/cli/extensibility/mcp/configuration).
- Kimi: exact 2.3.2 [server API](https://github.com/MoonshotAI/kimi-code/blob/b8dd3a134060c6e80ba4a1659335cd63b68483f2/docs/en/reference/server-api.md)
  and [providers](https://github.com/MoonshotAI/kimi-code/blob/684bae9ea6a437f3bae98bba1dcb797a917d500b/docs/en/configuration/providers.md).
- Grok: [skills/plugins](https://docs.x.ai/build/features/skills-plugins-marketplaces),
  [MCP](https://docs.x.ai/build/features/mcp-servers),
  [BYOK](https://docs.x.ai/build/settings/reference).
- VS Code: [pinned gateway API](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/src/vscode-dts/vscode.proposed.mcpServerDefinitions.d.ts),
  [MCP operations](https://code.visualstudio.com/docs/agents/run/manage-mcp-servers).

## Grok and Qoder measured acquisition to fixed help

Genuine Grok run 40235269281, artifact 12615677952 (ZIP SHA256
`b8cccd65c059fab3d157edd4bb26474b7eab547458f4d8cf46222072660c7331`),
measured the existing official 1.0.70 gzip at 76,760,282 bytes/SHA256
`11d27a30adfb75c4b9b180072ab27412a1916f054ff5497ab70fb9a77d304bcc`.
Its sole `grok` executable is 195,700,192 bytes/SHA256
`22d281c5964c5444147328096ba2a221f3f0874c6858e27c825656a80f6e58e9`.
Freeze those pins; admit only acquire/help and the exact nine commands:
`--version`, `--help`, `inspect --help`, `mcp --help`, `mcp list --help`,
`mcp doctor --help`, `plugin --help`, `plugin list --help`, `doctor --help`.
Each has 30 seconds, exit zero, no capture error, empty stderr and strict UTF-8.
Require exact requested Usage command followed only by a standard Clap
metavariable or end-of-line, plus --help; main identifies inspect/mcp/plugin.
The source-defined version candidate is one whole `grok 1.0.70 (<7..40 lowercase
hex>)` line with optional ` [stable]`; retain the actual compiled commit/channel.
Public source snapshot `xai-org/grok-build@2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8`
is command guidance, not shipping-source provenance. Only `grok.help` may pass;
no inspect, trust, doctor operation, provider or MCP runtime executes here.

Genuine Qoder run 40235270603, artifact 12615882073 (ZIP SHA256
`09a2868892ec9c4eccd4a9f62d64a45c6024c977f51fa0bc524beb56ce5ae829`),
confirms the existing pinned archive and sole regular member `qodercli`,
182,982,112 bytes, matching the already frozen executable SHA256. Select that
entrypoint. Existing retained genuine 1.1.78 evidence (run 40079977099) already
binds exact version stdout `1.1.78\n` and `plugins` help; its parent Usage is
`qodercli plugins|plugin [options] [command]`. The next help-only table is
`--version`, `--help`, `plugins --help`, `plugins list --help`, `mcp --help`.
Require exact version and requested command-specific Usage, --help, zero exit,
no capture error and empty stderr, within 30 seconds each. Unknown main/MCP
grammar fails with bounded evidence; do not guess a successful shape.
Only `qoder.help` may pass. No import/list operation, session plugin, SDK,
user-cache registration, scope discovery or MCP runtime is part of this slice.
These pins and fixed-help additions retain every later qualification flag false.

## Kimi initial API research amendment

This next slice qualifies only observed research IDs, not a product version gate
or public-install lifecycle. Keep `kimi.help`; register `kimi.scope-observation`
only with the following actual native observations. Kimi 2.3.2/source
`b8dd3a134060c6e80ba4a1659335cd63b68483f2` remains the exact pin.

Root owns a finite `Context.service(label, arguments, observe, project='a',
timeout=60)` adapter. It reuses hosted admission, explicit environment,
before/after runtime identities, selector stream limits, bounded in-memory
capture and owned process-group cleanup. The callback receives bounded live
snapshots and the original deadline, not an unrestricted process launcher.
Only literal loopback HTTP is admitted, without proxies or redirects, with a
five-second/request deadline, one MiB/body and at most 60 fixed API requests.
The whole research collector remains limited to 360 seconds. No SDK, ACP,
TypeScript, npm, alternate MCP fixture or direct fixture call is introduced.

Root adds a separate real Git project C and supplies `ctx.native_probe`, built
from existing `repotests/testdata/agentplugins_native_probe` by exact Go 1.26.10.
Record compiler/probe identities; include the probe in immutable-file checks.
Projects A/B/C and outside cwd share the one original fresh HOME. The leaf owns
only benign uniquely named SKILL.md and `.kimi-code/mcp.json` entries in these
roots and an independent user marker. All authored paths remain ordinary,
single-link, bounded and outside alias traversal. MCP stdio commands select
the exact supplied probe, per-scope argv/cwd/events file and fresh nonce using
its existing `UAP_TEST_ROOT`, `UAP_TEST_EVENTS`, `UAP_TEST_NONCE` and
`UAP_TEST_REVISION` contract. Each entry sets documented startupTimeoutMs and
toolTimeoutMs to 3000; the fixed sequence uses at most 36 HTTP requests within
the shared ceiling of 60. No tool call is sent during this initial slice.

The only native service argv is `kimi web --no-open --host 127.0.0.1 --port 0`.
The exact source accepts port zero and reports the bound port. With NO_COLOR=1,
require `Kimi server ready`, the complete line
`  Local:    http://127.0.0.1:<port>/#token=<token>` and a matching
`  Token:    <token>` line. The token is 43 base64url ASCII characters.
Read only original HOME's `.kimi-code/server.token`: ordinary same-UID parent
mode 0700, regular/single-link token mode 0600, exact 43 bytes, no-follow held
read with before/after identity. Its bytes must match both native banner fields.
Register any parsed token privately immediately. Unrecognized or incomplete
startup quarantines the raw streams; they must never enter a failure artifact.
Validated streams are redacted before admission. HTTP uses Authorization only;
never publish bearer-bearing fragments, query strings or exception text.

Success JSON must be whole, duplicate-free, bounded, with integer `code:0`,
`msg:'success'`, bounded request_id and the following typed data. Other exits,
shapes and errors fail; only the two exact negative responses below are planned.

| Native request | Required observation |
| --- | --- |
| `GET /api/v1/healthz`, no auth | HTTP 200, `data.ok:true`. |
| `GET /api/v1/meta`, no auth | HTTP 401 and exact code 40101; proves native bearer admission. |
| `GET /api/v1/meta`, bearer | `server_version:'2.3.2'`, nonempty server_id, ISO started_at, dangerous_bypass_auth false, backend v2, empty open_in_apps, and true websocket/file_upload/fs_query/mcp/tasks/terminal capabilities. Optional experimental_flags is a bool map; features contain typed name/state/meta; web_title is a string. |
| `POST /api/v1/workspaces` with exact root/name for A/B/C | Distinct IDs matching `wd_[a-z0-9._-]+_[0-9a-f]{12}`, exact canonical root/name, ISO created_at/last_opened_at and initial session_count zero. |
| `GET /api/v1/workspaces/{id}/trust` | Initially false for A/B/C. |
| `POST /api/v1/workspaces/{A-or-B}/trust` with `{}` then GET | Native true for A/B, C remains false. Perform before preservation baseline; never forge trust state or trust C. |
| `GET /api/v1/workspaces/{id}/skills` | Typed name/description/path/source rows select only the original-user and selected project research markers at exact SKILL.md paths. C skills may be visible: workspace trust gates MCP, not this catalog. |
| `GET /api/v2/mcp/servers?cwd=<A/B/C/outside>` | A/B contain user plus their own entry; C/outside contain only user. Require exact project `.kimi-code/mcp.json` origin, source global, mutable false, stdio command/argv/cwd and sorted env keys. User origin is original HOME's file with mutable true. Source global denotes a file layer, not user scope by itself. |
| `POST /api/v2/mcp/servers:test` with only `{name,cwd}` | A/B success true and native output identifies inspect_runtime; require fresh same-nonce/PID/session initialize and tools/list probe events in that request window. No inline server or direct fixture connection. C's named test returns HTTP 200 with exact code 40408 and no C probe events. |
| Repeat A/B/C catalogs, then `POST /api/v1/shutdown` with `{}` | Protected user/B/foreign bytes remain exact; shutdown data.ok is true. If the foreground CLI remains alive, send ordinary SIGTERM to the owned group and require exit zero, closed port and reaped group within five seconds. SIGKILL fallback is failed cleanup, never a successful case. |

The source pin covers web shared.ts, persistentToken.ts, privateFiles.ts,
access-urls.ts, web/run.ts, rest-meta/meta, workspace/workspaces/skills,
routes/v2/mcp and shutdown routes. The web launcher passes CLI version 2.3.2 to
the server; the embedded server package version is not the reported CLI version.
New inert contracts must cover private-token quarantine/redaction, malformed
JSON/identity, wrong project origin, wrong probe nonce/session, the untrusted
negative and incomplete service cleanup. Preserve all qualification flags false.
Inspect genuine evidence before a separate two-round amendment admits the exact
product version set {2.3.1, 2.3.2}, provider tool calls or the final public UAP
lifecycle test `TestAgentpluginsKimiNativeProjectComponentsLifecycle`.

## VS Code trusted development-host amendment

Use the already pinned and observed 1.143.1 publisher, genuine Ubuntu 22.04,
ordinary-user/NNP/zero-capability/loopback namespace, strict cgroup, authenticated
Xvfb and normal Electron sandbox contract. Retain the existing untrusted mode.
Add focused `trusted_*_linux.go` files to the existing `vscodeworkspace` package
main and a closed request mode `trusted-project-v1`; absence of mode preserves
legacy labels, observer pins, budgets and behavior. Reuse that package's private
authority, sealing, capture, display and cleanup services directly. Do not copy
the controller, add a process primitive, disable trust/sandboxing or export a
new framework. Unknown modes, stages, paths and operations fail before launch.

Reuse the dedicated publisher acquisition/materializer and its bounded parser.
Exact archive: `https://vscode.download.prss.microsoft.com/dbazure/download/stable/e646868227c3e3a9e3f24277c8c7220be42c0be7/code-stable-x64-1792691793.tar.gz`,
350,844,741 bytes, SHA256
`48f658052e54e020a1c1be7b37ac0f9ae39f07e8ccda805c63f053cfda72b2be`.
Actual retained inventory: 4,262 members, 1,034,475,520 expanded bytes,
1,031,068,984 regular-file bytes, zero omitted symlinks. Do not increase the
generic extractor caps. Verify publisher `code` (223,685,880 bytes,
`cf8508cc7ddde30cfccc70caeda877a9a99119953c4476dcb30a3eb1a2c415e9`),
`resources/app/out/cli.js` (275,169 bytes,
`bd393ad6f0d28ba0aa7cd79cc685617f5187096f092009e315a4f098349f7976`),
and `resources/app/product.json` (68,251 bytes,
`a141a1de8ed44415c0c527b1882362e6abc25e674be052d67c09da71a27fbd7e`).
Keep ELF-plus-cli.js version/help preflight; ELECTRON_RUN_AS_NODE=1 is confined
to that fixed preflight and is absent from GUI execution.

Freeze one minimal lock with the reviewed Node 24.21.0 archive and these packages:

| Package | Version | Integrity |
| --- | --- | --- |
| @typescript/typescript-linux-x64 | 7.0.2 | sha512-EYdf2cNg7rgCWJnxCdJ+F3V39O8ihb37eHAu1LK8oAFizgTQbPOK7zHHXbPt8rX24COqODXeI3sIf0fCXG7H/A== |
| @types/node | 22.20.1 | sha512-EANqOCF9QFyra+4pfxUcX9STKJpCLjMbObVzljIJomAWSnuSIEAvyzEU53GaajbXJEgdh0iEcPL+DGvpUd4k1Q== |
| undici-types | 6.21.0 | sha512-iwDZqg0QAGrg9Rav5H4n0M64c3mkR59cJ6wQp+7C4nI0gsmExaedaYLNO44eT4AtBBwjbTiGPMlt2Md0T9H9JQ== |
| playwright-core | 1.64.1 | sha512-BC4s8y6SEVA6Q1rSxZTto8hI1KUI3xXlHGZgvGqP4IqPdI/a6FX9H29IDNz7vKo9l21b6LfYosScPL6M3xxyDQ== |

The 2026-10-26 continuation honors the owner's TypeScript 7.0.2 minimum. Use the
official Linux-x64 compiler package directly, with no wrapper or optional
platform fallback. Its exact regular `lib/tsc` is 24,101,026 bytes, SHA256
`4f2de678286401759b3fb4475bafe35b8f32b4b3a07d92642bbf37eadc9b34a4`.
The official package integrity, binary identity and complete build-only library
closure are checked before and after compilation. This amendment received two
independent plan reviews; it does not change the old untrusted observer.
Use exact verified Node/npm distribution bytes and
`npm ci --ignore-scripts --no-audit --no-fund` in a temporary tool directory
before native network isolation. No ambient compiler/npm, lifecycle scripts or
browser downloads. Compile tracked TypeScript with the pinned `lib/tsc`, strict
ES2023/NodeNext and pinned types; emitted JS stays in the temporary build tree.
The VS Code declarations are build-only files from the exact publisher commit:
`vscode.d.ts` (742,500 bytes, Git blob `bd520cf7bb88454e99bd15da13c6b9c6ac56fd65`),
`vscode.proposed.chatPromptFiles.d.ts` (13,619 bytes, blob
`eb77adbd1a1084837ab2d7b8f669f8ecfc20f1b8`), and
`vscode.proposed.mcpServerDefinitions.d.ts` (3,302 bytes, blob
`81e75fe69d74e82bdfb4ca69d0e89b801e728118`). Fetch exact-commit raw source,
verify byte count and Git blob digest, and include it only in temporary compiler
inputs. No unpinned @types/vscode or emitted declaration runtime is introduced.
Vendor declarations may use skipLibCheck; our observer and driver remain strict.
playwright-core attaches to the publisher's Electron, never a substitute browser.
Seal the emitted driver/observer package manifests and module mode together
with the complete pinned playwright-core runtime JS/support-file closure;
TypeScript compilation does not bundle dependencies. Compiler/types stay in
the build directory, npm .bin links are excluded, and native execution must not
resolve modules from an unsealed tool cache or ambient node_modules.

The trusted mode admits verified Node plus the sealed compiled UI driver through
the existing strict primitive. The driver starts the exact publisher using the
existing same-process isolation-verifier prelude. Its observed namespace must
match the driver, and its PID must match the spawned child before execve.
Fixed GUI argv: `code --new-window --disable-crash-reporter --locale en
--user-data-dir <one-original-profile> --extensions-dir <owned-empty-extensions>
--extensionDevelopmentPath <sealed-observer> --extensionTestsPath <sealed-observer>/runner.js
--enable-proposed-api uap-research.project-runtime-observer
--remote-debugging-port=0 <exact-A-or-B>`.
Admit exactly one child-emitted `ws://127.0.0.1:<1024..65535>/devtools/browser/<id>`
with no credentials/query/fragment or conflicting endpoint, and exactly one
corresponding workbench page. Attach inside the same namespace, with no fixed
port, outside-network, alternate-browser or generic-dialog fallback.

Pinned source fixes the following finite UI grammar. Workspace startup title is
`Do you trust the authors of the files in this folder?`; click only
`Yes, I trust the authors` after matching the canonical folder in the details
and keeping `Trust the authors of all files in the parent folder '{name}'`
unchecked. Require the observer's exact folder and workspace.isTrusted true.
Trust A/B separately before preservation baselines; A must not implicitly trust B.
The separate `Trust Folder & Continue` flow is not an admitted fallback.

The gateway broker uses errorOnUserInteraction, so start each owned MCP server
through native UI first: `MCP: List Servers`, `Select an MCP Server`, exact
fixture row, `Select action for '<label>'`, `Start Server`. If required, accept
only `Trust and run MCP server <label>?` with action `Trust` and exact expected
configuration origin URI (inspect href without navigating). Reject multi-server
trust, Pick Trusted, unknown enablement/auth dialogs and all broad approvals.
Source at the publisher commit: workspace.contribution.ts blob
`6cccbced7b7c304d1e9678fab1559a584f9ecc44`, mcpCommands.ts
`1addb6dad8938da5442b43160b4400cf4a4adbce`, mcpRegistry.ts
`607334861d0fbcd8691e9b0a98c44b6a326c6937`, gateway broker
`8f90bc7d6eb0e47ceee42f73986ba9767b4f8d8a`. Unknown actual UI fails and requires
an amended interaction; source support alone is not runtime evidence.

The observer uses actual workspace/chat skills/MCP definitions and
lm.startMcpGateway, enabling only chatPromptFiles and mcpServerDefinitions.
It never grants trust, registers replacement definitions or directly calls the
probe. Combine the gateway label/address with exact definition command/argv/env/
cwd and newly observed probe PID/session, nonce, revision, package/data/cwd facts.
The stage challenge is the existing inspect_runtime write/read marker; do not
invent extra probe arguments or returned fields. Admit initialize/list/call only
through the vendor gateway. Keep endpoint/session secrets private.

Select exact `TestAgentpluginsVSCodeNativeProjectGatewayLifecycle` for the
existing required full A/B/user public install/no-op/update/local-source repair/restart/
remove/repeat-remove sequence and the five `vscode.*` qualification IDs above.
New observer sources live in `scripts/vscode-project-runtime-observer/`; the
native test lives in `repotests/agentplugins_vscode_project_native_gateway_test.go`.
The root owns toolchain preparation/workflow wiring; the editor owner implements
the focused controller and observer. Lifecycle success preserves user/B/foreign
configuration, recorded ownership and retained data. State remains Prepared.

Trusted mode alone has 900 seconds total from original monotonic start,
840 seconds work and 60 seconds final teardown/admission. Public UAP operations
have 30 seconds; GUI stage 60 seconds; trust and each MCP-start interaction
15 seconds within that stage; gateway requests 10 seconds/one MiB. Stop starting
GUI stages with less than 75 seconds work remaining. Dispose gateway/DevTools,
allow five seconds graceful shutdown, then existing strict tree teardown within
ten seconds total. Require driver/workbench/MCP tree reaped and cgroup empty;
final cleanup also proves Xvfb/display assets removed. Legacy 420/480-second
limits remain unchanged. Missing protocol, identity, native UI or cleanup fails.
This is only a trusted development-host/proposed-API path, not model/OAuth or
released-client qualification.

The original monotonic origin is established before acquisition and compilation
and is passed unchanged into the controller. Trusted root return and terminal
capture bounds are 870/880 seconds; the final envelope remains within 900.
Only the newly owned network namespace has loopback brought up, before dropping
to the original ordinary UID/GID and verifying zero capabilities/NNP again.
The sealed driver receives one closed stdin request, writes an exclusive READY
challenge after the exact native trust/MCP interactions, and consumes the
observer's atomic result. It keeps its leader alive until the existing strict
planned-shutdown helper closes stdin. It never opens another browser or invokes
the probe directly. Failure diagnostics are bounded to visible native dialog
titles/messages and quick-pick labels, with URL/opaque values removed.

For the original user, the existing public adapter remains Prepared; record
`user_native_install_qualified=false`. Preserve a separately pre-existing native
user skill sentinel through the real skill API without claiming that the public
user add created it. Freeze authored sources before any native process and
freeze the public user receipt/package/data immediately after its add. The A/B
project assertions use exact ownership objects, independently computed installed
skill/config digests, package/source identity, and fresh probe sessions. The
missing-component repair keeps the selected local input unchanged at revision
two. The separate existing TLS composed-source tests retain moving-source and
recorded-pin coverage; the native local fixture does not claim that property.

Frontend preparation and native evidence use separate flat closed ZIPs, each
under the existing admission limits. Hold the preparation digest across native
execution; publish post-native streams through the original directory identity
and no-follow descriptors. Unknown files, replaced directories, incomplete
stages, forced cleanup, or late results cannot qualify.

### Observed VS Code first-launch settings correction

Trusted run [40359430087](https://github.com/777genius/universal-agent-plugins/actions/runs/40359430087)
at source tree `e3ca3a42425acbf8777495ead681d5de2d8057e7` completed pinned
preparation but failed the first `empty-a` stage. Artifact `12667643294`
reports only the native dialog `Welcome to Visual Studio Code` during workspace
trust, changed protected user `settings.json` bytes and unproved teardown.
All five qualification flags remain false; no runtime lifecycle is inferred.

Before rerunning, author two current settings in the fresh original user profile:
`workbench.welcomePage.experimentalOnboarding=false` and
`extensions.autoUpdate="off"`. At the exact pinned publisher commit,
`startupPage.ts` blob `777f3ad872c6de562252c1635e423c63692a3225` honors the
onboarding setting before opening the optional welcome modal;
`gettingStarted.contribution.ts` blob `067580a4b11550f219f8031d4c9b3827c140121c`
registers that boolean. `extensions.contribution.ts` blob
`695859cadf68c11ffc5d7a94c3d4a5302343aa3f` accepts the current `"off"` value
without migrating it, while the old fixture boolean `false` triggers a native
rewrite to `"off"`. This source-supported correction does not establish that
the migration was the sole cause of the observed protected-byte difference.

Freeze these authored bytes before the first native child using the existing
strict inventory. Do not adopt a post-native settings baseline. Preserve the
exact workspace/MCP trust UI, updater suppression, normal Electron sandbox,
ordinary-user isolation, process primitive, source pins, deadlines and closed
artifact inventory. Do not add a welcome-dialog action, authentication step or
cleanup exception. Run the existing inert controller contracts, then rerun the
exact trusted lifecycle on genuine hosted Ubuntu 22.04. Any remaining native
settings change, unknown UI or cleanup failure still fails and requires its own
observed diagnosis. Two independent plan-review rounds precede implementation.

### Observed VS Code startup focus investigation

The corrected genuine [run 40363268218](https://github.com/777genius/universal-agent-plugins/actions/runs/40363268218)
at source tree `b89e5d8774d4c6c502dbb48117999afeceb443a9` again failed
`empty-a` during workspace trust, now with no visible dialog. Native artifact
`12669876292` (SHA256
`5b94fbd4aa55d6c291cf1957120a4d79dfa2105be43bd0b53664a623134f4f7c`)
contains no welcome modal or protected-settings error. Teardown remains unproved
and every qualification remains false.

At the pinned publisher commit, `workspace.contribution.ts` blob
`6cccbced7b7c304d1e9678fab1559a584f9ecc44` starts the trust modal only when
`hostService.hasFocus` is true or its focus event subsequently fires. The native
`nativeHostService.ts` blob `86a3d521e1bef4ff05c0d91c949c91cdca643e4b`
implements that getter with `getActiveDocument().hasFocus()`. The native
environment inherits `disableWorkspaceTrust` only from the explicit
`--disable-workspace-trust` argument (`environment/common/environmentService.ts`
blob `004d0614c938a3e9371c4f78a92a2a57b0058a10`); the frozen native argv does
not contain that flag. Neither development nor test entrypoint presence is that
getter's condition. The retained artifact does not measure native window focus;
focus starvation is a source-supported hypothesis, not an observed cause.

After admitting the sole actual workbench and before the unchanged trust flow,
record only the boolean result of the constant expression
`document.hasFocus()`, call the existing pinned Playwright
[`page.bringToFront()`](https://playwright.dev/docs/api/class-page#page-bring-to-front)
on that same page once, and wait at most three seconds for a true boolean.
Bound the complete focus step to five seconds inside the original 60-second
stage. Retain before/after booleans with stage/nonce in one bounded
`driver-focus` diagnostic in the already admitted merged stream. No document
content, endpoint, new artifact member or new result-envelope field is exposed.
Use the driver's existing bounded-promise helper for the focus action and reads.
Missing/malformed booleans or a false final focus fail before any trust or server
action; an initially false focus is retained as an observation.

This is normal activation of the already selected genuine workbench; it does not
set trust, invoke a hidden native command or synthesize a focus event. Keep the
exact canonical-folder trust dialog and unchecked-parent assertion, all existing
MCP UI/API, source/tool pins, settings baselines, process primitive, sandbox,
deadlines and cleanup gates. Do not substitute a resource-trust dialog or accept
absence of the expected dialog. Run existing inert contracts and pinned strict
TypeScript compilation, then inspect the next genuine hosted artifact. Two
independent plan-review rounds precede this narrow driver correction.

### Observed VS Code integration-test dialog suppression

Genuine [run 40365228366](https://github.com/777genius/universal-agent-plugins/actions/runs/40365228366)
at source tree `7d4ed4d9a114f26f6c2d1af834331f7e715b31b5` measured native
document focus true both before and after activation, but still found no
workspace-trust dialog in `empty-a`. Native artifact `12671090860` has SHA256
`a6ad556f079a2b78715aa89f226d860984f840b6f527d351eed4945b97fe262d`.
Focus starvation is not supported by that observation. No stage completed;
cleanup and all five qualification cases remain unproved.

The exact pinned publisher's
`src/vs/workbench/services/dialogs/common/dialogService.ts`, blob
`20ddfdcf4ec10c6ef509095f948604b37bad4e8a`, refuses `prompt()` whenever both
extension-development mode and an extension-test entrypoint are present. The
frozen trusted argv selects precisely that integration-test mode. This is a
source-supported explanation for the missing dialog; it must not be treated as
an approved trust decision or bypassed by changing a native trust service.

For trusted mode only, remove the `--extensionTestsPath <observer>/runner.js`
pair from both the controller's fixed argv and the driver's exact argv validator.
All other arguments, proposed APIs, publisher and module pins stay fixed. This
supersedes only that pair in the earlier trusted argv contract. Keep the legacy
untrusted observer's independent integration-test invocation unchanged.

Use the owned development extension's existing `onStartupFinished` activation
event and sealed `extension.js` main module to invoke and await the existing
`runner.js` exported `run()` function. The publisher's normal activation loader
is `extHostExtensionService.ts` blob
`42008549e96dff8258de6d38d1b8c3e89df8735d`. Declare only this observer's
`capabilities.untrustedWorkspaces.supported=true`, supported by the pinned
`extensionsRegistry.ts` blob `4f9fc31696f3c6d40d70320cda4d10a076fc8e86`.
That lets the passive observer wait for the existing sealed UI challenge while
the workspace is initially restricted. It registers no command, definition,
tool or trust permission. Keep the complete sealed runtime closure unchanged.

The observer still waits for the driver challenge and requires actual canonical
folder identity and `workspace.isTrusted=true` before observing installed skills
or calling the gateway. The driver still accepts only the exact native workspace
trust dialog with the parent checkbox unchecked, then the existing exact MCP
trust/start flow. No trust database, hidden command, synthetic UI event, native
source patch, blanket approval or sandbox override is introduced. Preserve all
original-user baselines, nonce/session joins, deadlines, artifact admission and
strict cleanup. Run existing inert contracts and pinned strict TypeScript
compilation, then inspect genuine hosted lifecycle evidence. Two independent
plan-review rounds precede this narrow controller/observer correction.

### Observed VS Code startup-prompt setting

Genuine [run 40368973193](https://github.com/777genius/universal-agent-plugins/actions/runs/40368973193)
at source tree `00a4a30cf9b4065feca1b0eec577afca3b61126f` used normal
extension activation but again failed `empty-a` with native focus true/true and
no visible workspace-trust dialog. Native artifact `12672882701` has SHA256
`db8b4be131ece70b3121b6b2d153d84fef63f50d00e0667ebd9db01243aaed04`.
No stage or qualification completed, and cleanup remains unproved.

The same exact pinned `workspace.contribution.ts` blob
`6cccbced7b7c304d1e9678fab1559a584f9ecc44` registers
`security.workspace.trust.startupPrompt` with default `"never"` and explicitly
returns from startup trust prompting for that value. The fixture previously
set trust enablement but omitted the prompt setting. Author only
`security.workspace.trust.startupPrompt="always"` in the fresh original user's
settings before the existing strict preservation baseline. This requests the
already frozen native dialog; it grants no trust and does not substitute any
other interaction. Native trusted workspaces still skip prompting, while A/B
must each receive their own exact native trust decision. Preserve the normal
development-host invocation, canonical folder/unchecked-parent checks, passive
observer handoff, all existing MCP UI, settings bytes, sandbox and deadlines.

### VS Code source and installed-package receipt framing

Independent source inspection found an additional fixture mismatch before any
native run reached public installation. VS Code has no
`QualifiedLocalInstallerFacade` trait, so its fixed public local-source adds use
the existing lifecycle service and `providers.Stager`. The provider records
installed `managed_package_directory` digests from
`packagesnapshot.Builder.Build`, while the current trusted fixture uses its
newer `packagedigest.DigestCaptured` source digest for that installed receipt.
The authored `SourceDigest`, selected source binding and package revision
correctly retain the newer acquired `PackageSnapshot.TreeDigest` identity.
No native installation or product defect is inferred from this source finding.

Preserve the current bounded, held, ordinary-file/directory capture and its
canonical parent, original UID, single-link, path/depth/size checks. Keep
`packagedigest.DigestCaptured` policy validation and source identity unchanged.
Add only a private installed-receipt helper within `vscodeworkspace`, using the
provider's exact lexical NUL-delimited `dir`/`file`, slash-relative path,
executable boolean, decimal byte length and held file bytes. It has no algorithm
selector, alternate fallback or returned-receipt adoption. Route only installed
managed-package expectations through it; maintain complete exact object
count/kind/ID/name/path/digest equality and all separate probe/launcher checks.
Do not add a production digest API or change any provider, adapter or installer.

Keep the acquired-source parity test and add meaningful inert user-skill and
mixed project cases through the actual VS Code adapter, project planner and
`providers.Stager`. Compare held installed bytes with the actual staged object
and `packagesnapshot` verification, require the source identity to remain the
newer snapshot digest, and demonstrate the two framings differ. Retain negative
source/receipt/probe-adoption/link checks and exercise file/executable/empty
directory changes. No vendor process executes in these tests. Independently
review both subsections twice before implementation, then rerun the exact
genuine hosted lifecycle; every unobserved or failed stage remains unqualified.

### Observed VS Code pinned-scanner cache handoff

Genuine same-head retry of [run 40373529167](https://github.com/777genius/universal-agent-plugins/actions/runs/40373529167),
job `121768023210`, executed source `ee4556e222cc14e5e1c355428b42e8d4df4a7ace`
(tree `46a998ba83fc8e6d3f905ab37d7129c598d950d7`). Native artifact
`12677341866` has SHA256
`e9f7b93e0c68f99895d86c9ca2f5ac23919b75e2e3013a4307e6b1446eb1ec5d`.
Both `empty-a` and `empty-b` completed with separate exact native trust decisions,
actual trusted canonical folders, inherited user skill discovery and publisher
exit zero. The next public `add-user` failed before installation: its ordinary
security evaluator attempted to download lintai inside the loopback-only network
namespace and DNS access was refused. Installed-package checks and native MCP
stages were not reached; all five qualifications remain false.

The frontend already acquires archive SHA256
`2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da`
and its exact executable member SHA256
`b6723451f155eb49e84acb34d1f06de929bddfcff6238cd4e38cdc6519396fba`.
`sealTrustedInputs` currently seals this as `<base>/executables/lintai`, but
`trustedCommand` sets `AGENTPLUGINS_HOME=<trustedRoot>/installer-state`.
Production `compose.go` constructs `securityscan.ReleaseScanner` at that home's
`security/lintai` cache, and `release.go` resolves only
`0.1.3/linux-amd64/lintai` underneath it; putting lintai on PATH cannot select
it. This is a dedicated fixture handoff omission, not a production scanner or
network-policy defect. The existing ordinary native fixtures already prepopulate
this exact supported cache before their public installer operations.

The first cache plan failed both independent reviews: a root-owned read-only
leaf can be temporarily substituted through a native-owned writable ancestor,
and post-command equality cannot prevent executing that substitution. Source
inspection also rules out a root-owned sticky installer-state directory:
`statev2.Store.Save` and `processlock.Lock.Acquire` unconditionally chmod their
state/lock parent to `0700`. The user-scope public CLI must own that directory.
Do not change those production semantics or assume sticky modes survive them.

Use this exact fixture-only filesystem seam instead, before any native child:

| Node below sealed base | Owner/group | Mode and authority |
| --- | --- | --- |
| `trusted` | `0:0` | `0755`; prevents replacing the state ancestor |
| `trusted/installer-state` | original UID:GID | `0700`; unchanged CLI state/lock/temp/managed writes |
| `trusted/installer-state/security` | `0:0` | `0555`; one exact task-owned self-bind mount |
| `security/assessments` | original UID:GID | `0700`; real assessment-cache temporary/atomic writes |
| `security/lintai`, `security/lintai/0.1.3`, `security/lintai/0.1.3/linux-amd64` | `0:0` | `0555`; fixed nonwritable scanner ancestors |
| `security/lintai/0.1.3/linux-amd64/lintai` | `0:0` | `0555`; exact pinned, ordinary single-link ELF copy |

All paths are the fixed descendants above, never caller-selected mount operands.
Keep the existing immutable sealed scanner source and archive/member pins.
Create the fresh cache copy with bounded held-parent/file operations, exclusive
creation, no alias following or replacement, and exact ELF/hash checks. Precreate
only empty ordinary `a-events.jsonl` and `b-events.jsonl` at `trusted`, owned by
the original UID:GID with mode `0600`, before sealing that root. The probe's
existing `O_CREATE|O_APPEND` works on these files without directory write access;
`trustedEvents` already treats absent and zero-byte logs as the same empty byte
stream. No native event is fabricated. Its marker still writes only plugin data.
Other existing home, project, result, source and temporary children retain their
original-user ownership and writable layouts; later authored source/result writes
stay inside those precreated children.

One self-bind mount of the exact `security` directory in the controller's current
mount namespace prevents renaming/unlinking this entry through the writable
state parent. Do not add a new mount namespace, recursive mount, alternate view,
scanner override or mount on a system/shared directory. Every fixture process
and existing view of this one task path sees the same protected mountpoint.
The nonwritable root-owned subtree protects scanner contents; the native-owned
assessment child remains writable. Primary Linux source at commit
`e8f897f4afef0031fe618a8e94127a0934896aba` backs this seam: `fs/namei.c` blob
`9342fa6a38c2bad85c13144b8d8ae4940e88e7e6` returns `EBUSY` for rename of
old/new local mountpoints and rmdir/unlink of mountpoints; `fs/namespace.c` blob
`5a51315c6678145467520800ceedc3378df5e7da` makes that test namespace-local
and requires namespace `CAP_SYS_ADMIN` for mount/unmount. This is design support,
not a claim about the actual runner; the hosted filesystem/UID contracts below
must succeed, or native execution is refused. Original native UID, NNP and zero
capabilities remain unchanged, including no authority to unmount this boundary.

Hold and record the original dev/inode, UID/GID and declared mode of every
ancestor from the admitted sealed base through the cache and both branches,
plus the cache file's size, single-link identity and digest. The root-owned
`trusted`, `security`, `lintai`, version and platform directories and cache file
retain their owner/mode and linked identities throughout child execution; the
security mount remains bound throughout that interval. The native-owned
`installer-state` and `assessments` directories may legitimately chmod themselves
and perform the documented state/temp/assessment writes. Their original inode
and owner/group remain fixed, and their mode must be `0700` at each existing
`checkTrustedAssets` stage boundary; their mode is not an immutable in-child
invariant. Directory mtime/ctime/size are not frozen where entries legitimately
change. Native ownership of `installer-state` does not authorize renaming that
directory, because its `trusted` parent is root-owned and nonwritable; likewise,
`assessments` cannot be replaced through its nonwritable `security` parent.

Record and require the exact security mount ID independently of dev/inode;
missing, detached, replaced or foreign/stacked mount state fails closed. Recheck
all original records and declared boundary modes in every `checkTrustedAssets`;
never recapture a replacement baseline. Keep strict read-only scanner-subtree
preservation too. Once sealed, `trustedDirectory` and `trustedWrite` must not
chown, chmod or rewrite a root-owned protected ancestor/cache node. A controller
helper request for any recorded directory may only verify its original record
and current boundary mode, not repair a changed one. The native owner's normal
CLI writes and mode operations above remain permitted. Other ordinary fixture
mutations retain their existing held IO guards.

After the existing child/display supervision returns, release only this exact
recorded task mount, after matching all held ancestry and mount identities, using
ordinary unmount with no lazy/force/fallback behavior. Then retain the existing
held-root deletion. Unexpected identity, busy/unmount failure or root-cleanup
failure makes cleanup incomplete and qualification false. A resource-unmount
attempt after a failed exchange cannot turn that child's failed tree proof into
success. No controller/native sequence may execute after releasing this boundary;
no new evidence member or successful-cleanup exception is introduced.

The public CLI must continue using its unmodified required ReleaseScanner,
normal security policy/report validation and ordinary assessment cache. Do not
add a scanner override, fake security result, inherited cache, PATH fallback,
network interface, native download, alternate pin or vendor execution locally.
Keep source/receipt equality, original-user authority, native trust/MCP UI,
sandbox, command tables, original deadlines, closed artifacts and qualification
rules unchanged. After the observed public failure the existing display exchange
was unsuccessful, its strict supervisor reported forced cleanup and display
unlink authority was withheld. Retain that honest cleanup failure and the current
success-only teardown/asset-unlink contract; no separate cleanup change is
justified by this evidence.

Add inert held-identity contracts for exact cache placement, missing or changed
file bytes/mode/inode/owner, original ancestor replacement/mode/owner/symlink,
hard links, unexpected destinations, pin mismatch and mount identity/cleanup
refusals. On the genuine hosted privileged test path, build the same finite
layout using inert ELF bytes, then actually drop to the original UID/GID with
zero capabilities. Denial tests target the root-owned `trusted`, `security`,
`lintai`, version and platform directories and scanner leaf: attempts to rename,
exchange, unlink, rewrite or chmod those nodes, including temporary substitution
of the security entry, must be denied before mutation. Also require denial of
renaming/replacing native-owned `installer-state` and `assessments` through their
root-owned nonwritable parents. These negative tests do not prohibit the native
owner's chmod or documented contents writes within those two writable
directories. Root-only chmod-and-restore demonstrations do not satisfy this
contract. Positive tests in that same UID require owner chmod to `0700`, real
statev2 save, mutation lock, state/temp writes, assessment-cache store/atomic
replacement and normal event-log append to succeed. Check the declared native
directory modes at stage boundaries, with owner mode drift detected there;
root-owned chain owner/mode and mount binding stay protected during execution.
Independently exercise real ReleaseScanner
cache resolution with an expressly inert executable and a transport that
refuses/counts network requests: PATH alone must miss; the seeded cache must run
without download. Empty inert output is not a security report or a qualification.
The hosted contract must also prove exact ordinary unmount and owned-root cleanup;
unsupported privileges/filesystem semantics fail before any vendor, without a
weaker fallback. Local environments without these privileges may run only the
remaining inert contracts and cannot claim that hosted gate passed. No vendor
process executes locally. Two independent reviews of this revised plan precede
fixture implementation, independent code review and the exact hosted lifecycle.

### Observed scanner metadata-contract timestamp collision

Genuine hosted [run 40389182925](https://github.com/777genius/universal-agent-plugins/actions/runs/40389182925),
job `121805371213`, checked exact source
`bd42cfedc8a8c48dce0615f008981089c3590598` (tree
`459e041d038a2aa9251b148c3bda7f9c93c4b00d`). Its inert controller contracts
failed in `TestTrustedScannerHeldAncestryRejectsReplacementAndMetadataDrift/file-bytes`:
the metadata-only held-node check accepted a same-size rewrite from
`held original` to `held changed!`. The log does not contain the actual stat
timestamps; a same-quantum timestamp collision is an inference from that check's
size/mtime/ctime comparisons. No native lifecycle ran and no native artifact was
admitted. This result does not establish failure of the separate pinned digest
and full subtree inventory checks in `trustedScannerCache.verify`.

Keep held-node verification's existing inode, ownership, mode, single-link,
size, timestamp and mount rules. Make its byte-size mutation contract
deterministic by naming that subcase `file-size` and writing distinctly longer
bytes. Extract only the existing unchanged pinned-digest and strict-inventory
tail of `trustedScannerCache.verify` into `verifyContents`, called at the same
point after all existing ancestry and mount checks. Do not add hashing, change
admission order, relax an authority check or move a production boundary.

Exercise that exact content helper with an actual ordinary temporary cache
subtree. First require its original digest and inventory to pass. Then require
refusal of an equal-size byte rewrite even after restoring the original mtime,
and independently require refusal of scanner mode and empty-directory inventory
changes. The metadata-only subcase no longer claims to prove byte integrity;
the production content check proves that separate contract. Preserve the genuine
hosted original-UID/self-bind/cache-route gate unchanged and require it before
the editor. Two independent plan reviews precede this fixture-only amendment,
followed by independent code review, focused inert validation and a fresh exact
hosted lifecycle. All unobserved native stages remain unqualified.

### Observed VS Code inert deadline contract on a short-uptime host

Genuine hosted run 40389182726, job 121805371389, checked source bd42cfedc8a8c48dce0615f008981089c3590598 (tree 459e041d038a2aa9251b148c3bda7f9c93c4b00d). Before any untrusted native workbench observation, TestTrustedDeadlineKeepsOriginalOriginAndClosedMode failed at trusted_lifecycle_linux_test.go:47: "legacy budget changed monotonic task budget authority refused". The later missing ZIP is a consequence of never starting observation, not native evidence.

The inert test constructs its real CLOCK_MONOTONIC origin by subtracting 100 seconds without checking uptime. deadlineAtMaximum correctly refuses nonpositive, future or over-budget origins. A host with less than 100 seconds of monotonic uptime therefore cannot satisfy that test's setup. The logs contain no sampled clock value; short uptime is a source-backed inference, not observed telemetry. The roughly 21-second controller build/test log interval does not itself measure system uptime.

Change only the deadline test in install/integrationctl/research/vscodeworkspace/trusted_lifecycle_linux_test.go plus this recorded plan. Let the artificial elapsed interval be min(100 seconds, CLOCK_MONOTONIC nanoseconds / 2), require it to be positive, and derive the one original StartNS by subtracting that interval from the sampled monotonic time. Require the same legacy and trusted work deadlines to retain that origin: 420 seconds minus elapsed and 840 seconds minus elapsed, with the existing one-second tolerance. Add explicit zero and negative origin refusals alongside the existing future-origin and closed-mode negatives. Do not sleep, skip the test, reset an origin, mock the production clock or change production deadline logic, limits, native execution, permissions, acquisition, cleanup or qualification.

Two independent reviews of this exact plan precede implementation. Review the exact implementation independently, then run the focused actual deadline/Trusted inert contracts and changed-lines lint using the existing shared caches. The corrected scanner-content candidate b1f23a707894d83e7d3f3315304a54de6f265374 remains independently reviewable; this is a separate test-only correction. Genuine hosted untrusted/trusted runs remain mandatory and all unobserved native stages remain unqualified.

### VS Code Required-lane lint completion

The published scanner-content and short-uptime corrections are reviewed local
source `ea13f5bd067752b12da917972373a7c2c61bfa0a`, tree
`355cd394b6caf21a17e230949a3040f00ad1963e`, published as
`73f6b9c3e26709409d8ad9e1502683675683ebad`. Focused trusted contracts pass;
the genuine hosted UID/mount and complete editor lifecycle remain mandatory.
Do not fold this separate lint completion into those earlier evidence claims.

An unfiltered golangci-lint 2.13.2 run over the VS Code controller package found
37 issues. All are introduced by this VS Code PR relative to exact program
parent `b16b5c5d28cdc734856ac414521d40d9c8d8dfd5`, rather than inherited
parent debt. The six trusted files below are absent at that parent. The other
three parent blobs are `69769e21d82a22da8f9beeaa5cac63dad587491c`
(bootstrap), `df94165a627843df50e9a33534fd25ed1e568923` (workbench), and
`20ee7541e104e2a693cf6741271e0e1cc413ef00` (files), independently checked
through the GitHub file API and against the local parent source.

| File below `install/integrationctl/research/vscodeworkspace/` | Findings |
| --- | --- |
| `trusted_fixture_linux.go` | 14: errcheck 3, funlen 3, gocognit 4, gocyclo 3, revive 1 |
| `trusted_contract_linux.go` | 3: funlen, gocognit, gocyclo |
| `trusted_lifecycle_linux.go` | 3: funlen, gocognit, gocyclo |
| `trusted_observe_linux.go` | 9: funlen 2, gocognit 3, gocyclo 3, staticcheck 1 |
| `trusted_receipt_linux_test.go` | 2: errcheck, prealloc |
| `trusted_lifecycle_linux_test.go` | 1: predeclared `copy` name shadowing |
| `bootstrap_linux.go` | 1: file length crossed 500 lines |
| `workbench_linux.go` | 1: file length crossed 500 lines |
| `files_linux.go` | 3: preserved default-limit parser wrappers became unused |

Clear these actual Required-lane findings through cohesive extractions and the
small correctness/style fixes below. Keep the existing package, Linux build
tags, original entry points and concrete fixture authority. Do not change
production adapters or installer behavior, linter settings/baselines, directives,
client pins, native argv/environment, process primitives, time budgets, trust,
permissions, source/receipt identity, event grammar, artifact membership or
qualification. No generic runner, validation framework or alternate engine is
introduced. New helper files are in this same controller package only.

#### Concrete extraction and preservation map

- `copyTrustedTools`: extract the existing bounded filesystem-walk callback into
  a named tool-entry copier using the same context, root/target, UID, expected
  and seen maps and one original entry counter. Keep manifest checks before the
  walk, exact per-entry check/copy/seen order, complete-member check, package
  validation, final manifest rehash and output write in their present order.
  This is a finite fixture helper, not a reusable copy abstraction.
- `trustedFixtures`: extract the contiguous home/profile/foreign-user setup and
  project/source setup into named fixture helpers. Keep nonce creation and
  trusted-state initialization first, existing directory/settings/foreign/user
  writes in their present order, then A/B/user project/source creation, scanner
  preparation and protected-baseline registration. Preserve every literal and
  original ownership boundary. Keep scanner sealing before native execution.
- Move the existing package capture/digest functions into
  `trusted_package_linux.go`; extract only their walk callback to reduce nested
  complexity, retaining the same captured-entry order, counter/byte/depth bounds,
  parent checks, ordinary IO, context checks and both distinct digest framings.
  Move receipt functions into `trusted_receipt_linux.go`; leave fixture/state
  and preservation helpers in `trusted_fixture_linux.go`, below its file limit.
- `trustedReceipt`: separate exact state read/parse/selection, immutable selected
  source/package validation, generic binding validation and project-only
  data/locator validation into named helpers. Keep every existing condition,
  error and first-failure order. Retain the one-client iteration and original
  assignment order for Data, Skill, Binding and Installation, then compare
  source/native skill bytes and validate object ownership.
- `trustedReceiptObjects`: separate untransformed-source comparisons/digest,
  project executable/skill ownership and native MCP receipt construction from
  final ownership-inventory comparison. Keep source checks before installed
  digest, executable checks before skill/MCP checks, exact independent expected
  projection and strict inventory equality. Do not derive expectations from
  native output or combine acquired and installed framing.
- `trustedSequence`: keep its original entry point and replace contiguous blocks
  with named phase helpers: empty baselines; user/A/B add and receipts; installed
  A/B observations and B protection; unchanged A reconciliation/repair; changed
  A source/update; missing A repair/restart; A removal and B observation; repeated
  A removal; final asset/event capture. Pass only the exact existing old-package,
  retained snapshots and event bytes between phases. Preserve shallow snapshot
  semantics, unprotect/reprotect order, all public argv/labels and native stage
  order. Only the original final point may set Complete=true.
- `validateTrustedObservation`: split closed driver/proof/exit checks, closed
  observer/context/PID/skill checks, per-selected-server definition/UI checks,
  gateway-call/fresh-probe/facts checks and event correlation into named helpers
  in `trusted_observation_validation_linux.go`. Preserve strict object keys,
  normalization of authored expectations only, check/error order, cardinality,
  namespace/PID exclusions and single-session admission. Set Sessions[session]
  only after the same event-history/prefix and initialize/list/call validation.
  The De Morgan rewrite of the exit refusal must accept exactly the existing
  zero-exit/no-signal or nil-code/SIGTERM alternatives.
- `validateTrustedProbeEvents`: extract per-line decode/identity/time validation
  and the existing finite initialize/list/call transition/count block into named
  helpers in `trusted_probe_events_linux.go`. Keep bounds 3..32, exact event
  grammar, ordering, permitted repeated list, required counts and error strings.
- `trustedObserve`: extract driver request/result-path/frame preparation, the
  existing bounded result-poll callback and partial-observation capture. Keep
  the initial original-work-budget/asset admission, exact sealed request paths
  and GUI argv/env, 65-second context, 25-ms poll, start timestamp immediately
  before the same strictInput call, partial evidence on failure, final asset
  recheck, successful-stage append and observations append in the same order.
  No helper starts a fresh task budget or new process.
- Move `finish`/`publish` intact from bootstrap into `sealed_output_linux.go`,
  and the four closed observation-envelope/context/policy/identity validators
  intact from workbench into `observation_envelope_linux.go`. Retain names and
  callers. Adjust imports only; this clears the two file-size findings without
  changing cleanup, publication or untrusted observer behavior.
- Preserve `readJSONValue`, `readJSONObject` and `readJSONArray` and their 256-item
  semantics. Route the existing default-limit parse through those wrappers;
  let the 4096-item trusted-manifest path retain the parameterized functions.
  Preserve depth limits, duplicate-key rejection, trailing-input checks and
  token order. Do not delete wrappers, add implicit fallback or widen either
  input's supported limits.
- Explicitly check the three previously unchecked deferred descriptor closes at
  their current cleanup points, joining close errors with the original result
  without masking it; successful behavior and defer order remain unchanged.
  The receipt test checks snapshot removal in its existing deferred cleanup.
  Preallocate only the existing expected-argument slice capacity, and rename the
  local `copy` variable without changing its independent pointer copy.

Before implementation, obtain root and an independent second plan review of
this exact text and address findings. Preserve the reviewed pre-lint source as
its own commit. No source is retired: the moves retain every implementation and
legacy entry point, and helper extraction preserves the existing contracts.

After independent implementation review, run the entire affected controller
package's inert tests, including its legacy/untrusted, scanner, receipt, closed
JSON and deadline contracts, plus the existing Python frontend/toolchain and
wrapper contracts that bind these names/entry points. The privileged hosted
UID/mount test remains explicitly skipped locally and required on the genuine
runner. Run unfiltered golangci-lint 2.13.2 on this entire package and its whole-
file size/architecture checks, require zero issues, and inspect the diff for
unchanged command tables, literal settings, guard/order and cleanup behavior.
Use the already approved private temporary/cache capacity for validation;
execute no vendor binaries locally. Commit/push only after independent code GO
and these checks, then inspect fresh genuine hosted artifacts before claiming
any native completion or merge qualification.

### Bounded diagnostics for an exit-one scanner failure

#### Observed failure and limits

Genuine VS Code trusted run 40400045559, job 121843130833, at source
73f6b9c3e26709409d8ad9e1502683675683ebad/tree
355cd394b6caf21a17e230949a3040f00ad1963e passed its inert hosted contracts,
completed empty-a and empty-b, then failed public add-user before installation:
"security assessment failed before installation: lintai returned invalid JSON".
Native artifact 12687549067 retains that public error, but not the internal
scanner's stdout, stderr or exit. All five qualifications remain false and
native/display cleanup is not proved. Frontend cleanup is a separate success.

CommandScanner runs the actual admitted executable once with the fixed argv
scan-agent-plugin <packageRoot>, cwd packageRoot, its existing allowlisted
environment, default 30-second context and 8-MiB stdout/64-KiB stderr limits.
It currently accepts exit zero or one and drops stderr for both. The evaluator
then independently rejects empty/non-JSON output, unsupported report/policy
identity, runtime errors and other invalid assessment facts. The observed error
therefore establishes accepted exit zero/one and a refusal for empty, oversized
or non-JSON stdout, not the actual scanner exit or a measured cause. The later
inert regression below proves the intended stream cap was bypassable.

Static inspection of the already acquired exact GNU archive
2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da and
member b6723451f155eb49e84acb34d1f06de929bddfcff6238cd4e38cdc6519396fba
found a strong GLIBC_2.39 version need (Flags:none) and the associated undefined
pidfd_getpid/pidfd_spawnp symbols. The standard Ubuntu 22.04 libc baseline is
2.35. GNU glibc 2.35 loader source reports a missing strong version and exits
one before main. This is a strong compatibility inference; the actual loaded
libc version and loader diagnostic have not been captured. The upstream
v0.1.3 tag source emits a JSON report before a normal findings exit and prints
ordinary command errors to stderr with exit two; source is not proof that the
released bytes exactly reproduce its build.

#### Exact production change

Change only install/integrationctl/agentplugins/adapters/securityscan/scanner.go
plus a focused new inert CommandScanner test file and the recorded plan. The
shared production adapter owns this error; do not introduce a VS-only wrapper,
scanner substitute, extra command/preflight or second execution. Root will
coordinate integration through the existing program/client PRs.

Keep timeout and both overflow checks at their current precedence. Keep the
existing non-exit-one error branch, argv/cwd/environment, deadlines, limits and
return values unchanged. In the existing accepted-exit-one branch only, check
json.Valid on the already captured stdout. If it is false, return failure with
a fixed diagnostic identifying actual exit 1, the exact stdout byte count, and
strings.TrimSpace of the already bounded stderr, using the same stderr policy
as the existing non-one failure branch. Example diagnostic shape:
"lintai security scan failed (exit 1, invalid JSON, 0 stdout bytes): <stderr>".
Do not return captured stdout as a successful scan in that branch.

Retain valid-JSON exit-one output byte-for-byte for the existing evaluator and
its full report/identity/runtime-error/assessment validation. Do not equate
json.Valid with a valid security report. Exit-zero behavior, including its
existing later invalid-JSON rejection, stays unchanged. No successful outcome,
report, assessment or cache entry may be fabricated or rescued by diagnostics.
The change tightens only an already failing exit-one/non-JSON path and makes
its actual process failure visible before the evaluator's generic error.

Do not emit raw stdout, report bodies, scanned file contents, environment or
credentials; the byte count is sufficient to distinguish empty from malformed
output. Retain the existing 64-KiB stderr ceiling and fail on overflow before
formatting any diagnostic. Do not enlarge buffers or add output files, artifact
members, result fields, general callbacks, inherited credentials or a debug
switch. The existing bounded public add-user log will carry the error through
the unchanged capture/admission path. No new external communication occurs.

#### Inert verification and review gates

Use an expressly inert temporary shell/helper executable, never a vendor,
through the real CommandScanner.Scan path with the same fixed argv and cwd.
Keep these process tests on supported POSIX test platforms; preserve the
existing cross-platform evaluator/report tests without weakening their gates.
Verify:

- Exact valid reports with exit zero and with exit one return unchanged bytes;
  the real evaluator still accepts/classifies the valid exit-one findings.
- Exit one with empty stdout and a loader-shaped stderr sentinel returns a
  failure exposing actual exit one, zero stdout bytes and trimmed stderr.
- Exit one with non-JSON stdout refuses and reports its byte count without
  leaking a distinct raw-stdout sentinel into the error.
- Syntactically valid but unsupported exit-one JSON still reaches and fails the
  existing independent report/identity validation.
- Exit-zero invalid JSON and non-one process errors retain their existing
  behavior. Stderr overflow remains an output-limit failure, never a truncated
  diagnostic accepted as a scan or a disclosure beyond the existing bound.

Run the focused scanner/evaluator contracts and affected unfiltered lint after
independent code review, using the approved private temporary/cache capacity.
Do not run the vendor locally. Obtain two independent reviews of this exact
plan before implementation and independent review of the exact implementation
before pushing. Then use one genuine unchanged Ubuntu 22 trusted workflow to
collect the actual diagnostic; retain its source/tree/artifact identities and
all failures. If it still follows the unchanged exit-zero invalid-JSON route,
report that limitation rather than broadening the code silently.

A host/pin/selection correction requires a separate evidence-bound plan after
reviewing that result. Preserve the prior Ubuntu 24 sandbox failure, GNU/musl
cache identity separation, native trust/permissions, process containment and
all qualification/cleanup rules. No host, archive, scanner selection or
sandbox change is authorized by this diagnostic amendment.

### Enforce the scanner's intended stream bounds before publishing diagnostics

This amends the independently reviewed exit-one diagnostic plan (scratch
SHA256 89ef9097a66617f12338b99f43c7ce112cc13c8e344fcf262e6bf80a0cdec09c;
recorded plan c845eb97b0bd9b954014a8139aea19fa8a44e08b). The initial diagnostic
implementation 7eefe3c9204fb9d1452fe7de76445dbd9ce7c9b7/tree
520917a812f91350d73670cc8784c23268f58c8b is NOT approved for publication.

#### Actual inert regression and corrected historical inference

The real CommandScanner process tests executed locally against an expressly
inert POSIX script. Every prior scanner/evaluator test and every new valid-report,
exit-one diagnostic, unsupported-report and exit-zero/non-one case passed. The
new stderr-overflow case failed: a script writing maxScannerError+1 bytes and
exiting one reached the formatted invalid-JSON diagnostic instead of the
existing output-limit refusal. Full package result: FAIL, 0.039 seconds. No
vendor binary ran and no result is native qualification evidence.

The existing limitedBuffer embeds bytes.Buffer and therefore promotes its
ReadFrom method. os/exec copies a child pipe using io.Copy; that fast path can
invoke the promoted bytes.Buffer.ReadFrom, bypassing limitedBuffer.Write and
its overflow flag. The only production limitedBuffer instances are the
CommandScanner stdout and stderr buffers in scanner.go. Their consumers need
only Write, Bytes, Len and String. No other caller requires ReaderFrom or a
promoted bytes.Buffer API.

Correct the earlier bounded-output inference in the recorded diagnostic plan.
decodeReport uses the same generic invalid-JSON error for empty output, output
larger than 8 MiB, or syntactically invalid output. The effective intended cap
was bypassable, so the VS73/5942 artifact establishes accepted exit zero/one and
one of those three refusal conditions; the actual scanner stdout, stderr and
exit remain unretained. Do not claim the old 8-MiB/64-KiB caps actually held.
The independently observed GNU ELF strong GLIBC_2.39 requirement and Ubuntu22
baseline remain a separate loader-compatibility inference, not actual captured
loader stderr.

#### Exact bound repair

Change only the existing scanner.go buffer implementation and the focused
scanner_process_test.go, plus this recorded plan/correction. Replace the
anonymous embedded bytes.Buffer with one named private bytes.Buffer field.
Update the existing bounded Write to use that field without changing its
limit, truncation, overflow flag, return-count or error behavior. Add only the
three explicit Bytes, Len and String forwarding methods required by the current
scanner callers. Do not implement ReaderFrom, embed another interface, expose
an unbounded writer or add a copy helper, sink, process primitive or execution.
All child output must consequently traverse the existing bounded Write even
when os/exec/io.Copy chooses an optimized method path.

Retain the intended 8-MiB stdout and 64-KiB stderr limits, existing timeout then
overflow precedence and already reviewed three-line exit-one/non-JSON diagnostic.
Keep valid in-bound output byte-for-byte, full evaluator/report/identity checks,
exit-zero/non-one behavior, child argv/cwd/environment and cancellation unchanged.
The repaired bounds intentionally refuse output that exceeded the declared cap
but previously slipped through the promoted method. Do not weaken an assertion
or enlarge a limit to make the regression pass.

#### Verification and independent review

Extend the actual inert CommandScanner process tests to cover BOTH stdout and
stderr at their respective limit+1, with exit zero and exit one. Each must return
no successful body and the existing exact output-limit failure, before any
invalid-JSON diagnostic. Retain the valid zero/one report byte-parity and real
evaluator classification tests and all other reviewed negatives. Keep failure
messages concise: report lengths/classification rather than dumping a large
stream or stderr error. Tests execute only the expressly inert temporary helper.

Two independent reviews of this exact amendment precede implementation. Record
it in the consolidated plan, including the corrected historical inference,
before code. Review the exact final implementation independently, run the full
affected securityscan package and affected unfiltered lint in the coordinated
approved private cache/temp slot, and require zero issues/failures. Never run a
vendor locally. Only then may root publish the combined bounded diagnostic and
request one genuine unchanged-host Ubuntu22 lifecycle to capture actual scanner
failure facts through its existing admitted public log.

No host, GNU/musl asset, release selector, cache identity, report schema, artifact
member, raw stdout disclosure, ambient credentials, native permission, sandbox,
deadline or qualification changes are authorized by this amendment.

### Scanner diagnostic validation: inherited lint findings and actual Required gates

Validation of final code `56856d510e3342db2472fbd354cfbbe7cb3f58f0`
(tree `ae5ca58ee7d2575b0c4d5da170826db12834441c`) found five unfiltered
lint issues in unchanged parent files, so the earlier plan's expectation of
zero issues from that broader invocation is corrected by observed evidence.
Do not describe the entire unfiltered package as clean or change unrelated
provider/cache code to meet that mistaken expectation.

Exact GitHub file metadata at parent
`b16b5c5d28cdc734856ac414521d40d9c8d8dfd5` matches the local unchanged blobs:

| File below `install/integrationctl/agentplugins/adapters/securityscan/` | Parent/current blob | Inherited findings |
| --- | --- | --- |
| `cache.go` | `bb5f7213bd0d0a04a1cec4879ba463eafb64e1d6` | line 67: unchecked deferred `os.Remove` |
| `release.go` | `46ae62b4f2b537069b65f224321d3e6fe3f4265b` | lines 107, 128, 180: unchecked deferred body close, temporary remove, gzip close; line 99: gocritic prefers `http.NoBody` |

The pre-diagnostic scanner.go blob at local c75 is
`403fcc9a38aa8875c224ab8ed792d15ec51c6b56`, also byte-exact to that parent.
Thus the actual changed-lines run from c75 covers the complete scanner delta,
and the new process-test file is included in full.

Actual final verification, using golangci-lint 2.13.2 and the approved private
cache/temp capacity with one CPU, is:

- Full securityscan package tests: PASS, 0.281 seconds, including both actual
  process streams at limit+1 for exit zero and one, valid reports and evaluator
  classification, bounded diagnostics, and the prior refusal paths.
- All enabled linters on changed lines from c75: zero issues.
- The actual Required whole-file size/architecture selection
  `revive,funlen,gocyclo,gocognit,dupl,depguard`: zero issues.
- Broader unfiltered package lint: the five inherited findings above, with none
  in scanner.go or scanner_process_test.go. No suppression or exclusion changed.

This is a validation-only addendum matching the existing Required workflow's
changed-lines plus whole-file size/architecture gates. It changes no product
source, tests, assertions, linter settings or baseline. Root and independent CLI
review must approve this exact addendum before publication; the independently
reviewed product code remains frozen. No vendor executable ran locally, and
native scanner cause/qualification remains for genuine hosted observation.

## Gemini and Copilot runtime-attempt amendment

This amendment first admits bounded, explicitly unqualified runtime research for
the already pinned Gemini 0.65.0/Node 24.21.0 and Copilot 1.0.103 distributions.
Wait for their genuine initial help/scope artifacts and two independent reviews
of this subsection before implementation. Select exact
`TestAgentpluginsGeminiNativeProjectRuntimeAttempt` and
`TestAgentpluginsCopilotNativeProjectRuntimeAttempt`; their only research case is
`<client>.runtime-attempt`. They cannot satisfy any of the five qualification
IDs. Capture the actual provider request, installed tool name, native output and
session identity, initialize/list/call events and returned result. Freeze the
observed closed contract after inspecting genuine artifacts, with two independent
reviews before implementing the final lifecycle. Unknown flags, schema, routes,
tool names and auth responses fail the attempted scenario. A useful bounded
failure capture is not a successful runtime attempt or an unsupported boundary.
The initial attempt is deliberately one public A package add and one fresh A
session with one write-marker tool call, followed by cleanup. A/B/outside roots
exist, but independent user/B installation, trust negatives and the complete
lifecycle belong to the later tests; the attempt claims none of that coverage.

Copilot's candidate here is its supported headless BYOK provider path, replacing
the earlier ACP-only candidate for these new tests. No ACP adapter, SDK, inline
MCP definitions or generic RPC framework is introduced. Existing ACP research
and legacy tests stay intact. Current vendor documentation identifies candidate
flags; the exact pinned help and hosted attempt decide what is admitted.

### Ownership, fixture authority and limits

Own only focused `_test.go` leaves with the existing
`repotests/agentplugins_gemini_project_native_` and
`repotests/agentplugins_copilot_project_native_` prefixes: `provider`, `attempt`,
`contract` and, after the later gate, `lifecycle`. Reuse `nativeFixture`, its
existing harmless `inspect_runtime` probe, ownership/event assertions and the
existing `httptest` scripted-provider pattern. Root owns hosted admission,
immutable acquisition, workflow selection, bounded process/artifact integration
and support wording. No production adapter, activation promotion or alternate
installer is part of this slice.

Use the actual built public `agentplugins` executable and production detection,
security, source acquisition, immutable storage, installer and transactions.
No host/security stubs or fake client executables. The verified Gemini bundle
has the vendor Node shebang: create one recorded npm-style `bin/gemini` symlink
to its verified `gemini.js`, with the verified Node first on PATH, for legitimate
public detection. Check the exact link, target and runtime identities before
and after each command. The native session itself invokes exact Node plus the
exact bundle entrypoint. Copilot invokes its exact regular ELF. No shell shim
or executable that fabricates a version or native result is admitted.

Each attempt has 180 seconds total, 150 seconds work and 30 seconds cleanup.
Each later lifecycle has 900 seconds total, 840 seconds work and 60 seconds
cleanup, measured from one original monotonic start. Public UAP operations have
30 seconds, help/list 30 seconds and each native session 60 seconds. Provider
requests have five-second deadlines, one-second read-header limits, one MiB
body/response limits and at most eight requests per session. Bound each native
stdout/stderr stream to one MiB; reject truncation/overflow. Reuse existing
ordinary `process.OS` descendant supervision and graceful-exit facilities rather
than the older unbounded installer capture or a new process primitive. Stop
starting lifecycle sessions with less than 75 seconds work remaining. Close
provider listeners and prove all owned native/MCP descendants have exited before
success. Forced termination, surviving children or incomplete cleanup fails.

Every fixture uses one fresh original HOME, real Git projects A/B and an outside
cwd; project paths never become HOME. Use independent public local packages for
the original-user skill marker and A/B mixed skill/MCP markers. Distinct keys are
`uapgeminia`/`uapgeminib` or `uapcopilota`/`uapcopilotb`; skill names/bodies are
also distinct. Original-user package is skill-only. Each installed stdio entry
selects the supplied verified probe, a separate event file, fresh nonce,
revision, argv and cwd using the existing probe environment contract. Public
installer output, recorded native ownership and selected installed bytes are
authority. Fixture-authored replacement entries and direct probe calls cannot
prove installed discovery. Foreign disabled native MCP, skill and preference
sentinels are ordinary controlled files authored before preservation baselines.
Validate ordinary parents, no aliases, regular/single-link leaves and bounded
held reads before every owned read/rewrite; preserve outside sentinels.

Author documented exact trust and provider/approval settings before baseline;
UAP must leave them intact. Gemini enables folder trust in original-home settings
and records exact canonical A/B `TRUST_FOLDER` decisions in `trustedFolders.json`.
Its original-home policy file contains priority-100 deny-all and BOTH
priority-200 allow rules, each selecting only the exact A or B `mcpName` plus
`toolName='inspect_runtime'`. Do not rewrite policy between A/B sessions. Copilot
uses exact canonical A/B `trustedFolders` in original-home `config.json` through
the already reviewed safe held-file writer; its per-session argv narrows tool
availability/approval without changing that file. No skip-trust, yolo,
`mcpServers.trust`, `COPILOT_ALLOW_ALL`, blanket tool approval or forged database.

The later never-trusted C subcase uses its own independent fresh original HOME,
public installation of a distinct C mixed package and C-only named tool
permission. Require native project-layer suppression and no C probe start;
an empty directory alone is not a trust test. It never changes the positive
A/B preservation baseline. Native catalogs prove skill discovery and selected
body/path; no assumed provider prompt format or LLM skill activation is claimed.

### Gemini native and provider grammar

Fixed native argv is exact Node, exact `gemini.js`, `--prompt <fixed-stage-prompt>
--approval-mode default --output-format stream-json --model gemini-2.5-flash`.
No resume, fake-response file, injected MCP definition or extra extension.
Original-home settings set `privacy.usageStatisticsEnabled=false`,
`telemetry.enabled=false` and
`experimental.adk.agentSessionNoninteractiveEnabled=false`, selecting the pinned
legacy headless loop. Set `GOOGLE_GEMINI_BASE_URL` to the fresh literal loopback
origin and `GOOGLE_GENAI_API_VERSION=v1beta`. Supply a fresh fixture-owned
`GEMINI_API_KEY` required only by this loopback provider; compare its exact
`x-goog-api-key` header privately and redact the key from all admitted artifacts.
The pinned GATEWAY mode supports this custom endpoint. Its config code otherwise
evaluates `loadApiKey()` before reaching the GATEWAY branch, so an explicit
fixture key avoids accidental keychain probing. This is test-provider
authentication, never vendor Google authentication/OAuth or an inherited account.

Admit only POST
`/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse` and optional POST
`/v1beta/models/gemini-2.5-flash:countTokens` without query. Require literal
loopback Host and the fixture key. JSON is whole, duplicate-free, finite and
bounded. Top-level stream-request keys are `contents` plus optional
`generationConfig`, `systemInstruction`, `safetySettings`, `tools`, `toolConfig`
and `cachedContent`, recursively validated to the pinned SDK 1.30.0 converter
types. Contents contain typed role/parts; only text, the selected functionCall
and matching functionResponse parts are needed. Reject other part kinds/routes,
methods, query parameters and unknown fields. Token counting admits its pinned
contents body and returns `{"totalTokens":1}`; it supplies no runtime proof.

For a positive session, require exactly the selected installed function
declaration `mcp_<exact-server-key>_inspect_runtime` with the existing probe
argument schema; the other project's tool must be absent. Emit one SSE
`data: <JSON>\n\n` candidate with `index:0`, `content.role:'model'`, one
`functionCall:{id,name,args}` part and `finishReason:'STOP'`. Name is the exact
declaration; args contain only the existing `nonce`, `operation:'write'|'read'`
and `marker` stage challenge. Start id as
`<exact-tool-name>__uap_<nonce>_<stage>`: pinned `turn.ts` preserves an already
qualified ID and otherwise prefixes it. Responses carry bounded numeric
`usageMetadata`. Require the next native request's matching
`functionResponse:{id,name,response:{output:<text>}}`. Pinned MCP text handling
wraps output as `<untrusted_context>\n<probe-JSON>\n</untrusted_context>`;
require exactly that one wrapper, remove it once and parse the inner probe facts
JSON. Do not expect the original MCP content wrapper or silently strip arbitrary
markup. The controlled fixture facts contain no closing-tag text needing escape.
Compare nonce,
revision, PID/session, argv, package/data roots, cwd and marker to the newly
observed probe events. Only then return a text-only candidate containing exactly
`UAP_DONE_<nonce>_<stage>` and STOP. No response echo alone passes.

Pinned stdout is the finite `core/src/output/types.ts` JSONL union, with ISO
timestamps: `init` has native `session_id`/`model`; `message` has role/content
and optional delta; `tool_use` has tool_name/tool_id/parameters; `tool_result`
has tool_id/status and optional output/error; `result` has status and optional
error/stats. Require one init, a fresh nonempty native session ID, exact tool
name/arguments and preserved call ID, successful matching tool_result, exact
final assistant text and one terminal success. Reject error events,
duplicate/out-of-order IDs, unknown fields, extra tool calls and unsuccessful
terminal outcomes. Result stats, when present, follow the pinned finite numeric
fields and bounded per-model map; they are bookkeeping. Match text-only
tool_result.output facts independently to provider output and probe events.

Pinned source closure is Gemini v0.65.0
`packages/cli/src/nonInteractiveCli.ts`, `config/config.ts`,
`config/settingsSchema.ts`, and core `src/core/contentGenerator.ts`,
`core/turn.ts`, `tools/mcp-tool.ts`, `output/types.ts`,
`output/stream-json-formatter.ts`, `utils/generateContentResponseUtilities.ts`
and `utils/textUtils.ts`.
Its core package pins `@google/genai` 1.30.0; that exact SDK's `src/models.ts`
and `src/converters/_models_converters.ts` define the REST paths/conversions.
Freeze the resolved source commits/blob identities with the genuine attempt.
See the [policy reference](https://geminicli.com/docs/reference/policy-engine/).

### Copilot native and provider grammar

Candidate fixed argv is exact ELF `-p <fixed-stage-prompt> --output-format json
--no-auto-update --disable-client-telemetry --disable-builtin-mcps
--available-tools=<exact-observed-native-MCP-wire-name>
--allow-tool=<exact-server-key>(inspect_runtime) --deny-tool=shell
--deny-tool=write --deny-tool=read --deny-tool=url --deny-tool=memory`.
These are literal argv elements, not shell expressions. Freeze each flag and
wire name from pinned help/the hosted attempt; do not guess name sanitization or
loosen to a wildcard/prefix. All children receive `COPILOT_OFFLINE=true` and
`COPILOT_AUTO_UPDATE=false`. The session also receives
`COPILOT_PROVIDER_TYPE=openai`,
`COPILOT_PROVIDER_BASE_URL=http://127.0.0.1:<ephemeral>/v1`,
`COPILOT_PROVIDER_WIRE_API=completions` and `COPILOT_MODEL=uap-fixture-model`.
Provider API key/bearer and vendor credentials are absent. Supported keyless
local-provider setup is not GitHub authentication or a claim of an OS firewall.

Admit only POST `/v1/chat/completions`, literal loopback Host, exact model and
`stream:true`. Requests use bounded typed messages and function tools.
The closed Chat Completions candidate accepts top-level model/messages/stream,
tools/tool_choice, stream_options, max_tokens/max_completion_tokens,
temperature/top_p, parallel_tool_calls, stop, presence_penalty/frequency_penalty,
seed, user, response_format and reasoning_effort, each with its documented type
and finite bounds. Reject other fields/routes rather than returning a generic
successful response. Message roles are system/developer/user/assistant/tool;
content is bounded text, null for a tool-call assistant, or typed text parts.
Function tool definitions have exact name/description/parameters and optional
boolean strict. Assistant tool_calls have id/type:function/function name
and JSON arguments; tool messages have exact tool_call_id and bounded content.
The attempt records the actual subset; freeze its exact request/response union
before qualification. No unrelated built-in tool, remote endpoint or provider
fallback is admitted.

Require the exact selected installed MCP declaration before emitting one
`chat.completion.chunk` SSE tool call: bounded id/object/model, choices index 0,
delta role assistant and tool_calls with index 0, unique id, type function,
function name and arguments containing strict JSON for the existing probe args.
Terminate that completion with `finish_reason:'tool_calls'` and `[DONE]`.
The next native request must have role tool, exact tool_call_id and returned
probe facts matching the fresh initialize/list/call event session and all facts
listed above. Only then emit the exact final sentinel as content, finish_reason
stop and `[DONE]`. Wrong-project/removed definitions must be absent. No direct
probe call or inline MCP server flag is permitted.

Exact Copilot 1.0.103 native JSONL event/session grammar is an explicit
runtime-attempt observation prerequisite, not an invented assistant.message
parser. Bound/privately capture the attempt; unknown event shape fails its
admission even when the capture helps amend the parser. Freeze exact keys,
ordering, terminal success/error and native session/tool identity from actual
evidence before qualification. Do not fabricate client session IDs from test
labels or treat provider/probe identity alone as native session identity.
Primary candidates are official
[BYOK](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/use-byok-models),
[programmatic](https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-programmatic-reference)
and [tool-control](https://docs.github.com/en/copilot/how-tos/copilot-cli/use-copilot-cli/allowing-tools)
documentation plus the pinned binary's actual help/observation artifacts.

### Later public lifecycle and qualification gate

After observing/freezing those contracts and the later two independent reviews,
select exact `TestAgentpluginsGeminiNativeProjectComponentsLifecycle` and
`TestAgentpluginsCopilotNativeProjectComponentsLifecycle`. These tests use the
supported imperative local-directory route from
[project installation](../../PROJECT_INSTALLATION.md), matching existing real
OpenCode/Cline fixtures: public `add <canonical-source> --no-save --scope project
--project-root <A-or-B> --target <client> --format json`; original-user add uses
explicit user scope. Local sources take no saved alias and create no portable
dependency agreement. There is no `deps sync` claim. Later selection is by
observed package name/installation ID or the canonical source for update.

Required sequence is empty baseline; independent user/A/B install; exact native
skill/config discovery and A/B tool write/read; identical repeat add and unchanged
update with stable ownership/native bytes; intact repair with truthful restaging;
A source advance and public update; exact native v2 skill/tool/result with the
retained marker; remove precisely one receipt-owned delivered skill directory
while the selected local source stays unchanged at v2; public repair restores
v2 and retained data; explicit fresh-process restart; exact A removal; fresh A
absence/no A probe events with still-functional B/user; exact recognized repeated
removal and byte-preserving no effects. No native children remain across UAP
mutations. Preserve original-user/B/foreign native content, receipts and plugin
data; compare exact selected source/tree/package/binding/data locator throughout.
Intact repair may truthfully mutate by restaging; do not require a false no-op.
Local missing repair proves reconstruction from unchanged selected local input.

Keep `TestProjectComposedSourceLifecycleSeparatesUserAndProjects` and
`TestProjectCopilotComposedSavedLifecycle` required as separate existing TLS
Git/ZIP/saved-dependency evidence. Their moving-source/recorded-pin proof must
not be attributed to the new native local-directory test. Preserve legacy native
fixture/protocol tests and Claude/Codex/OpenCode gates. Do not change production
source or repair behavior to fit a stronger fixture claim.

Final successful cases are `<client>.project-lifecycle`,
`<client>.native-restart-removal`, `<client>.mcp-initialize`,
`<client>.mcp-tools-list` and `<client>.mcp-tools-call`, only after their complete
native sequence and cleanup. Evidence joins actual native process/session/tool
identity, provider call ID and probe PID/session/nonce, returned facts and exact
installed authority. A connected label, catalog, raw stdout or provider echo is
insufficient. Add inert contracts for duplicate/oversized JSON, unknown routes,
foreign/missing declarations, wrong call ID/nonce/revision/data/cwd/session,
out-of-order streams, bad terminal results, timeout and surviving descendants.
Required blocked/skipped/not-reached stages fail, never substitute a config pass.
State stays Prepared. Label `provider_kind=scripted_loopback`, vendor
authentication false and real-model use false. Raw bounded evidence stays in
closed CI artifacts, not Git or recursively uploaded native profiles.

## Current-source runtime CI handoff

The October 26 continuation uses a separate `project-native-runtime.yml` job
and the small `project_native_runtime.py` frontend. Two independent plan rounds
and independent code reviews by the Gemini/Copilot and Kimi/Cline implementers
admitted the closed first Copilot attempt. No existing baseline gate is changed.

Only a fixed implemented test name, opt-in, source filename and deadline can be
selected. The workflow verifies actual GitHub-hosted Linux x64 and exact clean
source before setup-go; the frontend repeats admission before any compiler or
vendor child. It reuses the reviewed archive acquisition/materialization,
immutable file checks, capture budgets and closed ZIP writer. Build the public
installer, existing probe and repository test binary with exact Go 1.26.10,
checking held compiler identity before and after each build. Provision the same
pinned lintai release through bounded verified archive acquisition. No inherited
profile, authentication, proxy or loader environment enters the test.

Acquisition and compilation have one 480-second budget. The initial Copilot
attempt has its own original 180-second total, with five seconds reserved by
the outer process capture for reaping, followed by a bounded publication phase.
Future full lifecycle registrations require their separately reviewed exact
tests and original deadlines; this is not an arbitrary timeout override.

The frontend records source/build identity separately in `frontend.zip`. A
native `runtime.zip` is normalized only from bounded held bytes after checking
the fixed client's exact member inventory, command triples, source identity,
UTF-8, duplicate/special/oversized entries and explicit qualification state.
Copilot's initial schema admits its named installed snapshots, bounded provider
requests, probe events and fixed command table. Unknown native JSONL semantics
remain an intentional failed research attempt until a genuine artifact has been
reviewed. A closed artifact or successful build cannot qualify a failed,
skipped, absent or otherwise unobserved native test. Raw profiles are never
recursively uploaded, and the workflow publishes only these two named ZIPs.

The reviewed Gemini registration uses the exact
`TestAgentpluginsGeminiNativeProjectRuntimeAttempt` with the same 180-second
attempt bound. A successful attempt additionally requires all command triples,
both installed snapshots, provider requests, probe events, actual native
session/call identifiers and returned probe facts. Its five lifecycle
qualification flags remain false. Pinned Gemini source adds only the exact
`wait_for_previous` boolean property to the provider-visible tool schema;
validate that property exactly before comparing the original probe schema.
Never add it to the emitted tool arguments. The fixture API key is registered,
redacted and checked again at closed artifact admission.

The Kimi registration selects only
`TestAgentpluginsKimiNativeProjectComponentsLifecycle` and its original
900-second total/840-second work budget. It observes public local-source
installation, reconciliation, update, missing-component repair, fresh native
processes and removal through Kimi's authenticated local API. Named native MCP
tests prove initialize/list; tools-call stays false. As with the reviewed local
Gemini/Copilot sequence, repair uses unchanged selected local input. Moving-source
and recorded-pin behavior remains separately required in
`TestProjectComposedSourceLifecycleSeparatesUserAndProjects` and
`TestProjectCopilotComposedSavedLifecycle`; it is not attributed to these native
local-directory observations. The checked source-root override and canonical
Python executable are explicit inputs, with no ambient auth or loader variables.

## Qoder, Grok, Kiro and Devin installed-startup attempts

Two independent plan rounds and implementation reviews admit the finite shared
`agentplugins_project_native_cli_attempt_test.go` helper and four thin client
wrappers. Select only `TestAgentpluginsQoderNativeProjectRuntimeAttempt`,
`TestAgentpluginsGrokNativeProjectRuntimeAttempt`,
`TestAgentpluginsKiroNativeProjectRuntimeAttempt` and
`TestAgentpluginsDevinNativeProjectRuntimeAttempt`. The last uses the public
`windsurf` target, `AGENTPLUGINS_WINDSURF_PROJECT_RUNTIME_ATTEMPT`, and the
`agentplugins_windsurf_project_native_attempt_test.go` leaf. Each attempt keeps
one original 180-second total, 150-second work and 30-second cleanup budget.

Use exact already pinned archives, full regular runtime closure, actual hosted
Linux admission and the real public local-directory project add. Qoder and Grok
install skill-only packages; Kiro and Devin also install the harmless stdio
probe through the production managed launcher. Independently validate selected
source, package, Prepared binding, native ownership paths/digests, complete
Qoder import-view inventory, and exact native command/arguments/environment
against recorded package and data authority. No direct native configuration
replacement or user-profile relocation is permitted.

Qoder's measured help explicitly defines `--tools ''` as disabling all tools.
Grok's pinned public command-source snapshot instead parses an empty `--tools`
as no selector, so that option is not used. Its source-guided attempt uses
`--deny ''`, traced through the strict permission parser to Deny/Any/no pattern;
source similarity is not a claim of shipped-binary equivalence. Devin omits
automatic permission mode and uses an original-profile deny-only configuration
for the documented builtins and MCP prefix, with no allow entries or trust
grant. Kiro uses no-interactive mode with no trust or automatic approval flag.
Unknown actual options, permission behavior, authentication or tool protocol
fail without granting access or trying an alternate permissive mode.

The exact capture table is `git-a`, `git-b`, `version`, `add-a`,
`native-attempt`. The closed frontend admits only their complete command
triples, the fixed summary and optional probe events, bound to the exact source
commit/tree. Every initial native session, including exit zero, remains a
failed research attempt until its genuine raw output is independently reviewed.
All five qualification flags, vendor authentication and real-model qualification
remain false. A bounded retained failure cannot become a passing E2E gate.
Further source/provider/permission/lifecycle changes require their own observed
contract and independent review; legacy user and baseline gates remain required.

| Client | Pinned native version | Fixed restricted startup |
| --- | --- | --- |
| Qoder | 1.1.78 | `--print --output-format stream-json --no-session-persistence --permission-mode default --tools "" --plugin-dir <receipt-selected-view> <prompt>` |
| Grok | 1.0.70 (f50904eaee6c) | `--single <prompt> --output-format streaming-json --permission-mode default --deny "" --no-subagents --disable-web-search --max-turns 1` |
| Kiro | 2.34.0 | `chat --v2 --no-interactive <prompt>` |
| Windsurf / Devin | 3000.11.205 (6dea39440ae3) | `--print <prompt> --respect-workspace-trust true` |

Grok's source-guided deny semantics are from public snapshot
`xai-org/grok-build@2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8`,
`crates/codegen/xai-grok-pager/src/headless/cli.rs` blob
`99d5732f95ec023debeffc2f00cb4ae4532f0f0b`, together with the same snapshot's
`xai-grok-permission-rules/src/rules.rs` and `policy.rs`. The shipping commit
differs; no binary equivalence is inferred. Devin's original-profile config is
`auto_update:false`, `allow:[]`, `ask:[]`, and documented deny entries
`read,edit,write,grep,glob,exec,fetch,web_fetch,mcp__*`.

Actual Kiro research observed `error: failed to launch kiro-cli-chat` after its
version/help preflight. Its leaf now admits the three exact acquired launcher,
chat and terminal companions, checking ordinary ownership/mode and the pinned
full runtime closure before every child. Only that verified bin directory is
prepended to `/usr/bin:/bin`; there is no shared environment escape, alias,
native-profile move or acceptance of the observed failure as success.

## Cline first installed ACP capture

The pinned Cline 3.0.73 source at
`c718573538d76310f464541617ab665108a6c999` cannot bind the first noninteractive
desktop approval to the requested session: it discards that supplied session ID
and registers `activeCliSession` only after the first turn returns. Do not invent
a desktop session or bypass the approval wall. The separately reviewed next
candidate is the native ACP transport with `--auto-approve false`, one actual
`session/new` response, an empty injected `mcpServers` list, and one exact native
permission request. `allow_once` is permitted only when actual session ID,
provider call ID, tool name `uapclinea__inspect_runtime`, nonce and arguments all
match. All other permission requests are denied and the attempt fails.

Select only `TestAgentpluginsClineNativeProjectRuntimeAttempt` with
`AGENTPLUGINS_CLINE_PROJECT_RUNTIME_ATTEMPT=1`. Use public local-source A add,
the independently derived existing Cline nested `transport` projection with
direct receipt-selected probe command, arguments, environment and native cwd,
and the unchanged complete installed package. No production adapter change or
managed-launcher substitution is introduced. The supported original-user custom
provider/catalog selects one scripted loopback OpenAI-compatible endpoint with
external catalog refresh disabled. Its fixture key is never published.

The ordinary hosted frontend hashes the actual root-owned sudo, systemd-run,
systemctl and env executables. It creates only one fresh
`uap-cline-project-<32-hex>.service` with the original UID/GID, Delegate=yes,
NoNewPrivileges=yes and KillMode=control-group through systemd 255. Pass only the
closed existing environment and exact test binary, with literal
`--expand-environment=no` preserving arguments before `env -i`. No AI client runs as root,
no cgroup hierarchy is relabelled, and no delegation-root override is accepted.
The Go test verifies actual `/proc/self/cgroup`, cgroup2 ownership and NNP before
the existing OS duplex capability admission. The original CLOCK_MONOTONIC
nanosecond origin starts before service launch and is not reset: 150 seconds
work, 180 total. The outer capture returns by 165, reserving its existing five
seconds reap and ten for exact-unit cleanup. RuntimeMaxSec is a secondary limit.
If needed, stop and then kill only this owned unit; any such intervention fails
the attempt, and the final exact cgroup must be empty or gone.

Closed native evidence contains the fixed public command triples, bounded ACP
JSONL and its command/error receipt, at most two actual provider exchanges,
optional native probe events, and the exact source-bound attempt summary.
Successful planned duplex exposes no diagnostic stream in the existing OS API;
errors retain that API's bounded diagnostic. This limitation is explicit and
does not motivate a second process primitive. Every initial capture, including
an apparently successful exchange, fails pending independent review of actual
native/provider/probe bytes. All five qualifications, vendor authentication and
real-model flags remain false until a later reviewed full lifecycle proves them.

## Qoder receipt-selected catalog observation

Genuine pinned Qoder 1.1.78 help in
[run 40342548512](https://github.com/777genius/universal-agent-plugins/actions/runs/40342548512),
artifact `12664571770` (SHA-256
`7f00bd2c8f8cf54409fa464658b394a44bc10020a1e92874f4e333c45aedbc11`),
advertises `plugins list --json --plugin-dir <dirs...>`. The independently
reviewed amendment adds exactly that read-only command with the actual
receipt-selected closed import view, cwd A and a 30-second ceiling inside the
existing 150-second work / 180-second total deadline. It runs after public add
and before the existing restricted headless attempt. There is no global import,
registration, profile relocation or new provider permission.

Freeze the complete original `HOME/.qoder` subtree immediately before and after
this catalog command: entry membership, directories, full modes, original UID,
single-link ordinary files and held byte digests; bound it to 4,096 entries,
32 levels and 256 MiB. Any change, including a new cache file or empty directory,
fails. Require exit zero, empty stderr and strict whole JSON object/array, while
leaving its unobserved catalog schema opaque. The Qoder-only `native-catalog`
command triple is the sole additional frontend member set. Other clients retain
the original five labels. Raw capture remains an intentional failed initial
attempt with all five qualifications false until independently interpreted.

## Kimi separate installed single-tool attempt

Preserve the existing 900-second Kimi component lifecycle. Register the separate
fixed selector `kimi-call`, exact
`TestAgentpluginsKimiNativeProjectRuntimeAttempt` and
`AGENTPLUGINS_KIMI_PROJECT_RUNTIME_ATTEMPT=1` only when its own test and
`kimi_attempt.py` observer exist. Its actual client and acquisition identity
remain `kimi` 2.3.2; the phase is `installed-single-tool-attempt`, with an original
180-second total / 150-second work / 30-second cleanup budget. One native service
phase has at most 60 seconds including the existing five-second reap reserve.
Neither phase can reset or borrow the other test's deadline.

Two independent plan rounds reviewed nineteen exact source blobs at
[`b8dd3a134060c6e80ba4a1659335cd63b68483f2`](https://github.com/MoonshotAI/kimi-code/tree/b8dd3a134060c6e80ba4a1659335cd63b68483f2),
including the [native server API](https://github.com/MoonshotAI/kimi-code/blob/b8dd3a134060c6e80ba4a1659335cd63b68483f2/docs/en/reference/server-api.md),
tool naming/policy, session/prompt/approval routes, and the OpenAI provider
request/stream implementation. Author only the supported original-user
configuration: one literal-loopback OpenAI-compatible test provider with a
private fixture credential, manual permission mode, telemetry and session-title
generation disabled, at most two steps and one attempt per step, and the sole
enabled tool `mcp__uap-kimi-native-a__inspect_runtime`. Builtins and B tools are
not enabled. Public project add must complete before the first native service.

Reuse the existing Python service and token quarantine, existing Go descendant
supervision and harmless probe. Create the actual native session in A through
REST, submit the exact configured model/manual prompt and poll its real approval
and snapshot endpoints. Approve only the matching actual session, main agent,
positive turn, provider call ID, qualified tool, action and nonce arguments.
The decision omits scope, granting one call only. Require no probe tool call or
marker before that approval. Unknown or repeated approvals fail.

The scripted provider permits exactly two successful exchanges: one selected
tool call, then a final sentinel only after the real tool result contains the
expected fresh probe facts. Require native session/turn/tool-result identity,
provider history, initialize/list/call PID/session/nonce correlation, marker and
complete cleanup. Retain at most eight actual provider requests for failure
diagnosis. The closed artifact admits only its fixed summary, installed A
snapshots, optional probe events, redacted `python-api.json`, contiguous bounded
provider requests and six complete command triples: `git-a`, `git-b`, `git-c`,
`version`, `add-a`, `native-attempt`. Private provider and native bearer values
are registered before capture and redacted before artifact admission.

Every first capture deliberately fails with `runtime_attempt_complete=false`,
`test_failed=true` and all five qualification flags false, even if it appears to
complete. Actual raw approval/session/provider/probe grammar must be reviewed
before a later separately reviewed integration into the full lifecycle. This
attempt cannot inherit component-lifecycle success or claim vendor OAuth or
real-model qualification.

## Observed Darwin completed-process cleanup correction

Genuine baseline [run 40367376473](https://github.com/777genius/universal-agent-plugins/actions/runs/40367376473),
job [121728625314](https://github.com/777genius/universal-agent-plugins/actions/runs/40367376473/job/121728625314),
at Gemini source `6f8ea1173b8b1d343688299b5864ee7c1e1a86d7` failed its
inert capture contracts before vendor execution. The reported host is macOS
15.7.9 / 24G830, image `macos-15-arm64` `20260907.0337.1`.
The finite stdout child wrote 129 bytes against a 128-byte bound and exited.
Final cleanup's original `killpg(pid, SIGKILL)` raised `PermissionError:
[Errno 1] Operation not permitted` before the child wait and normal overflow
receipt. The same head's Codex macOS job `121728625811` passed all 75 contracts.
Neither that pass nor the later missing-artifact error qualifies the failed job.

Apple's published macOS 15-era XNU tag `xnu-11417.140.69`, commit
`43a90889846e00bfb5cf1d255cdc0a701a1e05a4`, explains a narrow possible race:
`bsd/kern/kern_sig.c` blob `d32c03ce89f56531a419436fd0ef0e944e0d8331`
filters zombie group members and returns POSIX EPERM when an existing group has
no eligible live member. An unreaped, already exited sole leader can therefore
produce this result. The actual log has no process-state telemetry; this is a
source-backed timing inference, not an observed zombie state or a claim that
this public source exactly matches the runner's kernel binary.

Change only `scripts/native_client_research.py:kill_group` and its inert
contracts. Preserve the first group-wide SIGKILL and existing missing-group
handling. Capture the original numeric group ID before that call. If and only
if the first call raises PermissionError, poll the original Popen child once.
A still-live child preserves the original permission failure. After its exit
is observed, issue one read-only signal-zero check against the identical
original group ID. Accept only ProcessLookupError from that check, proving
that the group is now absent. A successful check, repeated permission denial
or another ordinary OS error preserves the original permission failure.
Unexpected exceptions and interruption remain failures.

The new refusal branch must not retry a destructive signal after reaping,
signal a different group or individual child, escalate privileges, sleep,
retry without a bound or suppress arbitrary EPERM. An extant, reused or
unrepresentable group cannot become successful cleanup. In the same XNU
snapshot, `bsd/kern/kern_fork.c` blob
`9a6e5335a40d3a5acd390f5069ab2fa3b73cd784` reserves IDs against process,
group and session identities; `bsd/kern/kern_proc.c` blob
`060913ab13ed90608124e6b2c773911c2dc7ca1b` removes an empty group when
destroyed. Reaping a sole leader may end that reservation. The signal-zero
check avoids adding a destructive retry against a potentially reused ID.

Keep the shared helper's existing callers, first cleanup attempt, five-second
child wait, descendant handling, pipe closure, output caps, original deadlines
and error receipts. Do not alter collector argv/environment, native admission,
acquisition, trust, permissions, token quarantine, artifacts or qualification.
The common service retains its independent normal-exit, process-group,
closed-port and forced-cleanup refusal rules.

Deterministic inert tests must prove: a live child refuses without an absence
probe; an exited child checks the same group once; only a now-absent group is
accepted; a present group, repeated EPERM and unrelated OS error preserve the
original error; and this branch sends no second destructive signal. Retain the
real-process exact EOF/overflow, separate-stream, inherited-pipe descendant,
capture-interruption and non-POSIX import controls. Local Linux contracts do
not qualify the observed macOS race; the original hosted macOS contract remains
required. Two independent plan rounds precede implementation, followed by
independent code review and exact-source validation.

## Completion gate

For each client, retain the exact source commit/tree, archive and executable
identities, platform, selected tests/scenarios, real native observations and
explicit remaining vendor boundaries. Independently review the implementation,
resolve findings and inspect raw evidence rather than trusting workflow color.
All selected required tests must run and pass; runtime claims require their
correlated runtime proof. Keep prior Claude/Codex/OpenCode and user-scope gates
green. Merge client PRs into the program branch after their applicable checks,
then review and validate the combined source before merging the base PR.

Update current-source support guidance only to the actually observed level.
Do not publish a release or claim real-model/OAuth qualification from a fixture.
