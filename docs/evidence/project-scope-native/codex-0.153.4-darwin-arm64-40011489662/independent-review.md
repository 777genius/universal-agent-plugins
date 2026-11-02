# Codex project native evidence — independent macOS interpretation

> **Historical snapshot.** The observations and qualifications below belong to
> their recorded source. The [original repository directory](https://github.com/777genius/universal-agent-plugins/tree/3b93148c356fc5db6fe101d81d943160a5176b6d/docs/evidence/project-scope-native/codex-0.153.4-darwin-arm64-40011489662) preserves the
> files present before raw-output cleanup. Adjacent metadata and source remain;
> original manifests describe that snapshot, not the current directory inventory.
> Retrieve removed raw files there; see the [project evidence index](../../PROJECT_INSTALLATION.md).

Reviewer codex_claude, 2026-10-23. **PASS for the bounded native project-skills discovery and production file lifecycle actually exercised**, not full Codex project MCP/runtime qualification.

## Immutable artifact and execution identity

Root identified GitHub Actions run40011489662, job120612704190, artifact12527438580. Independently read the downloaded ZIP at `research/hosted-evidence/codex-darwin-40011489662/artifact.zip` (122 members, 505,497 total uncompressed bytes). Its measured SHA256 is `f51f7e9d15a4150d2a8ea8c26afd973b25ad6b1a69b9523363491c24d52e708b`. Independently verified all28 project-summary artifact digests against the exact `uap-native-evidence-689709854/` prefix; names shared with the separate user suites were not conflated.

Runner/evidence source identity agrees: remote installer base/head `0d1319f947c7f4403691b3a60b1193baa0074ee9`, committed tree `4b8935ef80c3834f7145dd01289772a48379ba3f`, no source patch, installer SHA256 `906be0c776457c34b7fa4b9beab8df77b9be3cd6015b9ef696932acc9c8b8e76`. Measured genuine client is `codex-cli 0.153.4`, macOS arm64, binary SHA256 `b973d440acac501fd2594a43e7ca9ce41e0a65b9dfb28d0d7a7837c99e1261e3`, acquired from official rust-v0.153.4 aarch64-apple-darwin archive with recorded archive digest. This is a disposable hosted run; the runner explicitly claims no OS/process isolation and does not block network. The project subtest starts fresh app-server processes with explicit profiles and no thread/model/trust/MCP initialization.

The original artifact's `native-tests.log` contains an actual `=== RUN TestAgentpluginsCodexNativeProjectComponentsLifecycle` followed by `--- PASS ... (2.25s)`, not SKIP. The summary's measured start/end and stage statuses correspond to the project records, separate from the two user/command suites also present in the ZIP.

## Raw native observations

Inspected each project RPC transcript, including outgoing initialize/skills-list requests and the received id2 `result.data` response. Each response selects the exact requested cwd and reports no skills-list errors. The transcript distinguishes repository scope and actual absolute project paths from user paths:

- After production UAP project add, A's `.agents/skills` contains enabled repository `fixture-guide` and rendered `uap-commands-fixture-echo`; B contains neither fixture. A/B each retain their foreign repository marker and the separate user marker.
- After invocation-rule update, A lists `fixture-guide` and rendered `uap-commands-fixture-wizard`, with echo absent. B remains fixture-free.
- After removal of A's owned wizard directory, a fresh native response contains fixture-guide but no wizard. After UAP repair, fresh A lists wizard again, while B remains fixture-free.
- After UAP project removal, fresh A and B responses contain neither owned fixture and still contain their foreign repository/user markers.

The independently inspected source subtest requires selected command/guide paths and rendered bytes, verifies A/B native discovery, exercises missing-directory repair, and checks foreign bytes at completion. The artifact's test PASS supports those assertions. Raw RPC establishes paths/scope/enabled/absence; byte equality and complete foreign config/cache preservation are asserted by the passing harness, rather than encoded as skill-body fields in skills/list itself.

The production add/update/repair/remove JSON outputs each report success. Add/update/repair remain prepared with installation verification, rather than manufacturing active attested state. Remove reports data retained; this is retained receipt/file lifecycle evidence, not a native workload demonstrating plugin data read/write.

## Concrete boundary retained

Native configWarning messages explicitly state project-local config/hooks/exec policies are disabled until the project is trusted, while skills still load. No trust was granted to make this result pass. This successful untrusted repository skill discovery therefore does not establish that project MCP config is effective. The project fixture is skills-only, and summary marks native MCP initialization and model use not_evaluated.

This evidence qualifies this macOS arm64 pinned-version project-skills discovery/add/update/missing-skill-repair/remove/foreign-preservation surface. It does not qualify Windows/Linux, every source transport, current-session MCP reload/removal behavior, account/authentication, model/tool use, project MCP initialization or all-client/release completion. Keep product project status conservative; root should append these bounded observations to aggregate evidence without pushing/canceling still-running leaf CI.
