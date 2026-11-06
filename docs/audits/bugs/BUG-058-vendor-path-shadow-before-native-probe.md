# BUG-058 — reviewed vendor archive is not the selected executable

## Reproduced root cause
In `.github/workflows/live-install-remaining-clients.yml` the runner's system toolchain precedes `$HOME/.local/bin` in PATH by design. The checksum verifies the downloaded Grok/Cursor Agent/Kiro/Devin/VS Code artifact but, for targets other than VS Code, a preexisting same-name native binary could win `command -v` and the subsequent `--version` and live audit. A hermetic PATH-shadow demonstration selects an unreviewed executable while the verified one exists. This would invalidate vendor provenance, not justify a lifecycle PASS.

## Narrow correction
Keep system utilities first. After the verified artifact is installed, before any native probe, require `shutil.which(BIN)` to resolve to the exact staged executable on both Linux and macOS. An unexpected runner binary fails the vendor-install step and the audit records 30 **client-unavailable** placeholders with preserved source provenance; no vendor script is launched. Do not change signed digests, ownership, profile selection, LintAI or process containment.

## Regression and two critical passes
- Security: hermetic shadow executable ahead of a verified path must fail without executing either fake binary; verified first must succeed.
- Correctness: do not prepend vendor directories ahead of system toolchain or reclassify failed verification as PASS. Keep original 30-source refusal ZIP evidence.
- CI: focused Python regression, Required/Core, remaining full 12 lanes and native full 18 lanes must be requalified on the pushed SHA.
