# Published CLI cross-platform acceptance — 2026-11-09

## Scope
Verify the exact public immutable `universal-agent-plugins@0.1.102` without model logins or vendor tool calls. Preserve existing release SHA/signature, source pins, path/foreign-file ownership and fail-closed refusals. No CLI code, release tag, registry feed or npm publication changes.

## Two complementary lanes
1. **Six OS/arch public cold bootstrap**: pinned release source test harness with a fresh npm tarball fetched from the public npm registry and real, unauthenticated GitHub release asset download. Require exact embedded SHA-256/size, cold and warm cache, public search, read-only doctor and dry-run, plus synthetic isolated add/info/update/remove lifecycle. Unlike producer draft proofs, this does not inject the local frozen binary. The synthetic Cursor presence proves installer behavior, **not native editor activation**.
2. **Full published 30-source matrix on Windows x64 and macOS ARM64**: for a trusted same-repository PR explicitly titled `[full-published-e2e]`, run all nine supported client targets (Pi Skill only), plugin and Skill where supported, over all 30 frozen sources. Preserve the ordinary PR behavior of two sources and the existing weekly/manual workloads; no fork can opt into the expanded budget. Pin the requested release to 0.1.102 for this one acceptance PR. Genuine installed native client CLIs and the existing strict lifecycle auditor are reused.

## Required evidence
- Original workflow status on the exact PR merge tree, and original zipped JSON/TAP per OS/target; no success inferred from queued/skipped work.
- Negative security controls across six architectures: corrupt cache re-verification, immutable retry, hostile symlinks/non-regular cache, lock ownership and cancellation (where platform supports the action).
- Source scope and native status must remain honest: `passed` lifecycle vs `prepared-lifecycle-passed` vs denied/auth-required; no claim of model invocation or authenticated MCP `tools/call`.
- Preserve existing #791/#794 merged P0 and published Remotion long-TEMP positive/negative receipts. Avoid unnecessary reruns or duplicated manual dispatch.
- Merge only after applicable exact-head review and CI; inspect post-merge main checks and any real failures. Do not publish a new release from this test-only change.
## First-run findings and strict remediation

- **OpenCode / plugins:** Four previously frozen Git source revisions can
  complete package add/list/repeat/remove without proving native component
  registration on public 0.1.102. Treat them only as verified *non-native
  negatives* with exact source and version checks and unchanged ownership.
  Unknown sources, new CLI versions or native residuals continue to fail.
- **macOS Docker Hub:** The pinned source requires the real `docker` CLI
  on PATH. The existing Homebrew CLI prerequisite is enabled only for the
  explicitly trusted 30-source PR run; never substitute a fake binary or
  disable runtime preflight.
- **Windows cache locks:** An existing frozen source unit test emulates Linux
  and checks POSIX mode 0700, which NTFS does not report. Keep that test on
  Linux/macOS; on genuine Windows x64/ARM64 instead prove two independent
  LOCALAPPDATA lock roots can simultaneously own the same target, releasing
  one never removes the other's lock, and foreign sentinels survive.
- **Public-vs-draft proof:** The six-OS acceptance mode must always use
  `public_release_download` with no local frozen binary. It does not claim
  authenticated native vendor tool calls or access to a real model session.

