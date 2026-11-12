# Kiro installation E2E without authentication

Baseline: main 446872e2b98c0e337da37969f0fd6fbc26772972 and separately reviewed
Kiro collector hardening from PR686 head 8efcc9e5869ef8c2ba23becdab972a2043d4f5a2.
The owner explicitly requests credential-free completion. No account, API key,
login, chat/model request, ACP session, trust grant or production adapter change.

## Finite acceptance

Run existing TestProjectKiroComposedSourceLifecycle for real loopback TLS Git
and ZIP acquisition, add A/B, locked repeat, read-only commands, repair, update,
cold import and removal with User/B/foreign preservation. Run existing
TestProjectKiroPublicSDKStdioDataRepairRestartAndRemove for prepared stdio
projection, reconstructed SDK after restart, pinned repair, retained scoped
data and remove. These are actual UAP composition/filesystem lifecycles with
explicit inert discovery/security/helper seams, not native Kiro loading.

In a separate Linux hosted job, reuse project_native_research acquisition,
Context, bounded subprocess containment, source provenance and runtime pin.
Reuse Kiro companion identity and root/fixture guards. Acquire only official
2.34.0 archive SHA256 11dce9286cf7401079cf4eac02a3d5982daac16a346d880109ec7b4a0b941040.
The child environment is closed and credential-free; isolated KIRO_HOME and
HOME are owned. Do not snapshot, inherit or inspect any credential value.

Capture version and agent validate --help. Require its actual grammar to
advertise --path. Then probe explicit owned valid, invalid-schema and missing
custom-agent files. Record raw bounded stdout/stderr/exit. Unknown grammar or
output fails pending review. Validation is only explicit custom-agent schema
validation, never proof that UAP skills or MCP were loaded.

Capture actual agent list and MCP workspace/global list in isolated owned
profiles. Only the previously measured exact exit1/error-null/empty-stdout/
64-byte not-logged-in refusal is an accepted expected denial. Unexpected
success is retained for review and fails this closed expected-denial contract.
All authentication/model/MCP runtime and native discovery flags stay false.

Archive exact source SHA/tree, immutable runtime identity, command receipts,
all original bounded streams and fixture preservation results. Retain evidence
on failure. Never reclassify historical runtime43212876549 as passing.
The historical runtime test, validator and failed evidence remain unchanged.
The ordinary runtime matrix excludes its authentication-dependent Kiro diagnostic;
the separate no-auth gate actually executes Kiro installation and native boundary
checks. Explicit workflow_dispatch with typed kiro_auth_research=true is required
to include that legacy diagnostic. Both runtime and research workflows pass the
Kiro secret only when event, explicit boolean and selected Kiro client all match.
Default Kiro research still runs without a key. No skipped authentication case
counts as a native PASS. This implements the owner's explicit no-auth scope;
no #878 file, production refusal or runtime qualification oracle is changed.

## Files

- scripts/project_native_kiro_no_auth.py: finite new runner reusing existing helpers.
- scripts/test_project_native_kiro_no_auth.py: inert negative/guard contracts.
- .github/workflows/kiro-no-auth-installation.yml: three-OS UAP lifecycle plus
  Linux pinned native no-auth observations; pull_request and path-scoped main push.
- docs/plans/2026-11-12-kiro-no-auth-installation-e2e.md: this contract.
- .github/workflows/project-native-runtime.yml: default no-auth selection and
  explicit manual-only legacy Kiro credential mapping (root-owned).
- .github/workflows/project-native-research.yml: preserve credential-free default
  research; the same explicit manual-only credential guard (root-owned).

Existing collector/test hardening may be reconciled by root; this patch does
not overwrite those files. No generated, production, public capability or
frozen VS Code files change.

## Estimate, reliability and review

Expected new runner 220-320 LOC, targeted tests 150-240 LOC, workflow 70-100 LOC.
Complexity 4/10: finite argv table and reuse of existing process/acquisition
authorities. Reliability confidence 7/10 for reused UAP lifecycles and known
refusals, 4/10 for unmeasured native validate output. Actual pinned observations
must settle that uncertainty; no permissive output parser buys a pass.

Run Python contracts/AST, YAML and embedded-shell syntax checks locally.
Actual Go compilation/lint, all three OS lifecycle tests and genuine pinned
native observations require final-source hosted CI. Original artifacts must
be checked independently before acceptance. Any native refusal is a declared
negative case, never a successful native lifecycle or model/tool observation.

## Sources

- https://kiro.dev/docs/reference/cli-commands/ documents agent validate --path.
- https://kiro.dev/changelog/cli/2-34/ pins the release context.
- https://kiro.dev/docs/cli/acp/ dates signed-out ACP initialization to2.37+;
  it does not establish a2.34 no-auth route.
- Original2.34 help/auth boundary: PR686 run43212876687, artifact13944208217.

