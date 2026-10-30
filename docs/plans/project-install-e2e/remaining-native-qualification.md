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

# Copilot failed-attempt evidence and original-config diagnostics amendment

## Observed evidence and limits

Genuine runtime run `40363828314`, job `121755712251`, at published
`4566b551b7c225a30b40c149a44f69d96cc2d0ce`, tree
`aeaf19c4a30668f17f83a111ac362c70ed6f95d1`, built and selected the current
Copilot test, then failed at the `native-attempt` command boundary with
`original authority changed: .../home/.copilot/config.json (<nil>)`.
Frontend artifact `12678603433`, ZIP SHA256
`1020004a73f256b40287914bdb3925f0899997f7226cfbac72c80f6613ee96a4`,
records `runtime_artifact_admitted=false` and
`runtime_admission_error="ValueError"`. No native archive was uploaded.
The config digest differed after a successful strict read. Its actual changed
bytes/fields, cause and exact pre/post boundary are not available from that
frontend. No migration, authentication or native tool outcome is inferred.

Local source `6fb04dc10b3e48ade631c4d65433c39afd5a9f60` has the exact
published tree above. Its fixture necessarily stores the installed project MCP
snapshot under `.github-mcp.json.installed`, because setup derives a name from
the parent `.github` plus basename `mcp.json`. The unchanged frontend accepts
only `mcp.json.installed` for that snapshot. Setup returned before the reported
line 406, so this unexpected member had already been held. An inert invocation
of the exact `attempt_members` function reproduces `ValueError: unrecorded
native artifact member`; changing only the inert member name to the existing
allowed name admits the same failed/unqualified envelope. This proves a
deterministic source contract defect. Because the historical raw archive was
not exported and only the exception class survives, the exact historical
admission branch cannot be inspected directly; an earlier ZIP check is not
independently excluded by the available bytes.

## Correct only the fixture's emitted snapshot name

In the Copilot Go fixture, give the two installed public snapshots explicit
existing names: `mcp.json.installed` for the selected project `.github/mcp.json`,
and `uapcopilota-SKILL.md.installed` for its selected installed skill. Retain
the same held bytes, receipt-selected paths, ordinary-file reader, content
checks and capture bounds. Do not broaden or change `attempt_members`,
`admit_runtime`, the allowed member set or shared Python parser. This snapshot
is installed project configuration, not the original private user profile.
Add an inert contract for actual fixture-emitted names and preserve unknown,
duplicate, partial-command, non-UTF8 and oversized member refusal.

## Use existing evidence to locate the failure boundary

`command` already checks protected authority before execution, then holds its
stdout/stderr/command triple before checking authority again. With the naming
defect corrected, a complete `native-attempt` triple in an admitted failed
archive proves the command returned before the failed post-command check; its
absence together with the boundary failure identifies the pre-command check.
Do not add a duplicate phase field or alter the exact command receipt schema.
Retain failed captures and the deferred existing `finish` behavior even when
the original-profile guard fails. An incomplete triple still refuses admission.

## Minimum bounded original-config change facts

Keep the original authored config bytes/digest frozen before version/native
startup. On a mismatch of exactly the fixture's original
`<original HOME>/.copilot/config.json`, keep the existing fatal refusal and
record one optional, bounded `original_config_change` object inside the existing
`runtime-attempt.json`. There is no new artifact member or private-profile ZIP.

The diagnostic may contain only: expected and observed SHA256 digests; baseline
and observed byte counts; a fixed snapshot status; whether strict parsed JSON
is semantically equal to the original authored object; whether the
`trustedFolders` value still exactly equals the original two selected project
folders; and a bounded structural summary of changed top-level members. For
each changed member export only its key SHA256 and before/after JSON type
(`absent`, `null`, `boolean`, `number`, `string`, `array`, `object`). Do not
export raw unknown key names, any member values, absolute trust-folder values,
credentials, tokens, raw config text or a recursively copied profile. A
semantically equal object can identify a byte-format change without accepting
that change or adopting a new baseline.

Use the existing no-follow ordinary held-file reader, cap this diagnostic read
at 16 KiB and the structural summary at 32 changed top-level keys. Exceeding a
bound records a fixed bounded status, with no partial structural summary.
Compare the
diagnostic bytes' digest to the digest observed by the failed guard; if it
differs, record only a fixed unstable status and no JSON-derived conclusions.
Missing, aliased, special, oversized, malformed/non-object or unreadable input
records a fixed status without error text or content. Unknown/missing nodes do
not broaden the original guard's acceptance. Produce a finite object no larger
than 16 KiB, within existing total/work/cleanup budgets, with no retries,
native child or filesystem writes during diagnosis. The snapshot remains in
private memory only; do not reread or publish unknown profile files.

## Verification and unchanged scope

Two independent plan approvals precede code. Expected edits are the Copilot
fixture and focused inert contract tests, plus its consolidated plan; shared
frontend/parser/allowlist and native provider behavior remain unchanged.
Contracts must prove correct snapshot naming, pre/post distinction through
the existing triple, semantically equal-but-byte-changed refusal, exact trusted
folder preservation checks, hashed/type-only changed-member summaries and
absence of canary key names/values/secrets in exported diagnostics. Cover alias,
read failure, malformed JSON, size/key-count bounds and a changed snapshot.
Every mismatch still fails the test; diagnostics can never promote a failed
attempt or any qualification flag.

Preserve all pinned acquisition/runtime hashes, native argv/environment,
loopback provider route/identity, permission restrictions, process supervision,
original-user/A/B ownership, state Prepared, deadlines, no extra native calls
and closed artifact limits. Run only focused inert contracts/lint locally.
Inspect a fresh genuine hosted failed capture before proposing any source,
product or runtime behavior amendment for the config mutation. Vendor
authentication, model use, native session grammar and all five lifecycle/MCP
qualification flags remain false.
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

## Qoder original anonymous-metadata fixture amendment

Accepted bounded fixture correction. Native-finish R1 and independent CLI R2 approved exact plan SHA256 `0500e7dcd839dd3b27a70ebf3da1ae57ecdac43637d0ee6a2c1e9cba5858558b` before implementation. No native qualification or authentication claim.

### Evidence and source boundary

Current PR523 source 1973a9551f87a6e349c3cfd7ba6a9b8ff11118b8, tree 3e8fb0fc2c56162e6345f1797fd3c5b3419c984b, genuine runtime40566031581/job122407532341, native12761729753 and frontend12762294585, produces a successful exact flag-selected 765-byte catalog before the complete original .qoder profile guard fails. The full delta is10 additions,1 change,8 omissions. Existing file-priority diagnostic independently maps three held regular-file path hashes to published literals: installation_id (36bytes0644), .auth/.credential-transaction (0bytes0600), .auth/machine_id (36bytes0600). Actual ID contents were neither exported nor read. The unobserved eight entries remain unknown; no stability promise follows from these three observations.

The published @qoder-ai/qodercli1.1.78 archive at https://registry.npmjs.org/@qoder-ai/qodercli/-/qodercli-1.1.78.tgz was checked against published SHA1 and SHA512 integrity. Its archive SHA256 is c5b237d2fd84ceb44af5427f281a7f1bcadd8c888fbed49197b5ce2f37dada4f; package/bundle/qodercli.js is34562870bytes, SHA256 ec7b2a2c1be206f525ca974c3b9455fbec9ce7411bc5dfdee55f1e82eb90df38. The npm bundle is source guidance only: its equivalence to the independently pinned standalone executable is not established.

Root independently rechecked the entire archive/bundle hashes and these source paths, without executing Node, the installer or any client:

- Installation reader at byte1016117 returns existing nonempty trimmed installation_id. getInstallationId at1016392 generates/writes a UUID only when absent, caching it separately from account credentials.
- Auth directory/transaction setup MAl at3843720 creates .auth0700, opens the empty .credential-transaction target in append mode0600 and enforces0600. The transient .credential-transaction.lock is separate and is not authored by this proposal.
- Machine reader FT at3850006 returns existing nonempty trimmed machine_id. getMachineId at3855626 returns the held value via PpA; absent identity takes the ordinary transaction/generation path, with hardware or random generation. PpA at3847740 records consistency of the identifier in process; it is not an account grant.
- Wet at3843328 selects distinct account credential files .auth/user or .auth/user.<environment>. No such credential file, token, login state or trust entry is created or modified here. The fixture remains without vendor account credentials; a later genuine authentication refusal stays a failure.

### Proposed finite fixture change

Change only the Qoder fixture constructor, before the first public/native child and before any preservation baseline. In its already-owned original disposable HOME/.qoder, create precisely these independently authored entries:

1. installation_id: one fresh independent RFC4122 version4 UUID, exactly36 ASCII bytes with no newline, ordinary single-link file0644.
2. .auth: an ordinary directory0700, owned by the current original user; refuse any preexisting entry at this previously unowned exact path.
3. .auth/machine_id: a second fresh independent version4 UUID, distinct from installation_id, exactly36 ASCII bytes with no newline, ordinary single-link file0600.
4. .auth/.credential-transaction: an empty ordinary single-link file0600.

Use fresh cryptographic randomness from the existing inert Go fixture process, not native-observed IDs, host hardware IDs, account data, imported HOME or native output. Do not reuse the public package/probe nonce as an ID or parse/copy any existing native auth file. Use existing held-parent/no-follow/exclusive-create helpers, exact modes and owner checks. Author only the above three files and their one absent parent; no unknown cache, directory, lock, registration or auth path is inferred from omitted diagnostics. If any target is unexpectedly present, special, linked or foreign-owned, fail without replacing it.

Record the three expected byte digests in existing protected-file authority immediately, before any child. Also hold the exact initial recipe authority for these three ordinary files and their .auth parent: device/inode, UID/GID, full mode, single-link files, exact byte lengths and independent byte digests. Add a narrow Qoder-only fixed-recipe verifier to the existing f.check before/after each already-selected command, activated only after this constructor recipe exists. This verifier rejects same-byte mode changes, inode replacement, changed parent/type/owner/link count or bytes from the first child onward; it must not wait until the catalog snapshot to notice a version/public-child mutation. It accepts no extra file or directory authority and reads no account credential path. Existing intraread held identity checks remain unchanged; this is not a promise of immutable access/change timestamps after ordinary reads or same-mode chmod. Keep the complete original .qoder catalog before/after snapshot and equality guard unchanged, including full modes, membership, unknown files/directories, links and existing special original-log rule. Every later byte drift or missing marker is fatal; no post-run adoption, exception, exclusions, cleanup deletion or fallback recipe. Source/native divergence, unexpected chmod or additional native file creation remains a genuine failed observation.

No new child command, native argv, provider, environment, permission, trust, source/installed digest, scanner, acquisition pin, deadline, supervision, raw member, diagnostic cap or admission schema is added. Current raw diagnostics stay three details/2KiB with full counts and omissions. All five qualification flags, runtime_attempt_complete, vendor authentication and real-model flags remain false, including a successful catalog or restricted startup. Project activation remains Prepared. No public adapter or installer production code changes.

### Implementation and meaningful checks

Keep the helper/client-only controls in new Qoder-specific test files with one guarded constructor authoring call and one guarded invocation in the existing f.check for spec.client==qoder; do not alter other clients. The extra held recipe field is private to this fixture and absent before construction; no common permission or profile snapshot policy changes. Unit tests call the helper on ordinary inert temporary directories only, never newProjectNativeCLIAttempt, a positive hosted guard or a vendor executable.

Check the exact four-entry recipe, types/modes/owner/single-link/byte counts, canonical distinct valid UUIDs, protected digests and unchanged original settings. Verify preexisting target or .auth, symlink/hardlink/special/foreign authority refuses without overwrite, including partial-recipe failures. Exercise the narrow fixed-recipe verifier to show a real same-byte chmod or inode replacement before the catalog baseline fails, as well as after a command. Exercise the existing full profile snapshot to show metadata bytes/mode or any new sibling still cause inequality/refusal. No tests may bless the current eight omitted native entries or infer native source equivalence.

Two independent plan GOs precede coding; independent exact code review and focused inert Go/affected-root/full-six lint gates precede root-only publication. Align actual program base a412ba5e137fdcb7e004393229b323211ed746e1 with separate preservation review. No force push, release, new PR or VS Code work. A fresh genuine hosted capture is required to learn whether the same exact catalog now preserves the profile; any remaining mutation/authentication/tool boundary is reported honestly rather than reclassified as success.

The original Qoder leaf was aligned to actual Cline-containing program base in
`ca88f79a399f1e0f8280a38476f862fd9667eb2e`, tree
`65edbdc10526a653d0d225a3fb531dea2910d4b3`. Independent static preservation
review verified all 5777 nonconflicting tree entries and every nonblank plan
line from both parents; Qoder-owned source was unchanged by this merge.
Focused inert verification and exact-code review are recorded separately once
completed; this accepted plan does not claim those gates or native success.

### Qoder anonymous-metadata amendment: exact implementation validation

Code commit `ff5c144de817423e5430b92ad9dc099ed01db760`, tree
`0a3c1a1d2d51d1fb812b801664d8e14e1348eef7`, contains only the private
Qoder helper, its inert contracts, twelve shared-fixture hook lines and the
accepted record above. The helper opens the original profile and absent auth
parent through the existing held no-follow mechanics, creates only the three
exclusive ordinary files, and verifies their bytes against independently
generated inputs before publishing protected digests. Each existing selected
command retains its original before/after check. The new private recipe check
also pins initial device/inode, owner/group, full mode, file length/link count
and byte digest; current linked identities are checked after held reads.

Root independently approved the helper/shared/accepted-record delta and
bounded test corrections. CLI independently reviewed exact final tree
`0a3c1a1d2d51d1fb812b801664d8e14e1348eef7` and gave CODE GO without remaining
findings. Tests preserve all child inodes during auth/profile parent replacement,
so those negatives specifically prove the initial parent identity check. Other
controls cover all three files' early same-byte chmod/replacement, changed
bytes/types/links, preexisting targets, exclusive partial-write refusal, exact
four-entry authoring and full-profile sibling/mode/byte inequality. Foreign
held UID/GID controls always run for all five nodes; an additional real chown
control explicitly reports unsupported privilege/user-namespace capability.
Ordinary reads and same-mode chmod do not pin access/change timestamps.

Focused inert `^(TestQoder|TestProjectNativeCLI)` passed in 0.139 seconds with
Go 1.26.10. The actual all-enabled root lint gate with
`--new-from-merge-base=a412ba5e137fdcb7e004393229b323211ed746e1` reported zero
issues. The unchanged unfiltered whole-root
revive/funlen/gocyclo/gocognit/dupl/depguard gate reported zero issues. Gofmt and
staged whitespace checks were clean. No client, public installer, package hook
or positive hosted guard ran locally.

No current native artifact has been relabeled or promoted. The genuine
1973a955 runtime 40566031581 still fails complete original-profile equality,
with all five qualification flags false and eight omitted entries unknown.
The npm/standalone equivalence caveat remains. Fresh genuine hosted evidence
must establish the next boundary; any additional profile drift or genuine
account refusal remains a failed observation. Root alone owns publication.

## Qoder exact auth-membership correction (2026-10-27)

