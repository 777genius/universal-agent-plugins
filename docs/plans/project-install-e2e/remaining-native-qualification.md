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
Original-home settings set `security.auth.selectedType='gemini-api-key'`,
`privacy.usageStatisticsEnabled=false`,
`telemetry.enabled=false` and
`experimental.adk.agentSessionNoninteractiveEnabled=false`, selecting the pinned
legacy headless loop. Set `GOOGLE_GEMINI_BASE_URL` to the fresh literal loopback
origin and `GOOGLE_GENAI_API_VERSION=v1beta`. Supply a fresh fixture-owned
`GEMINI_API_KEY` required only by this loopback provider; compare its exact
`x-goog-api-key` header privately and redact the key from all admitted artifacts.
The explicit API-key type passes the pinned native auth validator and still
uses the configured custom endpoint. The explicit fixture key also avoids
`loadApiKey()` and accidental keychain probing. This is test-provider
authentication, never vendor Google authentication/OAuth or an inherited account.

Genuine [runtime run 40349097306](https://github.com/777genius/universal-agent-plugins/actions/runs/40349097306),
native artifact `12668685097` (ZIP SHA256
`311209db69ad0ebb6f59f64bcd3c918019a5e1167aad2f8cb4dc8d5e95f8e870`),
failed before any provider request with `Invalid auth method selected.` The exact
source commit `6006c14a4decfe4b891c6263ad6ad5992e404b5b` selects GATEWAY from
`GOOGLE_GEMINI_BASE_URL`, but its CLI auth validator does not admit that type.
This amendment selects the existing API-key type in original-home settings;
the fixture key, loopback endpoint, native argv, normal auth validation, trust,
tool policy, acquisition and budgets remain the reviewed contract. It does not
enable external authentication or qualify the unexecuted provider/tool stages.
Pinned source blobs are `packages/core/src/core/contentGenerator.ts`
`f44ec6d2458473360db05f3150589cbd192c4a0e`,
`packages/cli/src/validateNonInterActiveAuth.ts`
`a15f4f83a20b515e98e69eee3786a22b48441408`, `config/auth.ts`
`1ca07f98eb4c588b2b0bef0d627bc9b373bd6e7f` and `config/settingsSchema.ts`
`f85279cdc3d78b2033a6e48f92eaf02febf1eaa0` (the last two under
`packages/cli/src`). Implement only after two independent plan reviews.

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

## Kimi installed local lifecycle amendment

Genuine Kimi API run 40238495966 at `ac3a7e5da565590fd85c4ff9795cae6b312ed6b1`,
artifact 12615878095 (ZIP SHA256
`cb14eec0bd7fc5040a43d4314255055a59ad1729a24dacb56ffcc894607381ef`),
confirmed the pinned 2.3.2 native bearer, A/B trust and separate A/B/C/user
catalogs with named initialize/tools-list and untrusted C refusal. Its
tools-call count was zero. Admit only exact product versions `{2.3.1, 2.3.2}`;
all existing executable, original-user and nearest-Git checks remain intact.

Select `TestAgentpluginsKimiNativeProjectComponentsLifecycle`, opt-in
`AGENTPLUGINS_KIMI_PROJECT_NATIVE_E2E=1`. Public UAP installs independent local
user/A/B/C packages. Seven fresh authenticated native web processes observe the
empty, installed, unchanged, updated, missing-repaired, restarted and removed
stages. Native API trust authorizes only A/B during the empty baseline; C stays
untrusted. Receipt-selected skills and independently reconstructed exact
stdio command/argv/environment/package/data/cwd authority must match the
unchanged installed native files before the API reads them. B/C/user managed
package inventories and their original receipts/native bytes remain exact.

This is the supported imperative local-directory route, with `--no-save` and
no saved dependency agreement. Missing local repair keeps its selected source
at A-two: it proves reconstruction from unchanged selected local input. Moving
Git/ZIP sources and repair from their recorded immutable pin remain separate
required gates, `TestProjectComposedSourceLifecycleSeparatesUserAndProjects`,
`TestProjectKimiComposedSavedLifecycle` and
`TestProjectKimiColdSavedImportPreservesPinnedAgreement`. Do not attribute those
source-resolution results to the native local-directory test or change product
repair semantics to fit a stronger local-source claim.

The native test may satisfy project-lifecycle, native-restart-removal,
mcp-initialize and mcp-tools-list only after the complete sequence and cleanup.
It keeps mcp-tools-call false, activation Prepared, vendor authentication false
and real-model use false. Provider-driven tool calls require their own observed
and independently reviewed amendment. Reuse existing private service capture,
token quarantine/redaction and closed artifact admission; never upload a native
profile or token file. The original total/work/cleanup budgets are 900/840/60
seconds, with no native children across UAP mutations.

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

## Observed Kimi local-source version discovery correction

Genuine Kimi runtime [40352260675](https://github.com/777genius/universal-agent-plugins/actions/runs/40352260675)
and native artifact `12667319096` reached the public local-source `add-a`, then
refused the supported pinned Kimi 2.3.2 executable because the installer passed
an empty client version. Read-only discovery intentionally does not execute
clients; the CLI currently requests version probes only for Directory sources.
The existing Kimi project precondition therefore also rejects direct packages.

Add one narrow CLI helper that requests the existing targeted version probe
only for an explicitly selected, detected Kimi project client whose version is
missing, and only during an actual add/update/repair or dependency mutation.
Reuse the existing detector, its timeout and original-user environment. Never
fall back to probing every discovered client, invent a version, relocate HOME,
change the Kimi supported-version guard or promote Prepared to Active. Preserve
already supplied selected-client evidence. Require exactly one successful Kimi
result with the same executable and original-user config/profile authority;
copy only its version, preserving all other original facts and unrelated rows.
Missing, duplicate, failed or changed-host results leave the operation closed.
The baseline retains its 2.3.1 guard; genuine 2.3.2 reruns combine this helper
with the already reviewed version admission in the existing Kimi leaf.

Wire the helper after complete target validation in direct add, update and
repair preflight. For dependencies, allow only selected install/update/repair
actions, validate the complete action target set before the first new probe,
and share the refreshed detected map between preview and actual preparation.
The prompted add path may probe only after selection. This helper introduces
no dry-run or read-only execution; preserve existing explicit `info --target`
version reconciliation. Removal must remain possible from recorded ownership
without the new probe; unchanged/removed dependency actions do not trigger it.
Absent targeted capability leaves the version precondition closed, before
installation mutation; there is no broad-probing fallback.

Use the existing CLI E2E fixtures with a detector that genuinely starts with no
version and records targeted calls. Cover direct and saved install/update/repair,
original-user and sibling-project preservation, removal without a client probe,
dry-run/read-only/unselected-client controls, and failed/mismatched probes with
no target mutation. These are installer/detection tests, not native discovery
qualification. Then rerun the existing genuine Kimi lifecycle and separate
tool-attempt jobs. Two independent plan reviews precede product changes.
## Kimi version startup profile and update environment amendment

Pinned Kimi 2.3.2 source `b8dd3a134060c6e80ba4a1659335cd63b68483f2`
performs staged-update handling, native worker installation and cache cleanup
before Commander prints `--version`. A PATH-only detector probe therefore
cannot establish the selected original-user profile/cache authority. Preserve
the existing targeted detector and its timeouts; do not add a broad probe,
version override or a second process primitive.

Implement Kimi's existing `VersionProbeEnvironment(configRoot)` port with only
`KIMI_CODE_HOME=<selected original-user ConfigRoot>`,
`KIMI_CODE_CACHE_DIR=<that exact ConfigRoot>/cache` and literal
`KIMI_CODE_NO_AUTO_UPDATE=1`. The existing detector may append its normal PATH.
Neither the project root nor a derived parent becomes HOME or USERPROFILE.
Do not inherit loader, proxy, authentication, account or shell variables.
Require an absolute, clean original-user root, canonical ordinary directory
ancestors and an absent or ordinary directory leaf; aliases, non-directories
or otherwise unrepresentable roots fail before a child. A fresh missing native
profile remains admissible when its canonical parent is valid. The cache path
is a fixed child of that exact selected user profile, never the project registry
or an ambient cache. Apply the same checks to the fixed cache child; an existing
alias or special leaf fails and an absent cache is allowed. Reuse the existing
path-policy checks.

Apply the same literal automatic-update opt-out before the first Kimi vendor
child in the owned Python collectors and Go lifecycle/tool-attempt environments.
Keep their already controlled original HOME, selected profile, acquisition,
runtime-file identities, private-token quarantine, capture and total deadlines.
The profile/cache variables above bind the detector's otherwise isolated version
probe; they do not relocate the collectors' independently controlled user home.
Do not expand any inherited environment allowlist beyond these exact supported
Kimi controls or copy a provider credential into version/help children.

Native startup may write its selected cache. Keep that activity distinct from
installer ownership and preserve native configuration, foreign settings/skills,
other projects and retained plugin data. Continue the hosted fixture's immutable
CLI checks before/after observation. `KIMI_CODE_NO_AUTO_UPDATE` suppresses
automatic checks/install/swap; it does not suppress native-cache bootstrap,
executable-sibling staging cleanup or a previously manually requested stage.
Do not claim manual-stage suppression or qualify changed executable bytes.

The exact source blobs are `apps/kimi-code/src/main.ts`
`f95d57c2a2867f0163c75c4559ad29b555f4c498`, `cli/commands.ts`
`3ec65112fca524d109618433aced6bc7a1f1ce75`, `native/native-assets.ts`
`51db1af9fc00e3b177cfbebd310d58a0c0504232`, `utils/paths.ts`
`70ce8f427706cde5413cd8df44623eb80d12d50d`, `cli/update/preflight.ts`
`9d6d1f17249bc3f4b1293f5eb8f9782c1691fe0d` and `cli/update/native-swap.ts`
`9b477efababa0137f614801a7c2b2c27059c05e6` (all under
`apps/kimi-code/src` after the first path), together with
`packages/agent-core-v2/src/app/bootstrap/bootstrap.ts`
`3e969174f868af9d13aff1077927d87fdcd7e62d`. Inert contracts require exact
profile/cache values, literal updater opt-out, rejection of unsafe roots and
absence of inherited canaries; do not execute a vendor locally. Two independent
plan reviews precede this adapter and leaf implementation. Genuine hosted
lifecycle/tool evidence remains required, with all unobserved claims false.

### Kimi bounded response-schema refusal capture

The initial single-tool leaf currently loses the native API response when an
unmeasured response shape raises inside the observer: the existing service
correctly quarantines every failed observation. Preserve that common service
and its quarantine contract. Add only a leaf-local, finite response-schema
refusal boundary after successful readiness and native bearer registration.

Only the key/object and nonidentity container-type checks for the new session,
accepted prompt, pending approval, snapshot and history responses may raise a
dedicated schema-refusal sentinel. Invalid JSON, identity or permission mismatch,
readiness, token, transport, limit, unchanged-file or cleanup errors never enter
this boundary. On the first schema refusal, stop all further session, prompt,
approval and tool actions. Record only a fixed failed-stage code and validated
IDs, never exception text or inferred success. Reserve one of the original
60 HTTP requests for the same exact authenticated `POST /api/v1/shutdown`;
require its existing acknowledgement and mark shutdown within the original
service work deadline. Do not issue an alternate request or reset a budget.

Only the unchanged common service's normal-exit, full-group-reap, closed-port,
complete-capture and deadline checks may admit the failed observation's recorded
HTTP bytes. Failure of any such gate retains quarantine. After that admission,
use the existing closed envelope, token/provider redaction, byte limits and
`native-attempt` stdout member, then exit nonzero so the Go test retains the
bounded evidence and fails. No shared-service, Go success, artifact inventory or
frontend success contract changes. Native capture/history completion and all
qualification, vendor-authentication and model flags remain false.

Inert tests inject extra session, approval and history fields and verify the
first refusal stops later actions, permits only exact shutdown, retains the
bounded response after successful cleanup and exits nonzero. Identity mismatch,
failed shutdown/cleanup and bearer/provider canaries remain quarantined. Two
independent plan-review rounds precede implementation.

## Observed Windsurf / Devin selected-version correction

Genuine [run 40353474164](https://github.com/777genius/universal-agent-plugins/actions/runs/40353474164),
job `121701552623`, native artifact `12670367419`, at source
`8412fdd9ea5947d981caf5106451836246834bcf` / tree
`b1dfbd9c293740005bee8c2c77c801b072192fd3`, printed the pinned
`devin 3000.11.205 (6dea39440ae3)` from the actual executable in 24 ms,
exit zero, under the controlled original HOME/XDG fixture. Public local-source
`add-a` then refused its empty detected version before target mutation. All five
qualification flags, runtime completion, vendor authentication and real-model
qualification remained false. This is an observed selected-version gap, not a
native session or supported authentication boundary.

Retain the exact `3000.11.205` project guard and the canonical selected
`devin`/`devin.exe` executable requirement. A new missing-version mutation probe
may select public `ClientWindsurf` only when that explicit target's primary
executable has that basename, using the same case treatment as the existing
adapter guard. Preserve legacy Windsurf-first detection/channel ordering.
Do not choose a secondary Devin surface or reinterpret a legacy Windsurf/editor
primary. `ConfigRoot` remains the selected legacy Cascade authority, including
its intentional empty value for a current-Devin-only installation.

Port the already reviewed Kimi selected-mutation helper and its existing
add/update/repair/dependency wiring into aligned Windsurf source
`5a9f620c9b0b720b16fbb4da8ac0dab367d30762`; extend only the finite eligible
client set to Kimi and the primary-Devin Windsurf case above. Before the first
new probe validate the complete selected/action set. Request only eligible
selected clients with missing versions through the existing targeted detector;
never fall back to broad execution. Require exactly one successful result for
each requested client ID, with unchanged executable, ConfigRoot, profile
namespace and authority. Validate every result before copying only Version into
the original detected map; a failed/missing/duplicate/mismatched result changes
no original row. Preserve unrelated rows and already supplied versions.

Keep the same user-scope, dry-run, unselected-client, unchanged/removed
dependency-action and removal exclusions. Preserve existing explicit
`info --target` reconciliation and the Skills preview/Check boundary. An absent
targeted capability leaves the existing adapter version guard closed. The SDK
continues accepting only already observed host metadata; its Discover/Prepare
paths do not gain native execution.

Windsurf's legacy/empty ConfigRoot cannot represent Devin's user profile through
`VersionProbeEnvironment(configRoot)`. Add one narrowly scoped environment case
inside the existing detector's version path for a selected primary Devin
executable, using its existing original `Detector.HomeDir`, `Environment`, GOOS
and filesystem probes. Do not add another public probe interface or alter
Windsurf's detected ConfigRoot/profile/selection facts. The exact allowed values
are `HOME=<original HomeDir>`; on Windows additionally
`USERPROFILE=<that same original HomeDir>`, plus explicit original `APPDATA`
and `LOCALAPPDATA` when present; on Linux/macOS additionally only nonempty
explicit original `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME` and
`XDG_CACHE_HOME` when present. Omitted/empty optional roots stay absent so the
vendor retains its normal defaults. Require Windows APPDATA to be representable
before probing, since the pinned docs identify it as user configuration
authority. The existing OS probe may append its ordinary PATH.

HomeDir must be an existing ordinary directory with clean absolute UTF-8
spelling, no aliases and no NUL/CR/LF. Every forwarded optional root must
also be clean, absolute and canonical, with ordinary existing ancestors and an
ordinary or absent final directory leaf whose immediate parent exists. Validate
through the detector's existing injected filesystem probes; create no profile,
cache or config. Invalid original facts fail before the child rather than
falling back to another home. Never derive HOME/XDG from ConfigRoot, the project,
its registry or a temporary fake profile. Forward no other ambient values,
including credentials, provider/model/permission settings, proxies, loader or
shell configuration. Kimi retains its already reviewed exact profile/cache
environment port unchanged.

Keep the native argv exactly `--version`, existing output limit and timeout,
private version-probe cwd and synchronous process cleanup. Do not invent a
Devin updater environment variable, add `--config`, rewrite native user config
or grant permissions. The genuine fixture's existing user-authored
`auto_update:false` config remains at its original selected HOME/XDG path;
its deny-only permission config and runtime hashes/deadlines remain required.
Version startup/cache behavior is not inferred from fast exit, and a changed
native runtime cannot retain the old qualification pin.

The reviewed archive SHA256 is
`2e65c8b93682258fc91f5e4cead92e3698338cebf1b31dbc32a3201cb63705c4`,
73,679,375 bytes, from the already pinned official 3000.11.205 URL. Its bundled
`share/devin/docs/reference/commands.mdx` SHA256
`996f969ba371f4dae62d70b63f7a6d5473ed48c3633abf9b120d9214cf0c8b75`
documents the version/config flags; `reference/configuration/config-file.mdx`
SHA256 `c43f1a70135662c6124ae794928a4719bcd0bcb2d198573c2bf39c76935acd91`
and `global-vs-local.mdx` SHA256
`8fc899cd33ea4185525efb120c8c13736419622ea3cf4b6cae9608b2d2daf1e1`
document original-user config locations, user-only updater preference and
independent project layers. No source-proven profile-inert version fast path or
updater environment opt-out is claimed.

The common product helper/wiring and necessary Kimi profile port must remain
byte-compatible when #524 and #528 combine. Carry only those reviewed product
files/meaningful inert tests, preserving the Kimi profile amendment's exact
contract; do not merge Kimi native collectors, tests, version admissions or
qualification claims into this Windsurf slice. Root aligns shared union code
between client PRs before the combined merge. Inert tests require exact
requested IDs, atomic failed-result preservation, coexistence/legacy-primary
refusal, empty ConfigRoot retention, original HOME/XDG binding and no credential
canaries, plus direct and saved public lifecycle/no-probe controls. Both
independent plan reviews precede implementation. Genuine hosted startup and
later full lifecycle/MCP proof remain separate required gates.

## Observed selected-version architecture and lint correction

Genuine Windsurf run [40375582347](https://github.com/777genius/universal-agent-plugins/actions/runs/40375582347),
`test-core` job `121757629831`, fails the existing architecture ratchet before
native execution: `cli/internal/agentpluginscli` newly names two client identity
constants, and `adapters/clientdetect` names one. The reviewed Kimi/Devin runtime
scope is unchanged, but the dispatch belongs behind the repository's injected
client capability contract. Start from integrated source
`f047ae1162d5320c03eb80fa7f11167ad3e81616`, tree
`c39c6fd3a31f890566eb7b00a912768cf4004f1f`, in isolated
`/tmp/uap-completion-cli-arch`. No product code is changed under this plan until
root R1 and an independent shell/capture R2 approve it.

### Adapter-owned project version requirement

Add one small optional interface to the existing `clients` contract:
`ProjectVersionRequirement`, with pure method
`RequiresProjectVersion(client domain.DetectedClient) bool`. It expresses only
whether that adapter's already selected detected host needs a version before a
project mutation. It performs no filesystem observation or execution and does
not declare a version supported. Kimi implements it with its existing requirement;
Windsurf implements it only for an absolute clean selected primary executable
whose case-normalized basename is `devin` or `devin.exe`. Factor that same pure
primary-executable predicate inside `clients/windsurf` for its existing exact
version admission guard, without changing its accepted version or identity
checks. Legacy Windsurf/editor primaries and secondary Devin surfaces stay
ineligible. No other shipped adapter implements this new capability.

The generic CLI helper looks up this capability through
`clients.As[clients.ProjectVersionRequirement](app.ClientRegistry, id)`, using
the registry already injected by the composition root. Remove the two client-ID
branches and their basename policy from generic CLI production code. Preserve
status/error/executable/missing-version filtering, ordered ID deduplication,
complete target/action validation, one existing targeted detector batch, exact
one-result-per-requested-ID identity checks, validation of all results before
copying only Version, and every already reviewed exclusion. The current default
registry still yields exactly the Kimi + primary-Devin union; this membership
lives in adapter implementations rather than another generic identity table.

Do not construct/import `clients/all` in the library or replace constants with
strings, aliases, hidden callbacks or AST tricks. A missing injected registry
returns the existing `clients.ErrRegistryRequired` before any new child, rather
than assembling a default. A registered adapter without the capability requests
no new version; missing targeted detector capability retains the existing
unknown-version refusal. Known versions remain original facts. The user,
dry-run, read-only, unchanged/removal and Skills preview/Check boundaries remain
as reviewed, and SDK Discover/Prepare gains no process execution.

### Adapter-owned original-host version environment

Preserve the existing `VersionProbeEnvironment(configRoot string)` interface and
all Kimi/Codex/Gemini implementations and semantics. Add a separate narrowly
optional adapter interface `HostVersionProbeEnvironment`, with method
`VersionProbeEnvironmentForHost(host clients.Host, client domain.DetectedClient)
([]string, error)`. This is an adapter environment-policy hook, not another
native execution/probing interface. The existing generic detector already has
the original `clients.Host` used for surface discovery and retains exclusive
ownership of whether/when to probe, argv, process primitive, isolated cwd,
timeouts, output bounds and synchronous cleanup.

After its existing profile-based environment hook, the generic detector may
dispatch the host-based hook by interface. Remove its Windsurf identity branch.
Only `clients/windsurf` implements this new hook. Move the existing Devin
environment builder/validation into that client package and express filesystem
validation through existing `Host.CanonicalDirectory` and `Host.Lstat`, with
HomeDir/GOOS/Env from that same Host. This relocates useful implementation; it
does not retire its capability or replace injected probes with direct OS reads.

For a non-Devin primary this hook returns no custom environment, retaining the
pre-existing legacy version path. For the selected primary Devin it returns
exactly the already reviewed original HOME and finite explicit XDG or Windows
values, including required Windows APPDATA. Preserve canonical spelling,
ordinary existing HOME, no aliases, optional absent final leaf only with an
existing ordinary immediate parent, no creation and no ambient credential,
proxy, loader or shell values. ConfigRoot remains the legacy Cascade authority,
including empty, and never supplies HOME/XDG. Invalid original facts fail before
the child. Retain the legacy callback's refusal when it cannot carry a required
environment; there is no PATH-only fallback. Kimi's exact three-variable
profile/cache/updater environment and its path checks remain unchanged.

### Mechanical lint corrections

Keep both public fixture broad-probe traps counted and deliberately failing.
Have `DetectWithVersionProbe` return their ordinary inert discovery rows alongside
the intentional refusal error instead of an unconditionally nil first result;
no vendor executes and no caller may accept the failed result. Add explicit
compile-time detector-port assertions where useful to bind the required method
signatures. Convert the existing Kimi alias test's value comparisons into an
equivalent tagged switch. Lowercase moved Devin error-message prefixes. No
lint suppressions, fake success, skipped cases or removed safety assertions.

### Files and bounded verification

Expected production changes are the two optional interfaces in
`clients/detection.go`; adapter-owned requirement methods in `clients/kimi` and
`clients/windsurf`; relocated Wind host-environment implementation and shared
primary predicate; generic interface dispatch in
`adapters/clientdetect/detector.go`; and capability lookup in
`cli/internal/agentpluginscli/project_version_preflight.go`. Preserve existing
mutation wiring. Update the meaningful adapter/detector/helper/public fixtures
for the injected registry and moved policy, plus the specified lint-only test
lines. No native collector/workflow or unrelated client behavior changes.

Require the unchanged `TestClientIDBudgetDoesNotGrow`, baseline-current, layer
boundaries and contract dependency closure to pass without editing their code,
baseline or allowlists. Add pure adapter tests and a default-registry contract
that only the two intended adapters request a new project version, with primary
and legacy selection cases. Keep cross-client coexistence, empty ConfigRoot,
atomic failed-result and no-capability/no-registry controls. Preserve original
HOME/XDG/Windows environment and unsafe-path/canary tests through the real
adapter/detector seam. Run the focused helper/preview tests, direct and saved TLS
ZIP public lifecycle controls, and affected-package lint. Existing saved Git
tests remain required and unmodified; an unavailable OSRunner `/proc` capability
is reported without a waiver or full-suite pass claim. All native runtime
qualification continues to require genuine hosted evidence.

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

## Observed Gemini wait-for-previous description correction

Genuine [runtime run 40367376528](https://github.com/777genius/universal-agent-plugins/actions/runs/40367376528),
job `121754582361`, ran source
`6f8ea1173b8b1d343688299b5864ee7c1e1a86d7`, tree
`017dd9814de77e1e7e7dd7b1e750c38b37282b02`. Closed frontend artifact
`12678257392` has ZIP SHA256
`46c65a8622da53758498c28e301a2223941abb4c901c0ae37ff6afbf58c813bb`;
native artifact `12677916698` has ZIP SHA256
`d558be3ebc28741af8e91011e8435cf5a286a2f105b73e0d8608275b9cf99d1f`.
The admitted native attempt reached the scripted loopback provider and retained
MCP `initialize` and `tools/list` records. The provider rejected the first
request with `Gemini native wait_for_previous schema differs`, returning HTTP
400; native exited 144 with no process/context error. No provider response or
tool call completed. Every qualification flag, vendor authentication and
real-model qualification remains false.

The mismatch is the fixture's exact description literal, not the schema's
boolean type or an observed external authentication failure. The current
`geminiProjectWaitDescription` in
`repotests/agentplugins_gemini_project_native_provider_test.go` is truncated to
153 characters, ending `Use true whe.`. Actual `provider-request-1.json` has a
204-character description. It exactly matches the already pinned publisher
source `google-gemini/gemini-cli@6006c14a4decfe4b891c6263ad6ad5992e404b5b`,
`packages/core/src/tools/tools.ts`, Git blob
`ab2870984aba77e77fc5bacb25009cd47698582e`, lines 566–570. The cached primary
source was independently rehashed to that exact Git blob. The full description
bytes, excluding a trailing newline, have SHA256
`7561a43ccbc741215d416fe9298b73e43dc6bddfebdd261af855156fe0994bc9`.

After root R1 and independent capture R2 approval, change only that fixture
constant to the full exact pinned description and add focused strict schema
regressions. Use the observed description bytes as an independently specified
test input rather than deriving every positive fixture from the implementation
constant. Accept this exact native/source description and reject the previous
truncated literal, changed or missing description, wrong property type and
unknown/missing schema fields under the existing exact-key validators. Retain
the current strict `wait_for_previous` object equality; do not allow arbitrary
descriptions, partial/prefix matching or alternate schema shapes. Existing
nonce/operation/marker, required/additionalProperties, tool-name/declaration and
request/response validations remain unchanged.

Start in isolated Gemini source based on local
`8ad5ad06327f0c72b973c2a6a6180e7b11c32b82`, tree
`603d7248054bafbb778a33edf849aa1d48e265d5`, corresponding to current published
head `1df80efe4a988fec76c29a9d73017d3a588b3b38`. Do not alter the root-owned
publishing worktree. This is a fixture literal/contract correction only: no
product adapter, native argv/environment, model, permission, auth, trust,
original-profile baseline, source/digest identity, provider sequence,
process/deadline/cleanup or artifact allowlist changes. The full native request
and private profile stay in closed artifacts; commit no raw profile or full
provider request.

Run the focused inert provider/schema contracts and relevant changed-line lint
without any local vendor execution. Preserve the genuine hosted runtime gate;
this correction does not establish that later response/tool/session/lifecycle
stages pass. Review fresh actual hosted output before any further amendment or
qualification claim.

## Kimi independent source and installed-authority proof before qualification

Independent inspection found a fixture proof gap in Kimi source
`361bd69930f7f641e232a6c60f6a215ddee8e9d4`, tree
`eddbcb5ce8b39bd945dad23c24b449a7468eb948`. Its lifecycle file blob
`740c6f01c0fb7d9261fccd58cd5b6794a52e0c7d` is byte-identical in combined
commits `6dab49e2` and `d0fc5852`. `receipt` currently accepts merely nonempty
source-binding/tree/manifest fields, partially checks only skill/MCP ownership
and ignores the managed-package object. `observe` then admits installed probe
and launcher hashes directly into the observer identity map. The final test can
promote lifecycle/restart/initialize/list flags after those incomplete checks.
This is a static proof gap; it is not evidence that a captured installed binary
was corrupt, and no prior run gains or loses qualification by assertion here.

Strengthen only the Kimi lifecycle fixture and its inert identity contracts.
Retain, for each authored scope/revision before public CLI mutation, immutable
copies of the complete finite source payload, executable bits and directory
membership. User source is the authored manifest and skill; project source also
contains the exact existing trusted probe and authored MCP document. Require a
fresh bounded held-file capture to equal that authored tree, including exact
manifest bytes. Compute expected acquired source identity using the existing
versioned `packagedigest.DigestCaptured` framing and the direct-local source
binding contract; derive full package metadata, version, inventory and manifest
SHA-256 from the frozen authored recipe. Compare the full recorded Source and
Package values, not nonempty fields. Do not derive expected identity from state,
installed content or a later mutable source reread.

Validate each binding against its known scope/original user context. Derive the
physical artifact from its installation identity and fixed authored name, the
client binding from that identity/client/scope/expected locator, and the exact
managed/native paths from the fixture's existing selected roots. Preserve the
first validated installation/binding/artifact/data authority across repeat,
update and repair; changing revision must not reset that authority baseline.
Require precisely the known managed-package object for the user skill package,
and precisely managed package, Kimi project skill and Kimi project MCP objects
for each mixed project package. Compare complete objects: kind, ID, logical
name, exact path, source-relative field, protection class and independently
computed digest. Missing, duplicate, extra or unknown objects refuse.

Before computing an installed-package expectation or preserving any installed
baseline, validate its complete bounded ordinary-file tree against the frozen
source and fixed known projection. Require every unchanged source member's
exact bytes/executable bit and every expected directory; permit only the known
Kimi user manifest or project MCP rewrite, plus the existing managed launcher
and metadata in mixed project packages. Derive those JSON/metadata expectations
from the authored recipe, selected package/data paths and already frozen
installer identity/version. Refuse extra files, directories, aliases, links,
changed source/probe/helper bytes, executable-mode drift or unknown projection
fields. The installed probe must equal the already frozen source probe; the
launcher must equal the already frozen public installer, never its own observed
hash. Reuse the reviewed strict held-entry installed-digest helper from
`829d6a86` with the Go1.22-compatible contract correction `c98c4303` as needed;
it computes the selected `providers.Stager` / `packagesnapshot` framing while
source identity retains its separate versioned framing. Derive skill ownership
from the authored skill tree and MCP ownership through the existing neutral
receipt codec using the independently expected server. Exact receipt equality
remains required; no alternate digest or returned-receipt adoption is allowed.

Only after these proofs may `observe` construct installed probe/launcher
identities, using their already trusted expected hashes. Recheck before every
native stage and before taking unrelated-package immutable baselines. Keep the
existing exact skill comparison, `kimiProjectValidateServer`, original-user/B/C
preservation, fixed native requests, fresh PID/session/nonce/probe events,
shutdown/reap/port gates and final genuine-observation predicates. The four
existing qualification flags may become true only after all these added proofs
and the existing full lifecycle succeed. tools/call, vendor authentication and
real-model flags stay false; no flag is arbitrarily flipped to hide the gap.
No collector argv, environment, trust/approval, process primitive, deadline,
artifact member, envelope or qualification schema changes are authorized.

Use cohesive Kimi-only source/receipt/payload proof seams and the existing
neutral capture/digest APIs. Inert positive parity must stage the fixed user
and mixed project recipes through the actual Kimi planner/Stager and prove the
distinct source/installed framing contracts without executing any vendor or
probe. Negative contracts must refuse nonempty forged source/tree/manifest or
binding facts; unknown/duplicate/missing ownership and wrong paths/digests;
source closure drift; installed probe/launcher/metadata substitution, extra
members, modes and aliases, including corruption before the first baseline.
Tests must prove rejection happens before observer identities or qualification
can be constructed. Existing genuine hosted lifecycle proof remains required.
Gemini and Copilot initial capture flags already remain false; this amendment
does not change them, and the same independent-authority requirement must be
reviewed before any later promotion there. Both independent plan rounds precede
implementation of this Kimi correction.

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

## Gemini owned-MCP listing diagnostic amendment

Accepted plan SHA256 `6c696cd8309f6da3ee9059ef887b14bb8358ed5e4fd57afe199a69b2655b3d5d`
received independent root R1 and Kimi-owner R2 before implementation. Exact
starting source is ef8c42bc5012f68d042b696edbf2801730246e9d/tree462634b2,
which retains the previously reviewed source-bound assessment-cache closure.

Genuine run40583616867/job122469600501 now retains the completed add-user
ownership proof, then fails only its listing-stderr guard at installed-user.
Native12770589174 outer SHA256
`ece142caacab9c75bec59e590717ea41818c84bccb76a754fd95505d446c3c9b`,
frontend12770514390 outer SHA256
`6706bf1185f132eb27cfa98bae98e23cb0e67023c005465e7bbf8e510da824c6`
retain closed24/25 members,24 frontend and34 nested stream joins. The next
listing exits0 in2263ms with exactly four expected skill rows. Its347-byte
stderr SHA256 is
`76222dc470d4288bb95b6a1c18044afad792af771c28d2018ecaa43b2d64e5a0`: two
own-server notification messages precede the existing MCP refresh tuple. Two
separate retained initialize/tools-list probe records share the held user nonce,
fresh PID/session and command interval; no call occurs. All qualifications stay
false and the incomplete provider-stage cleanup remains failed. No log text
becomes a qualification or complete discovery/RPC proof.

At pinned Gemini source6006c14a, mcp-client blob3d2c7beb synchronously logs the
exact contiguous registration/tools:{} missing-listChanged pair. Manager
blob04563803 awaits configured discoveries then emits the exact refresh suffix.
The unchanged probe declares only tools:{} and sends no dynamic notifications.
The listing helper now accepts only finite whole tuples for currently held
advertised user/selected-project recipes, at most two pairs in either pair order,
and the exact old prefix/suffix. Invalid/nil/duplicate/unknown/removed/unproved
recipes, other-project and impossible A+B sets refuse before candidate creation;
a nil/empty slice remains a legitimate baseline. Entire stderr is bounded1024B.
Legacy three tuples remain accepted, with no normalization/prefix/regex warning
allowance. A pinned actual347-byte/SHA golden and hostile scope/set/framing/error
controls verify this private parser.

Only listing diagnostics and inert contracts change. Existing complete skill
catalog equality, pinned builtin hashes, event nonce/PID/session/time/method
checks, no-call listing contract, live ownership/config/state/cache closure,
original native stderr, provider, permission/trust/authentication, process,
cleanup, acquisition and deadline gates remain mandatory. Existing Python mode,
raw schemas and deliberate final failed-only capture barrier remain unchanged.
No vendor/client/scanner/public installer executes locally. Independent exact
implementation review and coordinated meaningful gates precede root publication;
only a fresh genuine capture can establish progress through later stages.

Implementation review: root and Kimi-owner independently returned CODE GO for
exact staged treea66b0191, reading all three changed paths and the original
listing/advertised/event call chain. The actual all-enabled affected root gate
then identified one prealloc suggestion in the new invalid-recipe test setup.
The test now allocates capacity four and keeps the identical input sets and
cases; the runtime diagnostic helper is unchanged. Final reviewed source will
retain this mechanical test-only delta with its separate preservation review.

Final local validation: focused Gemini/shared runtime Go contracts PASS1.424s;
existing Gemini/current-runtime Python admission contracts62PASS9.998s. Actual
all-enabled full-root affected lint versus a412ba5e and unfiltered whole-root
revive/funlen/gocyclo/gocognit/dupl/depguard both report0issues. Gofmt and full-base
diffcheck are clean. The unchanged current ef8 24-member ZIP still admits only
its genuine failed/allfalse capture, and its retained347-byte stderr joins the
independent primary contract's exact SHA. No vendor/native/scanner/public
installer executes locally, and no complete lifecycle or qualification is
promoted. Root publication and a new genuine capture remain required.


# Kimi: exact model tool-result framing and finite native single-call completion

PRE-CODE plan. No source implementation before root and a second independent reviewer accept this exact plan. Root owns Git/GitHub mutations and publication. All genuine native/client/public-installer/scanner execution remains exclusively on an actual current-source GitHub-hosted runner. Local work consists of inert contracts and pure byte readers. Preserve the unpublished alternative26fdd4279d30cebc74811e84cfe9c9ba6e96ade1. No VS Code, other client, release, profile substitution, plugin authority expansion or native pin change.

## 1. Actual source-bound evidence and remaining failure

Current PR524 head is9e9779f2352101e8002070ac64ac30d24e2f9be5, treea77b1de54dfbc16522f603265a74954615be2bc4. Exact hosted run40586437481 is completed: component job122491615607 succeeds; single-call job122491615779 fails; inherited Cline job122491615841 succeeds. No pending run is cancelled, retried or superseded.

Component native12775170868:34578bytes, outerSHA256 fdd801ded762604e8c4bf6525c8c6d29619f29d4007479fb096e3fbcd341d3e0, inner7a8bd821a88209192b9fb7ca7123c54d3c0cda6a2afcafa1d6b635ee60f2ab06,67members. Component frontend12775275513:3417bytes, outerea310853ef8f10cdab4992c922d58c1baf38ca7f3262740b3c236a890bcec1de, inner339fe3375ce709b0bf54785854e051150518231c3801f53648e38876484d10ff,25members. The independent audit is `/tmp/uap-kimi-9e-current/component-raw-audit.json`, SHA256 ed3dce6833e381d5555e58ae696da7ce60aefd1604818cece4aefef51aa8e6e2. It verifies24frontend stream,14nested service stream and126HTTP response joins;22public commands;7fresh native server IDs;11distinct probe sessions with11initialize+11tools/list and0tools/call; all seven native services acknowledge shutdown, reap groups, close ports, with no SIGTERM/forced kill. Actual runtime36.951seconds. The component legitimately reports project-lifecycle/native-restart-removal/mcp-initialize/mcp-tools-list true; tools-call/auth/model false. Preserve this evidence and unchanged component behavior.

Single-call native12775780239:21365bytes, outerb1a896e2c0f47d4ff8351544c542f65f6dc02be0a74ae9fa8a58a7ab59a3c0c9, innerb50e9656245696306a6ba8352e7de8ec99ed6c76e7a2b47a05a84f7fde66649d,25members. Frontend12775087398:3546bytes, outer7a93803301bff7ad71b9260d23878dc89f0a61f858fabfe181ffcfc094ec7311, inner8ac7d7ce9932ea76293d57f285360aedf9983b5d5f9c6d797089e55daf1625a3,25members. Independent audit `/tmp/uap-kimi-9e-current/single-raw-audit.json`, SHA256 ee193ed9cdf2983a32508fcca75a9211bfa82ddf0ae6f7e7323992cae9aa01e1, verifies24frontend+2nested service+27HTTP joins and all30closed nested files. Public commands and native collector all exit0, no process/deadline error. Native service closes ordinarily as above.

Actual approval poll0 contains exactly one matching session/main-agent/turn0/provider call/tool/action/input, then one approval resolves true. The three raw probe events now are initialize, tools/list, tools/call, correlated to one PID/session/nonce and exact selected managed root/data/runtime facts. Provider requests1 and2 retain a stable cache key equal to the actual native session. Request2 has exactly the issued assistant call and the matching tool result. Its tool-message content is exactly one line `Wall time: 0.004 seconds`, LF, then the probe facts JSON. Current `kimiSingleToolAttempt.validateFacts` directly calls whole-document `projectNativeRuntimeJSON` on this content; that fails before facts/returned are recorded. An independent pure JSON replay reproduces the refusal.

Native history independently contains matching tool_use and raw unframed tool_result JSON, then an empty assistant message. The two snapshots report `last_turn_reason=failed`, and the last has no in-flight turn. This actual capture remains failed and unqualified after every correction in this plan. It does not contain the final success sentinel or an independently reached persistent-marker/unrelated-data check. Current code also intentionally calls t.Error after provider success pending native-history review. Both barriers need a strict reviewed success contract, not removal without replacement.

## 2. Authoritative pinned source, independently read without execution

The native2.3.2 archive/executable pins remain4715ccd1a591b64683929a14b26b3d07ac4c484e4da3881b22a2ccca56304042 /1cc5979fca0ea8c01c0a4d53806df8e65573cfbe7daf17ccff86db392807e4fa. GitHub tag `@moonshot-ai/kimi-code@2.3.2` resolves to annotated tag7aa8fa1eab3557ffac521a2139e4dd6583f70f58, which points to source commitb8dd3a134060c6e80ba4a1659335cd63b68483f2. Both API records are preserved under `/tmp/uap-kimi-9e-current/`. The complete nontruncated5346-entry primary Git tree and individually fetched blobs were verified using Git blob SHA1 plus byte length/SHA256. No vendor code or release payload was run locally.

Primary source files are under `/tmp/uap-kimi-9e-current/native-source/`; full21-file manifest SHA256 b82f252f58f2949485583fb428e8e819a5621ad7771cb751ff735cc8b242e288. Relevant frozen blobs:

| Primary path under source commitb8dd | Git blob | Specific contract |
| --- | --- | --- |
| packages/agent-core-v2/src/agent/contextMemory/toolResultRender.ts |2bf13ed32a098c16c9b8a4094c49f93e569f58d0 | MCP tools render wall time; header is `Wall time: ${(durationMs / 1000).toFixed(3)} seconds`, then LF and single text output. Notes/errors are separate rendering and are not accepted as successful facts. |
| packages/agent-core-v2/src/agent/contextMemory/loopEventFold.ts |f05f9a438ca699e4a0a43e2f3bc85c11895a67fa | Raw tool output is retained with toolCallId and duration; wall time applies to MCP names. |
| packages/agent-core-v2/src/human/llm/requester/bases/openai/format.ts |d074e9c1fb310d78f3dd1f497f28bb4481e2ed75 | OpenAI tool messages and existing request metadata grammar. |
| packages/kap-server/src/protocol/message.ts |7bd4b36116fd5c2202032171a40f2f99068cb62c | Message identity, session, role, timestamp and discriminated text/tool_use/tool_result content shapes. |
| packages/kap-server/src/protocol/session.ts |f408383451b5a5e4c3df563853ff89e4352cb278 | Session/workspace/config/usage; last_turn_reason enumerates completed/cancelled/failed. |
| packages/kap-server/src/protocol/rest-snapshot.ts |f8d5df7561f0462aabef890f0dd17b3be6042134 | Snapshot watermark/epoch/session/messages/in-flight turn/pending approvals/questions/subagents. |
| packages/kap-server/src/services/messages/messageProjection.ts |86e1f20a027194dd9d652c589373f9dd19625240 | Native tool_result carries raw output, call ID and optional is_error; assistant tool_use carries exact call ID/name/input. |
| packages/kap-server/src/services/messages/messageHistory.ts |0e99d0ffe5f000abfe352f07bd01173243468a22 | REST list is newest-first; bounded pagination has_more. |
| packages/kap-server/src/routes/snapshot.ts |26ce5a746e44e1c762bdd04f38c039b533d2ca91 | Snapshot messages are original oldest-first; in-flight null is a separate terminal condition. |
| packages/kap-server/src/routes/sessions.ts |8126c532406a62354c0fdc4eb278083824dcd3f9 | Session metadata.cwd and live busy/main-turn/pending/reason projection. |
| packages/agent-core-v2/src/features/dateChange/dateChangeService.ts |d52e17741c0ec2d563f995527d4a5481a113ca75 | Only the observed first-session date injection text and finite disclosure metadata. |
| packages/agent-core-v2/src/features/reminder/systemReminder.ts |2272a9962c883e62ec869dd8c63bdee0eb6d1f31 | Exact `<system-reminder>`, LF, content, LF, closing tag. |

Source is supporting evidence for a narrowly predicted success shape. It does not make the unsuccessful current runtime complete. Any future new shape remains refused with truthful failed evidence; do not broaden based on an error alone.

## 3. Exact correction: model-only wall-time framing

Introduce a pure Go helper for the model-visible returned tool text. It receives the already request/call-owned second-turn tool content and validates the existing whole1MiB bound before parsing. Require exactly one anchored header `Wall time: D.DDD seconds` followed by LF, where the integer portion is canonical unsigned decimal (no signs/leading zeroes except0), exactly three fractional digits, and numeric seconds is finite in[0,60]. The60second maximum is the unchanged native service's original total bound, not a new timeout.

Remove exactly that one header and pass the remaining whole body to the existing duplicate-rejecting, finite, UTF8, depth/node/size-bounded JSON reader. Do not search for a brace, trim arbitrary native prefixes, split on a last line, accept a suffix/note/error block, ignore unknown fields or fall back to another protocol. An unframed model tool response is refused by this new model-text helper. Existing raw-facts validation remains unframed for the independent probe/native-history JSON path; this separates two source-defined serializations of one owned result.

After framing, retain every current exact facts field, nonce/revision/marker/argv/cwd/managed-root/data check and the full fresh probe PID/session/nonce/time/order/count join. Keep the provider's issued-before-write ownership, exact route/host/single credential header, fixed model/tool declaration, request limit, stable canonical cache key, single call/result counts and two-turn sequencing. The actual current second-turn facts payload should pass this pure framing/facts portion, while the overall failed history still refuses completion.

## 4. Exact correction: independent native-history verification in Go

Keep `kimi_attempt.py` as the bounded capture-only observer: no additional native HTTP command, model request, approval, tool authority, profile change or polling budget. Its inner observations remain explicitly `native_capture_complete=false` and `native_history_grammar_frozen=false`; they describe capture without qualification. Add a pure Go history verifier over the emitted raw envelope in the source-selected test. Do not trust a new Python boolean or a caller-supplied expected native output.

The verifier takes expectations from the already independently validated fixture/binding/provider/probe: project A cwd/workspace, actual selected session/prompt/approval IDs, turn0, own provider call/tool/arguments, exact prompt text, expected final sentinel, exact unframed probe facts and original time window. It must:

1. Strictly decode the whole version1 envelope with exactly schema_version/files. Admit only canonical single ordinary member names. Require research.json, one exact native command stdout/stderr pair and every recorded HTTP response; reject missing/extra names, duplicate labels/filenames, unsafe names, oversized bodies and aggregate/nodes/depth excess. Verify UTF8 byte length and SHA256 of all2service streams and allHTTP bodies against research receipts. Preserve the1MiB whole outer envelope cap and current fixed maximum60HTTP requests. Approvals are a sequential prefix0..19 and snapshots0..9; no gaps or renewed budgets. Other labels/methods/paths/auth/status use the exact fixed source inventory. The successful native command argv/cwd/version and all five ordinary service cleanup booleans must match existing contracts.
2. Bind authenticated session-specific raw create-session/prompt/approval/snapshot/history receipts to the selected actual native session, workspace A cwd, own provider call and turn0. Keep prior workspace/trust/catalog checks unchanged. Require exactly one owned pending approval, one once-resolution, no remembered/blanket permission, no extra native prompt or approval action. Verify endpoint methods/paths/status/auth and exact success/error envelopes; ordinary unauthenticated health and the existing401negative case remain distinct.
3. Validate finite known session/snapshot fields rather than arbitrary maps: bounded IDs/timestamps, selected workspace/cwd/model, empty permission rules, archived=false; typed finite nonnegative usage/message-count/sequence data; known optional fields only. In all snapshots require no pending approvals/questions or subagents. Bound in-flight turn to selected turn0 and the source's finite tool/assistant/thinking shapes. Any nonempty running tool must be the single owned tool/call/args; no additional tool, thinking, free assistant text or delegated activity. Preserve and reject failed/cancelled terminal reasons. The final snapshot must exist within the original10poll bound, have in_flight_turn=null, busy=false, main_turn_active=false, pending_interaction=none and last_turn_reason=completed. Its epoch is nonempty/stable and sequence watermark is nondecreasing across snapshots.
4. Require bounded complete history with has_more=false and exactly four or five messages. Normalize only the documented newest-first history order, then require exact message identity/content equality with the oldest-first terminal snapshot's complete messages, also has_more=false. All IDs unique/bounded, same actual session; timestamps finite/nondecreasing within the original run window. No loose scan for a matching substring or arbitrary extra messages.
5. The chronological grammar is: exactly the accepted user prompt; optionally exactly one first-session date injection; exactly one assistant tool_use; exactly one successful tool_result; exactly one final assistant text sentinel. The optional injected date row has the observed user role, one exact system-reminder text part, origin.kind=injection/variant=date_change and closed disclosure(kind=date,renderGeneration=1,valid localDate, timeZone=UTC). Bind date text to the disclosure and UTC message timestamp; other date templates/metadata/messages are refused. The prompt has only the observed user origin metadata; tool/final assistant rows do not acquire arbitrary optional metadata. Other source protocol optional fields are rejected when not part of this exact fixture grammar.
6. The assistant tool_use must have only the owned call ID/name and exact nonce/write/marker arguments. The tool_result must have that same call ID, no error (is_error absent or false), and a string containing only one whole strict raw facts JSON object. Compare it structurally with the already independently validated unframed provider/probe facts, including PID/session/nonce and runtime authority. No tools/call count beyond one. The final assistant must contain exactly one text part equal to the own fixed final sentinel; empty/extra/partial assistant, wrong sentinel or a tool result alone fails.

Return a closed typed proof containing selected session/prompt/call/approval/turn, terminal reason, message/snapshot counts, exact final snapshot/history digests and matching probe identity. Preserve raw evidence; do not serialize private bearer/provider secrets. The current9e raw history MUST fail because its reason is failed and final assistant empty, even though framing/probe/result joins now pass. The verifier may admit a clearly labeled synthetic success contract; that is not native qualification.

## 5. Completion, cleanup and truthful artifact admission

Run native-history verification only after the native collector returns ordinarily and the provider has issued and validated exactly two owned completion requests with the full probe facts. Require the persistent native marker to equal the expected marker and unchanged remainder of the original data inventory, plus the existing immutable/original/foreign authority checks. Those later current checks remain essential; do not infer marker state from returned facts.

Replace the unconditional initial-history t.Error only with all of these completed checks. Add bounded ordinary provider shutdown inside the original deadline, with a finite exported cleanup receipt. Use graceful HTTP server shutdown, no forced connection close on the success path; only a failure path may force disposal, and that records failure, clears every qualification and cannot admit success. Reserve cleanup inside the constructor's original150second work/180second total origin; never start another origin or extend any native/HTTP/probe budget. A failed original work context or forced/errored/unfinished provider shutdown prevents completion. Require native acknowledged API shutdown, group reaped and port closed with no forced kill/SIGTERM, as already supplied and checked in raw.

After final source/data/cleanup/capture checks, and only then, set single-call top-level native_capture_complete/native_history_grammar_frozen/runtime_attempt_complete=true and test_failed=false, with the typed proof and ordinary cleanup receipt. This branch sets only mcp-initialize/mcp-tools-list/mcp-tools-call true. Project-lifecycle/native-restart-removal stay false in this single-call lane; vendor_authentication/real_model remain false and activation remains Prepared. It cannot inherit the component lane's lifecycle flags. Root may combine independently audited current-source component and single-call evidence later.

Update both Go archive admission and Python pure `kimi_call_members` to recognize an exact closed success pair alongside the preserved failed-only pair. Success requires every selected command triple, two provider requests, installed native projection/skill, probe stream and complete raw Python envelope, correct source/tree, three MCP flags only, own proof and ordinary native/provider cleanup. Reject partial/mixed-success flags, missing proof, failed commands, absent/mismatched source and any forced cleanup. Existing failure paths retain every original allowed prefix/raw diagnostic, false completion and all five false flags. Clear any staged success fields/flags whenever a late original authority/deadline/cleanup/capture check fails before serialization. Frontend actual test success remains required independently; admitting bytes never turns a failed test green.

The historical9e andb2bb raw captures remain byte-identical and failed-only when replayed with their ORIGINAL source identities. No old raw is retargeted to a new source or promoted to success. Component raw admission and stage flags remain unchanged.

## 6. Implementation surface and meaningful verification

Keep changes narrow to Kimi repotest helpers/contracts, the existing runtime pure-reader success branch/contracts and evidence documentation. Expected paths: existing `repotests/agentplugins_kimi_project_native_attempt_test.go`, provider helper if needed, one new finite history helper plus focused contract file, existing Kimi attempt/provider contracts where expectations need precise updates, `scripts/project_native_runtime.py`, `scripts/test_project_native_runtime.py`, and a concise Kimi validation record. Existing Python native collector, shared service/supervisor/acquisition, production client/provider/installer selection, component lifecycle, workflow routing, versions/pins and other clients stay byte-identical. If implementation requires wider behavior, stop for amended plan review.

Before code, two independent plan GOs. Implement in a new isolated exact-current-source checkout after root confirms the observed remote head; preserve concurrently advanced work and the unpublished alternative. Root alone commits/pushes. Before publishing obtain two exact-tree code critics and execute these meaningful local gates under root-owned serialized Go/lint slots:

- Pure model framing: actual0.004header+facts succeeds; malformed prefix, sign/leading zero, wrong fractional width/unit/CRLF/multiple headers, >60/nonfinite, error/note/trailing payload, duplicate/unknown facts fields and oversized/deep/invalidUTF8 refuse. Keep existing source-bound facts/probe event mutation tests.
- Pure raw envelope/history: actual9e current terminalfailed+emptyassistant remains rejected; its raw bytes/digests/flags remain unchanged and failed admission remains exact. A synthetic finite completed fixture alone exercises the positive grammar, explicitly labeled non-native. Mutations cover wrong session/workspace/cwd/model/prompt/call/tool/nonce/marker/root/data/PID/probe session, wrong ordering/reverse snapshot mismatch, duplicate ID, count/pagination excess, extra/missing/error tool result, missing/extra/wrong/partial sentinel, unknown fields/content/metadata, nonfinite/deep/oversized/invalidUTF8, stale/out-of-window timestamps, injected-date mismatch, unstable epoch/regressing sequence, absent terminal snapshot, active/failed/cancelled terminal, extra questions/approvals/subagents and receipt/hash/member closure errors.
- Provider/source/data/cleanup: preserve one-call authority and two-turn request checks; ordinary graceful provider shutdown passes inertly, forced/error/expired closure cannot qualify; missing/wrong marker and unrelated data changes still refuse; late failures clear staged completion/flags. No fake native binaries or skipped-host marker impersonation.
- Pure archive readers: failed pair unchanged, exact successful three-MCP-only pair, reject mixed flags/false-green/missing raw or proof/extra files/gapped provider sequence/failed command/forced cleanup/source mismatch. Actual current component67/25 and single25/25 archives replay as their original outcomes. Add no vendored executable fixture.
- Focused Kimi inert Go/contracts plus affected Python contracts, gofmt/whitespace, actual affected all-enabled lint and whole unfiltered six-linter gates. Do not run any native/vendor/public-installer/scanner path locally. Scope broader tests only to concrete shared-reader risks.

After accepted code and executed gates, root normal expected-head fast-forward publication triggers fresh genuine hosted CI. Do not manually retry unchanged source or declare completion from inert fixtures. Audit fresh exact source/tree/pins/raw source-to-output receipts, approval, provider/probe/native-history, persistent marker/data and whole ordinary cleanup. Component must remain independently green and audited on the same published source. Only an actual fresh native pass with independent raw review can qualify the tools/call slice. Any still-unfamiliar native history remains failed with its original raw capture for a new evidence-based review.

## Accepted Kimi framing/history amendment before implementation

Exact draft SHA256 `fb44cab8565ed9023f43ce45a0e2d1c8d5f06ea03cd9c0118b092003c151aaa7`
received independent PLAN GOs from root and GPT-6.1 Sol xhigh `native_e2e_finish`
before any source edit. Receipt SHA256s:
`7850e5f2a948b5074bf300deb1e0c67a860da921003ca65abc9b44d2fb4c4a21` and
`a5306ed37e714e594cfc6b61198dbdf9c871d3482afcf6481e85671023debf5f`.
Both rehashed all21 primary blobs and inspected the actual framing/native/probe
failed boundary. Closed typed proof must join full raw/HTTP/session/history
identities, marker/data and ordinary shutdown; success booleans alone cannot
qualify. Current9e failed native history remains refused. A reader mutation that
places success flags/proof over failed raw must fail; late failures clear staged
flags. Only future same-source actual component and single-call success may be
combined. Root freshly checked PR524 remains9e before this isolated checkout.

### Windsurf complete bounded profile-delta projection (2026-10-28)

The genuine090b failed component capture observes15 profile changes after the
account-free empty skills catalog. Its original three-entry projection remains
byte-for-byte unchanged. Five fixed pages at offsets0/3/6/9/12 now expose at most
15 independently bounded hashed details from those same held frames, each
within the original2KiB limit. Complete counts, explicit omissions and the
existing serialization fallback remain truthful. No extra filesystem read,
raw private path/body, native command, artifact member or profile exception is
introduced; the original protected/immutable and unequal-profile fatal remain.

Plan0227ee9609b5efbd1232f0e11a2465e79fbbbd0b7e8538eaa081501abdeea6b6
received root and Grok pre-code GOs. Root and the independent Qoder owner
reviewed all five frozen files under manifest
ba2760d3debbdd443ad4fadb427a2d5cea782a5dd00711f9447bbdeb45e81910.
Executed exact focused Go contracts:46pass/0skip/0fail, package2.662s; Python
contracts30pass. Affected all-enabled lint and complete root six-linter each
report0issues. A separately recorded accidental unfiltered all-enabled lint
invocation found existing unrelated diagnostics; it caused no source change or
suppression and is not substituted for either required gate. The genuine090b
14-member archive SHA60ee39861c3292730d39770c00f811158925fc8cb3743073257c102682da3185
replays with byte-identical failed result and all qualification flagsfalse.

The pure reader admits this optional field only at the actual completed failed
native stage, with exact five-page schema, original page0, strict scalar types,
counts/omissions, global uniqueness/order and ordinary record bounds. This
publication remains a failed-observation diagnostic. A new genuine hosted
capture and whole raw review must establish all actually observed profile
changes before any narrow initialization policy or full lifecycle claim.

## Windsurf sixth bounded diagnostic window (2026-10-28)

Two independent Sol reviewers approved exact amendment SHA256
`002ccf74d04c52ec6083ce0313bf1e5e89b13f7b6bdb95fe76269c85209eddc1`.
Actual `9a762821` run `40740022136` has two complete independent RAW audits:
14 native/25 frontend members,24 stream joins and234 pinned file hashes.
The exact first skills listing succeeds in185ms, then the original protected
config guard fails. The profile has16 changes; five windows expose15 unique
entries and omit one cache addition. All qualification flags remain false.

Add only the sixth fixed diagnostic offset15, retaining three details/2KiB per
window, the same held maps, original first five projections, global counts,
privacy, refusals and cleanup. The failed-only reader preserves historical
five-window captures and accepts the current six with exact ordering and joins.
Observed16/exact18/overflow19 controls preserve truthful omissions. No profile
ownership or unknown data/cache permission follows; no native argv, source pin,
environment, deadline, evidence member or product behavior changes. Replay old
failed captures unchanged and require a fresh hosted observation after review.

### Windsurf fixed native startup state (2026-10-29)

The exactd0ac8b3c diagnostic40753719131 closes all16 profile changes. Its native
artifact12847601802 is27524bytes/SHA256
`17c4529fb2791381fcfb180c8538da5c2d6a95f3e54b115bab365a1dc3b244d9`;
frontend12846173942 is26113bytes/SHA256
`825d1e694ecdc5665c2faca465654ddb015c98c17280dc33cf6f151406800ff1`.
Both API-bound archives have ordinary closed members and valid CRCs. The final
omitted directory is exactly `xdg-cache\0devin` (SHA256fddb766d...). All flags
remain false. No broader profile ownership follows from those path identities.

Independent static inspection rehashes the unchanged official archive/binary.
The binary's logger construction1e06eef..1e06faa uses the Rust format at8bb4774,
`{}_{}_{}.log`, with literal`devin`, local-time `%Y%m%d-%H%M%S` and process PID.
Actual `devin_20261028-195926_5805.log` joins the captured439030f9... path hash.
The maintenance lock helper2b05ce0 opens ordinary0666/umask0644, flocks EX|NB,
truncates and writes its decimal getpid plus LF. Actual5805LF matches1dadcee....
The config loader186526c..5288 migrates version0 but returns version1 without
that schema write; a valid existing36-byte installation UUID returns its
original bytes at1a2c5dd. The telemetry cache is exactly16bytes
`{"is_zdr":false}`, matching captured7445ff87.... These are source inspections
and byte/hash joins, not local native execution or authentication evidence.

Before the component fixture's first vendor child, author version1 beside the
unchanged two safety fields, a valid fresh installation UUID, exact telemetry
bytes, the empty `devin/cli/plugins/lock.json.lock`, and the finite observed
`devin/cli` directories under XDG config/cache/data plus data logs/plugins.
Preserve the shared updater and historical startup/diagnostic recipes. Freeze
all new seed bytes and original raw config. No post-child semantic adoption or
renewal of the original protected digest is authorized.

For registered non-version skills/MCP reads only, allow exactly one new ordinary
0644, current-owner, single-link `devin_TIMESTAMP_PID.log`, at most1MiB and16MiB
cumulative, plus an ordinary bounded maintenance lock containing that same
positive decimal PID and LF. Require a real calendar timestamp in the actual
command interval at second precision; the current hosted UTC spelling is
admitted conservatively, without claiming the vendor hardcodes UTC or changing
its environment. Prior logs stay identical; only the logs directory's content
timestamps may change while its inode/mode/owner remain fixed. Refuse every
other addition, removal, replacement, compression, raw-config rewrite, unsafe
entry or expired budget. Preserve the original150-second deadline and ordinary
process cleanup. The existing first-component capture stays deliberately failed
and all-false; supported complete lifecycle/native grammar remains required.

Grok/Qoder and integration independently approved this exact plan and final
four-file code. Review fixes bind the log window to the immediate OSRun end,
then copy the accepted held profile frame into the expected runtime subtree;
a later filesystem observation cannot replace authority. Late log/UUID/unknown
mutation controls prove refusal. Five new runtime controls plus the retained
config contracts pass in0.070s; nine Python controls pass. Affected all-enabled
and whole-root architecture/size six-linter gates report0issues. The broader
component suite passes with its unchanged owner-change control excluded because
this local UID namespace cannot chown to UID1 (EINVAL); that control is not
waived or changed. The original14-member failed capture and all24 frontend
stream/hash joins replay unchanged/all-false. No vendor process ran locally.

### Windsurf exact native telemetry timestamp rewrite (2026-10-29)

Current-source run40878254890 attempt2 still stops after the first successful
skills read. Its27-to29 profile delta contains the expected new log/maintenance
pair and log-directory metadata, plus a changed record for the exact seeded
`xdg-cache/devin/telemetry_state.json`: both sides remain16 bytes with SHA256
`7445ff8769cba2e1b004c754a770a85293e7548228e4fa3d363263c96c9b6290`.
The pinned binary's2e37ca0 helper joins XDG_CACHE_HOME, `devin` and
`telemetry_state.json`; its2e34180 serializer uses the `is_zdr` field.
Permit only Modified/Changed timestamp variation for that exact cache file,
requiring unchanged device/inode/kind/mode0644/owner/group/single-link authority,
size16 and exact fixed bytes before and after. Copy only its accepted frame record
into the existing cache expectation; never reread or replace a protected digest.
Keep original config/UUID/other-cache/old-log and late-write guards unchanged.
The raw diagnostic omits inode/owner details, so it does not itself establish
these predicates: the next genuine hosted capture must satisfy them or fail.
Both independent plan reviews approve this limited correction; the historical
failed capture remains failed with every qualification flag false.
Both independent code reviews also approve the exact predicate and accepted-frame
copy. Six focused startup/runtime controls pass in0.100s; affected all-enabled
lint and the unfiltered six-linter architecture/size gate report0issues.
