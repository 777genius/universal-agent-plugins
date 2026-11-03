# BUG-005 — Per-case evidence lacks the executing client and source-build versions

## Evidence (2026-11-04, current audit script and CI)

The 30+30 JSON files include the target ID, CLI path and fixture names, but not a captured client binary version, UAP CLI version, workflow commit SHA or exact fixture digest. The first nine-client workflow pins npm versions, yet its version command only writes to the ephemeral job log. The remaining-client workflow captures a vendor version in a separate artifact path, not in the per-target JSON. Current source mixes a PR-built Go CLI with a deliberately separate published npm smoke. Without machine-readable version provenance a historical PASS cannot be unambiguously compared with a new run or its source selection. CodeRabbit also raised missing client version provenance in its review of the main audit workflow.

## Small plan

1. Record `GITHUB_SHA`, `GITHUB_RUN_ID`, fixture SHA-256, OS/architecture and observed UAP CLI/native-client `--version` output once in each target/kind report. Use the same disposable allowlisted child environment for version subprocesses; keep output short and never include native configuration or tokens.
2. Mark failed, unsupported or empty version probes explicitly as **unverified**, never manufacture a version, and do not equate an executable path/version response with activated client usage. The audited install/remove result remains the authority for lifecycle claims.
3. Add focused subprocess-free regressions for the version-output classifier. Run the full real CI matrix on the resulting commit and compare all artifact metadata to its exact GH source SHA.

## Critical plan review

- **Evidence critic:** Merely pinning a version in a YAML command is not enough; retain the observed value in the same JSON as install outcomes and bind to exact source commit. Vendor installer/VS Code downloads may move even when source URLs do not.
- **Security/compatibility critic:** Do not capture full `env`, OAuth, commands with login credentials, or native user profiles. Failed version queries must be visible, not treated as success. Avoid introducing a new external dependency or modifying production adapters.

**State:** version and CI/fixture provenance collection plus focused regression pushed in PR #698; current-head live artifact verification pending.