Independent review found that an unknown auth sibling could enter the later
catalog baseline after setup. The [reviewed addendum](qoder-auth-membership-addendum-plan.md)
and [actual validation](qoder-auth-membership-validation.md) close that early
window while preserving the original metadata commits and complete-profile guard.
This source correction supplies no native qualification; fresh hosted evidence
and the existing lifecycle gates remain required.


# Qoder: complete the measured four-entry profile delta diagnostic

PRE-CODE draft. Read AGENTS.md, owner scope and capability-preservation rules.
Obtain two independent GPT-6.1 Sol xhigh plan GOs before implementation. This is
diagnostic-only work on existing PR523, not permission for native log mutation,
profile relocation, a native retry at unchanged source, a release or VS Code.

## Exact new genuine evidence

Published source 4f034eb374bb585f0bc409763e62f0058bebabed, tree
3d04ce2878cf93239cfa2f9a2564dc2c6a931bc3. Actual runtime40585824651,
Qoder job122486483093, fails the original complete-profile equality guard after
4.42 seconds. Every recorded prefix command exits0 without a capture/context
error. Public add succeeds Prepared; plugins list exits0 in1149ms and selects
the one exact receipt-owned enabled flag-view plugin with one skill and no MCP.
No native prompt, later lifecycle or authentication/model result was reached.

Native artifact12773802932 is4508B,16closed members, outer SHA256
750064bd0a084ff22e5109e3e114f6d7ed7a21bbfe98d3cafea4f9afddc3a581;
frontend12774277343 has25members, outer
a8dd9a901886a9c3eb7f28a090d6aaa7a370ddc76e1898f063760c0dd4e28d3e.
API identities, ordinary ZIP membership/CRCs, clean exact source/tree/build,
all24frontend stream joins and unchanged exact-source failed admission pass.
Root semantic audit independently joins official archive/executable pins,
public source/package/binding/physical/receipt identities and native catalog.
Record /tmp/uap-qoder-4f-current-40585824651/root-semantic-audit.json,
SHA25636c431d8799a7ae513ef30bab98452afa3e9a3fb9283fb9b8003aae8da9e0cff.

The complete delta now has15before/18after,3added/0removed/1changed.
Its area counts are exactly3logs and1log-latest; every other area is0.
The three retained details show changed latest symlink, added0755directory and
added0644file of994B. One entry is still explicitly omitted. Its name, kind,
mode and bytes remain unknown. The prior three telemetry directories no longer
appear as differences; that correction does not authorize any remaining change.

Pinned npm1.1.78 source defines a candidate per-run directory, qodercli.log,
manifest.json and latest-link replacement before CLI argument parsing. It does
not identify the omitted native entry or prove npm/standalone implementation
equivalence. No quiet/telemetry setting is assumed to suppress these writes.

## Proposed finite diagnostic amendment

Change only qoderDeltaBound's maximum retained metadata details from3to4.
Preserve the existing complete-object2KiB JSON bound,4096-entry input bound,
strict inventory-record grammar, finite areas/kinds/operations, hashed keys,
hashed file/link records, and all numeric/mode constraints. Preserve the exact
existing other-file priority and deterministic area/hash ordering. The first
three selected details must remain identical; the fourth is the next detail
under that same ordering. No raw name, path, link target, contents or credential
is added and no filesystem read, enumeration or command is added.

At exactly four valid differences the existing status becomes captured and
omitted_count becomes0; at five or more it remains entry-limit with exactly
total-minus4 omissions. The whole-object bound still refuses oversized output;
it is never increased or bypassed. This is a bounded formatter over the already
held unequal maps, not a new baseline, a permission decision or log classifier.

All complete-profile, auth/metadata, empty telemetry, no-follow identity,
original HOME, immutable source, catalog, deadlines and process cleanup gates
remain byte-identical. The original t.Fatal after collecting this diagnostic
remains unconditional for unequal original profiles. All five/native/auth/model
flags stay false. Python admission, commands, workflow selection and source pins
are unchanged; historical failed raw archives must re-admit byte-identically.

## Meaningful verification

Update the existing independent expected metadata/count assertions only where
the intentional fourth retained detail changes them. Keep all prior checks,
priority cases, hostile records, actual-filesystem nonmutation, private-canary
checks and the original fatal full-profile comparison. The existing Qoder
controls exercise actual filesystem drift and unequal inventory maps; there is
no separate Qoder fatal-child test. Prove the fatal caller remains unchanged
with its complete file hash. State independent literal key and content digests
for the fourth selected fixture entries; do not replace
assertions with expectations computed through the formatter being tested.

Exercise an actual inert filesystem with a latest-link replacement plus a new
log directory/two files: four details, exact counters, captured/zero omissions,
complete serialized object<=2048bytes, original maps/profile unchanged. Add a
fifth difference and verify exact one omission and unchanged prefix ordering.
Preserve zero/one/seven other-area cases with corrected exact fourth selection;
verify reverse insertion gives the same result. Extend the existing largest
valid file-record control to five entries, retaining four with truthful omission
and the unchanged complete-object limit. Unknown records remain refused.

Run focused Qoder/shared inert Go contracts, existing runtime Python contracts,
actual full-root affected all-enabled and unfiltered six-linter gates, gofmt,
whitespace and unchanged original4f failed-envelope replay. No native/vendor,
scanner or public installer executable runs locally. Serialize Go/lint slots;
do not overlap snapshot-sensitive Go with other Python fixture mutations.

## Review, source and publication

After both plan GOs, recheck remote head and create an isolated worktree from
the agreed current4f. Commit this accepted amendment to the existing consolidated
remaining-native-qualification.md before code. Limit code to the Qoder formatter
and its affected/new inert contracts; do not merge unrelated leaf sources.
Two independent exact implementation reviews and executed gates follow. Root
alone publishes a normal expected-head fast-forward to existing PR523 and audits
the resulting genuine capture. Preserve concurrent work and all failed history.
This amendment grants no eventual log allowance or full-cycle qualification.

## Accepted Qoder diagnostic amendment before implementation

The exact draft SHA256 `49e8fdec9188273f924542bf13469e44c83393ade9bc5c392ad0d5ddb68bfe4a` received independent GPT-6.1 Sol xhigh plan GOs from `vscode_finish` (R1) and `native_finish` (R2) before any code edit. Both reviewers inspected the current formatter and exact failed capture; the original full-profile failure and all qualification limits remain required.

### Factual pre-code correction and renewed independent acceptance

Before source edits, root corrected the reference to a separate fatal-child
control: Qoder has actual filesystem drift/unequal-map controls and an
unconditional fatal caller, not that separate Copilot test. The revised exact
plan SHA256 is `449e9c92195a9d3460a6a6ee7a9e76f6d94d3ccc9a7539922ed6e0c9027d21c8`.
Independent GPT-6.1 Sol xhigh reviewers `gemini_qoder_finish` and
`native_e2e_finish` both gave PLAN GO before implementation; receipt SHA256s
are `55cb807030414e335b55884c2187fc03f858748397546db2a6074f67eee4750e`
and `cd499270db43b6536fa61f197bd0bfabafee58d610fa0abc4f6c0f1f4c37c10e`.
Both checked the actual source and unchanged unconditional full-profile fatal
caller, whose complete file SHA256 is
`20ba3af567f4f0499220ff6ad7202993a4a4a292d6c41493f41cf174f804ba72`.
This factual correction changes no guard, authorized scope or intended tests.
The previous plan-only commit remains preserved in history.

### Four-detail implementation and executed validation

Reviewed code `393bd1cd777eee598d48000f53c0235f34c09e08`, tree
`782cd158b1d579da3ad8f30c44c13042d91f0619`, received independent CODE GOs
from `gemini_qoder_finish` and `native_e2e_finish`, both GPT-6.1 Sol xhigh.
Receipt SHA256s are `c59a544e54b12a5339e138307bf7ac2e6940d4f06ab570683676f03a1032c3dc`
and `ca520b77c7eb95774f73fb8ef7233c8c5d31bd7234316162ab9a29ab3637d71c`.
Only the three retained-count literals change in the formatter. Strict parsing,
priority comparator, whole-object 2048-byte bound, runtime reader and complete
unconditional fatal caller remain unchanged. Independently stated fourth hashes
and actual filesystem four/five-difference controls preserve the previous prefix,
truthful omissions, privacy and held-map/profile nonmutation. Four maximum valid
changed-file details serialize to 1845 bytes.

Executed on that exact code: focused Qoder/shared inert Go contracts PASS; full
root affected all-enabled lint against program base `a412ba5e137fdcb7e004393229b323211ed746e1`
and unfiltered six-linter gate both zero issues; existing runtime Python contracts
PASS. Full command outputs and their SHA256s are retained in the gate receipt.
Original failed4f archive re-admits with all16 members and summary byte-identical,
all five flags false. The native reader SHA256 remains
`f3403d72755fde23afe46aaafb709fe4a1f78be9691324c07e2bbfe3f125cdf6`.
No native client, vendor executable, public installer executable or scanner ran
locally. Fresh genuine hosted output is still required; this change grants no
log allowance, lifecycle qualification, authentication or model claim.

## Qoder exact run-log projection before any ownership allowance (2026-10-28)

Start at 1050bbbe163abf4c60f1ea13565b72bc864c6e75/tree 5e64b064acc40f52cf362ad43b59589dc829746a.
Run 40594441986/native 12777840418 measures exactly 3 added log entries and 1 changed latest;
all 4 details are retained, but added names and the new target/manifest remain unobserved.
Official npm 1.1.78 archive SHA256 c5b237d2fd84ceb44af5427f281a7f1bcadd8c888fbed49197b5ce2f37dada4f;
bundle/qodercli.js SHA256 ec7b2a2c1be206f525ca974c3b9455fbec9ce7411bc5dfdee55f1e82eb90df38.
Its bytes36498/39612/40361 define timestamp-random-PID runID, logs/runs/<runID>,
qodercli.log, manifest.json and latest replacement. This is publisher source guidance,
not proof of standalone equivalence. No supported off/init-suppression route was found.

Add only a failed-capture projection at the existing unequal-profile diagnostic before
its unchanged unconditional fatal. Reuse the held no-follow latest/target mechanics;
require its record to match the already-held after inventory and a source-format
relative target runs/<runID>. Never export an arbitrary target, path or unknown name.
Inspect only the exact manifest.json below that held ordinary owned run directory,
at most 4096 bytes, single-link 0644, stable held/linked identity and exact inventory
byte-count/hash. Do not read log bodies or enumerate additional profile content.
Strictly parse the bounded source-defined manifest object; unknown/malformed shape
returns a fixed refusal status, never raw text. Validate nested argv/env/log-file
types privately. Emit only checked runID/relative target, manifest size/hash, numeric
manifest pid/ppid, safe start/version/platform/arch/role, and finite equality/closure
booleans against held A/selected argv/profile and the original before/after maps.
No raw argv/env/project_id/full manifest/log body or path outside owned fixtures.
Keep the complete serialized projection <=2048 bytes; refusal output is status-only.
Label PID provenance manifest-only: no admitted native/supervisor PID is claimed.
No runner/API, argv/env, config/auth/trust, acquisition/pin, deadline or cleanup change.

Implement repotests/agentplugins_qoder_log_run_observation_test.go and _contract_test.go,
one hook in agentplugins_project_native_cli_attempt_test.go, and optional Qoder-only
complete-shape admission in scripts/project_native_runtime.py with matching contracts
in scripts/test_project_native_runtime.py. Preserve unmarked archives and other clients.
All qualification/runtime/auth/model flags stay false; no mutation allowance, adopted
baseline, new member/command, release or VS Code work. Inert checks cover exact source
layout/manifest joins, private canaries, unknown/duplicate/malformed/oversized data,
aliases/hardlinks/foreign authority, unstable/mismatched held identities and unchanged
maps/files/fatal guard. Two independent plan GOs precede code; exact code review,
focused Go/Python and affected/full lint gates precede root-only publication and a
fresh genuine hosted failed capture. Any ownership policy needs separate actual review.

Implemented after root and independent Grok PLAN reviews; independent Grok and
root CODE reviews found no remaining issue. Focused Qoder/shared inert Go passed
in 0.379s; all 30 Python contracts passed, including the final valid-Grok baseline
control. Affected all-enabled lint and unfiltered six-linter gates report zero
issues. The original 1050 archive's 16 members and complete failed/all-false
result replay unchanged. Timestamp and unsafe-leaf refusal controls were corrected
without weakening admission. No vendor ran locally; fresh hosted observation is
required before any supported log-ownership allowance or lifecycle claim.

## Qoder complete supported skills catalog cycle — accepted precode amendment (2026-10-28)

Root and an independent Sol reviewer read/rehashed and approved complete draft
SHA256 `16483ff2c375042cdfb39b813e35c2fffb26220b8dc366971ff1b231bb2412a1`
(17,922 bytes) before implementation; a third independent source review also
approved the same complete draft. Source is published `25ea0f0d708504a914aeaf864206e98fdcfa1cc8`,
tree `61331bbd744f447cc4cd86aabf742a2273d47e2a`. Two independent complete raw
audits of run `40731229834` joined API native artifact `12836830994` and frontend
`12836825965`, all16/25 members,24 stream receipts, source/tree/clean builds,
exact supported flag catalog and the finite log-run projection. The catalog
succeeded; the original profile guard correctly refused its four log changes.
All qualifications/auth/model remain false. This receipt grants no qualification.

The next single bounded full loop has14 public/system commands and20 fresh
`qoder plugins list --json` catalogs in four closed members: summary,
`qoder-cycle-public.jsonl`, `qoder-cycle-catalog.jsonl`, `qoder-cycle-proof.jsonl`.
Only supported project lifecycle/restart-removal flags may qualify. Bindings
remain Prepared/installation_verified; MCP/auth/model/skill execution, automatic
cwd discovery and native import/cache replacement stay unclaimed. Preserve the
separate old authenticated-startup test/reader. No production adapter, native
mutation command, pin, workflow, runner API, provider or model change.

Original source-authored schema2 User/foreignA/foreignB plugin records and their
manifests/skills are disabled once before children and immutable. Publisher
`getInstalledPluginRows` lists all three from every cwd with explicit projectPath
for A/B. The owned row is only the exact supported `--plugin-dir` receipt view;
never infer cwd hiding. After whole owned-package damage/removal, fresh queries
retain the SAME old view argument and require absence of its flag row plus the
complete original catalog. No empty replacement view or ignored stderr.

Retain the original pre-native full profile map, held identities and logs-absent
precondition separately from a derived admitted telemetry ledger. Git/UAP calls
must preserve the full expected profile. Only explicit native version/catalog
calls may create one exact fresh source-defined run and its manifest/log plus
owned latest pointer; the first run may create only logs/runs parents as well.
No other new path, old-record change, alias/hardlink/foreign leaf or baseline
adoption is authorized. Reuse existing no-follow held root/link/manifest guards;
join exact modes/owners/membership/hashes, fresh runID, millisecond command-time
window, fixed version/role/platform and expected cwd. Manifest/argv/log/env stay
private; safe finite run projection, lengths/hashes/deltas and ledger joins only.
PID/PPID remain explicitly manifest-only, without runner/PID provenance claims.

