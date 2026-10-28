# Remaining native project qualification

## Owner reprioritization, 2026-10-27

The owner explicitly deferred VS Code: it is not a priority and is skipped in
this continuation. Complete the nine remaining client leaves in PR521-529 and
their applicable end-to-end verification, independent review and merge gates.
Keep PR530 as a deferred draft; preserve its existing source, recorded evidence
and unpublished work. Do not run new VS Code qualification, change its dedicated
implementation or merge that leaf as part of this continuation. Its incomplete
qualification does not block the active nine-client program. Preserve existing
baseline regression coverage for previously supported behavior.

This latest scope decision supersedes earlier instructions below that require
VS Code follow-through or count it among the active remaining targets. It does
not change the evidence requirements for any active client, promote a blocked
native case to passed, or authorize a release.

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

## Grok opaque catalog research amendment

Genuine help run 40240232499/artifact 12616383397 confirms the frozen 1.0.70
archive/executable and `inspect --json`, `plugin list --json`, and `mcp list
--json` help. After two independent plan rounds, admit explicit `observe`
research only. Keep the nine fixed version/help probes unchanged. Require six
empty baseline catalog operations in A/outside, then author distinct text-only
user/A/B `.grok/plugins/<marker>/.grok-plugin/plugin.json` and
`skills/<marker>/SKILL.md`, and require nine populated catalog operations in
A/B/outside. No executable payload, hooks, commands, MCP definition, native
trust, provider, SDK or account mutation is introduced. Outside remains a
separate cwd, never HOME.

Each operation uses existing hosted admission, immutable runtime checks and
`Context.probe`, with max 30 seconds capped by the original shared 360-second
remaining deadline. Every required operation must return exact integer zero,
error None, empty stderr and strict UTF-8 whole duplicate-free finite JSON
object/list, bounded to one MiB, 32 levels and 8,192 nodes. Preserve raw streams.
The schema remains opaque; root kind, digest and marker-token occurrences are
research observations, never selected source/loading evidence. Owned ordinary
single-link UID files retain exact mode, bytes and original identity across each
probe, using descriptor-relative no-follow parent/file opens and held reads.

Only completed fixed observations may pass `grok.scope-observation` research;
all product lifecycle, native restart/removal, MCP initialize/list/call,
authentication and model qualification flags remain false. Failures, malformed
JSON, alias/replacement/mutation, timeout or missing operations fail the research
slice. Freeze genuine observed source schemas and native modes with a new
separate reviewed amendment before real UAP lifecycle implementation or a
native-source qualification claim. Existing skills-only project MCP refusal
remains intact.

