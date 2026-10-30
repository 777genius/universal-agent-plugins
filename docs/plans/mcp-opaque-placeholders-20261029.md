# MCP opaque-placeholder correction

Date: 2026-10-29. Base: `60967b8b6ca79245a61caf28895aa09c2c9d74a2`.

## Scope

Fix the reproduced Agent Plugins 1.0 section 9.2 violation: standard stdio
`args` and `env` values must expand only `${PLUGIN_ROOT}` and `${PLUGIN_DATA}`.
`${package.root}`, `${UNKNOWN}`, shell-like text and path-like argument values
remain literal. This is a conformance correction, not a resolver or runtime redesign.

Preserve the existing separately handled command compatibility path, reserved
environment-key rejection, path containment, staging/active-root distinction and
single-pass expansion. Do not delete legacy authoring/conversion capabilities,
change VS Code implementation or fixtures, add a fallback loader, or publish a release.

## Implementation and acceptance

1. Remove only the extra `${package.root}` argument/environment replacement from
   `clients/shared/stdio.go`; clarify the distinction from the command path.
2. Update the existing compatibility regression so command behavior remains
   checked while its argument expectation follows the standard.
3. Add meaningful regression coverage for typed and JSON-decoded inputs, known
   versus unknown tokens, single-pass replacement, and reserved environment keys.
4. Exercise actual standard loading plus the shared native MCP projectors,
   inspect emitted JSON and verify the source envelope is not mutated. Keep
   source, staging and future active roots distinct. No native vendor process
   or model account is needed to prove this transformation.
5. Correct the stale Skills-parser statement in the conformance profile after
   checking current source. Project-install availability is already corrected
   in main by the existing documentation work; do not duplicate that rewrite.
6. Read back the committed diff and obtain exact-head repository CI and review
   before considering merge. A test-only isolated reproduction is not a full
   repository or native-client E2E pass.

## Related research, not production scope

Package-manager policy, MCP deduplication and Skill runtime decisions are recorded
separately. No change to lowest-compatible fresh resolution, lock schema, package
identity, single-version policy, resource ownership or service startup semantics
is authorized by this fix. In particular, identical MCP names/URLs must not cause
implicit resource sharing.

## Evidence discipline

The prior interrupted session reported local fixes and experiments, but its
checkout and complete test logs were not available in this continuation. Main
still contains the offending replacement, so earlier local success is not reused
as evidence for this branch. The current environment has Go 1.23.2, no gh/Codex CLI
and no direct GitHub network access; publication uses the connected GitHub API.
Independent GPT-6.1 Sol/xhigh workers are not available in this continuation.
Self-review, executable checks and external PR reviews must be labelled separately.