Exact publisher bundle1.1.78 SHA256
`ec7b2a2c1be206f525ca974c3b9455fbec9ce7411bc5dfdee55f1e82eb90df38`
(34,562,870 bytes) defines logger39612–41200, Bun normalization `f9a`8516 and
`PGA/SJ`3529. Accept only exact held argv, or exactly one canonical bounded Linux
`/$bunfs/…` entrypoint at index1 with held executable and exact held argument
suffix; emit finite form without raw entrypoint. This is source guidance:
the full genuine run must confirm the shipping form or fail all-false. Registry
schema24599k, Boolean enabled-map24728830, resource collector24704176,
missing-directory loader24604992 and JSON renderer27075013 support the exact
catalog/control recipe; omit displayName in registry/manifest to avoid refresh.

A/B have independent same-name local sources and stable derived identities.
A-two changes skill metadata/body at the same version; keep its applied source
frozen during whole exact-owned-package repair. Public grouped intact repair
restages/mutates, unlike the single SDK Repair route. Repeat add/unchanged update
are byte-stable no-ops. Preserve original User/B/foreign/caller authority. Verify
installed zero DataReceipts and actual default-remove retained owner-only data;
never invent native markers, adopt caller siblings or purge. Source/native view
and installed payload are text-only and their framing is independently recomputed.

Keep original150/180 deadlines,30s per command plus existing5s exit grace,
1MiB stream/aggregate,16MiB total and80-member limits. Strict raw privacy before
base64 and after decoding, exact finite DTO/stage closure, old capture replay,
meaningful inert source/ownership/public API contracts and ordinary descendant
cleanup remain required. No review fatal/step barrier. Root plus independent
CODE reviews and focused Go/Python/affected/all-six gates precede root publication,
then one genuine hosted full-cycle run and two independent complete raw audits.

The actual inert public CLI contract confirms the stage-specific DTOs: repeated
add is a singleton no-op with no operation/group ID; other successful lifecycle
commands are grouped. Repair/removal report source `local`; add/update report
`direct local source`. Repeat-add progress includes the transactional-apply line.
Go and the independent reader close those measured forms without accepting a
cross-stage union. Original profile and lifecycle authority remain unchanged.


# Copilot: fixed empty native registry first capture

PRE-CODE proposal. Root authors implementation only after two independent plan
GOs. Read AGENTS.md, PROJECT_INSTALL_OWNER_SCOPE.md and capability preservation.
No VS Code changes, release, native local execution or classifier widening.

## Exact new actual evidence

Published PR527 source922b705222927897fe54bb45e8665d8873fd790d,
treebceb65e8cb04029e48012742a6975a22c6431481. Hosted runtime40584763860,
job122484811964 failed on the public add-user command at22:17:09Z.
Native12774483132 outer3232B SHA256
2fbdecf541f20a9f4584485a91268859b368194fc5d636c99c2cafe321f4c3bb,
inner46faf1136207357b8172fb73bcdad59a645ba7917b8d277adf550ddbef967506,
14ordinary closed members. Frontend12774782735 outer3456B SHA256
7b546cd3c597ff72b4184f8dbc0900454a1213865253ca6a4f021fe2b2ec256d,
25members. Exact clean source/tree/build,24frontend stream joins, actual pins and
unchanged exact-source failed-envelope Python admission pass. Independent audit
/tmp/uap-copilot-922-current-40584763860/independent-raw-audit.json SHA256
f7056506d11f174c91a4211c47855b4b540e3f3a62c8c3f56291239aa65d1cca.

Actual prefix is git-A,git-B,version,add-user. Public add exits1 in1248ms with
error=false,context_error=false and ordinary full-descendant closure. Strict JSON
reports preflight_failed,0success/1failed,mutated=false; the ordinary scanner has
3files and0findings. Exact stderr records target copilot identity preflight:
native identity ownership is indeterminate; refusing mutation. No fixture native
catalog, prompt or probe is reached. All5/runtime/auth/model flags remain false.
Read-only metadata is observed2before/4after,3changed/0omitted. The two new opaque
records are a0700directory and0600file; the third is the same originally owned
0700root directory identity with bookkeeping changed. Hashes do not authorize a
path/role/content inference or a profile allowance.

## Existing source and uncertainty

Current user-scope copilot.Adapter.InspectNativeRegistry delegates to
shared.InspectCopilotRegistry. With held native executable and runner, the source
calls exactly `copilot plugin list` through RunNativeRegistry and strict current
listing classification. Provider combinedIdentityState also refuses an
indeterminate prepared finding, so do not state the final public refusal alone
proves the specific internal classifier branch. Its internal raw stdout/stderr,
exit and cause are not exported by this artifact. No native registry absence,
known empty grammar, successful public registration or permission is established.

Relevant exact922 source files are
install/integrationctl/agentplugins/clients/copilot/identity.go,
install/integrationctl/agentplugins/clients/shared/copilot_identity.go (SHA256
d784b9e56c45e72f95deac76ae1a5e9b7930673b50979959e3a4523c056ad6e3),
install/integrationctl/agentplugins/providers/native_identity.go and
install/integrationctl/agentplugins/usecase/identity.go.

## Proposed smallest new observation

Preserve the entire existing922 first-public-add observation and controls; do not
change its fatal process guard or run a child after fatal failure. Add a distinct
failed-only opt-in `copilot-empty-registry-first-capture` observation using the
existing bounded collector, original HOME fixture, immutable executable/pin and
full-descendant supervisor. It runs only fixed git-A,git-B,version then one exact
`[held-copilot,"plugin","list"]` command from the existing fixed outside cwd.
Every command retains30seconds; total work150/overall180seconds remains fixed.
No public add, install, remove, provider, prompt or probe starts in this mode.
Use the already authored firstLaunchAt/trustedFolders recipe and frozen original
config, with all normal source/file/owner/no-follow/auth/probe guards intact.
The new mode never accepts a returned path or creates a replacement user profile.

Capture native-list raw stdout/stderr and normal command receipt under the existing
bounded member/byte/omission controls. The same held-metadata capture/projection
may describe this one command with a truthful fixed native-list stage. Preserve
its1024inventory cap,16detail cap,whole-object2048byte cap and existing private
path/content exclusions; never renew before/after values. Existing add-user
metadata bytes/stage and contracts remain byte-identical. If projection is shared,
accept only the two fixed stages add-user/native-list; retain1024entries,depth32,
16details and the2048whole-object bound for both, including refusal envelopes.
A new-stage byte-length increase must not bypass the existing whole-object bound. Snapshot before this one
command, with read-only deferred projection only; deferred work never launches a
child. A nonzero exit/capture/context/cleanup error remains fatal through the
original command guard. If command exits0, retain bytes as opaque evidence and
unconditionally t.Fatal at a review barrier. Listing interpretation and all5/
runtime/auth/model flags remain false regardless of returned text or parseability.

The new mode permits at most14total archive members (four command triples,
one metadata projection and one summary), forbids every provider/probe member,
and uses only prepare,before-list,native-list,review-barrier stages with matching
complete command prefixes. A zero exit never authorizes a positive flag.

Add the new mode to exact-prefix/closed-file admission and current source selector
without deleting earlier modes or changing old failed-envelope acceptance. Keep
workflow event/routing/admission, official source/runtime pins, real-host opt-in,
credentials, commands for other modes and all production adapter/classifier code
byte-identical. No automatic retry at unchanged source. Only root publishes.

## Meaningful verification and review

Before code, freeze exact source/head after two independent plan GOs. Inert
controls must establish fixed complete/prefix argv,cwd,timeout and member closure;
reject wrong/unrecorded/extra command,provider/probe,unknown mode/stage,oversized
stream/metadata and false success/qualification. Exercise one normal inert child
with output and one failed inert child to prove the original fatal behavior and
that no subsequent command runs. Verify config/source/metadata canaries and no
profile/map adoption. Historical922 source exact admission and the new reader's
unchanged922 replay must return the same original raw summary/member bytes; keep
previous189e success separate and readable. Existing source selectors retain
exact old guards and new mode selects only the new named opted-in test.

Run focused Copilot/shared inert Go contracts, affected Python contracts, actual
affected all-enabled and unfiltered six-linter gates, gofmt and whitespace under
the serialized slots. Do not execute any native/vendor, scanner or public installer
binary locally. Obtain two exact code critics and executed receipts before root's
normal expected-head fast-forward. Only a new genuine hosted capture can establish
what this fixed direct listing actually prints. Later grammar, registration,
profile allowances and full-cycle claims require separate evidence and review.

## Accepted empty-registry plan before implementation

Exact draft SHA256 `30858baf6e6d894641c59650d117e5f95672aaaa58b7bf2f2edf4712f8e7bc0e`
received independent GPT-6.1 Sol xhigh PLAN GOs from `kimi_e2e_finish` and
`gemini_qoder_finish` before code edits. Receipt SHA256s are
`7fb5c7f31e1246af7d89d020e0bbe53cd16c676c9ced5919e037d7a8386f26bc`
and `f2b5a84d34b44acb73f44757f9fb7e37bd91befe1ff92a38bac9da4903e900d4`.
Both checked the actual unknown internal refusal boundary and current source,
including fixed source/pin/profile/process gates and1024/depth32/16/2048 bounds.
Root implements this isolated diagnostic; all existing production and previous
observations remain preserved.


## Copilot unchanged inert process contracts: hosted execution amendment (2026-10-27)

Accepted exact pre-code plan SHA256 `435f0ff8fe6240e00497329e1e4068258f208fc7d0651f282afc4ad4c081de1c`. Independent plan GOs: native reviewer `091a987f16bcb3357e3a8988587850e8249cf142be8318198419223db5c47a73`, Gemini reviewer `eb8150f5ccc4b9aefb278b531e6433e434d0522684700bdda706e7c79145d16d`. The full local process gate remains failed on its recorded environment prerequisite; the proposed real hosted execution remains pending.

### Accepted execution amendment

PRE-CODE execution amendment to the accepted empty-registry diagnostic plan
30858baf6e6d894641c59650d117e5f95672aaaa58b7bf2f2edf4712f8e7bc0e.
Two independent plan reviews precede the workflow edit. No native capability,
profile authority, test assertion, containment prerequisite or release changes.

## Observed local gate limitation

Frozen diagnostic source 7e193758d73c5ab8a374ffdabc1ad27aa941c348,
tree 6740f5640cd45584054c7938e22006a889610ca0, has two exact independent code
GOs. Its full focused Go command actually failed. The two failures are
TestCopilotEmptyRegistryActualInertCommandKeepsOpaqueBytes and
TestCopilotEmptyRegistryOriginalFatalGuardCannotContinue. The unchanged ordinary
process supervisor refuses before running either owned inert child because this
environment lacks /proc/<pid>/task/<tid>/children. A separate diagnostic using
the unchanged supervisor reproduced the same refusal for immediate and naturally
delayed inert children, including outside the ordinary execution sandbox.
The original failed log is retained at
/tmp/uap-copilot-empty-registry-gates/focused-go.log, SHA256
0c5547b83c3eb0daffffe0a42e75d13c4142aefbc79edc581e2467e642ae4bec.
This is not a successful process gate. Neither test nor the supervisor may be
skipped inside source, relaxed, replaced with a fake process or labelled passed.

The existing Project Native Runtime admission job only runs Python selection
contracts; the native job compiles the Go test binary and selects the opted-in
native test. It does not execute the new inert Go tests. The Required workflow's
full Go lane only triggers on PRs to main/master, not this stacked draft leaf.
Therefore merely pushing the existing source would not resolve this gate.

## Smallest additional gate

Change only .github/workflows/project-native-runtime.yml: after the existing
selection step, add the already pinned actions/setup-go revision with Go1.26.10,
cache false, and a focused inert contract command. Both steps are conditional on
the actual selected matrix containing client copilot. Use the existing finite
matrix output, not a user-controlled command or new flag. The command is exactly:

    go test -json -p=1 -count=1 -timeout=120s ./repotests -run '^(TestCopilot|TestProjectNativeRuntime)'

The existing contracts job remains on ubuntu-24.04 with its existing five-minute
deadline, exact head checkout and contents-read permissions. Do not add native
opt-in, vendor acquisition, installer/scanner execution, credentials, environmental
eligibility overrides, shell eval, retries or fallbacks. These tests execute only
owned inert children, fixture HTTP/JSON and local source contracts. Preserve the
two process tests byte-for-byte and execute both without an exclusion in CI.
The complete native matrix still depends on successful contracts, so no native
observation starts if this gate fails. Existing native host admission, selection,
limits, pinned actions, workflow events/concurrency and native commands remain
unchanged. Other client-only matrices do not incur this additional Go step.

Add a small source contract to the existing runtime Python tests which verifies
the conditional two steps, exact pinned toolchain and full focused invocation,
and that native remains dependent on contracts. Use existing test conventions;
do not add a YAML library or general workflow parser.

## Honest execution and publication order

Preserve the original full local failure. A separately named local subset may
exclude only the two identified OSRunner process contracts, with the exact
exclusion recorded; this establishes only the remaining inert contracts. Execute
all affected Python contracts, affected all-enabled lint and unfiltered six-linter
gate. Fix only the two already measured QF1003 dispatch style findings through
equivalent tagged switches; preserve old mode behavior and command body guards.
Re-run gates and obtain two code critics for the exact final source, including
the workflow amendment and lint fix. Replay the previous922 failed and189e
readiness raw evidence with both old and new readers as already required.

After those gates and reviews, root may normally fast-forward the existing draft
PR527 source solely to obtain the missing hosted execution, after confirming the
old actual run has fully completed and the expected branch head is unchanged.
Record both process tests as pending, never local-pass. Do not merge, qualify or
claim the full gate until the actual exact-source GitHub run shows each process
test executed and passed. Obtain source/job identity and raw job log proof, then
audit the new bounded native diagnostic artifact normally. A CI process test
failure stays blocking and is diagnosed from its real result. A successful inert
gate proves its contract only; this new native observation still stops at the
unconditional failed-only review barrier with all qualification flags false.


## Copilot empty registry capture: local validation and pending hosted process gate (2026-10-27)

Final code source `c80e34bc9097397ec15e3b9f741b39c449157ddf`, tree `653e9b7abbb08ab6f23ad0b660bf69b7ea21f799`, implements the accepted failed-only diagnostic and the separately reviewed hosted execution amendment. Two final independent GPT-6.1 Sol xhigh code critics approved the exact source: Gemini receipt `afc3518eb9c95d61cc99e018bc9c86df0ca33ca5385d52dbe195fe3d0c28890f` and native reviewer receipt `f7b14a15a01c4bf153dfd8f5ababb454c30f5cdf16946445937c841ddc0777e0`. Removing the two added conditional workflow steps restores the complete previous workflow; the existing native job still depends on contracts. The two process-test bodies are unchanged (whole file SHA256 `d328951b55a897eeb81783216a80d6f7f8d2fbcee3328b9f491d2d6dab210b6c`).

The original full local focused Go gate FAILED because the real process supervisor could not obtain this host's `/proc/<pid>/task/<tid>/children` prerequisite. This failure is retained and is not a process-pass claim. The explicitly named local subset excluded only `TestCopilotEmptyRegistryActualInertCommandKeepsOpaqueBytes` and `TestCopilotEmptyRegistryOriginalFatalGuardCannotContinue`; its43 top-level tests /129 case-pass events passed. Actual affected all-enabled lint against `a412ba5e137fdcb7e004393229b323211ed746e1` and unfiltered six-linter gates each reported zero issues; all61 affected Python contracts passed. Executed local receipt SHA256 `a49225f173891a77f1d84121f59cf30786fca465876cab52bb10fccdd0df54e1`.

