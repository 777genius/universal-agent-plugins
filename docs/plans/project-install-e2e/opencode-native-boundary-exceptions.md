# OpenCode native fixtures: exact removed-contract scan exceptions

2026-10-25. Required run `40185974015`, job `121146724823`, passed its full
test and manifest stages, then failed the removed-contract boundary scan on
reviewed native-fixture controls and their plan. This amendment changes no
OpenCode adapter, fixture behavior or public environment compatibility.

In `scripts/check-removed-contract-boundary.sh`, retain the existing OpenCode
rule's expression, scanner function, failure behavior, rg/git-grep paths and
existing exclusions. Add exactly three literal file exclusions to that rule:

- `repotests/agentplugins_opencode_project_native_contract_test.go`
- `repotests/agentplugins_opencode_project_native_e2e_test.go`
- `docs/plans/project-install-e2e/opencode-project-native-gate-plan.md`

These files contain explicit disposable native observations and negative
ambient-inheritance assertions. They do not restore a public compatibility
surface. Add a short explanation beside the rule; add no directory wildcard,
new general exemption mechanism or exclusion for any other rule.

Two independent plan rounds precede the shell change, followed by independent
frozen code review. Validate shell syntax and the complete current-source scan.
In a temporary miniature Git repository, execute the actual copied script with
rg and with rg unavailable so its git-grep fallback is used. Both must admit the
three exact files and reject matching control references in CLI, SDK, an
adjacent test and an adjacent plan. Preserve the original failure and scanner
error handling. This bounded verification does not replace final Required CI.
