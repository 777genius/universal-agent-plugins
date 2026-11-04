# BUG-032 — Case-local directory descendants were created before parent containment checks

## Evidence and scope

One [remaining unresolved Strix inline thread](https://github.com/777genius/universal-agent-plugins/pull/698) notes that the audit calls `path.mkdir(parents=True, exist_ok=True)` for each case's `home`, `project`, `uap-data`, `home/.config`, `home/.cache`, `home/.local/share` **before** verifying all of their parent directories and the new directory itself. The outer case leaf is created with `exist_ok=False`, so a pre-planted complete leaf fails; nevertheless a malicious concurrent earlier child can still substitute a nested parent under that new leaf between operations. A later nested `mkdir` could otherwise traverse a symlink **before** the current post-creation containment check. This is a hardening gap in an E2E harness, not an observed out-of-profile write or a published product defect.

## Narrow patch

1. Keep the unique `case_root.mkdir(parents=True)` fail-if-exists rule. Resolve the exact disposable case root and, **before every nested mkdir**, reject a symlink in the requested path and reject a parent whose canonical resolved path is outside the case. Check the resulting path's canonical containment again after mkdir. All home/project/data and native-root/sentinel checks remain unchanged.
2. Add a subprocess-free regression that injects a pre-existing symlink at a nested directory **after the unique case root is created** (within the `Path.mkdir` test wrapper) and asserts no new files are written to the external target. Keep macOS `/var` canonicalization semantics.
3. Validate focused Python on Linux/macOS/Windows and current exact-head native CI. Do not change production installer, fake client detection, bypass LintAI, or introduce heavyweight portable dirfd architecture for this bounded CI proof. An adversarial same-UID race against a newly created directory cannot be absolutely excluded by any pre-check; the test must not overstate that guarantee.

## Critique before source change

- **Security critic:** Checking only `path.resolve()` after `mkdir` is too late to prevent writes through an already-redirected parent; guard the parent before each creation, then recheck the created directory. Never follow an external symlink.
- **Compatibility critic:** Preserve ordinary directory creation on macOS `/private/var` and Windows canonical paths, and do not broaden the audit to modify native vendor configs or the library's production path policy. One focused negative test is adequate.

## State

Real code-order gap and current Strix remark confirmed. Minimal fix and current-head full verification pending; the remaining thread stays open until source changes and focused regressions are inspected.
