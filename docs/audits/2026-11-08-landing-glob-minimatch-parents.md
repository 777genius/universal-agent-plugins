# Landing dependency advisory triage — locked-parent evidence

Checked the exact `landing/pnpm-lock.yaml` and `landing/package.json` from the #792 branch based on `main` `75e441a0`, not a guessed fresh resolution. This is a **read-only investigation**, not a fix or production runtime security attestation.

## Actual old-major parents in the pnpm v9 lockfile

| Locked package | Immediate locked parent | Classification / investigation |
| --- | --- | --- |
| `glob@10.5.0` | `archiver-utils@5.0.2` | Archiving dependency used through Nuxt/Nitro build tooling. Upstream `archiverjs/node-archiver#822` confirms `archiver-utils@5` depends on `glob@^10` and requests an upstream supported fix. |
| `minimatch@3.1.5` | `eslint@9.39.5` | Developer lint only; outside GHSA-g3hv-75wg-m249 after its reviewed range narrowed on 2026-11-04 to `>=7.0.0, <11.0.1`. Other advisories must be assessed separately. |
| `minimatch@3.1.5` | `@eslint/eslintrc@3.3.7` | ESLint legacy config compatibility, build/developer tooling. |
| `minimatch@3.1.5` | `@eslint/config-array@0.21.2` | ESLint config traversal; developer tooling. |
| `minimatch@3.1.5` | `serve-handler@6.1.7` | Transitive dependency of direct dev dependency `serve@14.2.6`; no direct `serve` invocation was found by repository-wide code search, but production exclusion must still be confirmed against actual generated output before claiming it. |

Other, distinct installed majors also exist: `glob@13.0.6`, `minimatch@5.1.9`, `minimatch@9.0.9`, `minimatch@10.2.6`. Under the current GHSA-g3hv-75wg-m249 range, minimatch 9/10 are affected but 3/5 are not. Upgrading one affected parent cannot establish advisory closure for all paths.

## Advisory scope and safe strategy

