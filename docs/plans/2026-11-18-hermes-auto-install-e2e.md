# Hermes Agent automatic installation — integration and E2E (2026-11-18)

## Current baseline

- Public native CLI 0.1.110 already includes target `hermes`. Its adapter uses the same UAP delivery plan, native configuration kernel, ownership receipts and recovery lifecycle as the other agents.
- User scope: Skills plus supported stdio/Streamable HTTP MCP. Project scope: Skills and, **only with a qualified standalone Hermes bundle**, an explicitly selected isolated MCP session.
- A confirmed project `add` can ask the user's native CLI to trust the exact Git root; the safety checks and existing User config/profile authority must remain intact.
- Source-installed Hermes Project MCP is an explicit documented limitation, not an authorization to inject a project server into the global User profile (issue #942).
- Native Hermes 0.23 qualification exists on three OS; its exact-head evidence cannot substitute for executing the final implementation in a new PR.

## Goal and boundaries

Make `--target hermes --scope user|project` a predictable, automatic and verifiable installation path, matching the existing client abstractions without introducing a second installer. Explicit confirmation, ownership, foreign-file preservation, profile isolation, native process cleanup and a truthful **Prepared** status are non-negotiable.

## Approach

1. Audit the current adapter and shared installer contracts against Claude/Codex/Pi; identify actual deviations before changing behavior.
2. Repair only proven Hermes-specific gaps and integrate through the existing `clients.Adapter`, `Lifecycle`, `PreparedSetupCompleter` and `SessionPlanner` contracts. No new generic framework, duplicated YAML writer or hidden trust grants.
3. Add focused regression tests for real CLI delivery/repeated add/update/repair/remove, explicit Git-root trust, profile isolation and negative cases. Use actual Hermes native installation on Linux, macOS and Windows via existing pinned E2E workflow; preserve exact source/runner identity and do not count skipped or queued jobs.
4. Inspect CI plus outstanding review comments; iterate fixes, re-run only necessary checks, merge only after actual required execution and final source review.

## Supported / unsupported behaviors

| Operation | User | Project |
| --- | --- | --- |
| Skills | Automatic owned preparation; restart/discovery | Automatic owned preparation; exact-root native trust after confirmed add; new session |
| stdio and HTTP MCP | Automatic owned `config.yaml` changes | Only inside qualified standalone bundled Hermes UAP session |
| Source-installed Project MCP | N/A | Explicit refusal with actionable alternatives until isolated native source-runtime safety is qualified |
| Foreign files/servers, unknown components | Never overwrite or discard | Never redirect to User or silently discard |
| Uninstall | Remove owned entries only | Remove owned entries only; preserve user trust choice |

## Acceptance

- Fresh, repeat, update/repair, remove, and recover from partial failure; User and Project A/B separation.
- Native list and discovery for Skills; MCP list/test and no-auth direct tool-call evidence where supported.
- No unrelated `config.yaml` mutation, no trust on plan/verify/update/repair, no project-MCP leakage into global User configuration.
- Current HEAD: Go tests, supported CLI E2E, 3-platform genuine Hermes native CI and required gates successful; code reviewed; PR merged and post-merge outcome checked.

## Size/risk estimate

Initial audit and targeted fixes: 100–400 lines, effort 5/10, reliability aim 9/10, confidence 7/10; larger source-installed Project MCP support requires a separately verified runtime design and must not be smuggled in as a bypass of the manifest check.