Original922 failed registration and189e successful installed-tool readiness archives re-admit under their exact old readers and the new reader with all14/21 original member bytes and summary semantics unchanged. Current-reader replay receipt SHA256 `9162f0c5a2de82123286d936767196af66b50887cec66461f149c37681bb9563`; exact historical-reader replay receipt `3c9d6bdb9fd3013f4d92535f72d19d4f5807a7a294d4ee4f647caadafb9b2e52`. The readiness result remains separate from the five broader qualification flags, which remain false.

This draft source is published solely to obtain actual hosted execution of the full focused Go command, with no exclusions, before native observation. Both strict process tests must have actual RUN and PASS records at the new source before the full gate or a merge can be claimed. No supervisor prerequisite, original profile guard, native opt-in or actual-host requirement is relaxed. No vendor client, public installer or scanner ran locally. The new native catalog capture retains its unconditional failed-only review barrier regardless of returned bytes; no registry grammar, registration, lifecycle, authentication or real-model success follows from publication.


## Accepted Copilot post-add registry observation amendment, 2026-10-28

Exact pre-code plan SHA256: `11faf859755a001ceb888f05ff3b071a0dbd9807b59618307168ac21ac191b4c`. Independent PLAN R1 root and PLAN R2 Kimi both GO before implementation. This is failed-only observation, not native activation or runtime qualification.

# Copilot: one failed-only direct registry observation after public user add

PRE-CODE observation plan. Two independent exact-plan GOs (root and Kimi) and root's accepted plan commit must precede implementation. Root alone performs Git/GitHub mutations and publication. Draft author performs source/raw reads and writes this scratch proposal only. No native/acquired client, public installer, scanner or vendor executable runs locally. Native owns the current serialized test/lint slot.

## 1. Source-bound measured reason

Plan base remains source `5781ef431714a1fb00dfa7a1c502e181699c9b54`, tree `19af10e298bdc10fe43c1392999a03cd5f2e47c2`, checkout `/tmp/uap-copilot-disabled-builtin-empty-current`. Later integration of other client tracks does not change this plan base. Root will separately reconcile composition/current remote state before publication.

Hosted run40604771816 admission122541415832 passed the actual full unexcluded Copilot inert gate with Go1.26.10. Native122544613268 completed failed at the intended public-first-add review barrier. Inherited Cline122544613212 completed success and its independent full raw audit is preserved; no pending capture needs supersession or cancellation.

Original actual artifacts are retained under `/tmp/uap-copilot-5781-independent-40604771816`:

| Artifact | ID | Outer SHA256 | Inner SHA256 | Members |
| --- | --- | --- | --- | --- |
| Native | 12781916168 | a966373206c3ee5a216ababd3ff7b3754cf8f50d9ca7334fda9da7664a2039cc | b901d186c3ec043619a8af486930d36172136bb9cd4508e6927d50183766b04d | 14 |
| Frontend | 12782145914 | da6d336af2e831d1ad992ddd277a2a15401720554e0d482589b73c3d8e896ebd | 064d5210d90862be0cb8b4381eb6cee59777e90715a4cd0b2411edb915149ca7 | 25 |

Two independent actual raw reviews are complete:

* Peer `/tmp/uap-copilot-5781-independent-40604771816/copilot-independent-raw-audit.json`, SHA256 `6ba02a976bd2db0bde65331033962fc6416af74ea0ed98869df2868c44150e73`.
* Root `/tmp/uap-copilot-5781-root-40604771816/root-raw-audit.json`, SHA256 `30562fe7a660c705cabeb120ad68faa6ad4d7b1df2109f10038133f6f41e6efd`.

All four commands git-A/git-B/version/add-user exit0 with error=false/context_error=false and ordinary full-descendant supervision. Public add takes2488ms; total4497ms. Its2831byte stdout SHA256 is `c14ba62e7310fc9f31b5aee315e02cbcf42c6c1c7fd0a0431d385abdb4da0185`. The versioned public result is add/success for native-proof1.0.0, one Copilot user target, installation `6dd5f5ca-4c29-4bd9-b5a7-c96b9444b586`, physical ID derived from that installation, and a matching public operation/security subject. It reports manual_activation_required, authentication not_checked and package_validated. The sole failure is the intentional review barrier reported at `agentplugins_copilot_user_registration_attempt_test.go:172`. Stage=review-barrier, all five qualifications/runtime/authentication/real-model remain false. No probe/provider exists.

The1928byte metadata projection reports before2/after12/changed11/omitted5. It exposes path hashes and finite metadata comparisons only. Neither names nor contents of omitted entries are known. No ownership, permission, profile exception or new baseline follows from this observation.

Current production source gives a concrete reason to capture the missing bytes: `CompleteCopilotActivation` returns `ManualCopilotVerification` for `ErrCopilotListContractUnknown`; the unattested fallback produces this manual-activation result. The post-install native list document is not exported. The public success therefore supports a new direct observation, but supplies no exact installed-list grammar, native enabled identity, ownership or runtime proof. Do not guess a builtin suffix, change a parser or activate the plugin from this result.

Pins remain Copilot1.0.103 archive `9aeb4430e9ac9523546db07b227c7aba179a4710a55f9ac29ad143b0f9e1b325` / executable `f19b3554a6b62605db3a26d5255ebe2a3ee39fcd6a357edb18301ea5037f6f0d`, and the existing scanner pins. The exact production disabled-builtin empty preflight and all generic/VS Code registry behavior remain unchanged.

## 2. One new observation mode and exact sequence

Add exactly one new failed-only mode/test. Preserve old user-registration, empty-registry and readiness entrypoints/readers/helpers so their historical artifacts remain readable under their original source identities.

| Field | Exact new value |
| --- | --- |
| mode | copilot-post-add-registry-first-capture |
| selected test | TestAgentpluginsCopilotNativePostAddRegistryFirstObservation |
| selected file | repotests/agentplugins_copilot_post_add_registry_attempt_test.go |
| opt-in | AGENTPLUGINS_COPILOT_POST_ADD_REGISTRY_FIRST_CAPTURE |
| CASES seconds / phase | 180 / runtime-attempt |

The fixed command sequence is exactly:

1. git-A: original held Git, `-c core.hooksPath=/dev/null init --quiet`, original projectA.
2. git-B: the same held Git/flags, original projectB.
3. version: `[heldCopilot, --version]`, original outside cwd, exact existing1.0.103 version check.
4. add-user: `[heldInstaller, add, heldPackage, --no-save, --scope, user, --target, copilot, --format, json]`, original outside cwd.
5. native-list: `[heldCopilot, plugin, list]`, the same original outside cwd and original HOME/environment.

Every command receives exactly30seconds through the unchanged command path. This is one added direct diagnostic invocation after the ordinary public add; the public installer's own internal native operations remain unchanged and are not counted as this new direct invocation. There is no direct pre-add warmup, second add, retry, pre-registration, profile adoption, project list, prompt, tool, provider, activation, enable, trust or inspection command.

Use the unchanged `newCopilotProjectAttempt`, `preparePackage`, authored original config, f.env, f.check and `f.command` process/receipt/guard path. The work context remains the same origin+150seconds, total remains the same origin+180seconds. The original30second command context and five-second descendant-exit grace remain exact; captured elapsed_ms can include that existing grace, so its finite receipt cap remains35000ms. Never renew/rebase an origin or replace an error with a later successful observation.

## 3. New flow, existing guards, and two metadata observations

The old `observeUserRegistration` cannot be called and then continued: its intentional t.Fatal is the old mode's boundary. Preserve its complete bytes. A new mode-local public phase repeats only that existing pre-barrier body: same original .copilot metadata capture, same exact public command through f.command, same strict bounded JSON and successful versioned-envelope validation. Require command=add/result=success/schema_version integer1/data object before the new mode can advance toward the direct list. Do not weaken or modify the existing common JSON helpers.

Keep the public phase in its own ordinary function scope. Its deferred callback performs metadata capture/projection and f.keep only. It never runs a command, schedules the direct list, rebases a guard or changes a failure stage. Thus a t.Fatal in the unchanged public process/config/pin/cleanup guard exits before any fifth command. A public JSON validation failure also stops before the fifth command.

Export the public phase metadata as `user-profile-metadata.json`, using unchanged `copilotUserMetadataAfter` / `copilotUserMetadataDelta` and stage add-user. When this phase returns normally, its deferred metadata projection has already settled. If it refused, set the finite after-user-metadata failure stage and stop; do not launch a new child after a refused diagnostic snapshot.

Only after the ordinary public phase and its metadata complete, enter a second ordinary function for the direct list. Capture immediately-before-list metadata from the same original `.copilot` root. This is a read-only observation snapshot: never assign it to `originalConfigBaseline`, f.protected, immutable pins or any authorization map. A before-list metadata refusal records the finite native-list refusal projection and fails without launching the fifth command.

Register a list-phase metadata-only deferred callback, then execute the exact native-list command through the unchanged command path. Its raw stdout/stderr and original command receipt are retained unchanged. The deferred callback uses the already-existing `copilotEmptyRegistryMetadataAfter` or the unchanged capture+`copilotMetadataDeltaAtStage(...,"native-list")` primitives and writes `native-list-profile-metadata.json`. It never starts a child or retries capture. After a normal return from this phase, refused metadata causes the finite after-list-metadata failure and no further action.

Each metadata file is independently bounded by the unchanged2048byte projection, maximum1024 inventory entries, depth32, sixteen displayed changed entries before byte-bound truncation, strict no-follow held metadata/parent identity checks, and the original context. No raw path, unknown file body, native registry/config body or arbitrary error string is added to either projection. The five omitted entries in5781 remain unknown and create no exception.

On ordinary fifth-command and metadata completion, set review-barrier and deliberately t.Fatal with a new precise raw-review message. Native list bytes stay opaque: no trim/ANSI removal/CRLF rewrite/section skipping/JSON parsing/absence classification or activation decision is made from them. Empty, unfamiliar or diagnostic stdout/stderr are retained as data under the original bounds. The unchanged outer artifact reader's UTF8 requirement still applies; invalid UTF8 cannot be admitted by inventing a new encoding allowance.

## 4. Exact finite stages and artifacts

Use the existing summary field set and optional finite `original_config_change` projection, with the new exact mode. Do not add provider_kind, inherited project activation, new success proof or inferred registry status. The new finalizer resets runtime_attempt_complete=false, all five qualification values=false, vendor_authentication_qualified=false and real_model_qualified=false, and calls t.Error if the test has not already failed. The unchanged outer finish therefore serializes the actual t.Failed() as true; it never fabricates a failed flag for a passing test. The admitted lane has probe_status=absent and no probe/provider member. If original finish encounters unexpected probe state, keep its truthful failure/refusal; do not normalize it to absent or admit it as this closed lane.

All commands form an exact prefix of the five labels; every present command has its complete `.command.json`/`.stdout.log`/`.stderr.log` triple. A label not in the recorded prefix has no triple. Previously completed commands must be clean0/errorfalse/contextfalse before continuation. Only the last current command in prepare/public-user/native-list can carry the actual captured failure. Never accept a later stage or command after an earlier failed receipt.

| Summary stage | Command-prefix length | Required metadata / truthful boundary |
| --- | --- | --- |
| prepare | 0..3 | No metadata; original Git/version/package preparation may fail. |
| before-user | 3 | user metadata is the add-user refusal; no public child started. |
| public-user | 3 or4 | user metadata required; original pre/post command fatal may occur before or after its complete captured triple. |
| public-result | 4 | Public command clean; raw public JSON validation may fail; user metadata required. |
| after-user-metadata | 4 | Public successful envelope verified, user metadata refused; no list metadata/triple. |
| before-list | 4 | Public successful envelope and observed user metadata; native-list metadata refusal, no fifth child. |
| native-list | 4 or5 | Earlier four clean/validated, observed user metadata; list metadata required, original pre/post list guard may fail. |
| after-list-metadata | 5 | All commands clean, valid public result, observed user metadata; native-list metadata refused. |
| review-barrier | 5 | All commands clean, valid public result, both metadata observed; intentional final t.Fatal. |

For every stage beyond public-result, independently verify the retained successful public add envelope. For public-result itself preserve invalid/malformed raw stdout as truthful failed evidence; do not mislabel validation as completed. `original_config_change` remains allowed only at a stage where the original config guard can actually fail (prepare/public-user/native-list), with the existing finite projection validator. Successful continuation stages cannot carry it.

Allowed files are only runtime-attempt.json, the exact prefix triples, `user-profile-metadata.json` when required above, and `native-list-profile-metadata.json` when required above. Full capture is exactly18members: one summary +15command files +two metadata files. The live Go prefix check runs before summary is added and therefore caps f.files at17; Python full admission caps18. No probe, provider, installed snapshot, second native-list slot or arbitrary diagnostic file is admitted. Do not raise generic ZIP/member/aggregate limits. Retain existing1MiB command capture/per-member mode bound, metadata2KiB each, archive32MiB and aggregate16MiB/ordinary-flat-member checks.

Both stage/member guards and the new Python reader validate exact prefix/closure, typed receipt fields, argv/cwd/deadline/cleanup, previous clean commands, metadata stage/status/bounds, source/pins/scanner/layout and all-false tuple. A captured process error stays an admitted failed prefix only at its actual current boundary. Unknown stages, malformed metadata, false progress and extra authority are refused.

## 5. Scoped source changes and whole-byte preservation

Expected implementation paths are exactly:

* Existing `repotests/agentplugins_copilot_project_native_attempt_test.go`: three finite new-mode hooks only—command guard switch case, command-prefix append eligibility, finish dispatch case. Keep every other byte of the command/check/environment/constructor/config/preparation/supervisor/finish serialization bodies unchanged. No generalized unknown-mode fallthrough or permissive mode-name prefix.
* New `repotests/agentplugins_copilot_post_add_registry_attempt_test.go`: exact new mode/sequence/entrypoint, separate ordinary public/list phases, metadata-only defers, truthful stages, closed prefix/finalizer. Reuse existing pure helpers without modifying them.
* New `repotests/agentplugins_copilot_post_add_registry_contract_test.go`: meaningful fixed command/stage/member/metadata/progression contracts.
* New `repotests/agentplugins_copilot_post_add_registry_process_contract_test.go`: authored inert scripts and the same Go test child to exercise the actual original OSRunner/fatal guard. No vendor/public installer/scanner process.
* New `scripts/project_native_copilot_post_add_registry.py` and `scripts/test_project_native_copilot_post_add_registry.py`: separate closed failed-only reader and independently authored synthetic contract envelopes. Reuse unchanged user authority/metadata/config-facts and typed pure primitives, without rewriting evidence into an old mode.
* Existing `scripts/project_native_runtime.py`: exact Copilot CASES test/file/optin selection above (seconds/phase unchanged), and one exact new-mode dispatch to the new reader. Old dispatches/functions stay complete-byte exact otherwise.
* Existing `scripts/test_project_native_copilot_user.py`: only the literal current-selection expectation changes to the new test/file/optin. This is the explicit root-accepted selection exception; preserve every old fixture/helper/outcome assertion, all other complete methods and the remainder of that selection method. Add new-mode selection controls in the new test file.
* Accepted plan and concise factual validation documentation, naming historical/fresh gates accurately.

