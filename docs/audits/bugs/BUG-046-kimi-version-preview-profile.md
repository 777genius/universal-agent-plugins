# BUG-046 — Kimi version preview must not initialize the selected profile

## Verified source risk

At PR #698 HEAD `5e16be5`, `PrepareSkills` calls targeted project version detection before CLI or picker confirmation. Kimi 2.3.2's upstream `--version` performs worker/cache initialization before processing the version argument. The old `VersionProbeEnvironment` passed the selected user's `KIMI_CODE_HOME` and `KIMI_CODE_CACHE_DIR`. A clean exit was not proof of a read-only probe.

## Minimal scope and implementation

1. Reuse the existing targeted version detector and synchronous, timeout-bounded version process. Add one opt-in client capability for a **disposable version profile**; only Kimi opts in. Do not add a second detector, feature flag, or client-ID branch to generic logic.
2. Before creating the scratch profile, fail closed if the selected profile contains the OS temp directory. Canonicalize the private profile, bind Kimi's existing supported environment variables to it, and delete it after the process has exited, including failures.
3. Keep the *original* detected executable, config root, version identity comparison, and the explicit Kimi 2.3.1/2.3.2 admission gate. Do not infer activation or a successful native install from a version response.
4. Run focused Go + Python regressions, Required, native E2E and final-head raw 30-source Kimi Skills. No old artifact counts as current-head evidence.

## Critical review, two perspectives

- **Security/ownership:** running `--version` is not guaranteed read-only. A disposable private cache and an environment containing no ambient API keys remove the known native-worker user-profile write path; the test injects actual cache writes on both success and failure and verifies deletion and unchanged user canaries. This is not a sandbox against a malicious same-UID executable.
- **Compatibility/false-positive:** preserve the selected executable and original profile identity in returned detection; never invent a version or relax the current Kimi constraint. A failed probe must retain an empty version and be rejected by the existing project precondition, rather than pretending the install succeeded.

## Status

Scoped source and regression fix committed in PR #698; independent final-head CI/review and 30-source live Kimi qualification remain pending. The initial Python harness-root regression was corrected separately on `c6f28f0`.
