# Antigravity intact MCP repair

## Observed defect and boundary

The released-source audit on CLI `0.1.108` and genuine Antigravity CLI `1.5.3`
installed the pinned Notion package successfully in User and Project scope.
Its unchanged update succeeded, but healthy repair failed in both scopes with
`no_supported_components`. Evidence: PR #909, run `43598197245`, job
`132457146329`, artifact `14116032855`, harness commit
`84e3597a2a5691a4c53f646610382ed35d756f09`.

The native adapter projects selected MCP servers into `mcp_config.json` and
removes portable `mcp.json`. It omitted the existing `clients.SelectionReader`
contract, so repair's digest-verified selection reader used the default
`mcp.json`, found no recorded servers, and correctly refused to infer MCP
delivery. The published `0.1.108` binary is immutable; a source fix does not
change this failed release evidence.

## Minimal implementation

- Implement `SelectionReader` in `clients/antigravity/adapter.go` with the
  native nested `mcp_config.json` layout shared by User and Project scope.
- Add a real native-staging round-trip test for both scopes, including refusal
  after artifact bytes drift. Keep the generic pre/post-read digest checks.
- Exercise real installer Prepare/Apply with an MCP-only package and intact
  User and Project installations. Require native bytes, exact source, binding,
  artifact/data ownership and unrelated scope trees to remain unchanged.
- Extend the existing three-platform native harness with intact repair before
  its partial-drift refusal, and with intact User repair. Observe the existing
  native MCP inventory and package bytes after repair.

No generic client-ID branch, new interface, state/schema change, Directory grant,
activation claim, public version pin, or release-audit exception is needed.
The existing missing-bundle recovery and foreign/partially changed ownership
refusals remain mandatory.

## Verification and release handoff

First run the new provider and MCP-only installer tests against the unfixed
adapter and record the expected failures. After the adapter change, run those
tests, affected adapter/provider/installer tests, `make lint` and
`make test-core`. Any host restriction is recorded as blocked, never PASS.

The existing `.github/workflows/antigravity-native-e2e.yml` builds the exact PR
candidate and invokes `scripts/e2e/antigravity-native-cross.py` with genuine,
SHA-pinned `agy 1.5.3` on Linux, macOS and Windows. Linux additionally runs
`scripts/e2e/antigravity-native.sh`. A local candidate may run the same harness
with `AGY_BIN`, `UAP_BIN` and a private `RUNNER_TEMP`; this is source qualification,
not qualification of a released binary. Cross-OS and exact-head results remain
required before merge.

Release preparation/promotion follows `docs/agentplugins-release.md`, including
its frozen tag, current CI/security gates, independently reviewed draft,
same-run platform/native proofs, protected environments and npm publication.
Only a newly qualified public binary and npm facade may replace `0.1.108` in
the separate released-source/UI audits. Do not overwrite a published version,
skip repair, or relabel the observed failed `0.1.108` receipt.

## Change size and risk

| Slice | Production LOC | Regression/harness LOC | Complexity | Expected reliability | Confidence |
| --- | ---: | ---: | ---: | ---: | ---: |
| Existing adapter selection seam | 7 | 201 tests + 27 net native-harness | 3/10 | 9/10 | 9/10 |

Production impact is one adapter method and its compile-time interface check.
Behavioral risk is bounded to reading the native file already written by this
adapter; no additional filesystem authority is granted. Regression tests and
the existing native harness cover both scopes and retain fail-closed ownership.

Validation results are recorded in the PR with exact commands and hosted run
identities after execution; this plan does not assert that they have passed.
