# BUG-062 — Pi manifest discovery refusal is not a generic installer failure

## Reproduction
Pi Plugins original ZIP #13431453346 on Pi 1.5.0 had 8 lifecycle PASS,
21 unsupported source/target combinations, and one nonzero refusal for the
pinned elements-of-style source: "pi package manifest overrides selected
extension discovery". No partial registration or foreign-file mutation.

## Cause and narrow plan
The production Pi adapter rejectPiManifest refuses a package.json containing
a pi field because it can override native extension discovery. The shared
live E2E harness wrongly calls this known source-layout policy refusal generic
install-failed. The exact diagnostic should be source/target unsupported,
never a lifecycle PASS.

Correctness review: recognize the single known diagnostic only after nonzero
CLI exit, preserve original source identity, negative registration, and
foreign-sentinel proof; assert arbitrary Pi errors remain unqualified.
Security review: do not modify the production Pi adapter, source trust,
manifest refusal, native process custody, or LintAI. Do not broaden to other
package.json errors. Add a no-network Python regression.

## Qualification
Re-run original full Pi Plugins 30-source ZIP on the final new HEAD and
Required/Core. Verify exact source, version, lock, repeat, deletion and
foreign-file evidence. Keep every source-policy refusal non-PASS.

See PR #698 issue comment #6662913214.
