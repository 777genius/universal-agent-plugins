# BUG-055 — VS Code staged CLI path and Windsurf unselected user channel

## Exact source and original failures

Source HEAD `4c3a7ef543d18f211510231432df22c111d17d7d`, [Remaining E2E run 42001276433](https://github.com/777genius/universal-agent-plugins/actions/runs/42001276433).

- **VS Code** original [Plugin #13402878508](https://github.com/777genius/universal-agent-plugins/actions/runs/42001276433/artifacts/13402878508) and [Skill #13402139421](https://github.com/777genius/universal-agent-plugins/actions/runs/42001276433/artifacts/13402139421) ZIPs: official Microsoft 1.144.0 Linux x64 `.deb` downloaded, `sha256sum -c` passed against the separately reviewed digest, and `dpkg-deb --extract` completed; `command -v code` then failed (the original artifact contains an empty `vendor-proof/executable.txt`). All 60 rows explicitly say `client-unavailable`, `cases_attempted=0`. The unprivileged extracted `.deb` must use its real `usr/share/code/bin/code` launcher, not assume its package-installed `/usr/bin/code` absolute link works within a temp root.
- **Windsurf/Devin user Plugins** original [ZIP #13403171230](https://github.com/777genius/universal-agent-plugins/actions/runs/42001276433/artifacts/13403171230): genuine pinned Devin CLI 3000.11.234, 30 cases attempted. Exactly 29 nonzero refusals report `ClientConfigRoot must be an explicit absolute clean path`; one package is documented source-unsupported. Production `clients/windsurf/detect.go` deliberately does not select a legacy Windsurf user channel from a standalone Devin executable. This is an unselected profile, **not proof of package installation failure**. No vendor profile or source was modified.
- Four real Kiro Plugin failures in [ZIP #13402208006](https://github.com/777genius/universal-agent-plugins/actions/runs/42001276433/artifacts/13402208006) are scanner downloads returning HTTP 504. Keep the security scan fail-closed; this patch does not retry or bypass LintAI.

## Minimal fix and critical review passes

**Correctness:** In the VS Code-only `.deb` branch, explicitly require the exact regular executable `$RUNNER_TEMP/vendor-root/usr/share/code/bin/code` and add a link from the runner's disposable `$HOME/.local/bin/code`. Require the resolved selected CLI to equal that approved file before any `--version` or audit case; a missing launcher or shadowing host executable keeps bootstrap non-PASS. All other vendor branches stay unchanged. In the E2E classifier, treat only the precise nonzero Windsurf/user root-selection refusal as existing `target-unavailable` rather than generic `install-failed`; retain all downstream checks and no PASS on refusal.

**Security:** Existing independently pinned SHA-256 runs **before** any extraction. The staged executable must be a regular non-symlink file without redirected ancestors, and the observed selected CLI must resolve to that exact reviewed artifact; no `sudo`, package maintainer scripts, `--no-sandbox`, vendor install shell or profile mutation. The version probe remains `env -i` with a disposable profile. Windsurf remains non-PASS without an authentic selected user channel. Preserve LintAI 504 and process-containment refusals.

## Targeted regressions / acceptance

- A hermetic no-network test requires the VS Code digest, extraction ordering, exact staged launcher and selection guard.
- An isolated mock CLI with the real Windsurf diagnostic must retain nonzero install, no post-install registration, intact foreign sentinels, no cleanup and `target-unavailable` status.
- Re-run the **actual** 30-Plugin/30-Skill VS Code and Windsurf cases on the final HEAD, inspect original ZIPs and all applicable Required/Core fast/security checks. A verified editor `--version` is a prerequisite, not native discovery, activation, or lifecycle PASS. Until the hosted runs qualify, do not merge.
