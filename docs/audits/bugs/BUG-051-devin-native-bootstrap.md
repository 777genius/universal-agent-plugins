# BUG-051 — Windsurf/Devin native 30×2 E2E blocked by unverified bootstrap script

## Source evidence

The live remaining-client workflow currently obtains `https://cli.devin.ai/install.sh` and computes an **untrusted observed** checksum. Correct security policy refuses to execute it without a separately reviewed expected checksum, producing `client-unavailable` in both 30-source Windsurf/Devin audit lanes. This is neither an installer product failure nor successful native source lifecycle.

Independent immutable [Homebrew Cask commit c9adad21951a075e35556930e7396a87818f8980](https://github.com/Homebrew/homebrew-cask/blob/c9adad21951a075e35556930e7396a87818f8980/Casks/d/devin-cli.rb) publishes the exact genuine Devin CLI **3000.11.234 Linux x86_64** official archive:
- URL: `https://static.devin.ai/cli/3000.11.234/devin-3000.11.234-x86_64-unknown-linux.tar.gz`
- SHA-256: `201370cddd2924294e3ca6b46fd8ece49b5763953b644a1dd823fce5490cb41a`
- Binary: `bin/devin`.

Independent publisher [CognitionAI/devin-cli](https://github.com/CognitionAI/devin-cli) confirms the artifact is the official Devin CLI; its release automation's [package builder](https://github.com/CognitionAI/devin-cli/blob/main/scripts/publish_npm.py) itself verifies the same upstream `static.devin.ai` manifest archive digests before extraction, rather than relying on the mutable install script. The existing Windsurf target is already wired to the true `devin` executable and isolated `.devin` root. A real `--version` still establishes only executable presence, never subscription/authentication, native activation or model execution.

## Scoped fix, no special exceptions

1. Substitute the independently pinned release tarball for the unreviewed shell installer in the **Windsurf** 30-Plugin and 30-Skill source lanes. Never run `install.sh`, `curl | bash`, arbitrary scripts or `sudo`.
2. Keep the existing literal expected SHA-256 check before any unpack/execution and existing self-reported digest only as negative/positive download evidence.
3. **Reuse** the Cursor project-Skill Python `tarfile.extractall(filter="data")` path; no second unreviewed tar extractor. Select exact source-relative `bin/devin` regular executable, ensure executable bit and no symlink, expose it only in disposable runner `HOME/.local/bin`. Version probe still runs with `env -i` and disposable HOME/XDG/TMPDIR to avoid CI credentials/profile mutation.
4. After real runner bootstrap, run all 30 unique signed/GitHub Plugins and 30 upstream Skills through exact source, lock, repeat-add, native attribution, removal, and foreign sentinel oracles. Store original ZIP. Fail closed on auth needs, CLI incompatibility, publisher mismatch, native activation, process containment or missing provenance. Do not mark model login or manual setup as PASS.

## Two independent critical perspectives

**Supply-chain/security:** Immutable upstream tarball must match Homebrew's independently pinned digest. Safe extraction and exact regular executable path are required. A native version probe must not see GitHub tokens, real user profiles or credentials. Missing digest or path **fails**, not an exception.

**Correctness/compatibility:** Although the product uses Windsurf adapter naming, `devin` is the native client surface. A successful `devin --version` cannot certify 30 source installs; a subscription/auth or incompatible adapter cannot be papered over. Keep reported non-PASS states and original per-case outputs, especially for unrelated source/target incompatibilities.

## Qualification

This commit is a security-aware bootstrap enabling **genuine attempts**, not proof of successful installs. Current source HEAD needs original 30×2 ZIP evidence, focused/Required CI and independent final-head review before any squash merge.
