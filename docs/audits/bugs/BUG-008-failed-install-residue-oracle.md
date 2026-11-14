# BUG-008 — Failed installs were not checked for partially registered native state

## Real evidence and limitation

The historical 30-source Grok and Claude artifacts from [41727674724](https://github.com/777genius/universal-agent-plugins/actions/runs/41727674724) and [41727674760](https://github.com/777genius/universal-agent-plugins/actions/runs/41727674760) show several correctly refused third-party source trees (symlinks), but some failed attempts still changed native files (Grok 28, Claude 2 per case). Their `native_identity_files` were empty, so these deltas may be ordinary client bootstrap state; **they do not prove partial plugin installation**. However the harness only calls `agentplugins list` after successful add, and cannot distinguish harmless first-run files from a registered partial plugin after a nonzero add. This is a negative-path E2E proof gap, not an established installer defect.

## Scoped plan

1. After a completed nonzero `add`, call the real public `list --target ... --scope ... --format json` inside the same disposable environment. Inspect any matching installation name and target *without requiring an `installation_verified` status*, since a damaged partial entry could be `pending`.
2. If the failed add left an identity-attributed native file, matching Skill file, or managed installation record, emit a distinct `failed-install-residue` status with JSON evidence. Do **not** reinterpret unrelated native bootstrap changes as a product bug and do not run unsafe attested cleanup automatically on failure.
3. Unit-test malformed/unrelated list responses and positive name+target matches; let live CI verify negative sources on the final SHA. Any confirmed residue requires a separate root-cause fix in the adapter/installer.

## Critical plan review

- **Rollback/data-integrity critic:** A nonzero exit is not sufficient negative evidence; failure paths must be inspected for partial registered state, including a pending record rather than only verified installations.
- **False-positive/security critic:** Different native file hashes from the client's first launch are not proof of UAP-owned residue. Use name+target registration and existing identity-attributed native file/Skill oracles; never mark a failed install as success.

**State:** post-failure real list inspection and pending-record regression pushed in PR #698; original-source artifact deltas remain observations, not confirmed product defects. Current-head E2E pending.