The old user/empty/readiness Go entrypoint/helper/contract files, metadata capture/projection helpers/contracts, Python readers and old empty/readiness tests remain whole-byte exact. In particular, preserve both strict empty-registry actual process test functions and their whole file. Preserve all production Copilot/shared/VS Code parser/status/lifecycle/registry runner/adapter code, workflow, other clients/CASES, collectors, pins, security scanner and process/environment/authority code. No production or native activation/registry parser fix belongs to this amendment. No new CLI flags, native profile/registry seeding, config allowance, baseline adoption, acquired executable/log fixture or scanner execution is added.

## 6. Meaningful inert and independent-reader controls

Under the explicitly serialized slot, Go pure contracts exercise all five exact argv/cwd/30second labels, every allowed finite stage/prefix and required file set, refusal of skips/repeats/reordering/extra commands/flags/wrong executable/project cwd and60second substitutions. Check18total/17pre-summary caps and both independent2KiB metadata bounds; wrong metadata stage, missing/duplicate/unknown metadata, forged status/type/omission arithmetic and unexpected provider/probe/member refuse. No scalar bool may satisfy integer counts.

New inert actual-process contracts use authored local scripts and the same Go test executable child, never an acquired/native/public installer/scanner or spoofed hosted eligibility. Exercise a clean inert add and one opaque list retaining exact stdout/stderr, including unfamiliar/non-JSON output, while original config bytes/digest/pin maps remain unchanged. Test an add exit7 and an add-time original-config mutation: original f.command must fatally stop and the list marker must be absent. Test list exit7 and list-time original-config mutation: preserve that actual failed prefix and prove no after-guard marker or later command executes. Metadata-only defers must still run and may record finite refused status; they cannot start a child or change the current failure stage. Metadata before/after refusal controls prove no fifth command after public-phase metadata refusal, and no list command after pre-list metadata refusal. Preserve the existing actual /proc capability guard and both original strict process tests unchanged; do not skip or spoof it.

Independent Python synthetic positives cover all finite failed prefixes, both metadata observations and the full18member intentional-barrier envelope, always all-false. Use independently authored inert public JSON and opaque list bytes, not modified captured logs. Coherent negative controls update related lengths/hashes where applicable and exercise missing/partial/extra triples, sixth/repeated command, stage/prefix contradiction, prior failed add followed by list, list claimed completed on error/context/forced cleanup, wrong public schema/command/result at continuation stages, source/layout/pin/scanner drift, all success/auth/model/qualification mutations, count/bool/elapsed bounds, metadata omissions/stages/duplicate path hashes/whole byte caps, invalid UTF8/duplicate JSON and unexpected probe/provider/authority fields. Old mode plus new-only members and new final barrier over the old four-command capture must refuse; do not normalize it into a full new capture.

Source preservation checks compare complete old files/functions, not merely named assertions. Record actual output/exits/source hashes for all checks. Pure synthetic acceptance proves the closed diagnostic schema only, never native registry interpretation or qualification.

## 7. Gates, historical replay, publication and next raw review

Execute locally available new pure Go contracts, the affected runtime/new-mode/old user/empty/readiness Python contracts and exact selector checks under root's serialized slot. Run gofmt/diff checks and affected all-enabled lint against `a412ba5e137fdcb7e004393229b323211ed746e1`, plus whole unfiltered revive/funlen/gocyclo/gocognit/dupl/depguard. Broaden only for a concrete remaining risk. Record the actual commands, source, exits and log digests; do not predict counts or transfer a historical pass as a new execution.

The actual local platform previously lacks the /proc task/children interface required by unchanged OSRunner guards, even unsandboxed. Do not repeatedly run known-incapable local strict process tests, add exclusions to the required hosted command, weaken assertions, introduce skips or impersonate a hosted environment. Compile/pure local checks and historical failures remain accurately labeled. The unchanged workflow supplies the genuine hosted full gate after source publication and before native execution:

```text
go test -json -p=1 -count=1 -timeout=120s ./repotests -run '^(TestCopilot|TestProjectNativeRuntime)'
```

Its conditional pinned Go1.26.10 setup, no-exclusion command, original two strict process cases and the new TestCopilot process contracts must actually RUN/PASS with no skips on the exact new source. Native already depends on admission. No workflow change is needed. Root publication records this fresh hosted gate as pending until measured; neither the old5781 admission nor a local synthetic pass is described as a new hosted pass. The historical5781 receipt is `/tmp/uap-copilot-5781-independent-40604771816/admission-hosted-inert-proof.json`, SHA256 `2133ee2348716dd8c0152d3524bf8beea559b581d954c5f52c96de525cb70f63`, original raw log SHA256 `c58702fbb238d4d4af49981a529ca27e656b9047254af4524a85e5ea660a3f89` (131RUN/131PASS, zero skip/fail, unchanged process file d328951b…).

Scratch pure readers replay original189e readiness,922 failed user add,2c2 empty-registry barrier and5781 successful-public-add barrier artifacts under their original exact source identities. Verify archive/member/summary/raw bytes and hashes unchanged, original failed/pass distinctions and all-false outcomes preserved. The new mode neither retargets nor promotes an old capture. Preserve the already-audited inherited Cline results. No old capture/log/binary enters Git.

Freeze exact source and obtain root plus independent Kimi CODE reviews before root publication; critics verify finite mode hooks, old whole-byte preservation, callback/late-failure behavior, metadata opacity, no rebaseline and no production/VS Code change. Root alone composes current remote state and normal-FF publication; do not retry unchanged or supersede any pending genuine capture.

Only a fresh source-pinned hosted run may supply the post-add listing. Independently audit API/source/tree/build/pins/closed ZIPs, exact public and native argv/cwd/raw streams/receipts, both opaque metadata observations, original process/config/pin guard and intentional or earlier fatal boundary. A clean native list plus review barrier still has all flags/runtime/auth/model false. If the original guard or metadata fails, retain that failure and stop; do not introduce a profile exception. Any eventual parser/activation proposal requires the actual new raw bytes, source-backed diagnosis and its own two pre-code critics. No such product change or registry grammar is authorized here.

## Baseline preservation hashes

The following table is generated from the exact5781 source. Existing mode files/readers/contracts and production/metadata/workflow helpers are whole-byte preservation anchors; the shared attempt/runtime and selector test have only the explicit new-mode/selection exceptions above.

| Path | SHA256 |
| --- | --- |
| `install/integrationctl/agentplugins/clients/shared/copilot_identity.go` | 8344e9718dd9ce9fa21d7a09316ec23620f7350cb742a5ff5cfdab79c9849331 |
| `install/integrationctl/agentplugins/clients/shared/copilot_lifecycle.go` | 1c9d61deffedc62d558d17b0ee27e9b947589b79b53fadfd8b1a12993110ee4b |
| `install/integrationctl/agentplugins/clients/shared/copilot_status.go` | 7fb2f5f03491f036d613217284c830a87efebfe15827038fa43401417be69ec4 |
| `install/integrationctl/agentplugins/clients/shared/native_registry.go` | 9be2175d88e1ab6eeee8d1f0b812fafee5d43a5a67fc039abfd675cef2caa115 |
| `repotests/agentplugins_copilot_project_native_attempt_test.go` | 8cc6cfdc416259c09852873274b07098c9f6ccd43e6b7f919a5f0b4afcad7be0 |
| `repotests/agentplugins_copilot_user_registration_attempt_test.go` | 45447d0a35f8dede1f95d505f098c4485f3695402f76ccdfabd0e4100a8cb36d |
| `repotests/agentplugins_copilot_user_registration_contract_test.go` | ca23e14493bf907f20a2e2af2b5d9cde1f6b32f961719add33805b804a3e14d1 |
| `repotests/agentplugins_copilot_empty_registry_attempt_test.go` | 9e78c5fb43db2ecf4007e9dd271a4aa26010f75b96879cb30633e4419e8e3caa |
| `repotests/agentplugins_copilot_empty_registry_contract_test.go` | ac7cba746373eaf9ecc0993db4959d93919775beaa079877e48678eba285e08d |
| `repotests/agentplugins_copilot_empty_registry_process_contract_test.go` | d328951b55a897eeb81783216a80d6f7f8d2fbcee3328b9f491d2d6dab210b6c |
| `repotests/agentplugins_copilot_user_metadata_test.go` | db5f1d60f4c1e0934ab32e90c39e654a1368beb90b0c63c462f7908a559c0b15 |
| `repotests/agentplugins_copilot_user_metadata_projection_test.go` | 7f298659c71a0c4b529840380920a1b5ee333628158479967c6c2766feaea689 |
| `repotests/agentplugins_copilot_readiness_completion_test.go` | 9df4e392cb97e3365374201f44163552c9b99f60c9f5f54cb8f8891b09abdea8 |
| `repotests/agentplugins_copilot_readiness_contract_test.go` | 57f2bf81b41627ed342a8efc00f634c3e36bf19cc4c7f16897da88bd66a7d795 |
| `repotests/agentplugins_copilot_readiness_events_test.go` | 2a4683b6bded2ccad5f30b883f3800af84c0284e5ddaab4ab5d969a6007d2d47 |
| `repotests/agentplugins_copilot_readiness_exchange_test.go` | c641a4881f6393176a70f5c53388ed7ea9143f78b28faa7dbe885895ac6b7c1b |
| `repotests/agentplugins_copilot_readiness_identity_contract_test.go` | f51bc243f72be24e4615348fe2156be4759b9e00a1df265a3403ac57f9ded6a6 |
| `repotests/agentplugins_copilot_readiness_installed_test.go` | e478d85a649aedd6af7106079aeebf7724d5f810cff0a8e13b114faa3b7999a9 |
| `repotests/agentplugins_copilot_readiness_metadata_test.go` | 1a7a184227f50d575d61499acd52c9dbeb17e70efee4281f3ce27ef774ea7a72 |
| `scripts/project_native_runtime.py` | 0be88d5590ac950a64903ad22adc3b3b12624c9bd4341f0c5340beeac8674984 |
| `scripts/project_native_copilot_user.py` | fe75f19d141b755bf19c1ee7605963fa2f045bdcbca5690b38ea044609bb42c5 |
| `scripts/project_native_copilot_empty_registry.py` | 4bbf4e5e34ff8e908a867744beb14228fadfc5e91d8aacc60b242a8f96f1922e |
| `scripts/project_native_copilot_readiness.py` | 98cb758e0b6b1dcfe4953c8032e51f67fb2afe974e6887ca862c3c70de518cc6 |
| `scripts/test_project_native_copilot_user.py` | 299e89a5d0138a5e4e43cae955b03d704c627502ca02d2fe67ec8971581a610a |
| `scripts/test_project_native_copilot_empty_registry.py` | 48ea493b5c1d177183d39b6f5a08196bf8a3940695d76b820ccab5127dee1f3f |
| `scripts/test_project_native_copilot_readiness.py` | f339b7703c2d2358f953bd040285e242c1aef0db9bf4a73a3f1c41b1fd54dbdd |
| `.github/workflows/project-native-runtime.yml` | 6d50637892ee2a387052b42d5d9a53a8db68e7a54aa2d04272f49425cb17e3df |

Draft work performed source and already-audited raw reads only. No implementation, tests/lint/native execution, Git/GitHub mutation or publication has occurred for this proposal.


## Accepted pre-code plan: Copilot exact user mixed live listing (2026-10-28)

The following frozen proposal is accepted for its four code paths and two existing documentation paths. Its retained draft label records the original reviewable proposal; this acceptance authorizes implementation only after this root-owned pre-code commit.

Base source: `85dfdaeaa217cf74f5c5a6464426cd1b67c2f069`; frozen draft SHA256 `061e3db491efc478921c130f22ae1888638d6936ba61b3095ce5c7aa41c760df`. Independent PLAN GOs: root `790e496e8d7ad30f862ae60fa627308c900cb40a19def7664ec811afae44ea90`, Kimi `16d1a95b203da5210edb53873e77896fd118d7ef77af9b7b93b64e4dc83413f3`. Both critics verified all 27 source files, 206 declaration anchors and five evidence receipts. The new-only strict recognition, exact old fallback semantics, and internal-verification observability limit are required. No qualification or release is authorized.

# Copilot: exact disabled-builtin plus managed live-entry recognition

Status: **PRE-CODE DRAFT ONLY**. No implementation, parser change, native retry,
test or lint execution is authorized by this document. Root and a second
independent critic must approve this exact draft; root must commit the accepted
plan before any source edit. Root owns all Git/GitHub writes and publication.

## 1. Exact source, actual evidence, and decision

Plan base is clean source `85dfdaeaa217cf74f5c5a6464426cd1b67c2f069`, tree
`10d2b3810d11b8e8fe984f0ab19d28bab4317faa`, already available at
`/tmp/uap-copilot-post-add-registry-current`. Do not retarget that evidence to a
future source identity. No new checkout or acquired executable is needed locally.

Actual run40613833246, attempt1, native job122571919197 completed with the
intended failed-only review barrier. Admission job122571555875 passed; inherited
Cline job122571919162 also passed. These are separate outcomes. This plan does
not turn the native diagnostic job into a passed runtime test.

Both independent fresh whole RAW reviews are complete:

| Reviewer | Receipt | SHA256 |
|---|---|---|
| Root R1 | `/tmp/uap-copilot-85df-root-40613833246/root-raw-audit.json` | `d54a31c9bb6d772e7793ece8877eb9305bfc027cd24643ca99ea8e5ea0dbabf1` |
| Independent R2 | `/tmp/uap-copilot-85df-independent-40613833246/copilot-independent-raw-r2.json` | `c0b45dcf49faf16452d20b8cfbd1fbbcaadfa32ae80a8b6e7b2cb54692f066ef` |

Both reviews bind own original ZIP/API identities and all raw/member/stream,
source, build, compiler, pin, public result, direct-list and metadata joins.

| Original artifact | Outer SHA256 | Inner SHA256 | Members |
|---|---|---|---:|
| Native12784890825 | `f07b24bf429c1623e0825ba0dd7e46924914c2f284bcef61a194c48cbbfcf57b` | `1c6df23cc53a8028f18923382edcaec40e8834789a9b6ec1e9c394575634ff06` | 18 |
| Frontend12784436728 | `017d43a3c46d84eb04b2bbc4647fc2d18512a922ceaefb900bcbcbdc3493124e` | `9f189986f8c015fd5f83d878734091ac223b51c90fa0af66844801ed0287f938` | 25 |

The actual five commands were git-A, git-B, version, public add-user, and one
direct native-list in the same original HOME/outside directory. All had exit0,
error=false, context_error=false and ordinary descendant cleanup. Add took3509ms;
list524ms; total7088ms. Sole failure was the deliberate review barrier at
`agentplugins_copilot_post_add_registry_attempt_test.go:346`. No probe/provider
evidence exists; all five qualifications and runtime/account/model flags are
false.

The 396-byte LF stdout, SHA256
`eb239f40f75369e78916a00e3700e3a75b695d35cd0e48a52b4ce531690a1373`, has:

1. the exact 56-byte fixed prefix `Built-in plugins:` plus one disabled
   `browser-use (v0.0.4)` line;
2. the existing exact `CopilotLiveHeader`;
3. one `native-proof@agentplugins-71b878b366b9 (v1.0.0) (enabled)` entry;
4. one six-space `from ` line at the independently derived managed physical
   path `.../installer-state/managed/clients/copilot/native-proof-024a9ef8a9f0`;
