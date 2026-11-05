# Devin Global D0: source/runtime gate and exact-profile authority

2026-11-04. **PARTIAL source observation / BLOCKED runtime — not a native Global E2E PASS.** This is an evidence
checkpoint for [#710](https://github.com/777genius/universal-agent-plugins/pull/710),
not a change to the already reviewed implementation plan. Do not grant
Windsurf-wide user Skills/MCP support or merge based on this note.

Current 2026-11-04 audit: leaf head `ad6d5b85593b7320d64032d77b2ac65a6459ae51`, moving main `71565b1c2e6422f5841fe38b1db1f3c3fe9ceed3`; #698 `b7c335bfb06c1c2435916ec6f08a829b9bd7c7c3`, #704 merged as `a9c73541f7f5ce97a60578a31f16ec03d87565c2`, #707 `638601da9545f056d2202672a39581ba14e61249`. These are observed only, not integration bases. Read `AGENTS.md` and recheck all refs on any future code edit.

## Repository snapshot (recheck before edits)

Read before any product code: `AGENTS.md`, the accepted leaf plan,
`clients/windsurf/{detect,scope,version_profile,project_native,project_lifecycle,activate,plan}.go`,
`clients/detection.go`, `adapters/clientdetect/detector.go` and
`domain/clients_runtime.go`. On this check:

- #710 head `e6a808eb13bfea8985ebca3057e115368a0abc61`, draft, plan only;
  its base is `05338655fe9e9f829f454c7feb43bcacada9523d`.
- Moving `main` was `acedefd5fab319176fa4ff286349eeda0dd1ff62`.
- Independent moving PRs: #698 `5e16be5520457d10bd05fa7b151ee4cc03b363ec`,
  #704 `90ff528f8dde9d6b02119b1de0a47c5043b9a67e`,
  #707 `152e5b78bfbebe24a06c3757ddfffd418e8e5650`.
  These are observation SHAs, **not** integration baselines to silently overwrite.
  #707 remains first main-merge priority.

## Actual D0 execution — fixed Linux source evidence, blocked runtime

[One-shot GitHub Actions run #41874306904](https://github.com/777genius/universal-agent-plugins/actions/runs/41874306904)
ran on **Ubuntu 24.04, linux-amd64** against exact branch HEAD
`ad6d5b85593b7320d64032d77b2ac65a6459ae51` (2026-11-04 17:17 UTC).
The job's final state is **failure by design**, not flaky/green:
[retained JSON report (artifact #13345563632)](https://github.com/777genius/universal-agent-plugins/actions/runs/41874306904/artifacts/13345563632);
artifact digest
`sha256:7a1e99287a7bce868dbb789f4cbf3bc02ab5e74c065edfb5b61b9b64079508a6`.
The report explicitly has `qualified_product_e2e=false` and `native_rpc_events=[]`.

- **Binary/provenance observed:** genuine official Linux x86_64 archive
  `3000.11.205` from `static.devin.ai`, archive SHA-256
  `2e65c8b93682258fc91f5e4cead92e3698338cebf1b31dbc32a3201cb63705c4`,
  extracted executable SHA-256
  `642c82e32cb3dec90cf5ab01975520b425307872c7a650c9fb5b0653f37681f4`.
  The real `--version` completed with exit 0; historical native output
  is `devin 3000.11.205 (6dea39440ae3)`. The selected executable was
  the verified `<RUNNER_TEMP>/devin-global-d0/devin` in an isolated host.
  No evidence was obtained for Windows/macOS versions in **this** run.
- **Offline native source discovery observed:** genuine
  `devin skills list --json` exited 0, returned parseable JSON and named
  `uap-d0-global` in both independent Project A and cold Project B.
  `devin mcp list` exited 0 and listed *both* configured names
  `uap_d0_stdio` and `uap_d0_http` in both projects.
  This proves CLI-visible listings from original disposable
  `HOME/.config/devin`, **not** native initialization of either MCP
  connection, correct effective override source for all cases, a loaded Skill
  in a model session, or an installed UAP component/receipt lifecycle.
- **Agent source/call is BLOCKED:** the two authentic `devin -p`
  invocations (A/B) each exited 1 with `auth_diagnostic=true`.
  Neither configured inert MCP endpoint received even `initialize`;
  `tools/list` and `tools/call` are equally **not observed**.
  Both transport gates are false. No authentication token was injected,
  an alternate provider impersonated, or harness RPC mistaken for a Devin call.

The official [Commands & Flags](https://docs.devin.ai/cli/reference/commands)
document `skills list`, `mcp list/get`, `auth status/login` and
non-interactive `-p`, but **no supported offline MCP tool-call command**.
[MCP overview](https://docs.devin.ai/cli/extensibility/mcp/overview)
describes connections and tool execution by the agent; the
[configuration reference](https://docs.devin.ai/cli/extensibility/mcp/configuration)
confirms stdio and Streamable HTTP configs, not standalone `tools/call`.
The [Devin auth guide](https://docs.devin.ai/cli/enterprise/devin-auth)
describes credentialed account access; `credentials.toml` is a secret and must
not be collected/copied from the user or committed. No documented native/offline
alternative was verified as of this checkpoint. `mcp list/get` will not
be relabelled an RPC test, and ACP/editor integration will not be expanded into
an invented provider framework.

**Remaining prerequisite, not waived:** user-approved authenticable native
Devin CLI environment/provider plus exact host/profile/source provenance, then
client-driven `initialize`, `tools/list`, `tools/call` with inert stdio+HTTP
servers in the mixed A/B Global+Project cycle. Before that, this leaf stays
draft, *no new user capability or production edit*, no independent post-code
approval, no merge and no release. The original one-shot D0 workflow only runs
when its own workflow file changes; do not dispatch a duplicate run.

## Confirmed design collision in current adapter

1. `DetectSurfaces` computes one `DetectedClient` and chooses
   `shared.FirstPath(windsurfCLI, devinCLI)`. Coexisting binaries therefore
   select **Windsurf first**, even when a real Devin binary is also present.
2. `ConfigRoot` is `selectedLegacyRoot` (`~/.codeium/windsurf*`) and may
   intentionally be empty. It is **not** the Devin Global config authority.
   The `devin_local_config` discovery surface is only evidence and must not
   silently replace the current mutation root.
3. `RequiresProjectVersion` probes only an explicitly selected absolute
   primary `devin`/`devin.exe`; `requireCurrentDevin` accepts only
   observed exact `3000.11.205`. The detector has a bounded original-host
   version environment, but ordinary Detect deliberately does not probe.
4. `DetectedClient` currently contains one `ConfigRoot`, one
   `ExecutablePath`, and `Version`. Management startup reconstructs a
   selected target with root/executable but does not supply a version in
   `managementDetector.Detect`. This is a concrete **selection and propagation
   seam**, not permission to treat any Windsurf user target as Devin.
5. Current user RefinePlan/Activate/Deactivate are legacy Cascade routes:
   user Skills are prepared, and owned MCP entries target the selected Cascade
   codec/root. Current Devin Project uses the separate
   `shared.ProjectComponents` + `ProjectComponentMutation` with
   `nativeconfig.CodecDevin` and stays prepared even after installed files.
   Changing global capabilities unconditionally would misroute old receipts.

Canonical Global candidate is a **validated original** XDG user config root
`$XDG_CONFIG_HOME/devin` (or `~/.config/devin` when applicable) on Unix and
`%APPDATA%/devin` on Windows; component paths are `skills/<name>/SKILL.md`
and `mcp_config.json`. Project remains `.devin/skills` and
`.devin/mcp_config.json`. Current
[Devin MCP docs](https://docs.devin.ai/cli/extensibility/mcp/configuration),
[Skills docs](https://docs.devin.ai/cli/extensibility/skills/overview) and
[precedence](https://docs.devin.ai/cli/reference/configuration/global-vs-local)
support these *formats*. They do not supply the missing runtime observation.

## Atomic integrator request (same leaf only)

Provide the narrow selected-host/profile authority contract that carries
**which exact executable was selected, observed exact version, and original
canonical XDG/APPDATA root** consistently through public CLI/management, plan,
native mutation, binding/receipts and cold-process replay. Keep legacy Windsurf
as the default when ambiguous, and preserve existing Project eligibility.
Never implement `ClientWindsurf` blanket user capability or move old Cascade
receipts to Devin. A user-scope route must be admitted by its **recorded Devin
profile**; exact owned removal must continue working offline after a client
upgrade/uninstall. Reuse shared native components, kernel and launcher.
The integrator's central traits and the adapter implementation must commit in
this #710 leaf, **not** as a separate early main capability change.

## D0 unblock oracle, before production edits

On a disposable host with the hash-verified genuine binary, capture the exact
`PATH` selection, selected binary hash/version, original HOME/XDG/APPDATA
authority, clean A/B project roots and immutable source digest. With one mixed
source (Skill, inert stdio MCP and inert Streamable HTTP MCP), capture:

- Genuine Devin-driven Global Skill discovery for A and cold B, and selected
  Project A/B control; no mere file-exists or forged `--json` success.
- The genuine client's source of each effective MCP name (Global vs Project vs
  local override), observable `initialize`, `tools/list`, and
  `tools/call` for both transports with distinct nonces. If client auth,
  trust or any observer is unavailable, mark that cell BLOCKED, not PASS.
- Strict path/receipt snapshots before and after add, update, independent A/B
  replay, foreign-name collision, repair and exact offline removal. Legacy
  `~/.codeium` and unrelated user/project configs must remain byte-identical.

Only after this first source/call spike should the leaf implement admitted
Global logic, add focused facade assertions and request independent code
review. Avoid rerunning overlapping CI waves or initiating a release.

## 2026-11-05 narrow source-boundary preparation (not native qualification)

The owner now permits authorization-independent, narrowly scoped production/test
preparation while genuine model-session MCP calls remain unavailable. The shared
component kernel has a new **optional exact managed-package path**, selected by
the client adapter; an unset value retains Project's original contained-source
rule. This separates the native output boundary from a managed package in the
default or independently relocated UAP state root. It is deliberately not a
HOME-wide allowance or a new Global Devin capability. Focused lifecycle tests
cover exact-source rejection, Global-shaped native root, foreign MCP preservation,
owned repair/removal and byte-preserved legacy Cascade plus A/B project files.
Only the CI results on the resulting commit can confirm the tests. This update
does **not** change D0's source-only observation, qualified_product_e2e=false
or the requirement for genuine authorized client-driven MCP calls before merge.
