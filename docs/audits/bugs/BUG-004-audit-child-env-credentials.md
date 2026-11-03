# BUG-004 — Live E2E child environment inherits unrelated runner credentials

## Reproduction / evidence (source review, 2026-11-04)

In `scripts/live-install-audit.py`, `isolated_client_env` currently starts from nearly all `os.environ` and removes a fixed list of native-profile and Git configuration overrides. Therefore a synthetic parent variable such as `OPENAI_API_KEY=canary-not-a-real-key`, `GITHUB_TOKEN=canary`, `ACTIONS_RUNTIME_TOKEN=canary` or `AWS_SECRET_ACCESS_KEY=canary` remains in the environment handed to `call([cli, "add", ...], env, ...)`. The profile files are disposable, but credentials are *not* isolated. This is a test-harness exposure, not evidence of actual token exfiltration, and does not imply a vulnerability in the published UAP installer.

## Minimal correction

1. Replace the E2E child's broad environment inheritance with a short explicit allowlist of runtime/OS, locale, Git TLS trust and proxy variables needed for public-source installation on Linux/macOS/Windows. Preserve the case-local HOME, XDG, APPDATA/LOCALAPPDATA, client roots, TMP and security-related overrides already set by the function. Never forward general API keys, CI runtime tokens, GitHub credentials, SSH agent variables, arbitrary `GIT_*`, or `NPM_TOKEN`.
2. Add subprocess-free regression with injected canary credentials, native-profile and Git-config overrides; assert canaries are absent and required PATH/proxy/system values survive. Keep the existing outside-root refusal regression.
3. Run the targeted Python tests on every CI OS before live acquisitions. Verify live installs have not been broken by removing ambient credentials. If a public source genuinely needs authenticated access, record that as an external blocker rather than restoring token inheritance.

## Plan critique / scope

- **Security critic:** A blacklist can never enumerate all future secret names. An allowlist addresses the trust boundary; avoid logging even canary values in artifacts.
- **Compatibility critic:** Preserve only the OS/runtime plumbing explicitly needed by native CLIs and public network fetches; test macOS canonical paths and Windows. Do not sanitize inside the product, change CLI auth behavior or weaken the security scanner.
- **Evidence limitation:** This is a reproducible code-path exposure, not an observed exfiltration. No claim of safe credentials can be made about pre-fix runs unless the runner environment is independently known.

**State:** confirmed by source inspection; minimal fix and final-head CI qualification pending.
