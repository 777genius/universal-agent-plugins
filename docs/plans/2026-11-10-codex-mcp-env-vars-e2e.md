# #642 — Codex MCP: explicit environment *names*, no plaintext
> Design-first draft · 2026-11-10 · scope: **Codex Project stdio MCP only** · user request: plan → critical revisions → native spike → implementation → E2E → squash merge **only if security oracle passes**.

## 0. Problem and evidence

Today UAP's Codex project adapter projects selected Agent Plugins MCP entries through `clients/shared/ProjectOpenAIMCP` into staged `.mcp.json`, then reads `clients/codex/project_server.go` into `nativeconfig.Server`, and mutates only owned keys of `.codex/config.toml` using the existing TOML kernel. There is already collision protection, exact receipts, no-follow reads, atomic write and recovery. **Reuse those**.

Official [Agent Plugins 1.0](https://agent-plugins.org/specification) only expands `${PLUGIN_ROOT}`/`${PLUGIN_DATA}` in portable stdio inputs; an arbitrary `${TOKEN}` is literal, not a secret reference. Native Codex `mcp_servers.<id>.env_vars = ["NAME"]` passes **names** from the parent environment to the MCP child. [Codex source](https://github.com/openai/codex/blob/main/codex-rs/rmcp-client/src/utils.rs) verifies this. The [Microsoft APM candidate #3406](https://github.com/microsoft/apm/pull/3406) demonstrates the config shape but **also resolves secret placeholders in `args` and mismatched env keys**; do not copy its interpolation.

**Secrecy risk is independently confirmed:** previously pinned Codex **0.153.4** wrote a synthetic inherited secret into Codex-owned `CODEX_HOME/shell_snapshots/*.sh` during actual native execution ([UAP #642 evidence](https://github.com/777genius/universal-agent-plugins/issues/642#issuecomment-6632577675)). The [upstream bug #30971](https://github.com/openai/codex/issues/30971) is open. Latest tagged Codex inspected: **rust-v0.167.0, released 2026-11-06**; current shell-snapshot source includes credential-broker changes, **not equivalent to native non-persistence proof**.

## 1. Scope and UX

**Only this workflow:** owner explicitly opts into Codex Project `--scope project` stdio server; UAP writes name-only `env_vars` to one already-owned native entry; Codex's own runtime forwards it. Existing projects and non-opted server entries behave byte-identically. Other clients, User-scope, remote HTTP, OAuth, native Marketplace bundle registration and portable Agent Plugins interpolation are **out of scope**.

Candidate simplest authored input: a **target-qualified, names-only** member inside the **already accepted and preserved** `plugin.json.extensions` namespace (e.g., `"com.universal-agent-plugins.codex": {"mcp_env_vars": {"server-id": ["SERVICE_API_KEY"]}}`). There is already an extensions map with opaque namespace preservation. No separate manifest file, schema version, migration, global flag, new credentials store or new service. **Validate whether consuming this namespace is allowed by the existing conformance contract before implementing.** The v1 loader already admits a namespace object and preserves its bytes; **only `clients/codex` may interpret this one namespace**. If the extension has to be portable or affects ChatGPT, reject it and choose the existing *project-native configuration input* instead; do not invent parallel settings.

UX target: keep `--scope project` and normal add/update/repair/remove. At source/plan time reject **missing/empty/unsafe/ambiguous variable NAMES** before mutation. Do **not** test the variable's VALUE using the UAP installer's process environment: **Codex may be launched later with a completely different environment**, so installation-time `os.Getenv` is an invalid proxy. A missing runtime value must be reported as **prepared/unverified** (native preflight if available), with one action: `Set SERVICE_API_KEY in Codex's launch environment, restart Codex, then verify MCP.` **Never** prompt for, read/store, render or log a secret value in production. **Separate user consent** is mandatory; the package author's namespace alone is not permission to forward local credentials. Show the names + native snapshot warning in the existing install preview. Non-interactive installs **fail closed** unless an already-supported explicit approval record/option can express that consent; if none exists, add a single narrow Codex Project consent input (not global feature flags). If safe operation cannot be proven, report **prepared/unsupported**, not installed+working.

## 2. Minimal architectural delta (proposal, not yet implemented)

1. **Domain/source boundary:** parse the exact declared namespace in the Codex adapter at project selection (not arbitrary placeholders). Accept only explicit server IDs present in selected *stdio* entries and allowlisted `[A-Za-z_][A-Za-z0-9_]*` names (ASCII, max 128 chars, max 16 names per server), dedupe/sort deterministic; reject casefold collisions on Windows. Keep all values as **names only**. No cross-client extension propagation.
2. **Codex client boundary:** attach the names to the already selected Codex server after OpenAI projection, before `DesiredReceipt`/native activation, using existing Codex `Server` and `projectCodexServer`/TOML document codec. `env_vars` is a **server-level TOML array**, not inside `[mcp_servers.id.env]`. Do not rewire `ProjectOpenAIMCP` shared with ChatGPT. Source and desired ownership digest must both include the exact list.
3. **Security boundary:** never read the token value in UAP; reject names duplicated in literal `env`, any opt-in referencing `args`/`cwd`/command or URI/header, untrusted/foreign edits, malformed input, unresolved names, symlink configs, and attempts to silently turn off Codex snapshots. Never mutate a user/global Codex feature or shell startup file.
4. **Lifecycle:** add→repeat add→update with new name list→repair→remove; only managed exact member may change. A new name list requires renewed approval; an unchanged repeated install must not repeatedly prompt. No legacy adoption or rewrite; changing an already-managed entry uses existing receipts, foreign remains untouched.
5. **Proof:** no new production execution service, background daemon, CLI cross-platform flag framework, secret-value cache, helper wrapper, state migrations or feature flag. Keep special-casing in `clients/codex`/native `CodecCodex`; reject on other codecs.

## 3. Pre-code spike (mandatory, first)

- **P1 native compatibility:** pin the released genuine Codex 0.167.0 binary by immutable release artifact digest on **Linux amd64 + macOS ARM64 + Windows AMD64**. Use disposable project, `CODEX_HOME`, HOME and XDG; no real credentials, no network OAuth and no actual remote model: the model is a local scripted Responses fixture, Codex CLI/app-server is real.
- **P2 real child:** start one inert MCP stdio subprocess; call `tools/list` and `tools/call` via native Codex with a random *synthetic* value available **only to Codex parent**. MCP child reports only `boolean present` + stable one-way digest; no plaintext in logs. Prove the child **actually** saw the value (negative missing-value case must fail).
- **P3 negative oracle:** after complete process exit and crash/restart, recursively inspect project, `CODEX_HOME` (especially `shell_snapshots`), HOME/XDG, UAP receipts/state, argv, CLI evidence and original artifact ZIPs for raw/URL/base64/hex canary; no real secrets ever used. No symlink following. If a local trusted actor must set environment, do not claim that equivalent to safe default UX.
- **P4 isolated control:** compare bare native Codex with and without UAP config and with a shell command step that can trigger snapshot creation. If snapshot leak persists in current release, **STOP**: record reproducible safe evidence (no canary text), do not implement/merge secret-forwarding. Other code may remain design-only Draft until upstream fix. *Do not weaken the oracle*.

## 4. Tests and acceptance

Unit/conformance: extension parsing, negative/missing/malicious names, stdio-only, no arbitrary `${VAR}` conversion; three Codex project lifecycle mutations and native ownership digests; foreign TOML/comments/format preservation; duplicate cross-platform case; no secrets in config, logs, receipts, artifacts; negative oracle tests. Existing Claude/ChatGPT/Cursor/OpenCode non-opted fixture byte equality. E2E genuine Codex `tools/call` + install→repeat→update→repair→remove, fresh trusted session, 3 OS. CodeQL/Required/unit/landing only when changed paths warrant it; do not claim skipped as PASS.

**Merge gate:** 3-OS genuine native proof PASS; zero persistence findings across native snapshot family; CodeRabbit review threads resolved, clean CI on **exact HEAD**, reproducible original proof ZIPs, SHA-gated squash merge, postmerge main exact-sha tests. If unsupported or leak: keep feature **unmerged**, document blocker in #642; don't bypass authorization/third-party controls or ask for real secrets.

## 5. Estimates and non-goals

| Work | Production lines | Test/proof lines |
|---|---:|---:|
| Target-only parsing + preflight | 80–140 | 70–110 |
| Codex projection/ownership | 110–190 | 110–180 |
| CLI-visible status/consent using existing workflow | 45–95 | 45–75 |
| Native 3-platform proof and docs | 0–25 | 200–420 |
| **Total** | **235–450** | **425–785** |

**Estimate:** 660–1,235 changed lines. Risk: native upstream snapshot leak **9/10**, architectural regression **3/10** with strict scope, user workflow **2/10** if existing opt-in route suffices. No new tables, migrations, global gates, process owner or vault.

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
| R3–R4 | Pending | Update in this very file after each round |
| Native spike | Pending | No claimed proof |
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