5. exactly one final LF and no other bytes.

The fixed-prefix SHA256 is
`e83a8da5c004a1bfd6aa6645aacbe882ff506bf311b6a6c8c34833533e25caa2`.
The direct stderr is exactly empty. Dynamic path/marketplace/physical identity
were independently recalculated from public installation UUID
`5cfee740-c7d0-4cee-83d1-b23f462a6e63`; no raw string was adopted as ownership.
Raw logs and original captures remain scratch evidence, not repository fixtures.

Public add emitted successful mutation, but its activation is still
`manual_activation_required`, authentication `not_checked`, policy `allowed`,
verification `package_validated`. The generic live parser requires
`CopilotLiveHeader` as the first line; the captured later direct document begins
with the builtin section and has no independently valid legacy Installed
section. Applying the current verifier to these exact captured bytes therefore
gives Unknown. The observed public manual result and reviewed source identify
the existing `ErrCopilotListContractUnknown` fallback as the repair boundary.
The installer's internal verification stdout is not exported, so matching that
internal stdout or its precise parser branch to the later direct document is a
source-supported inference, not a separately captured branch trace.

**Selected candidate:** add exact recognition only for this measured whole
shape in eligible Copilot user requests, route both registry preflight and
activation verification through it, and reuse the current failed-only18-member
post-add capture unchanged to verify the real repaired public activation.
No new mode, selector, evidence format or metadata rule is needed.

## 2. Source trace and preservation boundary

Source and evidence hash inventory:
`/tmp/uap-copilot-user-live-builtin-source-context.json`, SHA256
`96c01f919aa21ef4c7acd2ada44c6b91f78f01b15b807c4dc1e841b6ef3b9da1`.
It contains exact source commit/tree, 27 whole-file hashes and declaration-line
anchors, the measured prefix hash, and the relevant original review receipts.

Important call sites and invariants:

| Source anchor | Current behavior / requirement |
|---|---|
| shared/copilot_status.go:40 `CopilotLivePluginStatus` | Header-first shared Copilot/VS Code parser. Preserve this entire file byte-for-byte. |
| shared/copilot_identity.go:39 `inspectCopilotCLI` | Runs the held `plan.NativeRegistryExecutable plugin list` via existing tree-aware registry runner; handles errors/nonzero before classification. Add only one eligible positive branch after expectedPath selection. |
| shared/copilot_identity.go:69 `copilotUserEmptyWithDisabledBuiltin` | Existing exact empty+disabled builtin admission. Preserve function and tests whole-byte exact. |
| shared/copilot_identity.go `CopilotRegistryFindingAt` | Shared generic grammar and owned/unowned mapping. Preserve function and all scanner helpers byte-for-byte. |
| shared/copilot_lifecycle.go:127 `VerifyCopilotPlugin` | Executes existing read-only listing. Add eligible exact recognition before its unchanged generic status switch. |
| shared/copilot_lifecycle.go:19 `CompleteCopilotActivation` | Existing Active/Installed success, unknown/manual, and error/failure handling. Preserve this whole function. |
| clients/copilot/lifecycle.go `Adapter.Activate` | Checks client identity; dispatches project requests separately; user requests call shared completion. Preserve entire adapter. |
| clients/copilot/identity.go `InspectNativeRegistry` | Dispatches project observations to component-specific inspection; user to shared inspector. Preserve entire adapter. |
| clients/vscode/lifecycle.go / identity.go | VS Code uses the same shared completion/inspector. Preserve all dedicated source and behavior; exact new branch must be unreachable for ClientVSCode. |
| shared/marketplace.go:13 `ManagedMarketplaceName` | `agentplugins-` + first12 hex SHA256 of trimmed physical ID. Reuse existing function; do not change derivation. |
| shared/command.go / native_registry.go | Original command/error and tree-grace behavior. Preserve whole files; no extra command, version probe, environment gate, or wrapper. |
| planner/planner.go | Holds detected executable in NativeRegistryExecutable. Preserve planner and all profile/ownership machinery. |
| post_add_registry_attempt_test.go:335 exact test | Existing ordinary public phase, metadata-only defer, ordinary list phase, final barrier and prefix/member guards. Preserve entire file. |
| readiness_completion_test.go:98 | Original readiness may complete but all five qualification flags remain false. Preserve finalization and every old reader. |

Whole-file hashes most useful for final review:

| Preserved file | SHA256 |
|---|---|
| shared/copilot_status.go | `7fb2f5f03491f036d613217284c830a87efebfe15827038fa43401417be69ec4` |
| shared/copilot_empty_builtin_test.go | `fc746bb5b7df509c4ac931982c17d019da53cbf8e6c52c69e66e9cebfe5d1d0b` |
| shared/copilot_identity_test.go | `aea80041bc701a5813a3be96773a7ef849701ad3ca39ee10bfa66ecdfa00005c` |
| clients/copilot/lifecycle.go | `75aa0d8a7863d463e6c77e276084682f69e3802acb7679da8d7d5cbbeddedd72` |
| clients/vscode/lifecycle.go | `433edd4b7b41608a90c0737d3331b39b6785cc1d4ec51a613672031316b50ff2` |
| repotests/agentplugins_copilot_project_native_attempt_test.go | `645e31328ee73f708a45f91e891c5c3031a947c972937fa8fec04be2b1b3f14d` |
| repotests/agentplugins_copilot_post_add_registry_attempt_test.go | `405863943f307505d90e0c91856e26e60436b486fd19d00336d49f9f6152ac9b` |
| original empty-registry strict process contracts | `d328951b55a897eeb81783216a80d6f7f8d2fbcee3328b9f491d2d6dab210b6c` |
| new post-add strict process contracts | `0177b11697e3a2c2e655dd7165576be564e0a2f42bd4b767e41515e646474b75` |
| scripts/project_native_runtime.py | `2327a1f1948566ca1d3296444943d15b6d2ee0303520da823fdec6b6b0c3a0e7` |
| scripts/project_native_copilot_post_add_registry.py | `4996d333006a3bfc4aeae8f0ead56d8c7b0953ef45844a822150ed2d4bec9666` |
| scripts/project_native_copilot_user.py | `fe75f19d141b755bf19c1ee7605963fa2f045bdcbca5690b38ea044609bb42c5` |
| scripts/project_native_copilot_empty_registry.py | `4bbf4e5e34ff8e908a867744beb14228fadfc5e91d8aacc60b242a8f96f1922e` |
| scripts/project_native_copilot_readiness.py | `98cb758e0b6b1dcfe4953c8032e51f67fb2afe974e6887ca862c3c70de518cc6` |
| .github/workflows/project-native-runtime.yml | `6d50637892ee2a387052b42d5d9a53a8db68e7a54aa2d04272f49425cb17e3df` |

Do not edit old generic, VS Code, user/empty/post-add/readiness readers or
contracts, runtime selector, production profile/ownership code, workflow or
capability implementation. AGENTS.md, the owner scope clarification and the
capability-preservation contract remain controlling. No legacy deletion,
implicit YAML fallback, release or VS Code qualification is in scope.

## 3. Exact new recognition and eligible routing

Add a private, small Copilot-only helper in a new shared-package file. This is
an exact positive observation; it is not a replacement generic listing parser.

### Held caller and identity prerequisites

The plan has ClientCopilot and explicit ScopeUser. TargetContext.Scope is user
or the existing unspecified default, never project/local; ProjectRoot is empty.
The held registry executable is nonempty absolute clean, NUL/CR/LF-free and
equals the executable actually used for this existing command. No new process
or filesystem acquisition is performed to establish that equality.

The declared name is nonempty, trimmed, valid for the existing ASCII live-entry
identity grammar, and is not case-insensitively `browser-use`. The physical ID
is nonempty, trimmed and valid as an existing namespace identity. The expected
marketplace comes only from `ManagedMarketplaceName(plan.PhysicalArtifactID)`;
the expected version is the exact `MarketplaceVersion(plan.DeclaredVersion)`
value validated by the existing live-entry version grammar. There is no version
wildcard, environment-spoof gate, or vendor-version command.

The expected managed path is an absolute clean non-root NUL/CR/LF-free path,
equals the held plan.ActivePath, and has basename plan.PhysicalArtifactID. Bind
the exact path; do not normalize a received path. Bound dynamic identity strings
to256 UTF8 bytes and path/executable strings to4096 bytes; bound this new whole
document comparison to8KiB. These are new-only refusal caps and do not modify
any generic parser or process capture bound.

For inspection with a managed binding, additionally require that this existing
binding is Copilot/user and its physical ID and target locator equal those held
plan values. This branch never creates ownership. The same exact expected entry
without an owned binding is Collision, not Expected or Clear.

For activation verification, require the request's Client.ClientID and
Delivery.ClientID also equal ClientCopilot; request.DeclaredName equals the held
plan name; request.BackendExecutable equals both the plan's held registry
executable and request.Client.ExecutablePath; Delivery.ActivePath equals the
held expected path. Existing identity mismatch and project dispatch stay exact.
These checks apply to the new branch only; legacy generic behavior stays intact.

### Exact whole bytes

For admission through the NEW helper, require existing command success with no
runner error/nonzero exit, a live original context at admission, and exactly
empty stderr. Its strict identity, context, byte-cap and stream requirements
apply only to this new branch. Accept stdout through that branch only when it
equals the constructed whole document:

```
Built-in plugins:
  • browser-use (v0.0.4) [disabled]
Live Plugins (loaded from a local marketplace directory, never copied):
  • <held name>@<derived marketplace> (v<exact held version>) (enabled)
      from <exact held managed path>
```

There is exactly one final LF. Do not strip a prefix, split away an unknown
section, normalize line endings, trim incoming bytes, accept another builtin,
ignore arbitrary suffixes, accept multiple live entries, or infer absence from
an unrelated entry. Reject empty/extra/duplicate/reordered headings or entries,
unknown status/version/path/name/marketplace, enabled builtin, missing builtin
state, ANSI/NUL bytes, whitespace variation, CRLF, missing final LF or extra LF.
All such documents fail the NEW helper and continue through each old unchanged
classifier/fallback with its exact prior result. The new helper itself grants
neither absence nor Clear on a mismatch. Do not strengthen or broaden legacy
behavior while adding this branch.

In particular, the generic verification function CopilotPluginStatus deliberately
skips a preamble while scanning an old `Installed plugins:` section. Appending
an independently valid exact expected Installed entry to the measured mixed
document can therefore still return Installed through that OLD grammar, although
the NEW helper refuses the extra bytes. The old generic inspection function
CopilotRegistryFindingAt instead rejects this preamble and remains Indeterminate.
Those two historical results must stay different and unchanged. Similarly, old
recognized verification grammar keeps its prior stderr/context/size handling;
the new branch's empty-stderr/live-context/8KiB rules do not rewrite it. Finite
malformed-new-shape fixtures with no independently recognized legacy section
continue to produce their original Unknown/manual result. No new legacy
allowance is introduced by preserving an existing positive result.

### The two edits to existing production functions

1. In `inspectCopilotCLI`, preserve the exact existing command and all original
   error/exit handling. After existing expectedPath selection and before the
   unchanged generic return, apply the new eligible exact positive check. Recheck
   ctx.Err before a positive result. Return Expected only for the matching held
   owned binding; otherwise Collision. Existing exact empty+builtin admission
   and all generic classifiers/scanners remain byte-for-byte unchanged.
2. In `VerifyCopilotPlugin`, preserve its command and all runner/exit failures.
   Before the original generic status switch, accept the exact eligible user
   observation above, with empty stderr and ctx.Err rechecked. Otherwise use the
   original switch unchanged. Do not change CompleteCopilotActivation,
   ManualCopilotVerification, install/register/update/uninstall behavior,
   attestation, recognized-negative errors or compensation.

VS Code requests fail the new branch's ClientCopilot guard before recognition.
Direct shared generic functions still return their old results on exactly the
same prefixed input. All existing legacy Installed and header-first live forms
continue through their old paths and tests. Project Copilot remains the original
component/native-config adapter, not this user marketplace route.

## 4. Concrete allowed files

Only these four code paths are proposed:

1. NEW `install/integrationctl/agentplugins/clients/shared/copilot_user_live_builtin.go`
   — private exact helper and small caller-context predicates.
2. NEW `install/integrationctl/agentplugins/clients/shared/copilot_user_live_builtin_test.go`
   — independently authored synthetic whole documents and inert runner contracts.
3. EXISTING `.../shared/copilot_identity.go` — only the explicit positive hook in
   `inspectCopilotCLI`; every other function remains exact.
4. EXISTING `.../shared/copilot_lifecycle.go` — only the exact eligible check in
   `VerifyCopilotPlugin`; every other function remains exact.

Root may append the accepted plan and final measured validation to the existing
`docs/COPILOT_POST_ADD_REGISTRY_VALIDATION.md` and the governing consolidated plan
`docs/plans/project-install-e2e/remaining-native-qualification.md`, following the
established precode-plan workflow. These are the two allowed existing docs paths.
No Python, runtime selector, mode, workflow, Go process-contract, production
adapter, VS Code, old test or raw fixture path is authorized to change. If an
unexpected need arises, stop and report the concrete dependency before editing.

## 5. Meaningful independently authored contracts

Use a new pure inert runner: no native executable, installer or scanner process.
Inputs use new `/inert/...` paths/IDs, not copied hosted captures. Keep the exact
public fixed prefix; derive its expected test identity independently. Cover:

- exact measured shape / exact name-version-marketplace-path;
- owned inspection Expected, unowned inspection Collision, wrong owned binding
  Indeterminate, and no Clear granted by the new branch for
  unrelated/missing/wrong live identities;
- one read-only verification call returns recognized success through the NEW
  helper only in eligible Copilot/user context; shared completion then returns
  Active/Installed while preserving authentication policy. Independent old
  recognized-grammar success remains unchanged for its original callers;
- both initial activation and VerifyOnly use the same exact recognition; initial
  inert transcript keeps existing marketplace-add/install/list commands and
  ordering, with no extra command or version probe;
- ClientVSCode/other/empty IDs, project/local/unspecified plan scope, conflicting
  TargetContext, executable/client/delivery/name/path mismatch, same browser-use
  name including case variants, invalid/canonicality/byte-bound identities;
- each finite NEW-helper whole-byte refusal in section3, disabled expected live entry,
  builtin version0.0.5/enabled/missing state, second unrelated/duplicate entry,
  unexpected stderr and command error/nonzero/canceled-context behavior; these
  are new-branch admission controls, not changes to old recognized grammar;
- malformed/unknown prefixed input retains each original fallback result and
  explicit attestation behavior. Add the concrete measured-document-plus-valid
  Installed-section counterexample: NEW helper=false,
  CopilotPluginStatus=Installed, CopilotRegistryFindingAt=Indeterminate.
  Also cover old recognized verification with nonempty stderr: the new helper
  refuses it while the old entrypoint keeps its prior result. Cases without an
  independently valid old section must still exercise Unknown/manual/failure;
- direct generic CopilotLivePluginStatus, CopilotPluginStatus,
  CopilotRegistryFindingAt still refuse the unmodified measured prefixed input
  exactly as before, while the independently valid legacy-section counterexample
  preserves each function's distinct prior result. All old
  empty/Installed/header-first live tests stay unchanged;
