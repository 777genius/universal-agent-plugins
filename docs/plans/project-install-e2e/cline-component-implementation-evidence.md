# Cline project components: implementation checkpoint

2026-10-24. The exact post-observation plan at remote `ae07f5e7` obtained both
independent R2 GOs, recorded before product code at `0a34182a`. This implements
the project-only component lane and the one coordinated shared correction.
No native collector code, vendor execution or runtime qualification is added.

## Source boundary

Project selection includes either project scope or a nonempty project root;
then both scope fields, original profile, exact managed package/namespace and
native component paths are validated. The new project kinds bind `.cline/skills`
and the fixed `.uap/clients/cline/mcp.json`, bypassing ambient user path helpers.
Original-profile overlap and aliases are refused. Staging must be the exact
ordinary contained child of its selected target; final delivery must be populated
and match all selected components. Existing user lifecycle and projections remain.

The adapter reuses Cline's existing neutral server/data contract and kernel codec,
shared ProjectComponents receipts and combined ProjectComponentMutation. It
preserves unrelated JSON members and returns Prepared after actual local file
verification. Early and direct composed preflight reject activation attestation.
The sole shared lifecycle edit forwards input.ActivationComplete through the
existing early preflight request. Recorded removal requires exact project
identity, never a client executable or global uninstall. Suggested session
actions use JSON-quoted structured values, fix cwd/arguments/environment and
explain that the explicit file replaces the whole MCP source for that process.

## Actual verification

- Existing Cline/domain targeted contracts initially PASS, 0.017/0.005 seconds.
- New real loader/planner/stager/kernel provider Cline contracts initially PASS,
  0.087 seconds. They cover ambient path poisoning, exact file ownership,
  unrelated disabled remote members, verify-only, repeated remove, malformed
  scope/client/profile/artifact/delivery, foreign/tampered native skill refusal
  and stdio data expansion. Independent skill/MCP selections were added later.
- Focused current Cline/attestation/preflight selection PASS: Cline 0.012s,
  providers 4.744s, public installer 38.281s, usecase 0.346s. Domain had no matching
  tests in this selection; it is not counted as a new domain test pass.
- Public installer first-add/replay/update/repair/reopen attestation refusal
  independently PASS0.857s before the combined selection: exact saved state
  unchanged, zero native calls, stable unchanged binding and host-free Inspect.
- Actual saved public CLI TLS Git user+A+B journey PASS22.777s. It covers locked
  sync, denied replay with exact state/native bytes unchanged, moving-source
  pinned managed-package repair, missing native-skill repair, selected update,
  host-free status, exact owned removal and user/B preservation. Its runner
  refuses every client/MCP command; this is file/source-contract evidence only.
- A first CLI fixture incorrectly expected repair to overwrite an externally
  edited native skill. Shared ownership correctly refused; the test now exercises
  damaged managed-package and missing-component repair, retaining a separate
  tamper-refusal regression. One subsequent retry timed out at initial loopback
  Git ref resolution before product work; the final unchanged transport/assertion
  run passed. Neither failure is counted as native evidence.
- Go formatting and diff checks pass. Affected Go vet and Windows compile-only
  checks are pending at this source freeze. Full relevant golangci-lint is not
  installed locally; standard vet does not replace either repository lint gate.

## Known generic observer composition gap

A separate actual disposable SDK diagnostic with its observer flag left at the
public default false produced incomplete/Mutated=true and persisted state size
0→4080 bytes on a foreign native skill collision. The foreign bytes remained
unchanged and native calls were zero. The SDK composes the richer observer only
when EnableNativeObserver && Runner!=nil; the fallback checks the managed package,
and shared component requireAbsent detects the foreign target after its delivery.
The raw diagnostic source/result is retained in the task runtime and reported to
root for a separately reviewed composition fix. It is not committed as a passing
acceptance test or represented as a pre-delivery collision pass. The actual SDK/
CLI acceptance fixtures explicitly compose observation with a runner that refuses
every command. This gap remains pending; do not weaken collision assertions or
silently broaden shared observation in this client-owned checkpoint.

Two independent frozen code GOs, relevant lint gates and the newly reviewed
current-source native evidence lane remain outstanding. Historical Cline source
attribution and static read-only dispatch proof do not close those product gates.
