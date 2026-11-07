# Agent-first CLI contract: balanced P0/P1 implementation plan

Status: approved working plan for one Draft PR. Baseline: `c5efe1f6bfdc` (main, 2026-11-06).
Owner goal: AI agents can discover, preview, install, inspect and recover plugins and Skills without TUI surprises, while people retain the existing TUI.

## Non-negotiable scope

- One PR and branch. Commit the plan first, then implementation/test/docs commits to the same PR; merge only after exact-head required checks and review.
- Keep **one** installer, resolver, transaction kernel, ownership model, lock and security evaluator. This is CLI contract/presentation work, not a runtime rewrite.
- Preserve the existing `--format json` payloads consumed by integrations. New fields are additive; do not silently rewrap old responses or break the Go/TypeScript Management SDK.
- Keep the default `user` scope on add/skills and the existing argumentless-install `uap.toml` heuristic. Expose and document the effective scope instead of silently changing it.
- Never bypass assessment, consent, auth, native client trust, foreign file ownership, conflict, rollback or recovery rules. `--yes` means user-authorized plan acceptance, **not** external authentication or an override of security policies.
- Do not touch frozen VS Code code, generated files, deprecated authoring CLI, registry feed or nine-language published site journeys. A new CLI release is outside this PR.

## P0: machine-readable CLI, explicit selection, Skill discovery

### P0.1 Stable machine-output/error boundary (estimated 410–710 total LOC including tests)

Files: `cli/internal/outputjson`, `cli/internal/agentpluginscli/{output.go,skills_selection_errors.go,skills_terminal.go,skills_review_output.go,deps_apply.go}`, `cli/cmd/agentplugins/main.go` only if required.

- Inventory existing JSON shapes before changing them. `add/list/info/doctor` use `outputjson.Envelope`; `deps` and Skills have legacy top-level shapes. **Do not break** either shape.
- Supply stable, sanitized error codes and retry/remediation hints for common automated failures: selection required, target unavailable/unsupported, invalid scope, conflict/foreign destination, security refusal, stale plan, network or timeout, pending recovery.
- For commands already producing an envelope, extend the envelope additively. For Skills and deps, preserve existing top-level fields and add a predictable small metadata/error object or structured fields; avoid wrapping in a breaking schema.
- Guarantee one JSON document on stdout for in-scope successful/planned/typed-refusal paths; progress goes to stderr. Generic command execution errors remain stderr with nonzero status until a compatible boundary is proven.
- Distinguish operation failure from a legitimate `needs_action` state. A JSON `success` does not imply native runtime authentication or a successful tool call. Do not turn unobserved native state into `ready`.
- No leaking filesystem paths, credentials, source diagnostics or raw provider messages in newly emitted public errors.

Acceptance: parseable one-document stdout; stable codes, nonzero on actual failure; no false success for preflight/apply failures; prior JSON fields remain unchanged; exact cases for multiple clients.

### P0.2 Consistent, non-interactive confirmation (estimated 200–350 LOC including tests)

Files: `agentpluginscli/{root.go,add.go,install.go,skills.go,lifecycle_commands.go,lifecycle_helpers.go}` and existing confirmation helpers.

- Add an explicit `--yes` for source-style `add` / `install <source>` and non-destructive maintenance where absent, reusing existing confirmation checks; existing `install --yes` and `skills add --yes` remain unchanged.
- Preserve existing accepted scripted `add --target` and `add --format json` behavior in this backwards-compatible iteration; document them as legacy auto-approval. Do **not** reinterpret `--format json` as explicit authorization in *new* code paths.
- No terminal picker in JSON/non-TTY mode. Missing target/selection fails before mutation with a stable diagnostic. For Skill mutation, continue requiring `--yes` outside interactive review.
- `--dry-run` takes precedence over `--yes`: no native target mutation. No automatic `--accept-security-risk`, `--auth-complete`, `--activation-complete` or purge.
- Avoid introducing `--force` or a second confirmation/permissions framework.

Acceptance: stdin closed/absent; noninteractive add/Skill/locked install/repair; no prompt, no unexpected bypass; pre-existing invocations still work.

### P0.3 Read-only standalone Skills listing (estimated 130–250 LOC including tests)

Files: `agentpluginscli/skills.go` plus a small `skills_list.go`; reuse the existing `skillsCatalogSession.scan` and sanitizing DTOs.