Every native child, including version/help and all catalogs, receives literal
`GROK_DISABLE_AUTOUPDATER=1` through the existing environment allowlist. This is
the documented suppression control in the official
[settings reference](https://docs.x.ai/build/settings/reference); it does not
claim network isolation or supply vendor authentication. Immutable runtime
identities remain checked before and after every child. Inert contracts assert
the exact supplied environment on every help/catalog call.
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

## Observed Grok 1.0.70 project inspection admission

Genuine runtime [40353442588](https://github.com/777genius/universal-agent-plugins/actions/runs/40353442588)
stopped before public `add-a` mutation because the existing project inspector
accepted only the previously measured 1.0.67 version. The follow-up genuine
read-only [observe run 40353442581](https://github.com/777genius/universal-agent-plugins/actions/runs/40353442581)
succeeded at source `3234be86f0a3217ebd2371dc5e8fa0b26d3dcd26`, tree
`74490cebf339e8538b1f237b478aceeb58667e3d`. Native artifact `12670385326`
(SHA256 `a0d9c2e0858b5bbb2297f4d6f6f99aa71a57cfbc3d9ba5ec908434dfe14db957`)
retains the actual bounded 1.0.70 inspection responses.

The measured empty A and populated A/B responses contain the existing identity
subset: exact `grokVersion`, canonical `cwd`, same `projectRoot` with an optional
trailing separator, boolean `projectTrusted`, and `skills`/`plugins` arrays.
Populated A/B each identify their own project plugin by exact name/scope/path
with `enabled=false`, plus the independent original-user plugin. Outside cwd
lists only the user plugin and has no project root. The separate plugin-list
command returned empty arrays even for these candidates; it is not a substitute
for project inspection. Native trust and MCP execution remain unqualified.

Admit exactly the measured 1.0.70 version alongside 1.0.67 in the existing
project-inspection parser. Retain strict duplicate/case-fold handling, exact
cwd/root, typed trust and collections, unique plugin names, canonical absolute
paths, scope checks, ownership and namespace collision rules. All other versions
remain closed. Update only the corresponding version-specific user guidance;
project activation remains Prepared and MCP qualification remains unchanged.
Add focused tests for both admitted versions and unchanged refusals, then rerun
the existing genuine Grok startup capture. No new command, alternate catalog,
trust grant, provider, permission mode, deadline or cleanup policy is introduced.
Two independent plan reviews precede this product correction.
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

## Observed Cline standalone CLI presence correction

Genuine Cline runtime [40357515451](https://github.com/777genius/universal-agent-plugins/actions/runs/40357515451)
at source `4a5c4d32e8882217a9439fb6833938731943011e`, tree
`9eeda83072ec9adae47b7b11b0bb7a7d40ef0a05`, executed the exact acquired
`package/bin/cline --auto-approve false --version` successfully and printed
`3.0.73`. The public project `add-a` then refused `target "cline" was not detected`
before any target mutation. Native artifact `12671534049`, SHA256
`ed0637625cba4c40ed2f3ccaac582deb3a024e2bc1613813faa97acc827e4560`,
and frontend `12671673849`, SHA256
`47d1583165a7e7a8af5282c5d3d655583a49a7f2cd8640f18d9ca03bf33c7a05`,
retain that result. The child's original controlled PATH already includes the
verified CLI's containing directory. No native ACP session was started.

The production Cline adapter currently observes only four editor-extension or
editor-globalStorage surfaces. Its supported project file preparation does not
require an editor or a version-probe executable, and its own activation guidance
already allows a fresh Cline CLI process. A real standalone CLI is therefore
missing presence evidence, not missing files to be fabricated in an editor home.

Append one `cline_cli` presence surface through the existing read-only
`host.BinarySurface("cline_cli", "cline")` mechanism. Keep the four existing
editor surface IDs, order and rules intact and keep ConfigRoot at the original
`HomeDir/.cline`. Preserve the adapter's empty ExecutablePath: this bounded
correction reports observed CLI presence for the existing file-based lifecycle;
it does not introduce native version probes, a primary executable selection
policy, a CLI version qualification or automatic client execution. The pinned
hosted fixture independently retains its exact acquired executable/version and
all ordinary-user, runtime-file, provider, approval and process checks.

Do not count an authored `.cline` directory or a project configuration as
installation evidence; without a supported editor surface or PATH binary the
client remains not detected. Add no fallback executable names, fabricated
extension directories, fake detector flags, profile relocation, extra native
invocation or change to project activation/ownership. Existing editor-only and
coexistence cases retain their original user authority. Prepared state and
all runtime/authentication/model qualifications remain unchanged.

Update the three detection goldens only with the new absent CLI surface for
their existing editor-only fixtures. Add focused detector tests for CLI-only,
editor-only, coexistence and no-supported-surface cases, preserving exact
ConfigRoot and the unchanged empty executable/version metadata. Both ordinary
and explicit targeted-version discovery must perform zero native probes for
Cline under this presence-only contract. Add a public local project add/remove
regression using the real detector with an inert PATH lookup and no editor
installation, verifying project ownership and original-user/sibling preservation;
no vendor executable runs locally. Then rerun the existing actual hosted ACP
attempt, with its separately reviewed installed-receipt framing correction.
Two independent plan reviews precede this product detection change.

## Observed native CLI installed-package receipt framing correction

Genuine Kiro run [40354148388](https://github.com/777genius/universal-agent-plugins/actions/runs/40354148388),
job `121708484979`, source `d650ef37ff5cd2f38f6b784ebcd9723c47e62cf7`, tree
`6b2be08d15092de328f2644478036f8c751e2b94`, completed public project add with
Prepared activation and installation verification, then refused the exact native
ownership comparison before any restricted startup. Native artifact `12672746098`
(SHA-256 `26403a83fe3bdf64819d29d652011a3265b78ee977310cbd068830d44dbad75b`)
and frontend artifact `12671093626` (SHA-256
`4582142ca5697d5ab25576759d2431945caaebb5bc493876e2f1377886020ff4`)
retain that failure. Independent reconstruction from the captured selected skill
and MCP entry matches their two recorded object digests; the fixture's managed
package framing uses a different production contract.

The exact fixed local-source/no-save public add in these four startup attempts
uses `runAddManyLoaded` and the existing lifecycle service. Kiro, Windsurf/Devin,
Grok and Qoder do not have `QualifiedLocalInstallerFacade`; that trait is set
only for Codex and Claude. `cli/cmd/agentplugins/compose.go` wires
`providers.Stager` into the lifecycle service. Its `assembleDelivery` records
`packagesnapshot.Builder.Build(...).Digest`, and its installed verification uses
that same builder. The shared attempt instead currently uses the newer,
domain-separated `agentplugins/adapters/packagedigest.DigestCaptured` result.
The four fixed specifications therefore need the same correction; no different
Grok/Qoder route or algorithm is inferred from their uncompleted runtime runs.

Change only the independently computed expected installed managed-package
digest in these fixtures to the exact selected provider's existing framing:
lexically ordered held entries; `dir` or `file`, relative slash path, executable
boolean and decimal content length separated by NULs, followed by exact file
bytes. Keep the current bounded held reads, full ordinary-file ownership/link
checks, canonical parent checks, entry/depth/byte ceilings and the existing
captured-path policy validation. Use no client-configurable digest choice and no
alternate digest fallback. Exact object count, kind, ID, logical name, path and
digest equality remain mandatory. Production routes and digest implementations,
source identity, installed source/receipt checks, native argv, profiles, trust,
permissions, deadlines, cleanup, artifact members and qualification remain unchanged.

Cline's separate installed ACP attempt uses that same non-facade lifecycle and
Stager route, and its installed `installedEntries` comparison has the identical
framing mismatch. Route that one installed-receipt expectation through the same
small strict held-entry helper in `agentplugins_project_native_runtime_test.go`.
Keep Cline's separate `sourceDigest` and package-revision expectation on
`packagedigest.DigestCaptured`: source acquisition/loading deliberately retains
the newer `PackageSnapshot.TreeDigest` identity. The installed receipt and
source digest must remain separate; neither is copied from a returned receipt.
The shared helper accepts only previously held ordinary files/directories,
retains captured-path policy validation, and offers no algorithm selector.

Replace the self-selected newer-builder parity contract with inert staging
through the actual four registered client adapters plus Cline, project planner
and `providers.Stager`, including the mixed Kiro/Devin managed-launcher
projections and Cline's separate native transport projection.
Compare the helper's held-byte digest against the actual staged managed object
and `packagesnapshot` verifier; explicitly show the newer framing differs and
Cline's independent source identity still matches the newer source builder.
Exercise content/executable/directory changes and retained hardlink/alias
refusal. No vendor process executes. Propagate the reviewed shared-helper/contract and caller deltas to the relevant
leaves; all initial completion and qualification flags stay false.

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

## Draft Grok installed prepared-candidate visibility capture

This proposed narrow follow-through awaits scope assessment and two independent
plan approvals before code. It adds one visibility stage to the existing Grok
installed-startup attempt, not another collector, lifecycle case or capability.

The concrete gap is the join between existing evidence sets. Genuine observe
run `40353442581`, source `3234be86f0a3217ebd2371dc5e8fa0b26d3dcd26`,
tree `74490cebf339e8538b1f237b478aceeb58667e3d`, artifact `12670385326`
(ZIP SHA256 `a0d9c2e0858b5bbb2297f4d6f6f99aa71a57cfbc3d9ba5ec908434dfe14db957`)
measured project candidate name/scope/path and disabled status, but those native
directories were authored directly by research. The composed saved lifecycle
test exercises the public installer with an inert native runner. The real
installed-startup fixture validates public add and installed skill/receipts,
but retains no independent raw `inspect --json` output after that add.
Production `inspectProject` deliberately permits file-valid Prepared output
when its native list lacks the candidate, so successful public add alone does
not establish candidate visibility. This is a missing research observation,
not evidence of a production bug or a reason to tighten preparation behavior.

After the existing `f.setup`, run exactly one Grok-only command, label
`native-inspect`, argv `<pinned-grok> inspect --json`, cwd selected project A,
timeout 30 seconds. Reuse the same command supervisor, original HOME, literal
updater suppression, immutable/protected checks, bounded streams and original
150-second work / 180-second total deadline. Other clients' command tables and
all existing native-startup argv remain unchanged. There is no additional
provider, login, trust grant, plugin enable, permissions change or native tool.

Parse only the already measured identity subset using the existing strict JSON
and key-folding helpers: exact `grokVersion=1.0.70`, cwd A, projectRoot A with
the measured optional trailing separator, boolean projectTrusted and typed
skills/plugins arrays. Require projectTrusted=false and exactly one selected
name in the complete plugins array, scope=project, path equal to the already
validated selected binding locator and enabled=false. Refuse ambiguous names,
wrong types, aliased/noncanonical paths, duplicate or case-folded identity
fields, missing candidate or changed version/scope/path/status. Preserve and
recheck authored and delivered skill bytes around this read-only command.
Require the selected locator to be a canonical direct child of A's exact
`.grok/plugins`, with the validated physical-artifact basename. Require its
ordinary owned directory ancestry and held native manifest/skill paths without
aliases or extra links before and after inspection; string equality alone
cannot establish this path authority. Reuse existing bounded no-follow readers.
Capture the full raw command triple; summarize only visible disabled Prepared
candidate identity. Do not treat counts, labels or retained files as skill
loading, active discovery or source revision/runtime proof.

Admit the new complete triple only for Grok in the existing closed frontend;
partial/unknown members and every other client's admission remain unchanged.
Root owns this minimal shared frontend change. Add focused inert native-JSON,
fixed command/cwd/deadline, wrong-client and artifact-triple contracts, using
the held genuine inspection bytes as the positive schema reference. Then run
the existing restricted startup attempt unchanged. The whole initial attempt
still deliberately fails; all five qualification, vendor-authentication and
real-model flags remain false. This stage does not establish B/outside
isolation, full lifecycle, restart/removal or a new supported native surface.
Further lifecycle/qualification work requires its own reviewed evidence and
definition. Independent code review and a fresh genuine hosted capture remain
mandatory; no vendor process may run locally.

## Grok empty-profile migration sentinel amendment

The genuine PR522 source `753936db2e50613c2a4310848a262ec4f25dea87`,
run `40553581491` / native artifact `12759346679` (outer SHA256
`f90a4502e5e0bac54dfbb37372a29916431a2c0b4fc7aedb0afed6fa5832531a`),
fails the original config digest guard. All eight model facts remain equal;
the sole added table has key SHA256
`dfd76b3ecbefe4606d9cbfa7e7c41884b22d214a0c13560bb7eb10e65c11851f`.
Independently verified public snapshot
`xai-org/grok-build@2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8` identifies
the exact source-defined table as
`marketplace = { default_skills_installs_purged = true }`.
Its canonical JSON value SHA256 is
`292bc27c01bd4c8e2df64aacd7e75d92db6d9e3f446970ea2d7d97f011250093`.

The primary shell `extensions/marketplace.rs` blob
`ea896d05e4471573974e273049d6deb868e92cd6` lines963–1082 reads a typed
boolean, returns before migration when true and sets it even for an empty
legacy registry. Shell `plugin/mod.rs` blob
`60d0d7fba53875e2456b6ebef0fc0744f4fe35ac` lines1346–1390 writes the
literal boolean into that table. These exact Git blob identities were
independently reconstructed. This does not resolve shipping build
`f50904eaee6c` to the public snapshot or export the changed raw TOML.

Root R1 and independent CLI R2 approved the exact plan SHA256
`30a05da404dc1de8103c030510aff5c34daf2d128f5ad6ea462c8a488e986e77`
before implementation. The disposable original HOME has no legacy installs.
Author only `[marketplace] default_skills_installs_purged = true` in the
initial provider recipe before its first write/vendor child. Freeze all bytes
and the original digest as before; any later format/removal/false-toggle
continues to fail the original protected guard. Preserve the eight model
fields, environment, commands, provider route guard, diagnostic limits,
process/deadline/admission and all false qualification flags. This grants no
marketplace source, trust, permission, account or plugin activation.

Inert contracts assert the complete three-table recipe, independently fixed
semantic hash and initial held bytes/digest. Actual inert test children prove
the original fatal for formatting, removing or toggling the flag. Existing
diagnostic count assertions retain one changed member against the new third
authored table. Focused `^(TestGrok|TestProjectNativeCLI)` contracts passed
in 1.161s with Go 1.26.10 and approved inert loopback access; the first restricted
invocation could not bind sockets. Actual all-enabled root lint versus the
PR520 base and unfiltered whole-root six-linter gate both reported zero issues.
Gofmt/diff checks are clean; the actual twenty-member native archive still
replays as failed/incomplete with all five qualifications false. No vendor or
public installer ran locally. Independent exact-code review precedes root
publication. A fresh genuine run must establish preservation;
the existing rejected GET remains `uri=other`, and this amendment accepts no
new HTTP route or final capture/qualification boundary.

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


# Grok: isolate the measured initial-title side call in a fixed fixture

PRE-CODE proposal. Two independent exact-plan GOs before implementation. Read
AGENTS.md, PROJECT_INSTALL_OWNER_SCOPE.md and capability preservation. No release,
VS Code, trust widening, real account/model, local native execution or generic
provider/model acceptance. Root alone publishes to existing PR522.

## New source-bound actual evidence

Current611f7dd79a923a3b8e445222cde8395bd1af1062,
treee2b94d93297bbbf9ef316af922a87c426ac083d7. Runtime40586113413,
job122486692802 fails after3.68seconds. Native12775012222 outer5333B SHA256
d1aa56111ae7e3270069b47abcc9a022dfdd7e3fa0ee444ccd43e1f70701c7d3,
inner e4869bdb8bd69c35a21090d7bd0fb441d4930f2b6358992e6a7707dd6fcfc3a9,
21ordinary closed members. Frontend12774612789 outer3395B SHA256
0a2b462b88809e6dcc861e8123a00d929e48709d9130c5b6aa70b6a0f645e32d,
25members. Exact source/tree/cleanbuild,24stream joins, official archive/executable
pins, public add/source/package/physical/binding/receipt joins and exact-source
failed-envelope reader replay pass independent audit
/tmp/uap-grok-611-current-40586113413/independent-raw-audit.json SHA256
f36b98f1cb8585efce2cdfd616557cabe4f3b0132aafb26e4b2ee1da7ed2f45c.

Public add exits0 Prepared. Actual inspect exits0 in68ms and selects exactly the
receipt-owned project plugin,enabled=false,projectTrusted=false,one provided
skill,zero loaded skills and no MCP. Native startup is canceled after364ms by
provider request-shape refusal: one captured authenticated literal-loopback POST,
zero responses,ordinary provider shutdown,forced_close=false. The body is an
initial-title call with model=grok-4.6,two messages,stream=true,temperature1,
max_tokens100 and forced session_title function schema. It is not the main
uap-grok-fixture request. The main provider correctly refuses it; no completed
native output or loaded skill is proved. Every5/runtime/auth/model flag is false.

## Exact publisher source and limits

Official xai-org/grok-build snapshot2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8
contains independently Git-blob/SHA-verified source retained in
/tmp/uap-grok-title-source, research /tmp/uap-grok-title-source-research.json.
Shipping buildf50904eaee6c is unresolved in that public Git snapshot; neither the
request's agreement with source nor the earlier optout proves binary equivalence.

- config/mod.rs blob568b51d0631e3fb2f1331b107501a5f9b1b72ea3,
  SHA256a84e2c680ec3ef795d2847c7b1832247e797392d7e9f7f2d17b410585c936f63,
  lines500-625: `[models] session_summary` / GROK_SESSION_SUMMARY_MODEL resolves
  the auxiliary title model; empty values fall back to compiled grok-4.6.
- session/summary.rs blobf0c3ae859ffb6ffe3f891fca4f2b458cca601209,
  SHA2568d9dafb70d64704c48d1b27330af4218b8ba5a51cb8b20814557f974d06d7806:
  SummaryGenerator starts an initial title task on first nonempty content.
- session/helpers/session_summary.rs blob0f2af81e78701043b8446a8bffb2272921f3802c,
  SHA256fde704829e87366b93280fe031e01aaaca2d5bd169a67d0bc777f3013f7f8837,
  lines188-244 constructs the measured two-message session_title request and
  reads a title from returned function-call arguments.
- agent/mvp_agent/agent_ops.rs blobfa77f1ec809cf3c944ba91895a73ba422a24054a,
  SHA256399735fd57b748735efaaef3f956c6b428af60bbe436754031e91a9342e776bd,
  lines85-135 resolves the configured auxiliary model's sampler; missing model
  resolution falls back to the primary sampler with the auxiliary model slug.
- agent/config.rs bloba430462ad3b8e0611368ad2051f093827d7babee,
  SHA25651eadad05c42e19954fe0822dd187dc59694252bec34d513cc329a05394b47a2,
  lines4489-4515 returns a catalogued custom auxiliary model's own endpoint/key;
  lines4597-4612 preserves them while stamping local context/max_retries.

The examined CLI/summary/persistence source has no verified initial-title disable
setting. GROK_TITLE_REFRESH controls later turn-checkpoint refresh and does not
suppress the initial SummaryGenerator. Do not add it as an assumed solution.

## Proposed narrow fixture boundary

Preserve the original main provider's literal authenticated POST route, loopback,
Host,MIME,credential,body/model,one-request/response,count,failure,cancellation and
cleanup predicates. Main provider admission/capture/serve/response functions and
the shared native command fatal/review-barrier path remain byte-identical. Do not
let the main provider accept grok-4.6,a title request,unknown models or a second
request. Original Prepared/untrusted/disabled inspection remains unchanged.

Only for the existing Grok attempt with its held main provider, start one separate
fixed loopback title fixture before configuring children. Generate its own fresh
private128bit fixture key,distinct from the main key. Author the original HOME's
already frozen config before startup with `[models] session_summary =
"uap-grok-title-fixture"` and a separate `[model.uap-grok-title-fixture]` entry:
that exact model, its held127.0.0.1:random/v1 endpoint, its dedicated env_key,
chat_completions backend,unsupported search/reasoning and max_retries0, matching
the established custom-model recipe. Keep default/main model,original HOME,
marketplace sentinel,existing shared-client optout and all other config unchanged.
Add exactly the held title key to the existing closed child environment; do not
import ambient GROK/session-summary/title keys or alter global environment.

The title fixture is a bounded one-exchange sibling with its own count, raw
request/response and finite receipt. Reuse only existing strict route/ordinary
transport cleanup helpers where this does not alter the main model boundary.
Require exact POST /v1/chat/completions,loopback peer,matching Host,relative URL,
empty URL host/raw path,one application/json and one exact Bearer dedicated key.
Reject redirects,GET/prewarm,missing/wrong/multiple credential or MIME and second
attempts immediately. Original context origins150/180seconds and native60seconds
are unchanged; listener limits/deadlines are no larger than existing provider.

Require whole strict duplicate-free bounded JSON with the exact source-derived
current schema: model uap-grok-title-fixture; stream true; stream_options exactly
include_usage true; temperature1;max_tokens100; exactly the measured system text
and current fixed harmless prompt wrapped in user_query; exactly one session_title
function with measured description,strict single-string field schema and exact
forced tool_choice. Reject extra/missing fields,wrong text/model/schema,tool calls,
NaN/trailing data and body>1MiB. Do not interpret or forward returned native paths.
After exact route/authentication and the1MiB read bound, require a whole strict
JSON object before retention. Check both original held fixture keys against raw
bytes and recursively decoded JSON strings/keys (including JSON-escaped spellings)
before any body capture. Any privacy match fails/cancels without retaining body.
Malformed/nonJSON/trailing/duplicate/nonfinite/oversized bodies and all credential
failures likewise retain no raw body. Only a safe bounded strict JSON object is
captured, with requests=1, before the full fixed title-schema check; a schema
refusal retains that safe request,records0responses,cancels and stays allfalse.
No response or raw capture follows malformed/oversize/privacy refusal.

Return exactly one deterministic OpenAI chat-completions SSE tool-call result for
session_title with fixed harmless title `Disposable plugin startup observation`,
finish_reason=tool_calls and one[DONE]. No generic text fallback or model request
forwarding. This synthetic title function is internal title metadata,never native
MCP,installation,skill loading,authentication or real model evidence. Main provider
continues its existing one fixed response only for its original fixture model.
Add a finite privacy refusal only to the new title receipt/validator; original
main provider failure grammar stays byte-identical. Both fixture listeners must
shut down ordinarily with descendants/connection
closure; either failure or forced close remains failure. Keep the original main admission/handler functions byte-identical. Extend only
the existing retention redaction hook to redact both held private keys from every
retained child/provider stream; no raw headers/key values. Its decoded/raw privacy
checks must refuse an unsafe title body before retention rather than rewriting
that raw request. Original main body/model/count/route predicates are untouched.

Export only three new finite names grok-title-provider.json,
grok-title-provider.request.json and grok-title-provider.response.sse with receipt/
request/response presence matching actual0/1counts. Receipt<=2048bytes,ordinary
closed output membership and existing stream/aggregate bounds remain unchanged.
Extend Python admission in a separate explicit title-member validator; do not
relax the original main receipt validator. Require exact fixed title role/model/
transport,finite failures,booleans/counts,coherent response and closure and private
key refusal. Old611/a1af/c6 failed archives must re-admit unchanged byte-for-byte.
The existing unconditional final review-barrier failure stays intact even if both
responses and native exit0 are observed. Every qualification flag stays false.

## Meaningful verification, review and publication

New inert controls independently build literal measured-title requests and assert
one exact response/count/receipt/membership. Reject main model/title mixups,
unknown/extra/missing fields,changed system/prompt/schema,wrong endpoint/credential/
MIME,oversized/duplicate/trailing bodies,second call,context failure,write failure
and nonordinary closure. Verify exact SSE tool-call name/id/argument/title/DONE
with independently stated literals/digests. Test both key canaries in retained raw
and encoded JSON and closed child env with conflicting ambient values. Exercise
configuration parsing for exactly two held distinct model endpoints/keys and
unchanged default/main/sentinel; no vendor process. Preserve all existing main
provider negative cases and exact old-source failed-reader replays. Verify the
main handler functions and original unconditional native fatal/review barrier
are unchanged by hashes/diff,not renamed or bypassed.

Freeze the reviewed plan in existing consolidated remaining-native-qualification
before code; create isolated checkout from fresh agreed current611 and preserve
concurrent work. Two independent exact code critics follow implementation. Run
focused Grok/shared inert Go,affected runtime Python,actual affected full-root
all-enabled and unfiltered six-linter gates,gofmt,whitespace under serialized slots.
No native/vendor,scanner or public installer binary runs locally. Root publishes
normal expected-head fast-forward only after gates and critics; then audit the
new genuine hosted capture. If shipping ignores the auxiliary model setting or
emits a different request,retain the refusal and plan from that evidence. No
broad fallback,changed trust,parser assumption or completed-cycle claim follows.

## Accepted dedicated-title amendment before implementation

Exact plan SHA256 `72cc3bcbe925d310806833836d6fa97d65dd1693ad0bcaaf9069e7939f8da07b`
received independent PLAN GOs from root and GPT-6.1 Sol xhigh
`gemini_qoder_finish` before any code edit. Receipt SHA256s are
`13a6e861fa554a9e106bdf9cde5711e4136a45b140086eab64c310068dbdc3bd` and
`134bb3ba23b7cd1ccbc553faa2f18a2d68a50df859ed6c82c469a157168ab614`.
Both rehashed all18 pinned primary files and reviewed separate auxiliary model
resolution and exact measured title schema. The original title config recipe
is authored before the existing initial baseline freeze; no later baseline
adoption is allowed. Safe bounded route/auth/strict-JSON/raw-and-decoded-key
privacy precede request retention; schema refusal sends no response. Main
provider handlers and existing fatal/review barrier remain unchanged.
Root freshly checked PR522 remains611 before this isolated checkout.


## Grok dedicated title fixture: final reviewed validation (2026-10-27)

The accepted separate-title plan `72cc3bcbe925d310806833836d6fa97d65dd1693ad0bcaaf9069e7939f8da07b` is implemented by code source `3c38d0d5143b4490ff67822954fe9794e55498a8`, tree `28bfe97d594d45a0a7d59767dd427c218d8889fe`. Two exact independent code critics approved the final source: root receipt `ddd9a66d794faf55fd350248f13a1e212a1c7092421670d455fa12401ede980d` and GPT-6.1 Sol xhigh Gemini reviewer `3e7beb62d23ea66ba1f262f87c367448be475e431bb03d03e0a7a6fada93204b`.

The held original HOME is authored once with separate main and initial-title loopback endpoints and private keys. The title handler admits one exact source-defined request and fixed internal session-title SSE response. Both literal and recursively encoded fixture keys are refused before raw request retention. The original main provider route/model/count/authentication functions, shared native process guard and unconditional final failed-only review barrier are unchanged. Three closed title evidence names have a separate bounded validator; all five qualification flags, native completion, vendor authentication and real-model qualification stay false.

Actual final focused inert Go contracts passed (package 2.150s); affected all-enabled lint against program base `a412ba5e137fdcb7e004393229b323211ed746e1` and unfiltered revive/funlen/gocyclo/gocognit/dupl/depguard each reported zero issues. The 41 Python contracts passed in 8.678s on preceding source `3edd581c5b9e3c39b7089dd4d9a5c78233d86b81`; the entire scripts tree is identical through final source (`2d4ac1a96e1f5ccf3a0d6ca7af546d4a39753406`). Original611 and a1af failed archives retain all21/20 member bytes and summaries with all flags false under original and current readers. Gate and transfer receipt SHA256 `954aba61be3972a92fb6a551e7e144b7b655e7a7e0ac58d11999e249b3fd296e`.

Two measured inert-test lint issues were corrected without changing the module version or any guard: context propagation and Go1.22-compatible HTTP request construction. Earlier failed logs remain in scratch audit evidence. No vendor client, public installer or scanner executed locally. Publication only schedules a fresh genuine hosted diagnostic; public-source/shipping equivalence, completed startup, loaded skills, user/project lifecycle and authentication remain unproved.


---

# Grok: bounded capture of the already-refused second main request

PRE-CODE diagnostic amendment. Two independent exact-plan GOs and root's accepted plan commit before source changes. Root alone performs Git/GitHub mutations/publication. No local native/acquired client/public installer/scanner execution. No production adapter, provider acceptance, trust/profile authority, native pin, workflow/selection, model, tool, permission or qualification expansion. VS Code and releases remain deferred.

## 1. Measured source-bound need

Current PR522 source `348e2ce8b3bca4f117673a3b6bf80bedf8a64975`, tree `b12330110d2dea50e613bf36ac3b3485c4cd2f1c`, checkout `/tmp/uap-grok-dedicated-title-current`. Hosted run40599296059 native122524071409 failed23:25:43 UTC. Inherited Cline122524071582 completed success23:28:09; all current captures are completed, preserved and independently audited. Do not rerun unchanged source.

Actual evidence is preserved under `/tmp/uap-grok-348-current-40599296059`:

| Artifact | ID | Outer SHA256 | Inner SHA256 | Members |
| --- | --- | --- | --- | --- |
| Native | 12778988075 | 6629d43cc1a3d835d95949afb06b279334f158cc166e847bfe9d949da30b3e86 | 96fd685041010dbf1d9bcfc7f11eb7680f145cf3160608a1ed5898bfad33e153 | 25 |
| Frontend | 12778368337 | 066b0b7fc95d8819779838b46df0b24add56bf6a8bceac148180be270ef67d60 | 9ee9b68f7c5317b1c2677584eec9077ca4a62b66504a1124cb7371fa9a64c915 | 25 |

Independent raw/source receipt `/tmp/uap-grok-348-current-40599296059/independent-raw-audit.json`, SHA256 `95a89455a84656cac9b6122258a54579ea8d1a45e3e780888cf2ca8934313fb1`, verifies API/ZIP/CRC,24 frontend streams, exact current source/build/pins/scanner and public installation/source/physical/binding/committed receipt joins. Root's second detailed raw review is pending when this draft is written; do not count the peer receipt as two completed raw reviews.

Public add exits0 Prepared. Actual native inspection exits0 and retains projectTrusted=false, plugin enabled=false, zero loaded skills and no MCP. No native skill execution or lifecycle proof follows.

The separately configured title fixture now succeeds exactly once:1220byte strict request SHA256 `6310901ba3d8138dbf06eb01e447139f815a42424513ed048263f39b1ced658a`;570byte exact SSE SHA256 `eea6c4c389c7e27cf0e6d1cc9e705bc84a4d77ae8c5afddaa846ec8255e6ed68`; correct credential, failure none, ordinary shutdown, no forced close. The original main fixture retains one42233byte request SHA256 `001a585d3fc552ec983c8b996b0b4fd0acc719cb1759710a6534cd4d481affff` and one394byte original response SHA256 `f2c19ea9dbc0ddbf7cc2facaa5d2d01877897afa0f6fea33cc5651c0ecc9f7b9`.

Native stdout1476bytes SHA256 `2cefc7bfe383c0a900df87811d4e0dacc14dccc34ae7c97e30960d70ef0ef562` has two available_commands frames, the main sentinel text, usage and a successful end_turn. This is followed by an actual second main HTTP attempt that hits the unchanged provider's duplicate guard. Main receipt remains requests1/responses1/credential_matched true/failure duplicate/shutdown_complete true/forced_close false. The native command is cancelled after369ms, exit-1/errortrue/deadline=true; total fixture3549ms. The deadline field is ctx.Err()!=nil, not evidence of an elapsed60second timeout. Its original child/capture/cleanup guard remains fatal. No probe or tool exchange exists; allfive/runtime/auth/model flags remain false.

### What source establishes, and what it does not

`grokProjectProvider.admit` checks the original context, literal loopback POST route, matching Host/relative URL/no raw path, exact single application/json and one matching Bearer key before assigning duplicate when attempted is already true. It calls failWithRequest and returns before serve reaches body read. failWithRequest cancels the original context. Therefore the second attempt passed these route/header/auth predicates, but its body/model/purpose was neither read nor retained. The requests counter counts retained requests, not HTTP attempts.

No current raw evidence establishes a retry, second auxiliary role, equivalent body, additional prompt or new valid model grammar. Public publisher source may inform future analysis but cannot reconstruct an uncaptured actual request. No new response/admission is justified. A bounded failure-only observation of the already-refused request is the smallest concrete next step.

Pins remain1.0.70 archive11d27a30adfb75c4b9b180072ab27412a1916f054ff5497ab70fb9a77d304bcc / executable22d281c5964c5444147328096ba2a221f3f0874c6858e27c825656a80f6e58e9; shipping buildf50904eaee6c. Public source snapshot2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8 is not claimed equivalent to that unresolved shipping build. This amendment changes no native configuration or source-guided protocol.

## 2. Exact diagnostic eligibility and refusal

Instrument only the original main provider's duplicate refusal in the existing canonical main+title Grok fixture. The title transport remains a distinct one-call fixture and is never eligible. A duplicate body may be observed only when all original route/header/auth/context checks already passed, and under the existing provider lock:

* This is the held main provider, with its distinct held title fixture/keys configured.
* The first original main exchange has already retained exactly1 request and written exactly1 response.
* No earlier duplicate diagnostic has been claimed.

Claim this diagnostic once, atomically. Do not increment original Requests/Responses, reset attempted/failure, start another model/HTTP request, change model admission, or grant a response. Concurrent duplicates, third/further attempts and all other original refusal reasons retain their original immediate refusal/cancellation with no additional body capture. Do not record a second refused-body slot or retry a refused/partial capture.

### Explicit ordering and unchanged origins

The diagnostic must read a body before invoking the original duplicate cancellation, because that cancellation tears down the handler's BaseContext and native child. This is an explicit narrow timing change for the one eligible already-refused request. It is not described as byte-identical immediate cancellation.

After the original duplicate decision/one-shot claim, read at most1MiB+1 through the existing request body under the original unrenewed5second HTTP ReadTimeout and the unchanged original work/total contexts. Do not set a new read deadline, extend a server deadline, sleep, rebase an origin or add a new context budget. Check the original context before and after observation; a cancelled/expired origin refuses raw retention. The pre-existing native60second/work150second/total180second limits remain fatal. If the original work context expires during a partial body, existing final cleanup may abort the read; retain a finite refusal, not partial raw bytes.

Regardless of capture result, the existing closed-provider HTTP400 refusal and failure=duplicate/cancellation path execute. Use an unconditional deferred failure/cancellation safeguard for the diagnostic branch so read/parse/privacy errors cannot return as accepted. No SSE/content response is sent for the second request, and no provider acceptance or counter increase follows. Any cleanup failure remains failure evidence. All other rejection paths remain immediate.

## 3. Raw privacy/structure gate before retention

A complete body must be nonempty, at most1MiB, UTF8 and one whole strict JSON object through the existing duplicate-rejecting finite Go decoder: depth32,16384nodes,4096byte keys, finite numbers, no trailing document. Reject array/scalar/null/malformed/duplicate/nonfinite/deep/oversized/truncated/invalidUTF8 without keeping raw bytes.

Before retaining the object, check both original held main/title keys against raw bytes and the fully decoded/re-encoded object. Reuse the existing recursive Grok credential/privacy scanner on raw and decoded representations to catch JSON-escaped/nested strings and keys under its existing shared resource budgets. Any credential/private-namespace or privacy resource refusal retains no body. No file/profile/header/URL/private key or untrusted field is copied into diagnostic metadata. Do not redact a failing body into apparent safe evidence; raw retention occurs only after strict structure and privacy pass.

A safe bounded strict object is retained opaquely even if its model/messages/other semantics are unfamiliar. That retention is diagnostic evidence of refused input; it is not a new provider schema. Do not classify the body as a retry/auxiliary/tool/result or act on its content. Keep the first original request/response and title request/response unmodified.

## 4. Finite additional artifacts

Add at most these two fixed Grok-only members, exported from the actual held main duplicate observation during existing finish:

| Member | Bound | Purpose |
| --- | --- | --- |
| grok-provider.duplicate.json | 2048bytes | Closed diagnostic receipt, never successful provider evidence. |
| grok-provider.duplicate-request.json | 1MiB | Optional complete raw refused JSON object after privacy gate. |

The receipt has exactly schema_version(integer1), role(`main-duplicate-diagnostic`), attempt_index(integer2), status, original_failure(`duplicate`), response_issued(booleanfalse), captured_bytes(integer) and captured_sha256(string). Status is exactly one of captured/body-refused/json-refused/privacy-refused/context-refused. Captured requires the raw member, exact positive byte length<=1MiB and recomputed lowercase64hex SHA256. Every refused status requires no raw member, captured_bytes=0 and captured_sha256 empty. No inferred cause/model, raw headers, profile content or arbitrary error string is serialized. No receipt is present when no eligible diagnostic was claimed.

The original main receipt remains the original schema and failure duplicate with Requests/Responses1. The title receipt remains unchanged. New artifacts cannot coexist with another client, missing main/title fixture evidence, nonduplicate main failure, incomplete first exchange, unknown receipt fields/statuses or a successful runtime tuple. No raw member may appear without its receipt; no captured receipt may omit/mismatch raw. Existing historical failures without these members remain accepted unchanged.

At the measured25member full capture, this adds at most2 members (27total); original per-member/archive aggregate/privacy bounds remain unchanged. Do not raise generic archive limits, admit arbitrary new member names or alter frontend evidence. The unchanged shared final privacy/secret-free checks must recheck exported bytes before persistence.

## 5. Implementation boundaries

Expected source edits are only:

* Existing `agentplugins_grok_project_native_provider_test.go`: small private one-shot observation state, exact duplicate-only hook and finish export. The original context/route/auth guards, first request capture/model predicate, fixed response bytes, original refusal message/failure value, counters, transport configuration and stop/cleanup body remain unchanged except the explicit bounded-before-cancel diagnostic ordering above.
* New small dedicated Grok refused-duplicate helper and inert contract files for bounded capture/privacy/schema and actual HTTP refusal behavior.
* Existing `scripts/project_native_runtime.py`: one exact Grok duplicate-member validator and narrow allowed-member union. Existing main/title receipt/response validators stay byte-identical.
* A focused Python contract addition covering only the closed new diagnostic; preserve all old tests/outcomes and exact selectors. Add no workflow/new native lane.
* Accepted plan and concise factual validation documentation.

Keep main/title model configuration, environment, title handler/schema/SSE, original user HOME/profile freeze, Prepared/untrusted/disabled inspection, public installation, command fatal barrier, all process/supervisor/environment/authority guards and other clients byte-identical. No change to the original number of accepted main/title exchanges or any qualification field. If implementation needs another path/behavior or privacy/budget exception, stop for exact amended review before editing it.

## 6. Meaningful checks and next actual evidence

Under root's serialized test slot, inert HTTP contracts perform the actual first accepted main request/response and one eligible second request to the held loopback fixture, with an independently authored harmless JSON object and separate held title key. Assert first response bytes/counters unchanged; second is HTTP400, never SSE/200, original context cancels, Failure duplicate remains, accepted counts1/1 remain, only one diagnostic retained, and ordinary existing stop behavior or truthful forced failure. Synthetic capture is not native evidence.

Exercise malformed/duplicate/trailing/nonfinite/null/array/deep/node/key/UTF8/oversize/partial bodies and raw/escaped/nested main/title credential strings and keys. Check zero raw retention on every refusal. Wrong/multiple auth, route/query/method/host/MIME/loopback/context, incomplete first exchange, title-provider duplicate and third/concurrent attempt cannot acquire another capture. Partial-body cancellation stays within existing server/original origins, fails truthfully, renews no context and sends no accepted response. A new synthetic raw object must remain ordinary data; no real tool/model is invoked.

Both Go artifact helper and independent Python reader contracts enforce exact member/receipt relation, status typing, byte/hash/privacy revalidation, fixed role/index/failure/false response flag, first-exchange receipt joins and client isolation. Coherent hash-updated malicious objects must fail privacy/structure, not only mismatch hashes. Unknown/extra receipt fields, altered counters, forged success flags, missing title/main evidence, raw without receipt, duplicate raw slot, unknown files and refused status with a body refuse. Existing failed-only runtime boundary/allfivefalse remains mandatory.

Execute focused inert Grok provider/title/project/native-runtime contracts plus affected Python runtime contracts, gofmt/whitespace, affected all-enabled lint againsta412ba5e137fdcb7e004393229b323211ed746e1 and whole unfiltered revive/funlen/gocyclo/gocognit/dupl/depguard. Record actual command/source/exits/log hashes; no skipped predicate or revised production assertion. No native/vendor/client/public installer/scanner execution locally. Broaden only for concrete remaining risk.

Scratch pure readers replay unchanged34825member capture and prior61121member/representative earlier captures with their original exact source identities; original bytes/hashes/outcomes remain unchanged. Preserve inherited current Cline success independently. No old capture is relabeled or promoted. Obtain two full exact-source CODE critics and measured gates before root normal-FF publication.

Only a fresh genuine current-source hosted capture may provide the actual second raw object or precise refusal metadata. Audit its exact source/tree/pins/public/inspection/first/title/duplicate evidence and full fatal/cleanup boundary independently before any new proposal. Even an observed safe second JSON body grants no response or completion; this lane remains failed/allfivefalse/runtimefalse/authfalse/modelfalse. No speculative second-auxiliary response, retry allowance, trust change or generic provider admission follows.

## Baseline preservation anchors

| Path | SHA256 |
| --- | --- |
| `repotests/agentplugins_grok_project_native_provider_test.go` | 6cb8f0f06e3e8a5ffec6dfb654e1566000ac9ca8dec1d450c4aa0d27a2421c65 |
| `repotests/agentplugins_grok_project_native_route_test.go` | e41da580ebea3277d155e6a443149bb4463b65cc40fcf3fa3fa6d6f7ba78ccf7 |
| `repotests/agentplugins_grok_project_native_title_test.go` | 76d595042a2fc2d3639cb4dad0568b8691485f342101689a76a97d844cf27658 |
| `repotests/agentplugins_grok_project_native_privacy_test.go` | 4df4765a0caec14e4aa55963cb3000e1204ed0a9c1d1afe122d46cf6210c5164 |
| `repotests/agentplugins_project_native_cli_attempt_test.go` | 07f40ed4e0def62f58698cd6d8ce839d407e6bc751e952680460affe39670791 |
| `repotests/agentplugins_grok_project_native_attempt_test.go` | ee4f42b4574190e5350a1dde9f0bc4f90517bd5d1da70d27d78916e317f1aea4 |
| `repotests/agentplugins_grok_project_native_environment_test.go` | dbbae16a28cfe9e893c30475ba8bfac343b41b06c3c264ad7dc0d2965b69cdb4 |
| `scripts/project_native_runtime.py` | 24f39c61856d689535ac2fea81e01890caa780f4b74630b78c12c0df39cd6aee |

This draft author read source/raw and wrote this scratch proposal only. No implementation, tests/gates/native execution or Git/GitHub mutations occurred for this plan.


## Accepted refused-duplicate diagnostic plan before implementation

Root R1 GO receipt SHA256 `089275e9328eff1c195bbc1915d56391868c0de6c7c4c7ba48c95aaab2f62969` and independent Kimi R2 GO receipt SHA256 `60d45fb3f2056ac8bfa2d9f0a8809efd9b372dbe7c720fc8c4097c9558aad4df` approve the exact draft above, SHA256 `670b875b7f2283058fcf3220a5cc3b222831e368c72b95bc10392e8bb233c0db`, before any implementation.

The draft-time pending root RAW review is now complete: root receipt SHA256 `32168fb35ff64fedba411599187159a3090a4556ccc4ce101a937549019ee423`, alongside independent peer receipt `95a89455a84656cac9b6122258a54579ea8d1a45e3e780888cf2ca8934313fb1`. Both preserve actual348 failed-only source-bound evidence; no qualification follows. Current native and inherited Cline jobs are complete and retained.

Both critics require the one eligible duplicate/first-failure decision to be atomic with no earlier failure overwrite, with no provider mutex held during bounded body/privacy work. Every path must execute original HTTP400 and duplicate cancellation, keep accepted counters1/1 and original read/work/total origins, and export only a settled finite receipt with no partial or private body. Independent Python finite byte/number validation and concurrent/partial-read controls are required by the accepted plan. Root alone owns commits/publication after two CODE GOs and measured gates.


# Grok: supported suppression of the measured post-turn dashboard summary

PRE-CODE PLAN DRAFT. Root and Native must independently approve this exact plan SHA256, and root must commit the accepted plan before implementation. This author performed source/raw reads and wrote scratch research/this proposal only. No implementation, test, native/acquired executable, public installer or scanner runs locally for this plan. Root alone owns Git/GitHub mutations and publication.

## 1. Exact source and measured reason

Plan base is published Grok source `b40da640c388d807cc3dc1fbb36c38e0d1b411ef`, tree `71b98e819bdd555154096c9c75850b9b40d3f54c`. Run `40610932429`, native job `122562097571`, has completed. Its actual evidence received two independent RAW GOs for failed diagnostic evidence only:

- Root `/tmp/uap-grok-b40-current-40610932429/root-raw-audit.json`, SHA256 `6fb432088fa50f3093c7240911be9695186fb3c077f3bdb96c1b28fcf464785c`.
- Peer `/tmp/uap-grok-b40-independent-40610932429/independent-raw-audit.json`, SHA256 `88c920ace4af34d476cdf133ea769751c136503e9149673718dfa48e3075fd9e`.

| Artifact | ID | Outer SHA256 | Inner SHA256 | Members |
| --- | --- | --- | --- | --- |
| Grok native | 12782438155 | 9cf3260936143bed9bf9bc53fe29fe67db8fe4ffa01ae720e45ecbfe691141a2 | 86ed0632dd8345b0a224d77563ca5f44708e69a35829524076d668ffd7946f16 | 27 |
| Grok frontend | 12782527675 | 8d331a482694e89814f8e6e7ff875d3f836ce86ad95ac98a5a9eac7c41e5deb0 | ee229372d55c8e33c1aec9379734ec695e26d5d106f62acde0a8566ce59a1ce8 | 25 |

The diagnostic retained the complete second main-route body: 1,424 bytes, SHA256 `303ad4817a23b68aa02717ecedb70404362cf1397324da5a4d25d12f60a86bb5`. Its receipt is index2, role main-duplicate-diagnostic, captured, original_failure duplicate, response_issued false. Original first main and separate title counters/responses remain untouched. This is a measured dashboard summary request over the completed reply, rather than an assumed retry.

Its system message is exactly the pinned `TURN_SUMMARY_SYSTEM`: 1,022 UTF8 bytes, SHA256 `9411e91b009d5f6672caa80f5aa9fa2eadbf4d1aa12d0b1488238afa64f58103`. Its user message exactly equals the source wrapper around the actual first main request's final user content and the fixed `UAP_GROK_PROVIDER_CAPTURE` reply. The model is the original main fixture, with streaming and include_usage true. The provider correctly refused this additional request and kept original cancellation.

Public add and Prepared ownership checks do not establish native activation. The measured project remains untrusted; the selected plugin is disabled and skills/MCP inventories are empty. Runtime and all five qualification flags, authentication and real-model qualification remain false. No old capture is upgraded by this proposal.

## 2. Pinned primary source and lineage limits

All cited files belong to public `xai-org/grok-build` tree `2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8`. Its complete 5,102-entry tree is non-truncated. This author independently recomputed Git blob SHA1, SHA256 and lengths for all eighteen previously retained title/source files and thirteen additional turn-summary files, and verified their paths/blob identities against that exact tree. Both actual ZIPs above were independently rehashed and every closed member's size/SHA/CRC checked.

Primary research receipt: `/tmp/uap-grok-turn-summary-kimi-primary-research.json`, SHA256 `3de4e360348bd3188ea35f584fb48538f3dba89245f78801d0ed3e0d8daae930`. Native independently researched the same source: `/tmp/uap-grok-turn-summary-native-research/independent-source-research.json`, SHA256 `5e72513507b8d734cd87aade73afc448dec03c2b2f88cc99f5fb0feea9d6ca74`.

| Source path under `crates/codegen/` | Exact Git blob | Source fact |
| --- | --- | --- |
| xai-grok-config-types/src/registry.rs | 645a34036de2a83981f8f90ba0e2cf0d7a334a40 | Lines229-236 define TurnSummary, key turn_summary, path features.turn_summary, env GROK_TURN_SUMMARY, default true. Feature::resolve uses requirements/environment/config/remote/default. |
| xai-grok-config/src/resolved.rs | 7ee2b55aa5d44dd399579b8eac7250173f28b35e | BoolFlag resolve around95-134 chooses requirement, CLI, environment, configuration, managed, remote, default in order, preserving explicit false. |
| xai-grok-shell/src/agent/config.rs | a430462ad3b8e0611368ad2051f093827d7babee | FeatureEntries around4224-4271 reads native TOML booleans;1492 maps them into feature_values. is_turn_summary_enabled1846 resolves Feature::TurnSummary; feature_sources2033 supplies the actual config tier. |
| xai-grok-shell/src/session/acp_session_impl/spawn.rs | 323fe32f305231bfac22aa60d43f3123efd1c38e | Actor captures effective_config.is_turn_summary_enabled() at2077; title_refresh is resolved separately around1778. |
| xai-grok-shell/src/session/acp_session_impl/turn_summary.rs | 84915ba9062b6fd394382e334cd35340133ad08e | restart_turn_summary returns before work if its gate is false or subagent. The helper is display-only and builds a tool-free request from the exact system/user source below. |
| xai-grok-shell/src/session/helpers/turn_summary.rs | 382718b80599b54c0bd985eb819a044ced13b8b3 | TURN_SUMMARY_SYSTEM and turn_summary_user_message match the measured second request byte-for-byte. last_turn keeps the actual last prompt and visible reply, excluding reasoning/tools/results. |
| xai-grok-shell/src/session/acp_session_impl/title_refresh.rs | e67a787a99e94cdc448276beba6018409158f45a | Later whole-conversation title refresh checks its own resolved title_refresh_enabled before starting. |
| xai-grok-shell/src/session/helpers/session_summary.rs | 0f2af81e78701043b8446a8bffb2272921f3802c | Initial session-title fast path starts around188 and is distinct from per-turn dashboard summary and later title refresh. |
| xai-grok-pager/docs/user-guide/26-config-reference.md | f5aa92e8599e821d81ca73b39049513bab4be2c0 | Row233 documents features.turn_summary boolean, default true, and GROK_TURN_SUMMARY. |

The public tree is not proven equivalent to shipping Grok1.0.70/f50904eaee6c. Existing source research explicitly retains that limit. Exact prompt correspondence establishes the observed request's specific source match; it does not prove every config path in the binary. The setting below is a supported-source candidate requiring a fresh genuine hosted observation. Unsupported/ignored/overridden settings must remain a truthful failed observation; do not add a fallback or reinterpret the response.

## 3. One fixed configuration change before original freeze

Append exactly this native TOML suffix to the existing complete two-model configuration during `configureGrokTitleProvider`, before its only original HOME config write, authoredConfig clone and protected digest freeze:

```toml
[features]
turn_summary = false
```

The exact appended bytes are one separating LF, `[features]`, LF, `turn_summary = false`, LF. The existing recipe already ends in LF. No dynamically discovered section/key, parser rewrite, config migration or file adoption is involved. This is a real TOML boolean, not the string false and not a `[turn_summary]` sampling-settings table.

Keep the same `f.base.Home/.grok/config.toml`, ordinary0600 write, original parents, original HOME/CWD and both fixed model endpoint/key references. Author the complete recipe once; only then write, clone and hash it. Do not edit config after the native client starts, renew a baseline from native output, change originalConfig/protected/immutable maps after execution or introduce another HOME/profile.

The existing child environment is constructed from a closed fixture environment and contains no GROK_TURN_SUMMARY or GROK_TITLE_REFRESH. Leave it byte-for-byte unchanged; do not add environment overrides as a second mechanism. Test that ambient true values remain excluded. No requirement/managed config override, remote fetch, credentials, trust, plugin enablement, tools or permission setting is introduced.

### Acknowledge the later-title-refresh effect

`Config::resolve_title_refresh` around2055 defaults to the current TurnSummary result when title_refresh is unset. Therefore false also makes later whole-conversation title refresh default false in this recipe. This incidental suppression is explicitly intended and bounded to optional post-turn display work. Do not claim that turn_summary exclusively controls one request.

Keep the existing initial session-title route/model (`models.session_summary = uap-grok-title-fixture`) and all its request validation/response/counter/cleanup unchanged. Its independently observed first-title request remains expected; the setting must not be used to remove that fixture or admit a different title shape. Do not add features.title_refresh as a separate setting. Shipping behavior is measured by the next raw capture, including truthful partial/failure outcomes if it differs.

## 4. Preserve every existing native/protocol authority

There is no new native/public command, argument, retry, model, endpoint, request grammar, response, counter or artifact slot. Preserve all original main/title route/auth/type/body bounds and exact fixed first SSE bytes. Preserve the new refused-duplicate one-shot claim, original first-failure reservation, unlocked bounded read/privacy, done handoff/export race checks, HTTP400 and unconditional original cancellation. If any second main request still arrives, the existing diagnostic must refuse it and retain only its already-admitted bounded opaque evidence; no successful summary response is added.

Keep original work origin150seconds, final origin180seconds, provider read/write5seconds, command contexts and ordinary full-descendant process supervision, original config/pin checks, raw privacy/resource bounds, no-follow filesystem/Prepared ownership checks, source/version/scanner pins, inspect command, no-probe/no-provider-qualification semantics and final all-false barrier. In particular, native exit0 still leads to the unchanged intentional raw-review fatal barrier; a clean startup does not activate or qualify anything.

The current 27-member capture reflects a reached duplicate. The existing reader already permits its actual finite optional duplicate presence/absence. Do not require a fabricated duplicate when suppression works, or remove a real duplicate when it fails. Keep exact existing allowed member unions, summary and failed-only modes. No Python reader or contract is changed by this configuration amendment.

## 5. Exact implementation surface and preservation exceptions

After two exact-plan GOs and root's accepted pre-code commit, implementation is limited to:

1. `repotests/agentplugins_grok_project_native_title_test.go`: append the fixed suffix to the complete original recipe at the existing configureGrokTitleProvider construction point, before the existing sole write/clone/hash. Preserve every other function in the file whole-byte, including title.configuration, all title request/response/serve/stop handlers and credential/listener setup. The minimal implementation can append the suffix directly to the existing returned recipe; no generalized configuration framework is warranted.
2. `repotests/agentplugins_grok_project_native_title_configuration_test.go`: add only `features: {turn_summary: false}` to the existing independently stated expected TOML map. This explicit one-literal expectation exception is necessary because the fixture recipe changes. Preserve the remainder of that test and every existing assertion; do not rename, skip or weaken it.
3. A new `repotests/agentplugins_grok_project_native_turn_summary_contract_test.go`: narrowly focused inert contracts for the fixed supported suffix, original freeze and ambient isolation described below. It may use the existing owned in-process fixture providers; it must not run acquired/native/public installer/scanner executables or impersonate a hosted environment.
4. Append concise factual plan/validation to the existing Grok validation record and accepted plan to `docs/plans/project-install-e2e/remaining-native-qualification.md`. Distinguish old failed evidence, local inert checks and future hosted results accurately. No raw capture/log/source vendor payload is committed.

Everything else is whole-byte preserved: all main provider, title handlers/configuration helper, duplicate/privacy/route diagnostics and existing contracts, selected project/Prepared authority, shared command/environment/process functions, collectors, Python runtime/Grok readers/contracts, production adapters/shared/VSCode, workflow/selection/pins and all other client files. New feature data is authored before the old guard's original freeze; no guard predicate or exception changes.

## 6. Meaningful inert controls and exact-source gates

The existing independent whole-map configuration contract must pass with its one extra boolean subtree. New inert contracts should demonstrate these concrete boundaries:

- Raw authored TOML equals the unchanged complete two-model recipe plus the one fixed suffix, and parses to the complete independently stated map with exactly one features table containing only boolean turn_summary=false. Preserve every original models/model/marketplace field, both distinct held listeners/keys, max_retries0 and env-key names. Reject accidental string false, true, duplicate section/key, extra feature or sampling-table substitution through the expected-recipe contract; do not make a new native parser.
- The same original HOME config is written and captured once with ordinary0600 metadata and protected digest covering the complete bytes; no project config is created. Keep every prior original-route/config-freeze assertion. Existing actual protected-config fatal-child controls remain unchanged and run in the focused gate; do not add a redundant new guard implementation or weaken fatal behavior.
- With ambient GROK_TURN_SUMMARY=true, GROK_TITLE_REFRESH=true and foreign HOME/model settings, the constructed child env still contains neither flag and keeps the original held HOME, both keys and existing shared-client optout. Parent process environment remains unchanged; other clients receive no Grok feature/env authority. Add these controls in the new file rather than changing old environment contracts.
- Existing strict first-main/title and refused-duplicate positive, malformed/privacy/Unicode/resource, partial/concurrent/third-request and closure tests run unchanged. A feature setting is not authority to alter provider behavior. No synthetic test claims that the vendor binary actually consumes the setting.

Root owns the serialized gate slot. Execute meaningful focused `TestGrok`/`TestProjectNativeCLI`/`TestProjectNativeRuntime` contracts and affected Go package checks with no vendor/native/public installer/scanner, plus affected Python runtime/Grok contracts or precisely recorded whole-tree transfer if every relevant Python byte is unchanged. Run gofmt/diff, affected all-enabled lint against `a412ba5e137fdcb7e004393229b323211ed746e1`, and whole unfiltered revive/funlen/gocyclo/gocognit/dupl/depguard. Record actual exact source/tree/commands/exits/log SHAs and real test counts/skips; do not predict them or transfer an old pass as a new run.

Verify whole-file/function preservation against the actual accepted pre-code base, including all Python modules/contracts and every protected handler/duplicate file. Replay actual b40/348 failed archives through original and candidate full readers with original source identities; raw ZIP/member/result hashes and all-false outcomes must remain identical. If root transfers a prior replay on a byte-identical complete Python tree, document that exact tree and receipt explicitly. No AST-extracted substitute reader or relabeled capture is allowed.

Freeze the exact source manifest and get two independent full/delta CODE reviews before root publication. The author does not approve their own implementation.

## 7. Composition, hosted observation and remaining qualification

This plan's evidence/recipe base is exact b40. Root separately owns program-parent composition. The read-only composition receipt `/tmp/uap-grok-parent888-readonly-composition-kimi.json`, SHA256 `eae34e2d1eb5d25c4b3e0286226fcb4380ee5caf291de33229090e214758bff7`, identifies three documentation insertion conflicts and clean disjoint Python composition. If root composes qualified Kimi parent before this amendment, preserve both full leaf readers/tests and record the actual new pre-code base/tree; these b40 hashes remain historical anchors, not permission to replace combined files with the Grok side. No unpublished stale083b or named1109 baseline path is changed or discarded. Other clients' completed/in-flight evidence remains preserved.

After reviews and measured gates, root alone publishes by the normal reviewed route and dispatches the unchanged genuine hosted workflow on the exact published source. Do not retry unchanged, supersede/cancel another pending capture, run the vendor locally or forge hosted eligibility. The next run must retain source/build/pin/API/closedZIP, public add and inspect raw, exact first main/title exchanges and ordinary cleanup. Independently audit whether a second main request exists and its truthful receipt/status. Absence after this supported setting is a new measured fact only on that fresh source; old b40/348 remain failed duplicate observations.

If the setting is consumed and native exits cleanly, the existing intentional barrier still fails with every flag/runtime/auth/model false. If summary or any other unexpected request persists, existing refusal/cancellation/diagnostic applies and no fallback is authorized. If config or original process/authority guards fail, stop and retain that failure. No new request shape, title response, trust/enable transition, registry admission or runtime qualification follows from this amendment. Any later activation/tool/lifecycle success path requires its own actual evidence and two pre-code reviews.

## Baseline preservation anchors

Hashes below are from exact b40. The first two files have only the narrow recipe/expected-map exceptions above; remaining files are whole-byte anchors. A separately reviewed parent composition must establish its own complete combined pre-code anchors before implementation.

| Path | SHA256 |
| --- | --- |
| `repotests/agentplugins_grok_project_native_title_test.go` | 76d595042a2fc2d3639cb4dad0568b8691485f342101689a76a97d844cf27658 |
| `repotests/agentplugins_grok_project_native_title_configuration_test.go` | 788c9ecd7c818f5477eff713960f3710d8d435d3e53b89e01af74ef1c16b0f87 |
| `repotests/agentplugins_grok_project_native_provider_test.go` | 90012dcf96cdfb882470e31a3feb52688d570ae64d9f356410ad7910c944e950 |
| `repotests/agentplugins_grok_project_native_provider_contract_test.go` | 6aeaf8af3953324b28075e042835b4875e3e66e2c780b831c4fb30bc93bba801 |
| `repotests/agentplugins_grok_project_native_privacy_test.go` | 4df4765a0caec14e4aa55963cb3000e1204ed0a9c1d1afe122d46cf6210c5164 |
| `repotests/agentplugins_grok_project_native_duplicate_test.go` | 53a29267624dd1e308f7a3fce50499c87308fec08581c9b1f7bbe807f7b91be3 |
| `repotests/agentplugins_grok_project_native_duplicate_evidence_test.go` | b2bcab1adcb962055232a72884505add1e8effdc32aaaf729cc0d3a608f91ea6 |
| `repotests/agentplugins_grok_project_native_duplicate_contract_test.go` | c81bc20643000614d26ef0c021b75bee56b4a7e6648325c9c91068f0420a98ff |
| `repotests/agentplugins_grok_project_native_title_contract_test.go` | f86600f85c5c24b7c7c08b10d6694d163b71aad092c3f3860c951a859a229b83 |
| `repotests/agentplugins_grok_project_native_environment_test.go` | dbbae16a28cfe9e893c30475ba8bfac343b41b06c3c264ad7dc0d2965b69cdb4 |
| `repotests/agentplugins_project_native_cli_attempt_test.go` | 07f40ed4e0def62f58698cd6d8ce839d407e6bc751e952680460affe39670791 |
| `repotests/agentplugins_grok_project_native_attempt_test.go` | ee4f42b4574190e5350a1dde9f0bc4f90517bd5d1da70d27d78916e317f1aea4 |
| `repotests/agentplugins_grok_config_contract_test.go` | 3153076a942ec8baf01c9b9d6e9ce328594e6dcbaa9f88f8e7eb25a2fd20ad47 |
| `repotests/agentplugins_grok_config_diagnostic_test.go` | 8c23fe40cf5fcddad742ae325f8c3cca5032dc00a88d2deaf278944e03fa5fc8 |
| `repotests/agentplugins_grok_config_refusal_test.go` | f32c416faeb9027bbed958317b563a23aff32a570e4170e2eea5dd24668139d4 |
| `scripts/project_native_runtime.py` | b8ab281512d562903f90a44fca1765a5d05b55bc977e9f0e71fd6c53de4f2d19 |
| `scripts/test_project_native_runtime.py` | d0ee55f930ed8ff790beb1b8a67baca894c7ee422f7b646949559a6c5136f82f |
| `scripts/project_native_collectors/grok.py` | 95ef4b4f3bad0c4dfbb82c652f834660ee0d994899ab0059a772d848517af56c |
| `scripts/test_project_native_grok.py` | 8e1d3c3267b55cfa29446a9d0509799fc9ff84d24256d1879d449e86b9d31fea |
| `.github/workflows/project-native-runtime.yml` | 11140878d0b2ca882a76b0a5830b25a1c5763247d76fec2c3dea9b22ba8096a1 |

This draft author performed pure source/ZIP/hash/JSON reads and wrote scratch research/plan files only. No source implementation, Go/unittest/lint/vendor/native/public installer/scanner execution, Git/GitHub mutation or publication occurred for this proposal.


## Accepted measured turn-summary suppression plan before implementation

Root R1 GO receipt SHA256 `a71549915420aa61f90a7ab05586e1e2280a516fa0b78eb3a0cfb2f48d45fd69` and independent Native R2 GO receipt SHA256 `a29a83c1a3b8fdf2ea5d70bc3e9e009ef88e58e974cc170d89b40ef07e90332d` approve the exact draft above, SHA256 `3bc56715f4b326634f99210fc7bde69a711c2edbe7c2880fece74ec950c4593e`, before implementation. Both independently reconstructed the captured summary messages from pinned primary source and verified all historical baseline anchors.

This pre-code commit retains original published b40 as its code base; parent composition is separate. Only the fixed supported TOML feature suffix before the original configuration freeze, one complete-map expected literal, inert contracts and validation documentation are authorized. Original initial-title/main/duplicate handlers, environment, deadlines, config/pin guards, readers, privacy, workflow and failed-only review barrier remain unchanged. Fresh hosted evidence must establish behavior of the pinned shipping binary. Root alone commits and publishes after exact CODE reviews and measured gates.


## Accepted Grok exact project trust and selected skill diagnostic

Pre-code base: `91402cd32f39342aaac9175c9e76bddbd360f2df`, tree
`c2f198f3ccf2bd8abfc4675c01cb2f153c1ad444`. Root and independent
other_clients PLAN GO were received before implementation, with the conditions
below. Publication still requires independent CODE reviews and actual gates.

The genuine run40615904857 observed clean native exit with one strict main and
one initial-title fixture exchange, while the project remained untrusted and
the selected skill absent. This amendment only captures the next boundary.
All five qualifications, runtime completion, vendor authentication and real
model qualification remain false; the intentional raw-review failure remains.
Grok project MCP is unsupported. Full installed lifecycle is separate work.

### Exact supported fixture setup

Before any child, append `[plugins] enabled = ["uap-grok-attempt"]` to the
complete existing original-HOME two-model recipe, before its existing final
`[features] turn_summary = false` table and sole config write/clone/hash.
This is a supported name selector, not path-specific activation. The selected
physical ID depends on the subsequently generated installation UUID. The
existing manifest, binding, source and delivered-byte checks must still join
the single selected ordinary project package.

Author original-HOME `.grok/trusted_folders.toml` once, exclusively, mode0600,
with only `[folders."<exact canonical owned project A>"] trusted = true`.
Check the ordinary canonical project and original HOME, refuse HOME/root/B,
overlap and aliases, and freeze the complete trust bytes under the existing
protected digest before any child. Refuse existing trust/config entries rather
than replacing or adopting them. No timestamp is needed; the publisher makes
`decided_at` optional. No `--trust`, trust feature optout, environment override,
`plugin-dir`, `[plugins].paths`, registry mutation or native tool is added.

### Measured selection and failed-only admission

Preserve the old disabled/untrusted Prepared checker and its negative tests.
Share its identity checks through an explicit expected-selection boolean; the
new selected branch requires exact canonical project/plugin identity and both
trust booleans true. It additionally requires exactly one selected skill with
the authored name/description, `userInvocable=true`, disabled absent or literal
false, and exact source `{type: "plugin", plugin_name: <name>, path:
<receipt-selected installed SKILL.md>}`. Reject folded identity aliases,
duplicates, foreign paths and a trust-only plugin entry without that skill.

Use the same real `native-inspect` command, cwd, deadline and held source
authority, with an explicit selected-mode call. Add one finite Grok-only
`fixture_selection` marker before inspection. Record completed selected
`native_inspection` only after the exact raw check passes. The existing Python
reader gains only a strict optional marker branch: join completed observation
to raw inspection, successful exact inspect command/cwd, binding/profile/target
and installed skill. Preserve failure-before-inspection capture and old
unmarked archives, the closed members and every failed/all-false predicate.
Do not add an artifact slot or infer load from plugin `enabled` alone.

The strict provider is unchanged. Loaded skills may change its next request;
an unrecognized shape is refused and captured under existing failed-only
diagnostics. No success shape, extra response, retry or protocol allowance is
introduced before genuine fresh raw review.

### Bounded code and validation surface

Change only Grok title recipe/whole-map expectation, Grok inspection helpers,
the shared attempt's Grok-only call, new focused Grok selection contracts,
the runtime reader and focused reader contracts. Existing turn-summary
contracts need only the complete raw recipe and protected count2 expectations,
plus creation of the now-required ordinary project in their inert constructors.
Putting `[plugins]` before `[features]` preserves their duplicate-key and
extra-feature negative controls. Preserve provider/title handlers, duplicate,
privacy, environment, production adapters, workflow, pins and all deadlines.

Run meaningful inert Grok/shared Go and Python runtime contracts, gofmt,
affected all-enabled and complete six-linter gates. Replay actual old archives
through complete old/current readers with original source identities. Obtain
two independent exact CODE reviews before root publication and the next genuine
disposable hosted run. No vendor, public installer or scanner runs locally.

### Pinned primary source guidance

All publisher paths below are under `xai-org/grok-build` tree
`2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8`, prefix `crates/codegen/`.
The snapshot is not proven equivalent to shipping1.0.70/f50904eaee6c.

| Path | Git blob | Relevant contract |
| --- | --- | --- |
| `xai-grok-permission-rules/src/trust.rs` | `9411e22ab04953bee3d9d68558e6b1bf0d896ab5` | Original-HOME exact folder TOML; optional timestamp; broad-root refusal |
| `xai-grok-workspace/src/plugins.rs` | `9cd0d8aef4b2336635db5627b3e77431b5378c45` | User config enabled names or IDs |
| `xai-grok-agent/src/plugins/discovery.rs` | `96b8caae336c6ae7a210141922e1454dacec7817` | Project trust; unlisted plugins disabled |
| `xai-grok-agent/src/plugins/registry.rs` | `31112af7aac163927f63e6c61f2e21268e5359f8` | Active requires enabled and trusted |
| `xai-grok-shell/src/inspect/mod.rs` | `bbbe3da44a5c73046ac0b514e3c124283f865afa` | Plugin enabled means trust; separate loaded skill source; camelCase userInvocable; false disabled omitted |
| `xai-grok-agent/src/prompt/skills.rs` | `d70dcb18f5845ced0d1129d490773703bcf0eac1` | Only active plugin skills get exact source stamp |
| `xai-grok-tools/src/types/config_source.rs` | `11f16ad8aee5243aa71c1697ac9c0f2a1645463a` | Plugin source serializes plugin_name and path |

The separate native `plugin enable` command is unsuitable for this prepared
directory: `xai-grok-pager/src/plugin_cmd.rs` blob
`36b53be9bc0e22eed45bc437573595072289548a`, lines610-622, requires native
install-registry membership. No such registration is invented.

### Implemented selection diagnostic validation

Two independent Sol/xhigh CODE reviews and the root review accepted the frozen
nine-file candidate. Focused Go contracts passed (62 top-level runs, 271 pass
events; one existing child-only helper skip), all 61 Python contracts passed,
and affected all-enabled plus complete six-linter gates reported zero issues.
The full original 91402 runtime archive replays identically through the original
and current readers (25 members and the complete failed/all-false result).
The Grok-only reader branch also has explicit unchanged-result coverage for
Qoder, Kiro and Windsurf. No vendor executable ran locally. These are source
and inert-contract results; fresh genuine hosted evidence remains required for
selected-skill observation and subsequent complete lifecycle qualification.

### Grok complete supported skills lifecycle (2026-10-28)

Real hosted run40717447621 at f794d6d1 now confirms the exact selected loaded
skill/source/description/userInvocable serialization and canonical path; native
inspect/startup exit0, main/title each1request/response and ordinary shutdown.
Its only test failure is the existing independent-raw-review barrier; all
qualification flags remain false. Native artifact12834025060 outerSHA256
bfdef920511b1c1c6200a349b2c4d86cbca85c053ccdbf203400b2010150e658 and inner
49fee32d67edd1f59297025f96a0bcfc6928abc3a03660ce775adbda2fd48b00 retain
the exact source/pins and25closed members. Root independently audits the raw.

Root and two independent Sol/xhigh reviewers accepted one bounded skills-only
cycle: qualify only project-lifecycle/native-restart-removal; project MCP,
vendor authentication, real models and skill execution stay unsupported or
unclaimed. Persisted Grok bindings stay Prepared/package_validated. Reuse the
existing hosted admission, original HOME, pins/scanner/process supervision,
150s work/180s total origins,30s commands and closed archive bounds.

Before any child, author/freeze original HOME config with supported exact
A/B/user enabled names and disabled foreign A/B names; trust only canonical
owned A+B. Hold independent original-user and foreign A/B native plugin/skill
bytes, inode/owner/mode/membership and caller data through every operation.
Distinct local A/B skills carry nonce/revisions; A-one updates to A-two at the
same version. Direct local repair keeps A-two frozen and requires its recorded
digest (repair_multi.go and installer/package.go), never invented remote pins.

Exactly14 public commands cover3Git roots/version, addA/B, repeatadd,
unchangedupdate, intactrepair, same-versionupdate, missingpackagerepair, removeA,
repeatremoveA and removeB. The public CLI repair_many/RepairGroup route restages intact repair
(mutated=true: lifecycle.go75-105, repair_multi.go321-328 and group_plan.go162);
missing exact owned-package repair restores the recorded bytes (mutated=true).
Actual public inert execution confirms partial-leaf absence refuses unchanged,
while whole package absence repairs. Validate the sole managed_package_directory
receipt and derived A/.grok/plugins/<physicalID> plus closed source/native bytes.
Nonrecursively remove only plugin.json, skills/<name>/SKILL.md,
.grok-plugin/plugin.json and their known empty directories, ending at that exact
active root; refuse any extra entry, never RemoveAll. Keep state/source and
parent/foreign/User/config/data authority, then require exact active-path absence.
Installed skill-only packages have zero data receipts; default final-binding
removal creates real retained owned data/marker/receipt, validated from
remove_prepare.go and PluginDataManager and preserved afterward.

Exactly20 fresh `inspect --json` processes cover outside/A/B initial, installed,
stable, updated, damaged, repaired and removed/repeated/final states. Require
whole exact plugin/skill catalogs, authored description plus exact source path,
invocable/disabled semantics and unchanged User/foreign controls. Plugin.enabled
means trust only. Damaged A lacks its selected plugin and skill; final A/B/outside lack both
managed sources. No provider or extra native activation command is needed.

Exactly4members contain summary/public/inspect/proof JSONL with bounded raw
bytes/hash/base64 records and source/state/binding/ownership/removal joins.
Apply the existing Grok privacy scanner before encoding and after strict
decoding (no provider exists to trigger the old conditional keep scanner).
Failed actual prefixes and every late failure keep all flagsfalse; only full
closure plus ordinary cleanup/preservation and actual test exit0 can qualify
the two applicable flags. Retain old diagnostic evidence/readers unchanged;
no artificial transition review fatal, new framework, workflow or client pin.
Obtain exact independent CODE reviews and focused inert Go/Python plus affected
and complete six-linter gates, then one genuine hosted full-cycle capture and
independent raw audit. No vendor executable runs locally.
