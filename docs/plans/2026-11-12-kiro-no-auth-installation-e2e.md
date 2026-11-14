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
output fails pending review. The measured result for all three validation
inputs is an exact authentication refusal before schema validation. No schema
validation or loading of UAP skills/MCP is inferred.

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

The no-auth collector rejoins the original source commit, tree and clean status
after all 18 native observations, using three fixed Git commands through the
existing bounded capture mechanism. It never replaces the original provenance
with a later baseline. Materialized runtime file hashes are retained in the
summary and rechecked around that final readback. The original 360-second budget,
native argv, private-profile isolation and all qualification flags are unchanged.
There are 27 total captured commands and 55 evidence members, below the existing
60-command/128-member/32-MiB limits. A final mismatch leaves collection incomplete;
the separate finite native-capture fact does not override that failure.

The local run-level contract covers unchanged provenance and changed commit,
tree, dirty status, command error and a boolean exit impostor. All 24 inert
no-auth contracts passed locally. This does not establish hosted execution of
the amended collector, User saved preparation or Kiro native loading.

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

Expected new runner 220-320 LOC, targeted tests 150-260 LOC, workflow 170-180 LOC.
Complexity 4/10: finite argv table and reuse of existing process/acquisition
authorities. Reliability confidence 7/10 for reused UAP lifecycles and known
refusals, initially 4/10 for unmeasured native validate output. The actual local
capture settled the latter as a refusal before validation; final-source hosted
acceptance remains required. No permissive output parser buys a pass.

Run Python contracts/AST, YAML and embedded-shell syntax checks locally.
Actual Go compilation/lint, all three OS lifecycle tests and genuine pinned
native observations require final-source hosted CI. Original artifacts must
be checked independently before acceptance. Any native refusal is a declared
negative case, never a successful native lifecycle or model/tool observation.

## Measured isolation correction before hosted acceptance

A separately labelled local protocol capture verified the exact official2.34.0
archive and companion identities. Version and agent validate --help exited0;
the latter advertised --path. The full original HOME guard correctly rejected
the capture because the client created XDG_DATA_HOME/kiro-cli/data.sqlite3 even
with private KIRO_HOME. That two-command failure and original streams remain
retained; it is not a successful native installation or hosted/App proof.

Each finite probe therefore selects held private XDG config/cache/state/data
directories as well as KIRO_HOME. Original HOME/A/B/User tree guards remain
unchanged. The Kiro-only wrapper uses the existing Context.command and bounded
process helper, verifies pinned runtime files before and finally, preserves
the exact argv/cwd allowlist,20-second timeout and stream receipts, and records
the actual child profile paths. No shared environment override rule is relaxed.
Inert regressions require private DB initialization to preserve original HOME
and reject private/profile/XDG replacement.

The second actual local capture completed all14 finite commands with all
original guards preserved. Root and an independent reviewer each checked its
original ZIP SHA256 d8c4d5f989170f29b63827ab667a06355001188889a43e34851a70a1b08f79c7,
CRC and28 stream byte/hash joins. Exact version and206-byte help succeeded;
all three validation inputs and nine agent/MCP inventories returned the exact
64-byte not-logged-in refusal, exit1, error null and empty stdout. The final
parser admits only these measured full help bytes and12 exact bound denials.
Schema validation, native discovery/loading, authentication, model and native
MCP runtime flags remain false. Both earlier failed captures remain retained.
Local protocol admission does not substitute for actual hosted UAP execution.

The dedicated workflow additionally admits exact Go JSON parent and Git/ZIP
subcase RUN/PASS events plus the SDK parent, with no fail/skip or zero-execution
acceptance. It retains clean before/after source SHA/tree and original log
digests with the lifecycle artifact. Current implementation size is about240
runner lines,250 targeted-test lines and172 workflow lines; complexity remains
4/10. No Go cases or production adapters change.

## Sources

- https://kiro.dev/docs/reference/cli-commands/ documents agent validate --path.
- https://kiro.dev/changelog/cli/2-34/ pins the release context.
- https://kiro.dev/docs/cli/acp/ dates signed-out ACP initialization to2.37+;
  it does not establish a2.34 no-auth route.
- Original2.34 help/auth boundary: PR686 run43212876687, artifact13944208217.


## Prospective selective status observation (2026-11-13)

Root authorized this three-path collector/test/plan addition after source review.
Keep the original fourteen probes and their twelve exact measured authentication
refusals unchanged. Existing pinned `project_native_collectors/kiro.py` already
registers `mcp status --help` with the `--name` token; that is the prior source
basis for measuring this route, rather than applying newer CLI docs to 2.34.

After those original observations, capture the actual pinned status help under
its own private profile and original total budget. Require exit0, null error,
empty stderr, <=4096 help bytes, the exact child-command Usage spelling and an
advertised `--name <VALUE>` option before issuing any status request. This is a
finite syntax gate, not a claim that the unreviewed full help has been admitted.
Then request status for the independently authored disabled A, B and global User
fixture names from their selected A/B/separate User cwd. The User request names
HOME's original User fixture while preserving the separate cwd's foreign fixture.
Every request retains the same closed environment, fresh private KIRO_HOME/XDG,
immutable runtime companions, original HOME/User/B/fixture guards and 20s probe
limit. The original360s total deadline is not extended.

