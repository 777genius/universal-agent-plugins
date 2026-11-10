# BUG-028 — Codex native uninstall PATH re-resolution

## Evidence and reproduction
Strix independently identified a **harness attestation** risk on PR #698 (2026-11-04, HEAD `c7fbfecf8db6`). The same source audit probes a genuine `codex` binary but later calls `["codex", ...]` again for real native uninstall. Re-resolving PATH can invoke an untrusted executable instead of the original client and forge removal evidence.

## Minimal plan
- Resolve advertised native executables to canonical absolute paths **once on harness import, before untrusted installs**.
- Use that pinned path for Codex native plugin/marketplace removal and the client version probe; refuse attestation if it is absent or no longer a file.
- Reuse existing isolated environment and Codex-native content checks. Keep runner toolchain accessible; do not execute paid models or alter login/installation policies.
- Regression: both real native remove subprocess arguments must be the originally pinned absolute path. Missing pinned binary must fail closed without execution.

## Critic round 1 — correctness
A successful CLI exit from a shadow binary cannot authorize `--external-uninstalled`. The exact path must be bound before packages run; a repeat `shutil.which` later would reintroduce the issue.

## Critic round 2 — security and bounded scope
Canonicalize at discovery, but do not claim a shell PATH alone supplies code-signing or immutable executable provenance. This correction only closes the observed re-resolution path. Retain native post-remove evidence and re-run CI plus independent reviews on the final HEAD.

## Status
Implemented with a path-pin regression in the PR branch; current-head CI and independent Strix review must be rechecked. No merge authorization.
