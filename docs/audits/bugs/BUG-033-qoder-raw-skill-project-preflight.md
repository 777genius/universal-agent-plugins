# BUG-033 — Qoder standalone project Skills return generic PLAN_REJECTED on all 30 sources

## Reproduction: real 30-source negative evidence

At original live PR head `97378eac646ea6f64686d37b7c564910137a918e`, [Qoder Skill ZIP 13315321747](https://github.com/777genius/universal-agent-plugins/actions/runs/41814723215/artifacts/13315321747) has exactly 30 real attempted, pinned Skill sources, 30 `install.exit_code=1` and 30 public `PLAN_REJECTED` responses. Each following `list` completed normally with an empty installation set. The genuine detected Qoder CLI was 1.1.81. These are failures, not documented unsupported/native activation or absent client. The public error is deliberately generic; the specific internal failure is not yet proven.

The supported Qoder project implementation is a **Prepared** bundle with selected `skills/<name>/` under `.agentplugins-qoder-native/` and *optional* separate native import; it must not touch the original user registry/cache. Existing project Qoder `add` tests do **not** prove the distinct raw `skills add` route.

## Strict minimal plan

1. Reproduce against the existing disposable local Git/TLS source and project fixture with one real raw `SKILL.md`: invoke the production `PrepareSkills` (preserving its underlying error *in test logs only*) and then public `skills add --target qoder --scope project` with actual source acquisition. Assert genuine pinned bytes, closed prepared Qoder view, exact receipts, repeat identity and removal, plus untouched foreign user files and zero Qoder native commands.
2. If the RED regression identifies a concrete component/plan/profile mismatch, fix **only** that rejected boundary with its exact ownership/selection conditions. Do not add generic fallback, change the Qoder plugin/user native install path, permit unsupported components, or manufacture automatic activation.
3. Run the focused CLI integration test, security-relevant Qoder provider tests and existing required CI. Re-run the 30 real Qoder Skills at the resulting final head and inspect original ZIP JSON and source/commit provenance. The test fixture is *not* a substitute for the genuine Qoder CLI run.

## Plan challenge — two explicit critic perspectives, not independent approval

- **Correctness critic:** A standard package `add` success and an empty native user registry do not establish raw `skills add` support. Require the standalone normalize/resolution/preflight path, then the actual Qoder project prepared file view and immutable source bytes.
- **Security/compatibility critic:** Never lift the PREPARED boundary, skip LintAI or alter the user's Qoder native profile/cache. No external/native executable needed for the hermetic regression: a native call is a test failure. Raw internal error must not be exposed through the public CLI/PR artifacts.

**State:** 30-source negative evidence confirmed; focused hermetic regression added to PR #698 to identify the first failing boundary; production root cause and final-head live qualification pending. No PASS/merge claim.
