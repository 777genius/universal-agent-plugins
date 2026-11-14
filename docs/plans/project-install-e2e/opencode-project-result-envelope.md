# OpenCode project native assertions: preserve both public result envelopes

At `1a19373e024ba4c2ac0748fda1306ce0ffd95429`, native run `40191320869`,
macOS job `121162735576`, the project test fails on repeat add even though the
raw result now correctly contains `no_change:true`, `mutated:false`, Prepared
and installation verified. The old user helper insists on batch `data.status`,
which the public single `data.result` envelope does not contain.

Artifact `12597788376` is 648,648 bytes, SHA-256
`375dfd61790eac7b237c9fc2c9556d51cddcbd786a05cc6aed010b035da21f6a`.
Closed runner membership/digests and exact source/tree were independently
verified. The other four required native tests pass with no skips; project
positive A/B discovery passed before replay, with passive recorder counts zero.
Interrupted later stages are not qualified.

## Exact bounded correction

Only the project fixture's assertion reader and narrow inert contracts change.
Keep the original user `openCodeAssertCompleted`, every native command, stage,
state/file/runtime assertion, capture, limit and original raw JSON unchanged.

Before downstream project assertions, validate one of these actual shapes:

- Batch: retain the real `data.status == completed`, exactly one OpenCode
  target and its actual result. Reject malformed or non-single targets.
- Single unchanged project result: require actual top-level success,
  `no_change:true` and `mutated:false` booleans, OpenCode client, project scope,
  Prepared and installation verified, with no attestation. Do not invent any
  status, replay flag, activation or verification field.

Both shapes retain exact project client/scope/Prepared identity checks. Refuse
mixed `targets`/`result` envelopes and explicit failure or contradictory status.
Only the single assertion view is wrapped with the existing assertion-envelope
helper so current plan/mutation checks can read its actual result. Never rewrite
the raw captured map or evidence, or route user operations through this adapter.

Two independent plan rounds precede implementation, then frozen code review.
Inert contracts admit the actual batch/single cases, preserve raw input, and
refuse mixed, malformed/non-single, failed/contradictory status, wrong
client/scope, missing or ill-typed replay flags and contradictory lifecycle.
Run affected pure contracts/compile and configured changed-source lint. No local
vendor execution; exact final-head native replay remains required.