- `glob` [GHSA-3g3m-2fjr-6pw4](https://github.com/advisories/GHSA-3g3m-2fjr-6pw4) concerns post-abort filesystem walking; the published fixed line is `14.0.2+`.
- `minimatch` [GHSA-g3hv-75wg-m249](https://github.com/advisories/GHSA-g3hv-75wg-m249) concerns `optimizationLevel >= 2` + pathological patterns; the published fixed line is `11.0.1+`.
- The currently affected glob 10/13 and minimatch 9/10 paths require supported parent updates. **Minimatch 3.1.5 and 5.1.9 are not affected by this specific GHSA**; do not force-upgrade them for this finding. An override such as `archiver-utils>glob: 14.0.2` or globally forcing `minimatch: 11.0.1` violates some declared contracts and is **not** an approved fix merely because alerts disappear. Upstream archiver-utils lists `5.0.2` as latest; ESLint 9 deliberately retains minimatch 3.
- Site `nuxt@3.21.11` is on an upstream EOL line (Nuxt 3 EOL 2026-07-31), but migrating the entire site to Nuxt 4 in this narrow dependency PR is not justified without an independent compatibility plan and native browser E2E. This should be prioritized separately rather than silently bundled as a transient fix.
- Risk is **dependency presence confirmed**, not remotely exploitable production runtime confirmed. Determine if any vulnerable code lands in public output or server runtime with a real generated artifact, and whether untrusted user-supplied glob patterns ever reach those build-time APIs; do not assume either outcome.

## Conditions for a green #792

1. Use real dependency metadata and a supported fixed upstream parent release (or a separately reviewed patch with equivalent semantics) before changing package resolution.
2. Regenerate lock through the **actual pinned pnpm 12.3.4** tool, rather than hand-editing YAML, and record `pnpm why glob minimatch` plus `pnpm audit --json`. Do not call a vulnerable major “fixed” simply by removing the alert.
3. Run frozen-lock clean install, site build/Pages, real Playwright browser E2E, and tests of Nitro archive outputs and ESLint/config behavior after any major crossing.
4. Compare production asset graph and Dependabot state, close [issue #692](https://github.com/777genius/universal-agent-plugins/issues/692) only when real advisories are resolved or safe mitigation evidence is accepted.
5. Preserve Draft while upstream supported fixes and build/runtime exposure have not been demonstrated. No fake PASS or disabled advisories.

Current status: exact parent graph identified; no compatible fixed dependency resolution is yet proven or deployed.

References: upstream [archiver issue #822](https://github.com/archiverjs/node-archiver/issues/822), [ESLint 9.39.4 release](https://eslint.org/blog/2026/03/eslint-v9.39.4-released/), [Nuxt EOL notice](https://github.com/nuxt/nuxt/releases), and the two GHSA advisory records.

## Actual deployment route (checked)

The checked-in `.github/workflows/landing.yml` has the landing runner execute `pnpm run generate`, then copies **`landing/.output/public/`** into the GitHub Pages base `.e2e-pages/universal-agent-plugins/`; combined-pages validation assembles `.pages-dist/` and checks its final static paths. This is a statically generated Pages deployment, **not** a deployed Nuxt Node SSR process. The presence of `archiver-utils` under Nuxt/Nitro and of ESLint under development tooling is therefore a **CI/build/development supply-chain exposure** first, not proof of a network-reachable vulnerable glob parser on the deployed site. A dedicated artifact scan is still needed to prove that none of these library bytes enter public JS/assets.

Treat `serve@14.2.6` as an optional direct dev dependency until repository-wide references and the generated Preview/Pages scripts are checked; its `serve-handler` is not a site runtime route by virtue of being locked. Removing it safely would require **pnpm-generated** lockfile changes plus the existing browser preview E2E, not manual deletion of snapshots.

This distinction lowers the immediacy of unproven public runtime exploitation but does **not** close advisories, upgrade dependencies, or make the issue green.

## Nitro 2.13.6 supported dependency path (reverified 2026-11-08)

**New upstream result supersedes the earlier “no compatible parent route identified” assessment for the Nitro → archiver chain only.** This is a verified *upgrade candidate*, not a merged, installed, audited fix:

- Exact GitHub `nuxt/nuxt@v3.21.11` manifest `packages/nitro-server/package.json` declares `nitropack: ^2.13.4`. The locked graph presently resolves `@nuxt/nitro-server@3.21.11 → nitropack@2.13.4 → archiver@7.0.1 → archiver-utils@5.0.2 → glob@10.5.0 → minimatch@9.0.9`.
- Authentic `nitrojs/nitro@v2.13.6` `package.json` (tagged manifest Git blob `6ca57c098d8a6b1b89acd29c6905ff9b48e10f4a`) publishes `nitropack: 2.13.6` with `archiver: ^8.0.0`. Upstream [Nitro PR #4936](https://github.com/nitrojs/nitro/pull/4936) merged **2026-10-29**; [Nitro v2.13.6](https://github.com/nitrojs/nitro/releases/tag/v2.13.6) was published **2026-11-02**. That nitropack patch is **inside Nuxt 3.21.11's declared `^2.13.4` range**; a forced unsupported archiver override is *not* necessary to investigate this path.
- Exact `archiverjs/node-archiver@8.0.0` manifest does **not** depend on `archiver-utils`. Because it is ESM-only, qualify Nitro's supported adapter and archive behavior through real `pnpm` resolution and Nuxt build (do not swap the archiver version manually).
- **Other independent affected paths remain in this exact lock:** `@vercel/nft@1.11.0 → glob@13.0.6 → minimatch@10.2.6`; `@typescript-eslint/typescript-estree@8.70.0 → minimatch@10.2.6`; `eslint-plugin-import-x@4.17.1 → minimatch@10.2.6`. These were checked directly against the locked `snapshots` entries (lines 7760, 7651, 8707, 9081). Upgrading Nitro alone **cannot** close the glob 13 / minimatch 10 advisory or certify the complete landing graph.

**Next bounded implementation:** use the existing pinned pnpm runtime to resolve `nitropack@2.13.6` *within Nuxt's supported range* via the actual parent graph, regenerate the complete `pnpm-lock.yaml` (no hand edits), check `pnpm why glob minimatch` and `pnpm audit --json`, then run frozen install, lint, generated Pages, browser E2E and archive regression. Track remaining `@vercel/nft` / TypeScript-ESLint / import-x paths separately. The present Draft still contains **no package or lockfile change**, and no vulnerability or deployment gate is qualified as fixed.