- inspection preserves RunNativeRegistry's existing TreeCommandRunner selection
  and one-second grace; verification preserves RunClientCommand's existing Run
  path and original errors. Do not silently change one runner route to another.

The unit contracts must test outcomes through both affected production entry
points, not only mirror the new helper. There is no reason to alter any Python
grammar: run all existing affected Python contracts and historical raw readers
unchanged. A NEW-helper positive must never be inferred from a failed command or
nonempty diagnostic stream. Do not extend this guarantee into a change to the
preserved old verification entrypoint's recognized-grammar stream handling.

## 6. Gates on the final immutable candidate

After two PLAN GOs/root precode commit, implement only the allowed paths. Freeze
and hash source; root commits. Two independent CODE critics read the exact
commit/tree and compare unchanged functions/files. Do not rely on author review.

Run only with the parent-owned serialized slot and sufficient disk. Record every
actual command, exit and full log hash. No native/acquired client/public installer
or scanner may be executed locally. Never spoof a hosted environment, weaken a
process assertion, skip a required contract or claim a local full process pass.

Required checks:

1. Full affected production Go package tests (shared, Copilot, providers and
   usecase, plus the existing provider/generic regression coverage). Newly
   authored tests are pure inert contracts; any concrete local process-interface
   limitation must remain an actual failure and be handled through genuine
   hosted coverage, never an exclusion disguised as a full pass.
2. Existing repotest pure4/package compilation and all67 affected Python
   contracts, with their source whole-byte unchanged.
3. Affected all-enabled lint with the unchanged project policy/base revision,
   and whole unfiltered six `revive,funlen,gocyclo,gocognit,dupl,depguard`, zero
   issues required. Actual lint findings may justify only reviewed equivalent
   fixes; do not weaken lint policy.
4. Full current-module immutable original-source raw replays for189e,922,2c2,
   5781 and85df. Preserve ZIP/CRC/member hashes and original tuples.189e remains
   readiness-complete/test_failed=false with all5 qualifications false;922,2c2,
   5781 and85df remain failed/runtime-incomplete/all5false. Never rewrite them as
   repaired Active evidence. Keep actual source/tree identities from each raw.
5. Verify all old Python reader/test files and all repotest/process files,
   workflow, shared generic parser, dedicated adapters and owner-scope anchors
   byte-for-byte. In the two modified production files verify all non-target
   functions byte-for-byte. No selector literal exception is needed this time.
6. Fresh genuine hosted full unexcluded process gate on the published candidate:
   `go test -json -p=1 -count=1 -timeout=120s ./repotests -run
   '^(TestCopilot|TestProjectNativeRuntime)'`, actual Go1.26.10, every original
   and new strict phase/fatal parent RUN/PASS, zero skip/fail.137 prior passes
   are historical, not a substitute for the fresh candidate gate. Existing
   normal CI must also cover the changed production package tests.

Actual85df hosted process proof has137RUN/137PASS/0skip/0fail and raw job log
178857bytes SHA256
`3cb0300849cdade6bab99db0e39c00d5b08c405b6ee95f063a54dd3605d05a76`.
Root receipt `/tmp/uap-copilot-85df-hosted-process-gate/root-process-gate-audit.json`
SHA256 `fb89aa706559d5163e96c741ef4b4acd82cd935dd54f7ad48145636c0273fd14`;
independent RAW R2 re-downloaded and verified the same complete hosted log.

## 7. Actual native follow-up and repair acceptance

After all source/CODE/local/hosted gates and root publication, make one genuine
current-source run of the **unchanged existing post-add selected test**. Keep the
same Copilot1.0.103 binary/archive and scanner pins; original HOME/outside;
30-second command bounds; original150/180-second origins; descendant/config/
immutable/profile guards; two separate metadata-only ordinary phase defers; five
commands;18-member maximum; and all failed-only flags. Do not schedule an
unchanged retry or supersede an active capture.

Repair acceptance is concrete and stronger than merely reaching the barrier:

- all five command receipts clean and exact original argv/cwd;
- public result successful mutation, same ordinary schema/operation/UUID/physical
  derivations, authentication `not_checked` and policy `allowed`, but activation
  now **Active** and verification **installation_verified** through the existing
  recognized-success path, without activation/user attestation;
- no observed manual-contract fallback, FailedActivation or attestation in the
  fresh public result; the reviewed source and inert production-entrypoint
  contracts must support the candidate recognition path;
- native list retains the exact independently bound expected live entry and
  fixed disabled builtin tuple, stderr empty; both opaque metadata observations
  valid within original bounds; no config-change diagnostic or unexpected probe;
- sole final test failure remains the deliberate review barrier; runtime and all
  five qualifications/account/model flags remain false.

The unchanged18-member capture exports the public add result and the later
direct native listing. It does NOT export the installer's internal
VerifyCopilotPlugin stdout. Fresh public Active/installation_verified without
attestation, the independently bound direct listing, reviewed source and inert
entrypoint contracts together establish the observed repair behavior. A claim
that the internal NEW helper, rather than another preserved recognized grammar,
handled that unexported internal document remains a source-supported inference;
it is not an independently captured branch trace. No instrumentation, new
capture member or evidence-format change is authorized or needed for this slice.

The unchanged capture/reader intentionally permits historical manual public
responses as diagnostic evidence. Therefore `runtime_artifact_admitted=true` or
the barrier alone is not repair proof. Two independent fresh whole RAW reviewers
must explicitly compare the observed public activation/verification and later
direct listing with the reviewed source/contracts, while retaining the internal
stdout observability limit above. A still-manual result, a new unknown document
or an earlier fatal is a preserved new failure, not a reason to broaden this
parser speculatively.

## 8. Finite continuation toward full project E2E: deliberate tradeoff

The existing189e source `189e00d364ccde805c9ffc6738e61fc58ae277fc`, tree
`3f6dd05e28e7c6ab009c37d41cf804d6c3907743`, already has a genuine successful
21-member installed-tool-readiness capture. It proves one project component
installation and one fresh native27-frame/2-provider-request exchange with one
owned write call and3probe events. Its reader/finalizer explicitly keeps all five
project/native lifecycle qualification flags false. That useful proof stays
preserved and replayed.

Current project setup dispatches independently to repository `.github` skill/MCP
components and Prepared activation; it does not use the user marketplace
verification being repaired here. The current readiness test has no native
User/A/B isolation matrix, repeat-add/unchanged-update/intact-repair/update/
missing-repair/default-remove/repeat-remove lifecycle, fresh-session retained
marker reads after transitions, or native post-removal absence. Its single
observed27-frame grammar is not evidence for unseen empty/removed catalogs or
read/update/restart wire sequences. A same-mode rerun cannot fill those gaps.

Reusing the current18-member post-add mode first is the smallest candidate that
actually tests this user registration repair. Restoring the old readiness
selector now would exercise the separate project component path and would not
show whether the actual manual fallback was fixed. Combining both tests under a
regex would also collide with their exclusive output ZIP and alter the bounded
single-test frontend contract. Adding another alias/mode or combining HOME/
fixture authorities is unnecessary for this repair.

After the repaired user capture has its own two RAW GOs, the next reviewed slice
should construct a **proper finite project lifecycle** using the existing
transaction/state/data/source and measured readiness seams, while retaining
Prepared/auth-not-checked production wording. An eventual lifecycle candidate
must author independent Go/Python semantics, exact per-phase source/binding/data
and catalog joins, fresh native sessions for installed/updated/repaired/restarted
and removed states, honest retained-data/marker behavior and ordinary final
cleanup. Any newly encountered native document must first remain a bounded
failed observation; no unseen wire shape is invented or qualified by this plan.
The exact lifecycle sequence, budgets and new evidence schema require their own
two source-backed precode reviews before implementation. This plan authorizes
neither that larger implementation nor early promotion of any qualification.

## 9. Exit conditions

Return this frozen draft and source-context hash for two independent PLAN
critics. Keep source clean and do not run tests/lints while another worker owns
the gate slot. After acceptance, root commits the plan, author implements the
four allowed paths, both CODE critics review, actual gates are recorded and root
publishes. Fresh native review establishes whether the repair occurred. Keep all
prior raw outcomes and every limitation visible until the later full project
E2E evidence is actually complete.

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
the exact source/pins and25closed members. Two independent full raw audits
confirm this failed capture and its unchanged all-false result.

Two independent pre-code reviews accepted one bounded skills-only
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

The implemented leaf24dcb5f0 has two independent exact-source CODE GOs and
preserves every existing failure/ownership gate. Its composition with qualified
program parent73208da2 has two independent reviews: all owned Go bytes remain
identical, and the two shared Python files retain each reviewed client branch.
On the composed source,64 focused top-level Go contracts pass (274 pass events;
one unchanged child-only helper skip), the real inert public CLI repair contract
passes,82 Python contracts pass, and affected all-enabled plus complete six-linter
gates for both Go modules report zero issues. The original f794 native25-member
and frontend25-member captures still replay to the identical failed/all-false
result. These local gates do not qualify native behavior; the published complete
cycle requires one fresh genuine hosted capture and two independent raw audits.

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

## Copilot architecture ratchet amendment before project lifecycle (2026-10-28)

Genuine current-source run40624054507 at838c3e19/tree0aa6dadd reaches only
the existing post-add review barrier. Public user add now reports Active and
installation_verified without attestation/manual fallback; its later direct
listing retains the exact disabled builtin and enabled managed entry. This is
user repair evidence, not project lifecycle qualification. All five/runtime/
account/model flags remain false. Independent whole RAW review remains required.

Core40624054521 exposed TestClientIDBudgetDoesNotGrow: five ClientCopilot
references in clients/shared violate the unchanged architecture ratchet. Root and
independent other_clients reviewed and approved this narrow amendment before
implementation. Move the two exact user builtin recognizers and their complete
strict contracts into clients/copilot, where client identity policy belongs.
Shared orchestration receives explicit optional recognition callbacks after its
original successful command admission. Existing exported APIs pass nil, and the
Copilot adapter alone supplies its own recognizers on the existing user path.
Keep the same single registry/final verification command and unchanged generic
fallback. Positive recognition retains its original postrecognition context
check; preserve the older generic fallback's existing cancellation behavior.
Reuse the shared live grammar only after whole-byte exact tuple equality.
No budget increase, VS Code source change, new native command, evidence/schema/
reader/workflow/fixture change or broader lifecycle implementation is authorized
by this amendment.

Candidate validation must cover the moved strict scope/identity/ownership/path/
version/byte/context suites, nil callback legacy behavior, refusal to invoke
recognition after command error/nonzero exit and the original one-list install/
VerifyOnly sequence. Run the architecture package and full test-core plus
affected package/lint gates when execution is available. Remote-object preparation
does not execute these tests and must not be represented as passing them.

The implemented callback/matcher move received independent GPT-6.1 Sol/xhigh
CODE GOs from both client reviewers and a separate root source review. The
unchanged focused shared/Copilot/VS Code and architecture selection passed
157 cases with no skips or failures. Two independent original-byte audits of
run40624054507 verified both API-bound archives, 18 native and 25 frontend
members, all 24 frontend stream joins and the original failed-only reader.
They confirm the user repair result above, while every lifecycle/MCP flag
remains false. Full core, lint and fresh hosted checks remain required.

## Accepted Copilot complete User/A/B lifecycle plan (2026-10-28)

Final precode draft:24,324bytes, SHA256
`7f80f6896eece51b58c84b90b4f76535ce665b1784c648c3c9960a73a6b58438`.
Independent other_clients and Grok reviewers rehashed/read this exact revision
and returned renewed GO. It supersedes the earlier4a draft: source inspection
found Copilot user projection preserves portable MCP/data semantics, while only
project scope binds UAP managed-stdio/data. Do not assume native User PLUGIN_DATA
equals a UAP receipt or survives native uninstall. The goal-correct User fixture
is skill-only, matching the existing Cline control. A/B retain actual MCP/data
qualification; no User-MCP claim, production change or artificial second server.

Implement one complete bounded cycle with original HOME, separate A/B Git roots,
immutable Copilot1.0.103/scanner/build pins and original150/180-second origins.
17 public commands cover Git/version, user/A/B add, repeat add A, unchanged
update A, intact repair A, A-two update, exact missing-component repair A and
remove/repeat-remove A/B/User. Intact repair truthfully mutates; repeated removal
retains exact supported exit1/unchanged state. Keep direct-local A-two source
frozen: repair joins its recorded digest and restored bytes, without a moving
source/store-fallback claim. Installed skill-only User has no active DataReceipt;
default removal's real retained-owner allocation is verified from prior absence.

15 direct native lists preserve the exact measured managed User tuple until its
removal, then the exact empty/disabled-builtin tuple.14 fresh native sessions
include6 project one-call/two-turn sessions (2 writes/4 reads) and8 no-call
catalog sessions, with20 provider exchanges and6 initialize/list/call triples
(18 probe events). User plugin skill is visible until its removal; User supplies
no MCP tool. A/B independently join source, installation/binding/native/data
receipt identities, fresh sessions, retained markers, update/repair and removal.

Preserve two exact original native skills: HOME
`.copilot/skills/uap-foreign-user/SKILL.md` and B
`.github/skills/uap-foreign-b/SKILL.md`, with declared descriptions/frontmatter/
nonce bodies and exact source/path/enabled/userInvocable/commandName catalog
rows plus ordinary inode/owner/mode/byte checks. User foreign is always visible;
B foreign only in B, including final absence. Neither may be invoked. Use
supported `--excluded-tools` for19 observed builtin tools and omit
`--available-tools`; unknown MCP tools remain visible. Require whole catalogs
of2 measured builtin skills, applicable original foreign/phase-owned skills and
5 disabled builtin servers plus the phase-owned A/B connected server. No-call
phases have zero fixture MCP entries/declarations/probe activity, with exact
phase-owned User skill presence before User removal and none finally.

Reuse finite known event families/one-server call graph and their no-call
one-turn projection, not a new event framework. SDKRPC blob
`d269c4626b83c444a06bd14bf4c6ea77fe734a5a` supports array/provenance
semantics; shipping equivalence is unclaimed and unknown actual output fails.
Complete evidence has60 ordinary members:17 public triples, eight closed
stage/value JSONL aggregates for native events/commands/registry/provider
requests/responses/receipts/probe/proof and one runtime summary. Raw streams
retain exact bytes/SHA256/base64 joins. Preserve1MiB aggregate/16MiB total and
old ZIP bounds/modes. Go/Python independently validate every phase/source/
receipt/catalog/call/result/data/absence join. Only full ordinary descendant/
provider closure, original-authority checks and complete capture can set all
five project/MCP flags. Project Prepared wording, vendor-authentication=false
and real-model=false remain truthful. Two exact CODE reviews, inert gates,
original/current RAW replay and fresh actual hosted whole-evidence review remain
mandatory. No artificial review barrier, local vendor run, production/VS Code/
pin/workflow change or unchanged-source retry is introduced.

### Measured Copilot User command name correction (2026-10-28)

Actual d32d8fd6 run40734424271 completes the first installed-User native process
with15 exact events, one provider exchange, ordinary shutdown and zero probe
bytes, but fails the whole catalog check. Two independent full raw audits bind
both API artifacts,21 native/25 frontend members,24 frontend stream joins,
source/build/pins and exact User source/state/native/no-data receipts. The sole
catalog mismatch is the plugin skill commandName: actual
`native-proof:uapcopilotuser`, while the original prediction was `uapcopilotuser`.
The independently authored manifest declares exactly `native-proof`; all other
foreign/builtin metadata matches. No project session ran in this failed prefix.

