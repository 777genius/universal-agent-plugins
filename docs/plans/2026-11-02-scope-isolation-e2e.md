# User / project dependency state isolation E2E (2026-11-02)

## Why

Microsoft APM historical issue #794 wrote a global operation into the current project's lock. UAP uses explicit composition roots (`UserHome`, `DependenciesRoot`, `ProjectRoot`, `EnvironmentLockPath`) and must permanently prevent an analogous bug.

## Test-first scope

Use the **actual public Go CLI composition** with three disposable environments: user, project A, project B. No mock resolver and no network.
1. `deps init` and `deps lock` in all three contexts; assert the manifest and locked environment pairs have distinct, exact roots.
2. Modify only project A's valid target declaration; rerun `deps lock --scope project --project-root A`; project B and user lock bytes and file metadata remain identical.
3. Refuse contradictory `--scope user --project-root A` before any mutation. Explicit `--scope user` must always use user paths, never select a project by an unrelated project-root argument.
4. Refuse a nonexisting project path before touching any environment. Legitimate symlink aliases are canonicalized to their selected existing directory by the current project-root contract and must not be treated as independent project identities.
5. Repeat no-op lock and ensure no gratuitous rewrites; source provenance and native user/project state must not be conflated.

## Architecture and reviews

All changes should live in a new E2E test file in `cli/cmd/agentplugins`, not introduce new public APIs or abstraction. A failure may require an isolated fix at the composition root, never per-command cwd guessing. Strict SOLID/Clean/DRY, no new feature flags or VS Code edits.

- [ ] Real CLI 3-environment regression
- [ ] Actual CI on Linux/Windows/macOS with no skipped tests
- [ ] 3 lenses: source/scope security, data loss, architecture; fix feedback
- [ ] Squash merge only after current HEAD is qualified
