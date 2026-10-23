# Kiro project implementation — independent R1

Reviewed initial `9eeadc1a47650d95c803ae043fa9411ce51ae21c`, scoped Kiro diff `426a82b3..9eeadc1a`, and final correction `b3b10e7ac15b0c6d5049826b831c1f84d143513d` in `work/uap-project-kiro` on 2026-10-23. Reviewer did not author Kiro. **Final GO at b3b10e7a; the narrow scope-admission finding is closed.**

## Finding

`clients/kiro/scope.go:kiroProjectRoot` compares the original user configuration root only when `TargetContext.UserConfigRoot` is nonempty. A caller can therefore drop a known original user profile from the selected project context while retaining a nonempty detected `Client.ConfigRoot`; the scoped admission no longer verifies that the original authority was preserved. Require literal equality between the validated context's user root and detected config root (both empty remains valid), and extend the existing composed-provider mismatch table with omitted user context. This is a narrow project admission correction; user lifecycle should remain unchanged.

Closure: `b3b10e7a` changes the project admission to exact equality and adds the omitted-user case to the existing refusal/unchanged-byte table. The user branch is unchanged. Independently reran `TestProjectKiroScopeRootAndBindingAuthorityRefuse`: **PASS 0.027s**. No other blocking finding remains.

## Otherwise reviewed

The implementation selects explicit project `.kiro` native paths and `.uap` package/data/state without redirecting the detected user profile. It reuses the existing exact JSONC/locked native transaction and verified skill swaps/rollback, checks exact project artifact and recorded binding, and validates kinds, IDs, duplicate receipts, portable source paths, native paths and aliases before mutation. Core-owned package receipts remain outside the native writer. Stdio selection binds the trusted launcher to absolute selected package/data/cwd and active reuse refuses a different data receipt without rewriting immutable bytes.

Project add, prepare and verify-only remain prepared with installed-file evidence. The real provider preflight rejects all project activation attestations before its generic manual branch; no user ACP invocation, trust migration, account/model or MCP process is substituted for discovery. Genuine pinned Kiro version/help is kept separate from the project fixtures and does not qualify an active custom agent, engine or IDE. No hosted guard or native runner was changed.

The public SDK uses the production engine/filesystem/state with inert unexecuted launcher bytes and checks A/B data separation, old-pin repair despite moving source, a fresh engine, retained data and exact removal. Public CLI tests use production scope composition, solver/store/installer/native transactions with genuine loopback TLS Git/ZIP, plus labelled detection/security seams. They exercise no-op/read views, repair/update, portable export/cold import and isolation without claiming native Kiro execution.

Independent bounded verification at the frozen SHA: `go test -p=2 ./install/integrationctl/agentplugins/clients/kiro ./install/integrationctl/agentplugins/providers ./install/integrationctl/agentplugins/installer ./cli/cmd/agentplugins -run '^TestProjectKiro' -count=1 -timeout=3m` passed: client **0.009s**, providers **0.085s**, SDK **0.171s**, public CLI **5.387s**. No test/policy assertion was changed during this review.