Two independent plan and exact code reviews accept only that fixed User plugin
spelling in the Go/Python expectations. Literal positive catalogs and seven
negative namespace/name controls preserve plain project/foreign/builtin names
and reject bare, wrong or doubled User prefixes. The four-file reviewed
manifest is57a5ffd6e528ac5a5f7a4d23a6810d5eaa6c0b8b7480889965c0969586eba82a.
Both focused Go catalog contracts pass without skips;39 focused Python
contracts pass; affected all-enabled and whole-root six-linter gates report
zero issues. The original full archive still replays byte-for-byte to its
failed/all-false result. A fresh complete-cycle hosted run and two full raw
audits remain required. No native vendor executable ran locally.

### Copilot observed cached startup and public DTO correction (2026-10-29)

Exact59c1cf29 run40741435108 reaches five validated native sessions and two
MCP sessions, then stable-A emits tools-loaded before pending/connected. Admit
that observed order by removing only pending-before-tools; retain pending before
connected and both completion facts before the server summary, seven unique
closed event kinds and the original parent/identity/catalog checks. The legacy
readiness reader keeps its exact order. The same capture proves repeat-add's
singleton public DTO and repair's literal `local` source; removal uses the same
existing installed-source formatter. The Python reader selects the singleton
only for repeat-add-A and retains exact batch/target and shared identity checks.

Grok/Qoder and integration independently approved the plan and exact five-file
code.38 focused Python contracts pass; six actual public DTOs and all27 stable-A
events pass the corrected pure readers. Native artifact12844837031 SHA256
`3f9ed2784825ffffa708778d1e93f0a2af9440721fc7d5449aeccbb6ca4024a3`
and frontend12844687177 SHA256
`a65e54f115985db183c6138a1dd55fddc96b76b2b8d80f46af3af3572fe4d444`
match API sizes/hashes and closed36/25-member CRCs. The original failed archive
replays unchanged with all flags false. Targeted startup/runtime contracts pass
in0.033s. Broader unchanged inert-process tests still fail local process
containment admission; this is not a vendor result or a full-suite pass. Fresh
hosted complete-cycle evidence and independent raw audits remain required.

### Copilot observed repeated list requests in one probe session (2026-10-29)

Actual1f01bdd8 attempt2 run40878246362 reaches stable-A after the startup fix.
Native artifact12903145008 is76835bytes/SHA256
`830b9c13a9ec8d5dd2ecc785c5a6db3b4845375bc52f9f788e0c6219d8dafe1a`;
frontend12901458402 is3423bytes/SHA256
`7f71efdcf76465aca02d6a2bdf02fb179347d328fd9f98d2c9916eed5b9ef0dc`.
Both closed archives/CRCs,36native and25frontend members, source/build identity
and24frontend stream joins are verified. Whole-byte offline review joins six
native envelopes, nine exact scripted-provider exchanges/ordinary closures and
three observed probe processes. The final5846-byte append prefix SHA256 is
`e8c01b4711bd51764a329b27065769f629f66b4409c77e79e9ded0d01ce69627`.

Installed A/B each emit initialize/list/call. Stable A emits exactly
initialize/list/list/list/call within one fresh PID/session/nonce and command
interval; all four pre-call records have null facts, and the sole final call
matches native/provider returned facts. Accept only those two exact measured
method tuples in the existing stage-independent Go/Python probe verifier. Keep
empty no-call deltas, closed schemas, identity isolation/freshness, monotonic
bounded timestamps, append-prefix receipts and exact final facts unchanged.
Do not accept an arbitrary list-count range, another initialization or another
call. Literal positive and hostile contracts cover both forms and retained guards.

The recorded run remains failed with every qualification flag false. Its summary
still reports only five validated native/two validated probe sessions; no
post-stable state snapshot exists. The intact-repair pre-call authority supports
only offline stream/catalog/facts joins. This correction changes no production
code, provider, native argv, pin, environment, authority, deadline or completion
gate; a fresh full genuine lifecycle and independent raw reviews remain required.

### Copilot semantic repeated-discovery sequence (2026-10-29)

The next pinned1.0.103 run40890660942 reports initialize/list/list/call at stable-A;
the prior run had three lists, while installed A/B have one. Complete offline
native/provider/probe joins pass against the held pre-call recipe, without a
post-stable state claim. The harmless probe's `tools/list` handler is stateless.
The primary [MCP lifecycle](https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle)
requires initialization before operations, and [tool discovery](https://modelcontextprotocol.io/specification/2025-11-25/server/tools)
defines listing without a fixed request count. Scheduling is an inference; count
variation is directly observed. Both independent plan reviews approve replacing
count enumeration with exactly initialize, one-or-more tools/list, and one final
tools/call. Retain closed row schemas, one fresh PID/session/nonce, null pre-call
facts, exact final facts, monotonic command timestamps, append-prefix joins and
empty no-call deltas. Enforce the existing1MiB delta bound explicitly at the Go
helper boundary; Python already enforces it. Add repeated-list positives and
zero-list/reinitialization/post-call/extra-call/oversize/identity/facts controls.
No native argv, provider, pin, environment, time budget or completion gate changes;
all three failed captures remain failed/all-false pending fresh full lifecycle.

### Copilot repeated startup tool notifications on the qualified parent (2026-10-29)

Wind source16283a4/treeaf8cee9 run40954917050's Copilot baseline reaches
`native:repaired-a`, where the vendor exits0 with ordinary cleanup and emits28
events: tools-loaded, pending, tools-loaded, connected, skills, info, server
summary, tools-updated, then the existing20-event execution tail. Both tool
notifications have the same owned server/count and root parent, distinct UUIDs
and monotonic times. All128 Copilot-owned source paths equal qualified17b13226;
the failure is the inherited27-row cycle template, not a changed vendor pin.
The retained42/25-member native/frontend archives,159 embedded stream joins,
provider exchanges and append-only probe prefix close. The failed run has no
after-native-repaired-a state proof and keeps every qualification flag false.

Both independent plan critics approve a cycle-only semantic startup view:
require one-or-more identical owned tools-loaded notifications before the server
summary; keep the other six startup observations unique, pending before
connected, and connected plus every tools-loaded before the summary. Validate
every original raw row's closed shape, payload, ephemeral flag, fresh UUID,
root parent and command time before retaining one startup observation for the
existing graph/catalog validators. Keep even omitted refresh UUIDs in the global
identity set. Raw bytes/counts/hashes and the exact20-event tail stay unchanged;
legacy strict27 readiness and15-row no-call readers remain strict. The retained
vendor JavaScript handles this as a state notification; variable refresh
cardinality is an inference from the observed repetition, not an emitter-count
guarantee. Existing2MiB/60s bounds, provider/probe/facts joins, native argv,
environment, authority and completion gates remain unchanged. Inert one/two/eight
refresh positives and hostile payload/identity/parent/order/time/size controls
exercise this correction; fresh genuine complete evidence remains required.

The25 focused Python contracts and focused cycle/readiness Go contracts pass;
affected all-enabled whole-file lint and the unfiltered root six-linter gate
report zero issues. Offline readers preserve the qualified17b full verdict and
all three retained failed verdicts exactly. The actual28-row stream passes only
the corrected cycle decoder; its legacy strict reader still refuses it. These
checks run no vendor client and do not replace a fresh hosted complete cycle.

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

### Windsurf empty native MCP storage directories (2026-10-29)

Actual07756a69 run40888304062 passes the telemetry guard and reaches a successful
account-free `mcp list`, then refuses two new ordinary0755 directories:
`xdg-data/devin/mcp` and its empty `oauth` child. The complete seven-entry delta
contains only those directories, the expected log/maintenance changes and fixed
telemetry timestamps; original config is unchanged. Pinned path joins at638e237,
638e279 and638e29e build XDG_DATA_HOME/devin/mcp/oauth; constructor7e73880 uses
recursive0777 directory creation, yielding the observed0755 under the held umask.
Both independent plan reviews approve adding only this empty directory chain to
the existing initial pre-child seed. Freeze it before any vendor command, with
no later-write exception, OAuth file, credential, approval or config mutation.
The existing complete profile closure must reject later file/mode/identity drift;
this failed capture and every qualification flag remain unchanged/all-false.

### Windsurf exact empty native plugin lock seed (2026-10-29)

The genuine e9448dd6 capture (run 40894288914) completed six native commands,
then refused the first project-A skills read because the native client created
`xdg-data/devin/cli/plugins/lock.json`: ordinary 0644, 57 bytes, SHA-256
`cbce3a5464482eb4cfb7b152ad15183bcb5db5c5557c625ee0cec1edb582aa88`.
Its failed frame and all five false qualification flags remain unchanged.

One bounded reacquisition verified the exact original archive and the entire
217494824-byte executable against the existing pins, without executing it.
Independent static reviews confirmed the loader accepts the native serializer's
empty `requirements`, `resolved`, and `edges` arrays, in that order, with two-space
indentation and no trailing newline. The empty optional `resolve_failures` field
is omitted; these source-derived bytes hash exactly to the captured file.

Author only those exact 57 bytes as an ordinary 0644 file before any native child
and the initial baseline. Preserve the existing protected digest and complete
profile identity, content, metadata, and membership freeze without an exception.
Static inspection does not establish that every startup path skips a rewrite:
the next genuine capture must satisfy every unchanged predicate or still fail.
No temporary file, same-byte rewrite, replacement, credential, approval, config,
command, deadline, or qualification allowance is introduced. Both independent
reviewers approved this bounded plan before implementation.

### Windsurf exact empty native registry replacement (2026-10-29)

The genuine cd91f247 run40900325427 still refuses empty-A skills after six
ordinary successful commands. The seeded lock retains the exact57-byte cbce3a
content; its record and plugins-directory record change. All six delta paths,
source/build identity,20native/25frontend archive members and24frontend stream
joins close. The projection does not reveal inode or owner values, and this
failed frame retains all five false qualification flags.

A bounded same-pin static RAM read verified the full executable SHA and retained
the complete writer continuation. It derives the same-directory json.tmp path,
writes the canonical buffer, fsyncs, closes, renames over lock.json (syscall82),
and syncs the parent. Both independent source/plan reviews approve only this
fixture-owned empty registry replacement, without a same-inode claim.

For this exact path require before and after ordinary0644/currentUID+GID, single
link, positive inode, same device, and exact57-byte canonical hash. After mtime
and ctime must both fall inside the immediate OSRun start/end window. Only when
this exact file record changes may the exact plugins parent change timestamps;
keep its inode/device/type0755/owner/group/link/size/hash authority unchanged and
bound both timestamps to the same command. Keep complete membership closed,
including refusal of temporary leftovers. Preserve all original config/User/
foreign guards, protected digest, accepted-frame-only copy and final late check.
No arbitrary registry content, missing file, parent replacement, unknown path,
authentication, argv, deadline or qualification allowance is introduced.

### Windsurf frozen-tree refusal visibility, 2026-10-29

Hosted source `69a2a190612165992fc150cd8302539a0f3e312a`, runtime
`40920990159`, completed six ordinary commands through `empty-a-skills`.
Its complete HOME/XDG runtime closure passed, then the independent frozen
source/state/native/data/cwd tree equality refused a change. The failed capture
does not identify the changed frozen entry; no path or permitted write is inferred.
Keep that equality and all false qualification flags unchanged. On its existing
refusal read, select a deterministic changed root and prefer a changed file over
directory metadata. Report only root/key SHA256, finite change/count/presence
facts, record hashes and fixed field-equality facts, bounded to 2 KiB in the
existing fatal output. No raw path, body, extra filesystem observation, baseline
adoption, summary-schema change or runtime-write exception is authorized.

### Windsurf initial Git local-config excludes, 2026-10-29

The complete audited `cf496fe0` run `40927889017` fails after six commands at
`empty-a-skills`. The frozen-tree diagnostic identifies project A's
`.git/info/exclude`: its file authority is stable, but bytes, size and timestamps
change. The after-body is not uploaded and is neither inferred nor adopted.
The original HOME/XDG closure passes; every qualification flag remains false.

One bounded same-pin RAM acquisition verified the complete executable without
executing it. Retained publisher-source excerpts establish preferred `.devin`,
the two local filenames, same-root relative formatting, and exact existing-line
branches that bypass each exclude writer. Both independent plan reviews approve
only `.devin/config.local.json` and `.devin/mcp_config.local.json` before the
initial Git/cwd freeze, immediately after the existing recorded Git initialization.

Preserve all Git-generated exclude bytes and append only missing fixed LF rules.
Require held nofollow parent/file authority, ordinary current-owned 0644 and one
link, exact initial stat/hash, and unchanged inode/device/mode/owner/group after
the append. Existing exact rules cause no write. Replace only the validated file
record in the already validated fresh Git/cwd frames, then verify both live trees
before freezing. Refuse renewed preparation of a frozen Git root. No local config
file, runtime Git-write allowance, observed baseline adoption, native command,
credential, trust, deadline, archive-bound or qualification change is introduced.
The next genuine hosted capture must satisfy the unchanged full frozen guards.

The next run `40954917050` stops after `git-a` at the initial exclude-authority
predicate, before any vendor command. Its raw evidence does not disclose which
stat field or absence caused that refusal; all qualifications remain false.
Git's [initialization documentation](https://git-scm.com/docs/git-init) specifies
umask-dependent permissions; upstream `copy.c` creates non-executable template
copies with `0666` before umask. A universal `0644` requirement is unsupported.
For a present ordinary exclude file, bind mode to its already validated fresh
Git frame and preserve that exact mode through append and freeze. Keep full
stat/digest joins, current ownership, single link, nofollow checks, no-write
stability and final equality. Missing original files remain fatal; no creation
fallback, chmod, runtime allowance or diagnosis of an actual mode is introduced.

### Windsurf public snapshot scratch closure, 2026-10-30

The genuine `cded02aa` run `40985280283` completes ten ordinary commands through
the successful public User add, then refuses an original-profile change. Its
generic fatal does not export the changed key or after-frame; no post-add state
proof or positive native qualification is inferred. All five flags remain false.

The default legacy Stager uses `packagesnapshot.Builder` with an empty TempRoot.
Build creates a private snapshot child under the fixture's TMPDIR and Close
removes it. Two vendor-free inert contracts exercise the actual public User add
and the real default Build/Close path with a tiny inert HTTP recipe. The earlier
verified runs showed final empty scratch membership and unchanged directory
authority/size, with only its mtime and ctime changed. The complete profile
contract showed every other original record unchanged. Reconstructed contracts
retain that boundary; they do not qualify the original hosted stdio lifecycle.

For registered public User/A/B adds only, permit those two timestamps at the
exact TMP root when both complete TMP subtrees are empty. Keep type, device,
inode, mode, UID/GID, link count, size and empty directory hash fixed; require
ordinary current ownership and positive inode/link count. Both after timestamps
must fall inside the immediate OSRun interval. Every other profile entry and
native runtime guard remains unchanged. No TMP baseline is renewed or adopted;
TMP belongs to the complete profile frame, not the frozen payload trees. No
command, deadline, capture bound, approval or qualification flag changes. The
next genuine 23-command frame still requires complete independent raw review.