Only the existing exact exit1/error-null/empty-stdout/64-byte authentication
refusal is accepted as the prospective negative observation. Unexpected success,
changed stderr, timeout or unknown help stops further status requests and retains
original evidence for review. All native discovery/status/MCP/model/auth and
product qualifications remain false. Fifteen expected denials and eighteen
finite probes describe the proposed complete negative capture, not historical
native evidence. Genuine final-source help/status originals and independent
source/stream/hash/preservation review are still required before acceptance.
No native executable, authentication, ACP, model, policy, pin, workflow, production
adapter or public capability changed. Local22 inert tests pass through both
discovery and direct execution; none is a native observation.

## Saved User preparation and cold agreement coverage

The composed public fixture exposed a concrete difference between fresh saved
User `add` / `deps import --install` and imperative `add --no-save`. With a
Run-only rejecting runner, the latter selected the existing Prepare intent;
the saved paths rejected automatic MCP preflight because the facade did not
carry that preparation decision. The ZIP diagnostic reproduced both refusals
with zero native calls and unchanged original User configuration. A Project
export or a warm imperative install does not prove fresh saved User support.

The private facade now retries one complete dry preview only for a fresh,
supported Automatic User install whose registered activation preflight fails
again for the same client, plan and backend. Existing scoped traits must permit
Prepare without personal mapping; Project, unsupported plans, completion
attestations, updates and recorded installations/preferences do not qualify.
Cancellation and an available automatic preflight retain their prior behavior.
The original sealed source, installation ID and recorded state remain fixed;
only the failed preview's unconfirmed plan captures are discarded. The complete
prepared preview reruns the existing source, ownership and component guards.
Apply passes its agreed intent even if duplex becomes available later.

An early same-IID absence check is only an early refusal. The authoritative
check is in the existing non-capture confirmation planner, which core Add
invokes while holding the ordinary mutation lock. A cooperating writer that
publishes a retained same-IID installation before that lock is acquired causes
ErrPlanChanged before inner planning. The regression uses real install/remove
receipts and the actual lock, preserving exact retained state bytes, native
files and data. No new Request field, state schema, flag or lock is introduced.

The existing three-OS Git/ZIP fixture adds a separate fresh User world after
its unchanged Project journey: saved add, exact repeat, changed update, an
owned-Skill quarantine and repair, User agreement export, fresh User locked
import/install and remove. Exact source/binding identity, current revision,
original complete data receipts and a held retained-data marker are joined.
All original User policy and A/B same-name foreign MCP/Skill sentinels stay
protected. Preparation owns exact native file receipts; the historical User
trait still leaves NativeProfileRoot empty. Existing Kiro receipt verification
joins those files to the original User configuration root without claiming a
native registration observation.

The rejecting runner counts every process attempt; all public steps must retain
Prepare, package-valid verification and unattested outcomes with zero calls.
These are public file-lifecycle contracts, not native Kiro startup, discovery,
login, MCP initialization or model-use evidence. The original eighteen no-auth
native probes and their budgets are unchanged. Local composed ZIP execution
passes; final-source three-OS Git/ZIP CI and the separately retained native
negative originals remain required for acceptance.

## Complete retained authority across the composed lifecycle (2026-11-14)

The existing public Git/ZIP User and Project A/B fixture now compares the full
installation and source binding, the full client binding except its updated
timestamp and appended committed receipts, and the original receipt prefix.
An authored source update permits only its actual revision, tree and package
digest changes. Project managed digests also join the actual native objects
through the existing Kiro verifier; the source transport and package revision
remain independently joined. Exact repeats cannot adopt a changed source,
installation, binding namespace, data owner or earlier receipt history.

Both scopes now hold the original data directory, owner receipt and retained
marker through every warm command. The Project fixture includes one never-run
stdio component with PLUGIN_DATA so that it exercises real data allocation in
addition to its existing HTTP component. The complete owner JSON must equal
the recorded data receipt. User remains Prepare/package-valid; Project remains
Prepared/installation-verified. The rejecting runner requires zero process
attempts after each command.

Alongside the existing missing-Skill case, each warm journey quarantines the
exact owned managed package and repairs it through the public command. The
fixture first verifies the original receipt and managed digest, then holds the
stationary parent, original leaf and destination parent across the rename.
Opening the leaf through its retained parent uses the existing Windows sharing
semantics; a top-level os.Open or os.OpenRoot on the movable leaf would prevent
the rename on Windows. Both parent identities and the original moved leaf stay
checked, and no foreign directory or authority is adopted.

Cold locked install keeps the complete source/package identity, creates a new
installation and disjoint data receipt, then repeats and removes while retaining
the new original data directory. Remove preserves full source/package/data
records and the existing exact User Prepare reinstall preference while clearing
the removed binding and environment receipts. Seven continuity controls cover
unchanged authority, an appended receipt, and five detached-authority cases.

Independent source review accepted the final two-file fixture. On Linux with
Go 1.26.11, its final public ZIP User/Project journey and all seven continuity
controls passed in 13.551 seconds. The complete log SHA-256 is
70eb3eab42e7d75153132be601f5d296057769ad889bf980d854fb66de9c1a6e.
An earlier draft correctly exposed a missing expected User reinstall preference;
the oracle now checks that exact existing production value. These local results
do not establish Windows/macOS execution or native discovery. Final-source
three-OS Git/ZIP runs and the separate eighteen no-auth native originals remain
required; their commands, pins, budgets and qualification flags are unchanged.
