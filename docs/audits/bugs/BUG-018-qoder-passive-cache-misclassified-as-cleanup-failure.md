# BUG-018 — Qoder passive cache mistaken for active uninstall residue

## Reproduction from actual GitHub Actions ZIP

Run [41766817078](https://github.com/777genius/universal-agent-plugins/actions/runs/41766817078), artifact [13294421447](https://github.com/777genius/universal-agent-plugins/actions/runs/41766817078/artifacts/13294421447), on PR head `5e79cf1a2d34a900f7fb5a542b39d134c8b29057` (not the current HEAD). The Qoder plugin job was **canceled during a later push** after 11/30 cases: 11 show `cleanup-failed`, despite real add / exact list / repeat-add / remove exit 0 / post-remove empty UAP list / preserved foreign sentinel. All 11 reports have `native_residuals` limited to two locations:

- `.qoder/plugins/cache/...`: downloaded local package cache, not Qoder's active `installed_plugins_v2.json` registry.
- `.qoder/logs/runs/.../manifest.json`: historical execution logs, not active plugin bindings.

There are **zero** residual entries outside these two prefixes in the inspected 11 records. This is a **harness false-negative candidate**, not permission to call any old failed run PASS or to ignore active Qoder settings. The old job's cancellation itself explains the missing 19 results; it is not a CLI failure. Actual Qoder CLI version `1.1.81` and UAP CLI `0.1.95` are recorded in JSON.

## Minimal plan

1. In `native_identity_residuals`, ignore *only* regular files inside Qoder's two explicit passive prefixes when determining active uninstall residue. Keep symlink, unreadable or nonregular files fail-closed. Apply the same no-symlink-before-cache rule to the preexisting Codex cache exception.
2. Keep `installed_plugins_v2.json`, `settings.json`, and other Qoder native configuration under normal residual scanning; an identity left there must remain `cleanup-failed`.
3. Record retained passive cache/log paths separately in the per-case JSON for transparent disk-state evidence. Add focused regressions with fake disposable profile paths and no real agent calls.
4. Re-run required Python tests and the full live audit on **new** PR HEAD. No cancellation or historical artifacts qualify final-head E2E.

## Critical review round 1 — safety

Do not blanket-ignore `.qoder/`, ignore any client-recognized active registry, or follow symlinks in the cache. If a native package is still registered or an unreadable/symlink object remains, refuse PASS.

## Critical review round 2 — evidence semantics

The intended claim is *active native removal*, not *deletion of every downloaded package or log*. Retained passive disk bytes are explicitly reported. Other clients' behavior is unchanged; unsupported, auth/containment and other client defects remain non-PASS.

## Status

Independent source ZIP reviewed, narrowly-scoped change and regression **planned**, not final-head qualified.
