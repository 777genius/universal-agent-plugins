# #642 — Codex MCP: explicit environment *names*, no plaintext
> Design-first draft · 2026-11-10 · scope: **Codex Project stdio MCP only** · user request: plan → critical revisions → native spike → implementation → E2E → squash merge **only if security oracle passes**.

## 0. Problem and evidence

Today UAP's Codex project adapter projects selected Agent Plugins MCP entries through `clients/shared/ProjectOpenAIMCP` into staged `.mcp.json`, then reads `clients/codex/project_server.go` into `nativeconfig.Server`, and mutates only owned keys of `.codex/config.toml` using the existing TOML kernel. There is already collision protection, exact receipts, no-follow reads, atomic write and recovery. **Reuse those**.

Official [Agent Plugins 1.0](https://agent-plugins.org/specification) only expands `${PLUGIN_ROOT}`/`${PLUGIN_DATA}` in portable stdio inputs; an arbitrary `${TOKEN}` is literal, not a secret reference. Native Codex `mcp_servers.<id>.env_vars = ["NAME"]` passes **names** from the parent environment to the MCP child. [Codex source](https://github.com/openai/codex/blob/main/codex-rs/rmcp-client/src/utils.rs) verifies this. The [Microsoft APM candidate #3406](https://github.com/microsoft/apm/pull/3406) demonstrates the config shape but **also resolves secret placeholders in `args` and mismatched env keys**; do not copy its interpolation.

**Secrecy risk is independently confirmed:** previously pinned Codex **0.153.4** wrote a synthetic inherited secret into Codex-owned `CODEX_HOME/shell_snapshots/*.sh` during actual native execution ([UAP #642 evidence](https://github.com/777genius/universal-agent-plugins/issues/642#issuecomment-6632577675)). The [upstream bug #30971](https://github.com/openai/codex/issues/30971) is open. Latest tagged Codex inspected: **rust-v0.167.0, released 2026-11-06**; current shell-snapshot source includes credential-broker changes, **not equivalent to native non-persistence proof**.

## 1. Scope and UX

**Only this workflow:** owner explicitly opts into Codex Project `--scope project` stdio server; UAP writes name-only `env_vars` to one already-owned native entry; Codex's own runtime forwards it. Existing projects and non-opted server entries behave byte-identically. Other clients, User-scope, remote HTTP, OAuth, native Marketplace bundle registration and portable Agent Plugins interpolation are **out of scope**.

Candidate simplest authored input: a **target-qualified, names-only** member inside the **already accepted and preserved** `plugin.json.extensions` namespace (e.g., `"com.universal-agent-plugins.codex": {"mcp_env_vars": {"server-id": ["SERVICE_API_KEY"]}}`). There is already an extensions map with opaque namespace preservation. No separate manifest file, schema version, migration, global flag, new credentials store or new service. **Validate whether consuming this namespace is allowed by the existing conformance contract before implementing.** The v1 loader already admits a namespace object and preserves its bytes; **only `clients/codex` may interpret this one namespace**. If the extension has to be portable or affects ChatGPT, reject it and choose the existing *project-native configuration input* instead; do not invent parallel settings.

UX target: keep `--scope project` and normal add/update/repair/remove. At source/plan time reject **missing/empty/unsafe/ambiguous variable NAMES** before mutation. Do **not** test the variable's VALUE using the UAP installer's process environment: **Codex may be launched later with a completely different environment**, so installation-time `os.Getenv` is an invalid proxy. A missing runtime value must be reported as **prepared/unverified** (native preflight if available), with one action: `Set SERVICE_API_KEY in Codex's launch environment, restart Codex, then verify MCP.` **Never** prompt for, read/store, render or log a secret value in production. **Separate user consent** is mandatory; package-authored namespace alone is not permission to forward ambient credentials. Reuse the existing install preview, but it must show **exact names** and a clear native snapshot warning. Existing `--yes` is a blanket confirmation, **not sufficient** for new credential forwarding; require one explicit Codex Project-only opt-in parameter for automated calls, and interactive confirmation of the same risk. No extra standalone screen, wizard, feature flag, or secret prompt. If safe operation cannot be proven, report **prepared/unsupported**, not installed+working.

## 2. Minimal architectural delta (proposal, not yet implemented)

1. **Domain/source boundary:** parse the exact declared namespace in the Codex adapter at project selection (not arbitrary placeholders). Accept only explicit server IDs present in selected *stdio* entries and allowlisted `[A-Za-z_][A-Za-z0-9_]*` names (ASCII, max 128 chars, max 16 names per server), dedupe/sort deterministic; reject casefold collisions on Windows. Keep all values as **names only**. No cross-client extension propagation.
2. **Codex client boundary:** attach the names to the already selected Codex server after OpenAI projection, before `DesiredReceipt`/native activation, using existing Codex `Server` and `projectCodexServer`/TOML document codec. `env_vars` is a **server-level TOML array**, not inside `[mcp_servers.id.env]`. Do not rewire `ProjectOpenAIMCP` shared with ChatGPT. Source and desired ownership digest must both include the exact list.
3. **Security boundary:** never read the token value in UAP; reject names duplicated in literal `env`, any opt-in referencing `args`/`cwd`/command or URI/header, untrusted/foreign edits, malformed input, unresolved names, symlink configs, and attempts to silently turn off Codex snapshots. Never mutate a user/global Codex feature or shell startup file.
4. **Lifecycle:** add→repeat add→update with new name list→repair→remove; only managed exact member may change. A new name list requires renewed approval; an unchanged repeated install must not repeatedly prompt. No legacy adoption or rewrite; changing an already-managed entry uses existing receipts, foreign remains untouched.
5. **Proof:** no new production execution service, background daemon, CLI cross-platform flag framework, secret-value cache, helper wrapper, state migrations or feature flag. Keep special-casing in `clients/codex`/native `CodecCodex`; reject on other codecs.

## 3. Pre-code spike (mandatory, first)

- **P1 native compatibility:** pin the released genuine Codex 0.167.0 binary by immutable release artifact digest on **Linux amd64 + macOS ARM64 + Windows AMD64**. Use disposable project, `CODEX_HOME`, HOME and XDG; no real credentials, no network OAuth and no actual remote model: the model is a local scripted Responses fixture, Codex CLI/app-server is real.
- **P2 real child:** start one inert MCP stdio subprocess; call `tools/list` and `tools/call` via native Codex with a random *synthetic* value available **only to Codex parent**. MCP child reports only `boolean present` + stable one-way digest; no plaintext in logs. Prove the child **actually** saw the value (negative missing-value case must fail).
- **P3 negative oracle:** observe the **live** Codex process/filesystem while a real shell tool runs (transient snapshot files may be deleted on normal exit); then, after process exit and crash/restart, recursively inspect project, `CODEX_HOME` (especially `shell_snapshots`), HOME/XDG, UAP receipts/state, argv, CLI evidence and original artifact ZIPs for raw/URL/base64/hex canary; no real secrets ever used. No symlink following. If a local trusted actor must set environment, do not claim that equivalent to safe default UX.
- **P4 isolated control:** compare bare native Codex with and without UAP config and with a shell command step that can trigger snapshot creation. If snapshot leak persists in current release, **STOP**: record reproducible safe evidence (no canary text), do not implement/merge secret-forwarding. Other code may remain design-only Draft until upstream fix. *Do not weaken the oracle*.

## 4. Tests and acceptance

Unit/conformance: extension parsing, negative/missing/malicious names, stdio-only, no arbitrary `${VAR}` conversion; three Codex project lifecycle mutations and native ownership digests; foreign TOML/comments/format preservation; duplicate cross-platform case; no secrets in config, logs, receipts, artifacts; negative oracle tests. Existing Claude/ChatGPT/Cursor/OpenCode non-opted fixture byte equality. E2E genuine Codex `tools/call` + install→repeat→update→repair→remove, fresh trusted session, 3 OS. CodeQL/Required/unit/landing only when changed paths warrant it; do not claim skipped as PASS.

**Merge gate:** Linux pinned canary pilot first to avoid wasting 3-platform CI, then 3-OS genuine native proof PASS; zero persistence findings across native snapshot family **including files while the process is alive**; CodeRabbit review threads resolved or an explicitly documented inability to obtain review must not be misrepresented as approval, clean CI on **exact HEAD**, reproducible original proof ZIPs, SHA-gated squash merge, postmerge main exact-sha tests. If unsupported or leak: keep feature **unmerged**, document blocker in #642; don't bypass authorization/third-party controls or ask for real secrets.

## 5. Estimates and non-goals

| Work | Production lines | Test/proof lines |
|---|---:|---:|
| Target-only parsing + preflight | 80–140 | 70–110 |
| Codex projection/ownership | 110–190 | 110–180 |
| CLI-visible status/consent using existing workflow | 45–95 | 45–75 |
| Native 3-platform proof and docs | 0–25 | 200–420 |
| **Total** | **235–450** | **425–785** |

**Estimate (initial, revise after spike):** 660–1,235 changed lines, with an additional 80–200 lines only if exposing an explicit consent parameter must traverse the existing CLI/management boundary. Risk: native upstream snapshot leak **9/10**, architectural regression **3/10** with strict scope, user workflow **2/10** if existing opt-in route suffices. No new tables, migrations, global gates, process owner or vault.

## 6. Required review rounds (edit THIS file, no separate competing plan)

- **R1 — correctness & standards:** identify each semantic assumption and verify against real loader/manifest/native writer and Codex source; adjust source contract before coding.
- **R2 — adversarial security & UX:** challenge parent environment vs snapshot persistence, false-green native test, missing env UX, foreign file and Windows casefold; remove any gratuitous step.
- **R3 — architecture & scope:** enumerate touched files/calls; reject shared ChatGPT/portable changes, migration/feature-flag additions, duplicate state and extra abstractions.
- **R4 — evidence/merge readiness:** pass/fail/unsupported matrix, exact native binary SHA/three OS, branch/run/artifact provenance, independent review. Only proceed when evidence rules are executable.

## 7. Progress / decision log

| Stage | Status | Evidence |
|---|---|---|
| Initial plan | Draft; PR creation pending | Based on #642, APM #3406, real UAP Codex source and release 0.167.0 |
| R1 | DONE — corrected installer-env vs Codex-parent-env confusion | See review log below |
| R2 | DONE — hostile package, child process, UX consent and audit oracle | See review log below |
| R3 | DONE — reuse existing CLI preview/permissions; no parallel system | See review log below |
| R4 | DONE — pinned native artifact, transient leak oracle, original evidence requirements | See review log below |
| Native Linux spike | First iteration FAIL (genuine Codex ran shell; MCP not discovered); **not** a secrecy pass. Revised trusted-project + live-scan iteration queued | [run #42863491111](https://github.com/777genius/universal-agent-plugins/actions/runs/42863491111), follow-ups #42864100790 and #42864292953 |
| Native 3-OS proof | Pending | Needed only if Linux proof and security acceptance pass |
| Production implementation | NOT STARTED | Security gate first |
| E2E / merge | NOT STARTED | No bypass if negative oracle |

### R1 — standards/source/UX review (2026-11-10)

**Finding R1-1 (critical):** installation-time `os.Getenv` is **not** Codex's future launch environment. Original plan improperly promised a missing-value preflight. **Correction:** validate only declared names before mutation; runtime availability is checked via native Codex invocation; remain `prepared/unverified` until real child receipt. Avoid fake `ready` and incorrect instructions to set variables in the installer process.

**Finding R1-2:** existing `conformance.Plugin` already parses `plugin.json.extensions` into opaque, validated namespace objects. `clients/codex/project_objects.go` calls shared OpenAI MCP projector and then Codex-only `readCodexProjectServer`; `nativeconfig.Server` currently lacks stdio Codex env-name array, whereas `projectCodexServer` already renders native TOML through the document codec. This permits a **small** Codex-only adapter without changing the portable mcp.json format, other clients or the core ownership kernel. Exact namespace semantics still need contract tests.

**Finding R1-3:** the native `env_vars` field includes **string and source-object forms** in current Codex docs. Scope v1 to **plain strings, implicitly local source** only; do not invent or copy `source=remote` support. Existing `CodexAuthOptions` is exclusively remote; do not overload it with stdio secrets.

**Evidence:** source files `conformance/plugin.go`, `clients/codex/project_objects.go`, `clients/codex/project_server.go`, `nativeconfig/types.go`, `nativeconfig/codex_server.go`, and current Codex MCP docs. Decision: ACCEPT WITH CORRECTIONS; P1–P4 native gate unchanged.

### R2 — adversarial security and UX review (2026-11-10)

**Finding R2-1 (critical):** a third-party package `plugin.json.extensions` can declare any forwarded variable name; package author consent cannot authorize exposing a user's ambient credential to an MCP child. **Correction:** surface exact names plus a one-time, versioned **user approval** at install preview; a new list or target binding requires renewed approval. Reuse the installed approval/operation contract if it exists; only add one narrow Codex-only input otherwise. Noninteractive clients fail closed. Do not add a broad secret-selection dialog or promise magic auto-detection.

**Finding R2-2:** APM PR #3406 still interpolates arguments, including name/value mismatches, even with `env_vars`. **Correction:** restrict opt-in names to bare, unambiguous aliases; reject at least selected MCP `args`/`cwd`/`command` values containing explicit `${NAME}`, `${env:NAME}` or `<NAME>` forms; **do not reinterpret** or alter values for non-opted portable entries. Do not attempt to scan real secret literals by reading host secrets; that would create a second dangerous exposure path.

**Finding R2-3:** native `env_vars` imports from Codex **parent**; a successful child `tools/call` is insufficient to prove secrets were not also written to Codex shell snapshots. **Correction:** force at least one actual shell-command runtime path in an isolated session, inspect *all* disposable native state after process exit (not only TOML), and include a control run without UAP. Exact source CLI and integrity evidence are required. A fail on unrecognized report paths is safer than a fake clean result.

**Finding R2-4:** Codex execution can start asynchronously, so 'installed' is configuration committed, **not** a claim of observed authentication. Use existing `ActivationPrepared` and one 'restart Codex, verify MCP' user action; never create a second wait loop, new daemon, or special status database. Unsupported or missing runtime value does **not** justify storing secret values in receipts.

**Open challenge:** if *any* released supported Codex defaults persist the canary in shell snapshots, a forwarding implementation cannot be described as 'non-persistent' merely because UAP writes only names. Owner consent is not a substitute for the security proof; fail closed per the P4 gate. Decision: ACCEPT WITH CORRECTIONS, *not production-ready*.

### R3 — Clean Architecture, SOLID/DRY and minimum UX review (2026-11-10)

**Finding R3-1:** `agentplugins add` already owns a plan preview/confirmation (`--yes`), but `--yes` is a *generic* installation approval and can run on untrusted package sources. **Correction:** display env-name list + native snapshot risk in the existing preview. Only an *explicit* target-qualified positive acknowledgement can enable forwarding in noninteractive usage; do not infer authorization from `--yes`, an installed plugin or a package author. This might require one narrow input through the existing command/management contract; account for the small incremental LOC instead of a new UI wizard.

**Finding R3-2:** generic `shared.ProjectOpenAIMCP` also serves ChatGPT and other OpenAI dialect readers. Keep namespace interpretation in `clients/codex` and Codex-only `nativeconfig.CodecCodex`. Do not change generic `conformance.ParseStdio` or `domain.MCPServer` for all agents, and do not modify `CodexAuthOptions` (it currently handles remote token references). This preserves Clean Architecture by keeping a client-specific schema at the client adapter boundary.

**Finding R3-3:** published `plugin.json.extensions` is an opaque map, so interpreting one **own** namespace is legal only for explicit Codex-native delivery; do not turn it into generic Agent Plugins 1.0 interpolation, and do not treat unknown third-party namespaces as executable instructions. Enforce strict duplicated/unknown fields **within our namespace only**; other extension namespaces remain opaque and untouched. No extra authored file; **zero migrations**.

**Finding R3-4:** Codex already sets `ActivationPrepared` after native project installation. Reuse this one human-visible status, standard `--scope project` UX and existing exact-membership ownership digest. Do not add credential-lifecycle state, a new status enum, global settings, a token vault or a secret-scanning background service. Revalidation belongs in the test harness, not production.

**Check of touched production areas:** `clients/codex` projection/selection; `adapters/nativeconfig` Codex codec; a single opt-in through existing installer/CLI input and preview. No global server generator, no Go DB/schema change, no migration. Decision: ACCEPT CONDITIONAL, native P1–P4 evidence still mandatory.

### R4 — release evidence, original artifacts, fail-open pitfalls (2026-11-10)

**Finding R4-1 (critical):** Codex 0.167.0 still contains `fs::write(output_path, snapshot)` in `codex-rs/core/src/shell_snapshot.rs` and a `shell_snapshots` directory; its executor also contains an unnamed FD transport, which is **not** proof all capture paths are fileless. A post-exit-only filesystem scan could falsely pass after native cleanup. **Correction:** require a **live interval scan during a deliberately long-running native shell tool**, plus post-exit, crash/restart and original artifacts. A green result from the initial small script without live observation is a *methodology check*, not security acceptance.

**Finding R4-2:** the public release `rust-v0.167.0` is a genuine Nov 6 stable GitHub release. Pin immutable Linux artifact `codex-x86_64-unknown-linux-musl.tar.gz` SHA256 `9d64836106d5b374e90ee4454c7ebcde7ca7106558bd7e41a34b9f26ebda251a`. Its macOS ARM64 and Windows AMD64 assets must each have independent asset SHA and native-runner provenance; never pretend Linux evidence covers them.

**Finding R4-3:** avoid **test circularity**: the local scripted Responses model must issue a real built-in shell tool, observe the resulting tool response, then call an **actual connected MCP stdio child** that proves receipt by digest only. A configuration inspection (`codex mcp list`), model request, or static synthetic reconstruction is **not** sufficient. The negative missing variable case must fail rather than show as expected skip. Exclude real accounts/keys and any OAuth.

**Finding R4-4:** CodeRabbit's explicit draft review command was rate-limited; this is **NOT** a positive independent approval. Record the four actual design critiques and their source justifications without claiming independent model agents, collect real CodeRabbit comments once available, and require a meaningful peer/security disposition before a sensitive production merge. No Strix-billing bypass.

**Result of fourth self-review:** plan has 4 concrete corrected rounds, but the **native secrecy uncertainty remains a hard GO/NO-GO gate**. **NO production implementation before the amended live-snapshot oracle.** The preliminary one-shot workflow can fail and provide negative evidence; do not soften it or interpret a missing screenshot/file as a passing security condition.

### Native spike checkpoint — initial evidence (2026-11-10)

- [Codex native security spike #42863491111](https://github.com/777genius/universal-agent-plugins/actions/runs/42863491111) used the **real pinned Codex 0.167.0 Linux binary** (official artifact SHA256 verified), not a model substitute. It invoked a genuine built-in shell tool, but ended **FAIL** because the MCP server was not advertised; therefore child `tools/call` and receipt were absent. Treat this as *an incomplete native protocol test*, **not** as proof the canary is safe. The preliminary post-exit scan also cannot rule out transient files.
- Corrections: trust the disposable project explicitly like the existing native UAP fixture; accept both direct function and deferred `tool_search` MCP advertisement; hold shell execution long enough for **live snapshot scanning**, then check post-exit. All values are randomly generated **synthetic only**. The `symlinks` count is observational (do not follow links), not automatically a security breach: some are Codex-owned internals unrelated to leaked content.
- Native or upstream release source may still leave canaries in snapshots. If observed, record only path hash/stage/result and stop before any production changes. If no canary is observed, require a complete MCP child receipt and three native platforms before production GO; a single Linux run is insufficient.
