# P1 — Fix real /landing glob and minimatch dependency blockers

> **Status (2026-11-10):** Issue #692's vulnerable dependency graph was fixed separately by merged [PR #829](https://github.com/777genius/universal-agent-plugins/pull/829) (main lock has patched glob@14.0.4 and minimatch@11.0.5). This PR #792 is **preventive regression enforcement only**: retain a fail-closed lockfile graph check for the two scoped advisories and run it in both Landing validation paths. No dependency remediation, override, security waiver, or release occurs here. The historical triage below describes the pre-#829 state.


## Evidence
[Issue #692](https://github.com/777genius/universal-agent-plugins/issues/692): Dependabot reported `glob` / `minimatch` `security_update_not_possible` because the lockfile resolves old majors. This may be **transitive** build tooling; advisory existence does not automatically establish runtime exploitability. The published website and CLI E2E gates do not certify this separately.

## Minimal plan
1. Inspect actual `landing/package.json`, `landing/pnpm-lock.yaml`, `pnpm why glob minimatch`, Node/pnpm engines, and parent package graph. Enumerate resolved paths / advisories / production vs build-only exposure; obtain current metadata instead of guessing.
2. Prefer a supported upstream update that resolves fixed compatible package versions. Do not force incompatible global overrides merely to quiet alerts. Keep unrelated dependency churn out.
3. Validate clean `pnpm install --frozen-lockfile`, dependency tree, `pnpm audit`/advisory status, lint, typecheck, site build, registry tests and real browser E2E at desktop/mobile, light/dark.
4. Check generated artifacts and GitHub Dependabot alerts on the exact PR SHA. No silent suppression or security-ignore. Preserve unrelated site PRs (#773 and SkillSpector).
5. Review, full applicable CI, safe merge, post-merge site smoke and confirmed resolution; if upstream cannot resolve safely, retain the issue with explicit mitigation, not invented PASS.

## Non-goals
No Nuxt major migration solely for a transitive advisory, no security alert disabling, no blind pinning, no unrelated app changes.

## Verified current lock graph (2026-11-08)
Read the **actual 11,397-line** `landing/pnpm-lock.yaml` on `main`, without modifying production dependencies:
- `glob@10.5.0` is pulled by `archiver-utils@5.0.2` (Nuxt/Nitro build archive path). `glob@13.0.6` also exists independently. The relevant 2026 advisory requires `glob>=14.0.2`; no compatible fix within `^10` is available.
- `minimatch@3.1.5` is pulled by `@eslint/config-array@0.21.2`, `@eslint/eslintrc@3.3.7`, `eslint@9.39.5`, and `serve-handler@6.1.7`. Other versions `5.1.9`, `9.0.9`, `10.2.6` coexist. **Advisory ranges must be checked individually**: [CVE-2026-118764 / GHSA-g3hv-75wg-m249](https://github.com/advisories/GHSA-g3hv-75wg-m249) affects `>=7.0.0, <11.0.1`, **not** the 3.x/5.x copies; never label every lower major vulnerable on the strength of that single advisory. Check other distinct advisories on 3.x/5.x independently.
- `eslint` and `serve` are development/build-side dependencies in this app; `archiver-utils` comes through the Nuxt/Nitro archive toolchain. Whether an attack can reach production requests needs an explicit source-based analysis, not an ungrounded assumption.
- The current `landing/package.json` has `eslint ^9.39.5`, Nuxt `^3.21.10`, and `serve 14.2.6`. Dependencies cannot be safely "fixed" by blindly overriding major `glob@10` to 14 or `minimatch@3` to 11. Upstream archiver support and an ESLint 10 migration require compatibility testing.
- GitHub issue [archiverjs/node-archiver#822](https://github.com/archiverjs/node-archiver/issues/822) tracks the old glob major. ESLint maintainers state `@eslint/eslintrc` is frozen; switching ESLint generations may be needed.

**Next implementation criterion:** identify a supported compatible upstream release/resolution that really removes the vulnerable versions, regenerate and verify the *full* frozen pnpm lock in a real Node/pnpm environment, and prove browser/build/security E2E. Until then, keep this PR Draft and the alerts visible; do not fabricate a locked graph or silently waive vulnerabilities.

## Upstream compatibility checkpoint — 2026-11-08
- GitHub's reviewed [glob advisory GHSA-3g3m-2fjr-6pw4](https://github.com/advisories/GHSA-3g3m-2fjr-6pw4) explicitly covers `>=9.0.0, <=14.0.1` and fixes in `14.0.2`. `archiver-utils@5.0.2` still requires `glob@^10`. Upstream [archiver 8.0.0](https://github.com/archiverjs/node-archiver/releases/tag/8.0.0) was released 2026-05-08 and no longer has the old `archiver-utils` dependency, but replacing Nuxt/Nitro's `archiver@7` with this major is **not** certified compatible and must be driven by a supported parent update.
- ESLint upstream [PR #21492](https://github.com/eslint/eslint/pull/21492) merged **2026-11-07 15:21 UTC** to require `minimatch>=11.0.1`. The most recent upstream **published GitHub release** as checked on 2026-11-08 is [ESLint v10.14.0](https://github.com/eslint/eslint/releases/tag/v10.14.0) (2026-10-30), **before that merge**: do not assume the unshipped fix is available in a versioned dependency. Besides, this landing presently uses ESLint 9; do not jump majors or force a global `minimatch@11` override without parent/API tests.
- Scope the remaining review to the concrete patched advisory **and dependency path**, not generic old-version warnings. If safe compatible upstream versions do not exist, retain the narrow Draft and visible alerts; no frozen lockfile regeneration by hand, no green security claims without `pnpm audit` and native browser/build E2E.
