# Hermes Agent integration

Published [native CLI 0.1.110](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.110) (source `1b7d2db803dd5eea2d632d8f4e149b4d7bb4a48e`) includes Hermes and Antigravity. UAP configures supported components automatically; follow the client-specific remaining actions. Signed Directory package eligibility remains separate. Installation does not prove native runtime activation or vendor authentication.

Historical source lifecycle: Hermes 0.23.0 on Linux x64/macOS arm64/Windows x64 at UAP `a50c8a245f1c273efb3d171f4c54ed540475336a`, [run 42813277862](https://github.com/777genius/universal-agent-plugins/actions/runs/42813277862). Subsequent [#884](https://github.com/777genius/universal-agent-plugins/pull/884) proves native User stdio `tools/call` on all three OS through the official legacy `run_agent.py` driver and a local scripted provider. Default `hermes chat`, vendor authentication and trusted Project Skills execution remain unqualified.

The Hermes Agent target ID is `hermes`. This first implementation preserves UAP's
immutable-pinned package source, transaction journal and exact ownership while
materializing **Skills and a scoped subset of MCP servers** into the native locations Hermes documents:

| Scope | Native Skills destination | MCP handling |
| --- | --- | --- |
| `--scope user` | `$HERMES_HOME/skills/<name>/SKILL.md` | Exact-member `mcp_servers` edits in `config.yaml` (stdio and Streamable HTTP) |
| `--scope project` | `<project>/.hermes/skills/<name>/SKILL.md` | MCP only in an explicitly confirmed isolated UAP session with a verified standalone Hermes bundle; source-installed Hermes supports Project Skills, not Project MCP |

Project MCP for a Hermes installed from source is not supported yet. Hermes has
no project-level `config.yaml`, and UAP never writes a project server into the
user config, because it would run in every repository. A dedicated Hermes profile
for source-installed Hermes remains undecided and unimplemented ([issue #942](https://github.com/777genius/universal-agent-plugins/issues/942)).
The pinned official source launcher prepares home-indexed dependency state and
performs update recovery before ordinary configuration handling. Pointing it at
`<project>/.hermes/uap-session` can provision another dependency environment;
disabling lazy installs alone does not make startup passive or supply missing
dependencies. Named native profiles share their owner runtime but live outside
UAP's current Project-contained ownership contract. UAP preserves the source
refusal rather than bypassing the standalone manifest or creating synthetic
package-manager state.
The verified standalone host instead uses the explicitly confirmed UAP project
session described above; ordinary Hermes startup does not select that MCP home.
This host-distribution requirement does not restrict plugin Git sources.

The smoke selects an explicit isolated, nondefault `HERMES_HOME`. UAP captures
this override and `HERMES_DATA_DIR_SUFFIX` from the environment. Without an
override, the native defaults are `~/.hermes` on POSIX and
`%LOCALAPPDATA%\hermes` on Windows (or `~/AppData/Local/hermes` when
`LOCALAPPDATA` is empty), with the literal suffix appended to the directory
name. Platform-default selection has separate injected-host tests; the native
lifecycle verifies the explicit profile. Existing legacy directories do not
change the selected authority. See the [profile correction source review](plans/2026-11-10-hermes-windows-profile.md).

Hermes honors project Skills only after its own project trust conditions. An
explicit, confirmed Project `add` with Skills asks the user's own Hermes CLI to
trust the selected Git repository root. Planning, dry runs and verification
never change trust, and UAP never equates installed filesystem bytes with live
availability.

### Project trust on explicit add

Hermes 0.23 keeps project trust in `skills.trusted_project_dirs` of the profile a
bare `hermes` selects. Trust covers the whole Git repository root, both
`.hermes/skills` and `.agents/skills`, including Skills added later, until
`hermes skills untrust`. Skills that Hermes' `skills_guard` flags as dangerous
stay quarantined and do not load. Trust is the user's preference: UAP removal
never revokes it. Start a new Hermes session after installation.

Where the scope is disclosed: the plan's next actions state all of the above.
The interactive review card shows them before its confirmation, and human
output prints them as planned actions before applying. An explicit `--target`
or `--yes` counts as confirmation under the existing CLI policy, the same as for
Gemini CLI, Codex, Claude Code and Copilot trust; non-terminal and JSON runs
require `--target`. Such runs show no review card before trust is granted, and
JSON output carries the disclosure only in the plan and the result.

Only an explicit `add` grants trust, including a repeated `add` that finishes
an older Skills-only installation. Update, ordinary repair and `deps sync` never run the
trust command, so a `hermes skills untrust` stays in effect; they report that
UAP did not change trust and how to grant it manually. A saved `add` passes this
consent only for its explicitly selected dependency closure. Installer SDK hosts
grant only when they set `Request.ProjectTrustConsent` for an explicit add and
supply `LookupExecutable`; otherwise run `hermes skills trust` inside the project.

[Current source completion work (#1001)](https://github.com/777genius/universal-agent-plugins/pull/1001)
adds the same disclosed confirmation to fresh/repeated raw `skills add` and the
Skills picker. Repeating a saved plugin `add` with the single explicit target
`--target hermes` can finish eligible prepared Project setup through the existing
reviewed repair. The complete source must be valid and Skills-only for this new
repeat-repair consent; mixed Skills/MCP or other components cannot qualify via a
Skills subset, including when repair must recreate an absent projection. This
optional completion leaves unchanged mixed bindings unchanged and adds no native
version probe for them. Review guidance reflects the effective consent: a repair
without consent explicitly reports that it does not change project trust. This
paragraph describes source behavior under qualification, not a newer published
CLI release.

A newly reviewed and confirmed re-add can intentionally restore trust after a
user revoked it. Ordinary maintenance never inherits add consent. A user-approved
retry may retain the identical reviewed trust scope; a changed profile, executable,
source pin, operation or retained ownership requires a fresh full review.
Preparation, review, recheck, decline and dry run do not invoke
native trust or write configuration. Installation, binding identity and source
pins remain unchanged during an exact repeat. Trust alone leaves status **Prepared**. The current source candidate can complete
pure Skills only after a fresh native catalog confirms the selected profile,
project trust, exact owned Skill paths and bytes. This uses the selected Hermes
runtime and its own bootstrap/dependency lease; Hermes may complete native startup
setup and write its scan caches. Read-only verification never starts that
completion action. A confirmed repeat reuses the existing owned-package repair
flow; User Skills completion grants no Project trust. These source changes remain
under native qualification and do not describe a newer published CLI release.

UAP never edits `config.yaml` for trust and never uses `hermes config set`,
because a sticky active profile can make a bare `hermes` read another file. It
runs `hermes skills trust <root>` with the user's environment and no `-p`, and
only after all of these hold:

- the root is the absolute physical `git rev-parse --show-toplevel` result, the
  nearest ancestor with `.git` of the selected project, and neither the home
  directory, a directory containing it nor a volume root;
- `hermes config path` names `config.yaml` in the Hermes home UAP selected;
- `HERMES_MANAGED` (other than `false`, `0`, `no`, `off`, `brew`, `homebrew`),
  a `.managed` or `.container-mode` marker, a non-file
  `HERMES_CONFIG_BACKEND` (environment, home `.env` or `.op.env`, managed
  `.env`) and a non-empty managed `config.yaml` are all absent. Any managed
  configuration makes each Hermes save delete every administrator-pinned key
  from the user's file;
- the existing trust list can be read; UAP holds its own configuration lock so
  two UAP operations never save the same profile concurrently.

The command exits 0 even when Hermes saved nothing, so UAP reports trust only
when stderr is empty, the first line is exactly `Trusted: <root>` or
`Already trusted: <root>`, a fresh read contains the root and every earlier
entry, and `skills.project_discovery` is not false. Otherwise it reports that
trust was not changed or could not be confirmed and says to run
`hermes skills trust` inside the repository before a new session. Public JSON
stays path-free: the exact command with the absolute root and any earlier
entries missing after a managed save appear only in human output.

On the pinned version, a successful `hermes skills trust` saves the whole
`config.yaml` itself, not only the trust list. On the default file that Hermes'
installer creates, the save also added 28 missing `gateway` defaults, dropped
an empty `agent.reasoning_overrides`, wrote `null` values as empty values,
emptied whitespace-only lines and changed the file mode from 0644 to 0600;
comments stayed (observed on Hermes 0.23, 2bd378e). UAP does not control this
Hermes save, so the lifecycle E2E compares MCP settings and trusted roots
semantically.

**Strict scope:** owned Skills and scoped MCP install/update/repair/remove,
no reuse of foreign skill/MCP names. User MCP uses the existing atomic native
config kernel, strict bounded YAML parsing, per-entry receipts, and
post-write verification while preserving unrelated settings and comments.
It re-encodes only the top-level `mcp_servers` block, so every other line,
including blank lines, stays byte-for-byte; a new block goes at the end of
the file. Inside that block yaml.v3 normalizes layout. If the block cannot
be replaced on its own, for example in a flow-style file, UAP re-encodes the
whole file: settings and comments stay, blank lines do not.
An ambiguous YAML document, foreign edit, symlink, unknown tag/anchor or
unsupported transport fails closed. No user YAML is ever changed by Project
MCP. Packages requiring hooks or unqualified components are refused.

**Historical source acceptance gate:** the recorded three-platform run satisfies the native
lifecycle gate for its exact UAP head only. The current published release and source are linked above; they do not extend this historical run's proof. The native smoke proves
MCP listing, connection and tool discovery over loopback Streamable HTTP; it
does not perform `tools/call`, a model turn or an authenticated provider session.
The historical runs above observed Project discovery without trust; the current
lifecycle checks trust after an explicit add, that update and repair keep a
user's revocation, its absence after a dry run or a redirected active profile,
and loading in a new process, not execution. Portable commands are projected as Skills;
explicit-only commands are withheld because Hermes cannot enforce explicit
invocation. SSE and hooks remain unsupported.

Sources: Hermes [Skills system](https://hermes-agent.nousresearch.com/docs/user-guide/features/skills/)
and [MCP](https://hermes-agent.nousresearch.com/docs/user-guide/features/mcp/).
Implementation criteria: [reviewed plan](plans/2026-11-09-hermes-native-integration.md).

## Native CI acceptance

`Hermes Native Skills and MCP E2E` checks out NousResearch/hermes-agent at exact
commit `2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19` (v0.23.0), installs it inside an
isolated CI home with browser/computer use disabled, and builds UAP from the PR.
The workflow acceptance contract uses the same lifecycle on Linux, macOS and
native Windows, with the pinned official installer for each platform:

- User first/repeated add, update, repair and removal, with actual native Skill
  catalog lookup and content reading through the installed Hermes runtime.
- Native `hermes skills list`, `hermes mcp list`, and four `hermes mcp test`
  connections to a bounded loopback fixture, observing initialize/tools/list.
- Project A/B isolation, update and repair of A while B retains its original
  Skill, exact-root trust after an explicit add, loading in a new process and
  removal that keeps trust. After `hermes skills untrust`, update and repair do
  not grant it again; a dry run and a sticky active profile leave trust unchanged.
- Structured unsupported Project MCP refusal with `mutated: false`, unchanged
  User YAML bytes, and preservation of foreign Skills/MCP entries.

| Native platform | Evidence |
| --- | --- |
| Linux x64 | PASS in [run 42813277862](https://github.com/777genius/universal-agent-plugins/actions/runs/42813277862), at the exact UAP head above; [Linux evidence](https://github.com/777genius/universal-agent-plugins/actions/runs/42813277862/artifacts/13762793764). |
| macOS arm64 | PASS in the same run and at the same UAP head; [macOS evidence](https://github.com/777genius/universal-agent-plugins/actions/runs/42813277862/artifacts/13763179484). |
| Native Windows x64 | PASS through the pinned native PowerShell installation in the same run and at the same UAP head; [Windows evidence](https://github.com/777genius/universal-agent-plugins/actions/runs/42813277862/artifacts/13763404124). |

Each OS passed User Skills/MCP first and repeated add, update, repair and
removal, Project Skills untrusted discovery and A/B isolation, and four native
loopback HTTP `initialize`/`tools/list` exchanges. The result does not extend
to Project MCP, trusted execution, tool calls, model use or provider login.

### Earlier Linux checkpoint

[Run 42802367363](https://github.com/777genius/universal-agent-plugins/actions/runs/42802367363)
passed on Linux at `b3ed848effaca7d436c1306e06f2d24e67b9e0b2` with Hermes 0.23.0;
its [retained evidence](https://github.com/777genius/universal-agent-plugins/actions/runs/42802367363/artifacts/13757427815)
records 13 native Skill observations and four MCP initialize/tools/list
exchanges. This historical result retains its original identity; the newer
three-platform run supplies the current source checkpoint.

The pinned Hermes [installation documentation](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/README.md#quick-install)
supports Linux, macOS and native Windows; Windows is not WSL-only. Matrix artifacts
record the actual UAP checkout and PR head separately, the Hermes source pin,
runner OS/architecture, lifecycle exit and observed MCP methods. A queued,
skipped or failed run is not qualification, and earlier Linux evidence cannot
qualify a newer candidate head or another operating system. Keep platform and
release promotion gated on the applicable current-head results.

### Source completion acceptance

The existing `hermes-native-e2e.yml` source-only Linux, macOS and Windows lanes
run the external current-source UAP lifecycle first. They then execute two
bounded public-Cobra journeys in a compiled CLI test executable: raw HTTPS
Skills fresh/repeat add, and saved Skills-only TLS Git plugin repeated add.
Acquisition and security use the existing inert local fixtures. Client detection,
version probing, native process supervision, installer composition, native trust
commands and installed Hermes runtime are real. The in-process Cobra tests do
not qualify a separately released executable.

Each journey checks native untrust, maintenance without regrant, newly confirmed
re-add, unchanged source pins/installation/binding identity, selected Skill path
and content from `skill_view(preprocess=False)`, retained foreign User/Project
content and trust persistence after removal. Explicit test-owned native untrust
cleans up after that persistence assertion; it is not installer rollback.
The current candidate also requires actual production catalog query streams and
settlement for each qualified positive result, plus a pure User Skills first/repeat
journey. The two Project journeys retain their trust, maintenance, source identity
and foreign-content assertions. Original streams, source/tree and executable
hashes, vendor pin and scenario results are retained. A skipped or empty test selection
cannot produce a successful native receipt. These new checks are disabled for
release-tag calls, whose existing released-binary proof remains separate.

This source acceptance is pending execution on the final PR source; no new native
PASS is claimed here. The local environment lacks the proc children interface
required by the production process supervisor and refuses before child startup.
Native acceptance therefore requires actual GitHub-hosted jobs. No hosted-state
spoofing or unsupervised fallback is used. The new production-query Ready gate
remains pending until those jobs execute the final combined source; existing
external-observer evidence does not satisfy it. Source-installed Project MCP,
modern `hermes chat` model execution and live-provider authentication remain
outside this proof.