- `agentplugins skills list <owner/repo|URL> --format json` lists exact candidate names, relative paths, validity/diagnostics, and source revision when available. Must be deterministic and bounded by the existing scanner; never execute Skills, start the browser or open a modification transaction.
- Accept `--path` for a specific repository-relative folder and `--ref` for ambiguous Git refs using existing source grammar. No new discovery protocol, network service, crawler or registry.
- Include invalid/unavailable candidates with their reason rather than falsely reporting them installable; disclose incomplete scans and do not silently turn partial inventory into complete.
- Fail closed for an unsafe source or malformed path and sanitize JSON responses.

Acceptance: single-, nested-, duplicate-name, invalid and incomplete catalogs; no TTY; no project/user changes; CLI smoke.

## P1: lifecycle clarity, scope transparency and documentation

### P1.1 Derived lifecycle summary and next actions (estimated 180–340 LOC including tests)

- Add additive `readiness` and structured `next_actions` to `add`, Skills result and `deps status` where existing data permits. Classify as `ready`, `needs_action`, `failed` or `unknown` from *recorded evidence*, not inferred native success.
- Actions carry stable code, actor (`agent`, `user` or `native_client`), and text. Keep legacy `next_action`, `facts`, materialization/activation/auth/verification fields intact.
- Avoid a persisted state machine or implicit automatic repair; status is projection-only. A manual attestation is not proof of native runtime verification.

### P1.2 Explicit scope provenance (estimated 80–170 LOC including tests)

- Add additive scope selection facts (effective scope and `explicit|argumentless_project_manifest|default_user`) to previews/results where a selection already exists.
- Explain scope in new agent-oriented documentation and machine examples. Preserve existing defaults; do not automatically rewrite global state.
- Verify exact working-directory `uap.toml` behavior, explicit overrides and failure for symlinked manifest.

### P1.3 Agent-oriented usage documentation (100–180 lines)

- New `docs/AGENT_CLI.md` with two short examples: complete Directory package, standalone GitHub Skill; show inspect/dry-run/apply/status, `--target`, `--scope`, `--project-root`, `--locked`, `--yes`.
- Document current stable release versus source-only features and exact lifecycle evidence levels.
- Add a concise README link under the CLI capabilities; never claim automatic activation of external clients or released source-only features.

## Implementation steps & commits (same PR)

1. Plan + two critical passes; commit and open Draft PR.
2. Machine JSON diagnosis and confirmation additions, tests and push.
3. Read-only Skills list using existing catalog session; tests and push.
4. Derived readiness/next actions + scope provenance + docs; tests and push.
5. Integration review, fix findings in same branch, run exact-HEAD checks and native E2E relevant to behavior, then merge SHA-guarded squash.

## Tests (risk-based, no redundant matrix)

- Fast: targeted `go test ./cli/internal/agentpluginscli/... ./cli/internal/outputjson/...`, golden JSON assertions, `make lint`, `make test-core`.
- E2E: built CLI with stdin closed, JSON stdout/stderr separation; selected user/project scopes; one Skill source nested folder and bad metadata; `--yes`/dry-run; select two clients with partial failure; verify no mutation on selection/foreign/security refusals.
- Existing CLI project-native, rollback, recovery and cross-platform CI where touched. No fake mocked 'native activated' proof.
- Required PR CI checks for exact final HEAD, review threads addressed; do not merge on running/skipped/failed checks.

## Critic pass 1 — compatibility and upgrade risks (incorporated)

- Original proposal 'one universal JSON envelope for every command' would break existing callers. Corrected to **additive existing shapes**, with documented exceptions and predictable codes. Full versioned schema may be a separate later major-version proposal; not in this PR.
- Original proposal 'require --yes everywhere' would break existing scriptable `add` flows. Preserve old acceptance and make `--yes` the recommended explicit route; do not expand auto-consent.
- Avoid changing implicit project selection; expose why the scope was chosen.

## Critic pass 2 — security, correctness and UX (incorporated)

- `--yes` only accepts the prepared plan. It cannot assert native activation, authentication, security override or ownership.
- The Skills list shares existing bounded acquisition/parser and must not fetch untrusted scripts for execution; an incomplete catalog is visible.
- Readiness is **derived**, not another persisted state; native trust/OAuth remain separate.
- Avoid duplicate installer/SDK, generic config injection, new agent server, and speculative flags.
- Prefer a handful of high-value hermetic E2E cases to hundreds of redundant tests; run actual required gates before merge.

## Definition of done

All P0/P1 acceptance criteria genuinely implemented, tests and relevant native checks pass on exact PR HEAD; one updated Draft PR throughout development; self-review findings fixed; no outstanding critical issue; merge to `main` only with verified checks and explicit SHA guard. If a genuine unverified native boundary remains, report it instead of claiming full E2E.
