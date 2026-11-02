# UAP × Microsoft APM: adversarial failure classes and architecture (2026-11-02)

> **Evidence-based comparison, not an APM clone.** APM issues below are historical reports, many already fixed; they are input to UAP's negative regression matrix, not allegations about the current APM release. The code observations were sampled against both repos on 2026-11-02. Scores are qualitative, provisional until final CI/E2E and reviews. Security, product coverage and enterprise policy are not reducible to the architecture scores.

## Sources

- [Microsoft APM](https://github.com/microsoft/apm), [APM README](https://github.com/microsoft/apm/blob/main/README.md), [Python packaging and lint policy](https://github.com/microsoft/apm/blob/main/pyproject.toml), [APM install service](https://github.com/microsoft/apm/blob/main/src/apm_cli/install/service.py), [APM pipeline](https://github.com/microsoft/apm/blob/main/src/apm_cli/install/pipeline.py), [MCP client factory](https://github.com/microsoft/apm/blob/main/src/apm_cli/factory.py).
- APM historical incidents: [#2296](https://github.com/microsoft/apm/issues/2296), [#2368](https://github.com/microsoft/apm/issues/2368), [#1935](https://github.com/microsoft/apm/issues/1935), [#1716](https://github.com/microsoft/apm/issues/1716), [#794](https://github.com/microsoft/apm/issues/794), [#450](https://github.com/microsoft/apm/issues/450), [#844](https://github.com/microsoft/apm/issues/844), [#615](https://github.com/microsoft/apm/issues/615).
- APM advisories [GHSA-xhrw-5qxx-jpwr](https://github.com/microsoft/apm/security/advisories/GHSA-xhrw-5qxx-jpwr) (non-contained plugin component paths, patched upstream in 0.8.12), [GHSA-q5pp-gvjg-h7v4](https://github.com/microsoft/apm/security/advisories/GHSA-q5pp-gvjg-h7v4) (source symlink dereference, patched upstream in 0.13.0).
- UAP [architecture](../ARCHITECTURE.md), source [grammar](../../install/integrationctl/agentplugins/sourcespec/source.go), saved [environment](../../install/integrationctl/agentplugins/environment/model.go), [sealed snapshots](../../install/integrationctl/agentplugins/adapters/packagedigest/snapshot.go), [archive admission](../../install/integrationctl/agentplugins/adapters/sourceacquisition/archive_policy.go), [environment coordinator](../../install/integrationctl/agentplugins/environment/coordinator.go), [native installer](../../install/integrationctl/agentplugins/installer), [E2E catalog](../../cli/internal/agentpluginscli/testdata/package_manager_e2e_cases.json).

## Confirmed change in this PR

**Real preexisting defect, resolved in code:** `environment.validateRefSelector` and `environment.validateSubpath` duplicated, but weakened, `sourcespec.ValidateRef` and `sourcespec.NormalizeSubpath`. Direct installation already rejected Unicode/control characters, Git reflog expressions, `..` and `.lock` refs, Windows device names and trailing-dot paths. Saved `uap.toml` with canonical GitHub repository previously accepted some of these inputs; resolution rejected them later or behaved differently by platform. UAP now delegates both saved validators to the existing source grammar with **no new abstraction or alternate parser**.

- `TestSavedGitRefMatchesDirectSourceAdmission` and `TestSavedPackagePathMatchesDirectSourceAdmission` prove parity on adversarial and healthy values.
- `TestManifestRejectsWindowsReservedAndAmbiguousGitPath` rejects unsafe intent at the pure domain boundary.
- `TestSavedSourceAdmissionRefusalDoesNotPublish` uses the real CLI and verifies refusal with no download, lock publication, immutable-store writes, or installer state writes; registered as a required four-OS package-manager case.
- `pmAgreementLockOnlyPreservesInstalled` uses real CLI, native Gemini integration, sealed ZIP acquisition, dependency lock and state receipts: a lock-only change of desired root must **not uninstall or rewrite** already-installed package/native Skill/settings/receipts. The repeated lock is byte/metadata idempotent; registered in the required E2E catalog.
- `pmAgreementSourceRebindingRequiresExplicitTransfer` uses two distinct archive URLs with **identical package bytes**. A change in publisher URL behind a saved alias must not silently rebind its existing native ownership, even if content digests match; whether refused during resolution or installation, deployed state is unchanged. Registered in four-platform E2E.

No change to existing VS Code adapters or the Agent Plugins 1.0/Agent Skills 1.0 formats; no change to accepted ADRs or TOML contract.

## Historic failure-class matrix

| APM report or advisory | UAP mechanism / outcome | New regression? |
|---|---|---|
| #2296: lock-only removes managed files | `deps lock` publishes portable agreement; `install/deps sync` owns mutations | **Yes:** lock-only preserves installed; must pass actual E2E |
| #2368: Git auth environment overrides Git hardening | `sourceacquisition` isolates inherited `GIT_*` variables and explicitly sets Git options; existing Git transport tests | Existing contract, requalification required |
| #1935: changed origin + version silently reused old dependency | `SourceKey`, requested selector, validated lock identities and exact replay | **Yes:** identical bytes at changed archive origin must not silently adopt existing owner |
| #1716: deployed Skill ignored by drift detection | Native Skill ownership and receipt/projection digest in installer | Existing adversarial native Skill/drift cases, requalification required |
| #450: repeated install changes lock timestamp | Deterministic `uap.lock` and no-op publication | **Yes:** actual installed-state + lock stability |
| #794: global writes project lock | Scope/manifest roots are explicit and separate; project-context tests | Existing scope cases, requalification required |
| #844: wrong native target MCP schema | Target-specific adapters; no generic schema or VS Code changes | Existing target adapter test suites |
| #615: proxy/network restrictions omitted | Source transport boundary, local/offline and credential policies | Existing source transport tests; no claim of exact proxy-only equivalence |
| GHSA-xhrw-5qxx-jpwr: plugin component path exfiltration | Sealed tree inventory + portable path checks before installation | **Yes:** reject nonportable saved package paths; existing snapshot/path tests |
| GHSA-q5pp-gvjg-h7v4: symlinked agent files leak content | Sealed snapshot validates contained symlink graph, archive formats refuse link entries; loader skips unsafe Skill pointers | Existing symlink and hostile archive suites |
| Windows TAR traversal/absolute path | `archiveNormalizePath` + portable Windows segments + strict TAR/ZIP extract | Existing hostile archive ZIP/TAR suites |

**Risk:** tighter saved-source validation may reject old manually authored `uap.toml` values that already failed at later direct acquisition. Expected migration is to use a portable exact ref/path, not to bypass integrity checks.

## Independent review lenses (human-readable review checklist)

1. **Security / supply-chain:** inputs must be bounded and validated before I/O, lock must bind canonical origin and immutable bytes; hostile links and paths cannot escape package root. Ref and path parity prevents a saved-vs-direct policy bypass. Verify all checkout/HTTP acquisition paths use the same policy, including cold restore.
2. **Lifecycle / data-loss / transaction:** lock-only must never call target mutation; no-op should not rewrite lock or installation metadata; foreign/unowned/native Skills cannot be adopted or removed; failed operations must retain original receipts and recover safely. Test both successful re-resolution and mandatory fail-closed behavior.
3. **Architecture / portability / DRY:** pure source grammar lives in `sourcespec`, `environment` delegates and owns dependency semantics; acquisition stays in adapters, transaction stays in installer. No added OS-specific special case in the domain, no duplicate version of an existing validator, no blanket framework or APM YAML reimplementation.

## Comparative architecture score (sampled, provisional; 0–10)

| Property | UAP after this PR | APM 0.35.0 | Reasoning |
|---|---:|---:|---|
| **SOLID** | **8.0** | **7.0** | UAP has Go domain/coordinator/interfaces and target adapters with explicit effect boundaries. APM has named service and target-factory abstractions, but its install pipeline still pulls many collaborators dynamically and maintains compatibility re-exports. |
| **Clean Architecture** | **8.0** | **7.0** | UAP's environment/source/installer ports and durable transaction boundaries are explicit. APM has an InstallService facade and pipeline phases, while the pipeline retains infrastructure and presentation-specific imports and implicit context wiring. |
| **DRY** | **8.0** | **7.5** | UAP now shares direct/saved ref and path rules; remaining client-specific behavior is intentional. APM centralizes client factories and rules but retains documented alias/re-export compatibility surfaces and duplicated orchestration seams. |

These are **reviewer judgments from sampled code**, not coverage or proof that UAP has fewer defects. APM is clearly ahead in some independent capabilities (enterprise inheritance, audit, SBOM, broader primitive support); UAP should only adopt missing safety invariants that fit UAP's architecture and scope. Do not inflate architecture scores based on feature count or historical incident count.

## Final acceptance (not certified merely by this document)

- [ ] Final SHA passes format/lint, Go tests, native cross-platform build and dependency/source gates.
- [ ] New case IDs show `run` + `pass` (not skip) in archived E2E report on Linux, Windows and both macOS matrices.
- [ ] Security, lifecycle and architecture review findings are resolved with tests.
- [ ] No regressions in global/project and Native Skills installation, no new feature flags, no VS Code edits.
- [ ] Only then mark PR review-ready/merge; do not call the work E2E-complete while jobs are pending.
