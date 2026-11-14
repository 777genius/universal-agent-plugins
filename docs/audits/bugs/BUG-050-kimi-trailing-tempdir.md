# BUG-050 — trailing TMPDIR separator rejects safe Kimi version probe

## Reproduced failure and evidence

On PR #698 HEAD `0d7ef514717a9a40068f3de1a667c9b0d41a8706`, CodeRabbit's 2026-11-05 review identified a functional rejection in `versionProbeEnvironment`. Unix Go returns a configured `TMPDIR` unchanged, including a trailing slash; `clients.CanonicalDirectory` deliberately refuses non-clean spellings. Thus a safe absolute `TMPDIR=/tmp/` fails before an otherwise permitted Kimi `--version` probe. A standalone Go `os.TempDir` check reproduced this exact trailing-separator behavior.

## Minimal plan

1. Clean only the **ambient temporary-directory input** before existing canonical/symlink and overlap checks. Do **not** normalize the selected `originalRoot`; its strict no-alias check stays unchanged.
2. Add a hermetic positive test with a trailing temp separator that proves the disposable profile is created under the expected canonical scratch directory, cleaned after use, and never writes into the selected profile.
3. Retain existing negative tests rejecting symlink-aliased selected profiles and temporary directories overlapping the selected profile. No new client behavior, feature flags, security exceptions, or native execution permissions.
4. Qualify this patch using targeted Go tests, Required/LintAI, and actual final-head live ZIPs. Until then it is a prepared correction, not final-head PASS.

## Critical review — two self-review perspectives (not external approval)

- **Profile/security boundary:** `CanonicalDirectory` still resolves temp symlinks, detects invalid ancestors and feeds its canonical result to the overlap check. The selected profile is not cleaned or auto-approved, and the original no-alias condition remains unchanged.
- **Compatibility/negative cases:** Go's `os.TempDir()` can preserve a harmless Unix trailing separator; Windows may normalize it itself, so the targeted regression skips only where the platform cannot reproduce that spelling. Other malformed/relative roots continue to fail through the existing canonical guard.

## Status

This scoped fix and regression are not final-head E2E proof. Required, focused Go, full native ZIP matrix, and fresh external review must be qualified after publication. Do not merge while any final-head gate remains unverified.
