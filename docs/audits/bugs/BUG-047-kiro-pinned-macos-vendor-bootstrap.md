# BUG-047 — Kiro native E2E executable provenance on macOS

## Reproduced gate and independent source

PR #698, source HEAD `442b1d9030addd21bee501f9a0bf087394a7ffea`: 60 Kiro Plugin/Skill rows are `client-unavailable` because the existing audit downloads `https://cli.kiro.dev/install` but correctly refuses to run a mutable unreviewed script. This is a **supply-chain safety gate, not an installer failure**.

The maintained [Homebrew Cask kiro-cli at immutable commit 7191fc89](https://github.com/Homebrew/homebrew-cask/blob/7191fc89db2794ad19444a2e7bf3de6c4c63c10e/Casks/k/kiro-cli.rb) independently records:

- Publisher-hosted archive: `https://desktop-release.q.us-east-1.amazonaws.com/3.0.0/kirocli-universal-darwin.zip`.
- Version `3.0.0`, SHA-256 `0a8f067c4fe6a23a231aa3a18e3e4332120ef9ed3d2d7b6eea907beb8b4cdd8b`.
- Binary path `kirocli/bin/kiro-cli`, supported on macOS.

The [Kiro 3.0.0 official migration guide](https://kiro.dev/docs/getting-started/installation/) confirms the current CLI is a standalone `kiro-cli` binary and has compatibility changes from 2.x. The archive's checksum must match at runtime before any extracted native code runs. Homebrew's release pin is an independent publication source, **not proof that UAP's Kiro adapter is functionally compatible**.

## Minimal change, single lane

Only `matrix.target=kiro` in `.github/workflows/live-install-remaining-clients.yml`: replace the mutable install script with the immutable archive and recorded reviewed digest; perform the existing digest verification **before** extracting unprivileged into the disposable runner directory; require the exact ordinary executable path and expose it via the existing lower-priority `$HOME/.local/bin` lookup. Never invoke the installer script or `sudo`. Other vendor lanes remain fail-closed.

Focused Python contract regression asserts pinned source/digest, verify-before-extract order, no script fallback, no symlink executable and safe PATH ordering. Run all existing security/isolation regressions, Required, and **actual 30 Kiro Plugin + 30 Kiro Skill source-case JSON ZIPs** on one final SHA. A successful checksum or `--version` is *not* a native install PASS.

## Critical review — two independent failure perspectives, not external approval

- **Supply-chain/containment:** A hash computed after download is not a trust anchor; use the separately published Homebrew Cask pin and verify exact bytes **before** extracting or invoking the binary. Avoid root-level installation and preserve the existing run/source-proof blocker on hash mismatch.
- **Native compatibility/false positives:** Kiro 3.0.0 may differ from older native registry semantics; treat real add/list/repeat/remove failures as failures or unsupported evidence, never relabel as PASS. Preserve Kiro's macOS runner instead of bypassing its cgroup containment on Linux.

## State

Change is scoped to this PR's audit CI and security regression, not product adapters, secrets, runtime auth, signed Registry policies or frozen VS Code behavior. **Final-head CI and real native proof are pending.** No security-review clearance or merge follows merely from this plan.
