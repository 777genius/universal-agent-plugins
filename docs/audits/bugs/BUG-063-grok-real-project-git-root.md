# BUG-063 — Grok project Skills need a real isolated Git root for native inspect

## Real reproduction
PR #698 source `7886689fd823cacc9f31035dc06d00408f7594d6`, original [Grok Skills 30-source ZIP #13438580451](https://github.com/777genius/universal-agent-plugins/actions/runs/42065905454/artifacts/13438580451): genuine SHA-256-pinned official Grok Build 1.0.70, all 30 source-backed installs fail with nonzero `grok project inspect contract (1.0.67 or 1.0.70) is not recognized`. The parallel [Grok Plugins ZIP #13438031348](https://github.com/777genius/universal-agent-plugins/actions/runs/42065905454/artifacts/13438031348) has 29 lifecycle PASS and one honest source incompatibility.

## Cause hypothesis and scope
UAP's `clients/grok/project_inspect.go::parseProjectInspect` requires a verified native `projectRoot` exactly matching the target project; official Grok upstream `inspect/mod.rs` computes `projectRoot` with `git2::Repository::discover(cwd)`, yielding null for an ordinary non-Git directory. The live audit already creates a real disposable Git worktree for Kimi project Skills but omitted Grok Skills. The raw ZIP does not include direct native inspect JSON, so this remains source-supported and reproducible through isolated Git discovery until the next true Grok 30-source E2E qualifies it.

## Narrow change and review
- Reuse the current no-network, hooks-disabled, pinned Git init helper for *only* Kimi/Grok project Skills. No new flags, new adapters, mocks of source installation, or production Grok parser change.
- Preserve disposable `HOME`, `XDG_*`, `GIT_CONFIG_NOSYSTEM=1`, empty global Git config, disabled prompts, existing source/manifest strictness, LintAI and foreign sentinel; never promote blocked or manual/prepared outcomes to PASS.
- Update the existing no-network target-matrix regression and add a real local `git init` test proving Grok inspect's precondition, isolated project root and intact foreign neighbor.

Correctness review: root discovery changes from null to exact real project path, retaining UAP's strict native identity requirement; inspect remains the actual CLI oracle. Security review: Git writes only inside its disposable project directory without hooks or interactive/network calls; no ambient user profile, registry or native application trust is bypassed. Both reviews are source-level, **not** final native 900-case approval.

**Acceptance:** publish only after preceding exact-head E2E ZIPs are collected; verify focused no-network tests and all applicable CI, then new complete 30-lane original ZIP set. Grok outcome may remain non-PASS for other independently observed reasons; diagnose rather than weaken contract.
