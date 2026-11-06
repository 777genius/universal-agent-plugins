# BUG-056 — pinned Grok Build Linux x64 native executable for full source E2E

## Reproduced gap

Existing current source `c904bbd77b701b5cceedec02118c9144b235fa9f` downloads `https://x.ai/cli/install.sh` into `vendor-install.sh` without an independently reviewed installer digest. It correctly refuses to execute it. Earlier 30-Plugin / 30-Skill original Grok ZIPs consequently had `cases_attempted=0` and 60 explicit `client-unavailable` source rows. This is **not a successful installation**, and executing a mutable shell installer is not an option.

## Independent publisher provenance

The immutable, GitHub-verified [Homebrew Cask commit `b75a8e94456970fa349c75b59ca5e4183dde8e28`](https://github.com/Homebrew/homebrew-cask/blob/b75a8e94456970fa349c75b59ca5e4183dde8e28/Casks/g/grok-build.rb) records the official **Grok Build 1.0.70** Linux x86_64 executable URL `https://x.ai/cli/grok-1.0.70-linux-x86_64`, SHA-256 `22d281c5964c5444147328096ba2a221f3f0874c6858e27c825656a80f6e58e9`, and binary name `grok`. This is the product in `clients/grok/detect.go` (native Grok Build), not an unrelated Grok chatbot or ngrok network tool.

## Scoped fix

1. In the **existing Grok-only** Remaining-Clients workflow lane, replace the unsigned installer with that exact official version and digest. Preserve `curl -fsSL` followed by `sha256_file -c -` as a mandatory execution precondition.
2. After checksum success, use `install -m 0755` into the runner-owned vendor-root and an exact disposable-home `grok` link. Require a regular non-symlink executable. Never execute downloaded shell scripts, `sudo`, or run an installer in the runner's real application profile.
3. Retain `env -i` for native `--version`, then the existing 30×2 source/lock/repeat/native/sentinel/removal oracles. An authentic version and preflight alone are not lifecycle PASS.
4. Add one focused no-network regression for immutable URL/digest, hash-before-exec, executable binding, environment isolation, and absence of `curl | sh`/installer-script execution. Verify current-HEAD original two Grok ZIPs and applicable CI before any merge.

## Two critical reviews

**Supply-chain and privacy:** the independent signed Homebrew Cask binds the exact vendor URL to an exact digest. Any source drift/mismatch must fail before execution, retaining a truthful `client-unavailable` record, without inheriting CI tokens or real profiles. Reject unsupported future vendor formats rather than falling through to shell execution.

**Correctness:** this binary is **Grok Build** and matches UAP's existing native adapter ID. The 60 historical `client-unavailable` records cannot be promoted; only observed real source cases on a **new source SHA** can qualify PASS. Auth, native activation and unsupported distribution cases remain non-PASS. Do not change product adapter or test fixture identities solely to obtain green totals.

## Status

Independent immutable source research and scoped workflow/test patch are prepared, **not yet merged or qualified by final-head CI**. Historical artifacts from `c904bbd77b701b5cceedec02118c9144b235fa9f` remain separate evidence.
