# Kiro installation proof extraction

## Decision and scope

The owner requested useful installation slices independently of agent or model
calls. This change extracts the Kiro installation tests from
`a610eadf5436dd3fb2b29fbb069615028b46ad00` onto current main
`2646893aa9ca547a84fd577d026d93e585e8611c`. All 30 files in
`install/integrationctl/agentplugins/clients/kiro` have identical Git blobs at
those two commits. The initial test-only extraction exposed a missing generic
installer dependency during real Linux execution: fresh User MCP installation
failed automatic activation preflight because the fixture deliberately has no
ACP duplex runner. The corrected slice preserves that fixture and adds the
existing facade path that offers Prepared installation without authentication
or native execution.

The revised allowlist is nine paths: three CLI test files, three existing
installer files, one installer test file, the existing Kiro no-auth workflow,
and this plan. The CLI test files retain the exact source blobs:

| File under `cli/cmd/agentplugins` | Source Git blob |
| --- | --- |
| `kiro_project_fixture_test.go` | `020a7602b27440094aba008afc58987b4f8aec18` |
| `kiro_project_lifecycle_test.go` | `ed1d90c2aed714430a63d8c1894a78bad387015a` |
| `kiro_project_archive_test.go` | `b5eb3ab7414f0ed986e27088aa57b91c21a296f8` |

The shared journey APIs used by these tests already exist on main. The other
branch's added optional transport context/root and diagnostic delegation are
not used by this extraction and are not copied. The four additional installer
paths are `prepare_package.go`, `apply_package.go`, `confirmation.go`, and
`hosted_preparation_test.go`; the latter retains exact source blob
`ff4c7f6a3cd8f077b197f201e55dc024655b235e`. The adjacent source changes for
`ConfirmPluginEnable` are excluded. No adapter, public API, state schema,
process runner, native Kiro collector, authentication/status research, release
workflow or public capability claim is changed.

## Fresh User preparation and confirmation

The facade retries a failed automatic preview as Prepare only for a fresh User
install with a Ready automatic plan for the same client, no Project target,
no selected-delivery variant, no activation/authentication attestation, and a
nonempty IID absent from the original captured state. The registered delivery
traits must already permit preparation without personal mapping. This uses
existing traits, with no client-ID branch or new capability table.

The same activation preflighter must independently refuse the original plan
under the original context. If it becomes available, the original failure
remains; cancellation remains cancellation. A live state read only detects a
same-IID installation that appeared after the original capture. The sealed
package, original recorded state and IID remain authoritative. The second
preview repeats source, scope, component and ownership checks with Prepare;
only failed, unconfirmed plan captures are discarded.

Apply passes the frozen preparation intent to the core, so later ACP
availability cannot upgrade an agreed Prepared install to automatic execution.
The confirmation planner repeats the fresh-IID check while the existing
mutation lock is held. Existing installation modes, retained preferences,
Project/update routes, owned drift and personal-mapping requirements keep
their original boundaries. This is not a retry after client execution or a
partial native effect.

## Installation evidence

The tests use the production CLI composition, solver, installer, native file
kernel, ownership receipts and persistent data manager with real loopback TLS
Git and ZIP source acquisition. Host discovery and security assessment remain
explicit inert fixtures. A rejecting runner must receive zero Kiro or MCP
process calls; results and saved bindings remain Prepared without activation or
authentication attestation.

The expanded lifecycle checks original User and project-B file identities and
bytes while project A changes; exact source, package, binding, receipt history
and data ownership; saved User repeat and update; repair of a missing owned
Skill and of the whole owned package; export and cold import; locked repeat;
and removal with retained data. Declared faults quarantine the original owned
package through held identities. A rename error fails the test; there is no
general Windows skip or replacement of the original ownership baseline.

Separate counterexamples reject detached source, installation, binding, data
and receipt history. Two archive controls exercise exact authored Skill bytes
with `core.autocrlf=true`, including the actual ZIP acquisition/installation
consumer.

## Existing CI lane and acceptance

The existing Linux, macOS and Windows composed-lifecycle jobs select four
top-level methods: `TestProjectKiroComposedSourceLifecycle`,
`TestKiroContinuityRejectsDetachedAuthority`,
`TestKiroUserArchivePreservesAuthoredBytesWithAutocrlf` and
`TestKiroUserArchivePreservesAuthoredSkillWithAutocrlf`. Their fixed nested cases
produce exactly 15 named RUN/PASS pairs. The existing admission reader rejects
missing, extra, failed or skipped cases and rejoins the unchanged source.
The SDK command retains `TestProjectKiroPublicSDKStdioDataRepairRestartAndRemove`
and adds six `TestFacadeHostedPreparation` methods, with exactly 25 named
RUN/PASS pairs in total. They exercise frozen intent and sealed-source Apply,
preflight availability and cancellation, appearing retained state, recorded
modes and drift, finite eligibility, and a real mutation-lock barrier. Native
execution is forbidden by their runner. The no-auth native job, workflow
triggers, time and output limits, source joins and final acceptance condition
are unchanged.

The pre-extraction main-compatible surface has three-OS installation evidence
at source `70d21018f004f5855c62c5e60aeeafff14f32012` in
[run 43643879259](https://github.com/777genius/universal-agent-plugins/actions/runs/43643879259).
That run passed all five jobs, including CLI Git/ZIP and SDK lifecycles on three
OS. It is historical evidence for the existing surface, not a PASS for these
new test bytes or the expanded CLI and SDK admission.

The initial five-path candidate had source-only independent review. Its first
Linux CLI run passed two of four top-level methods and failed two. Both User
ZIP entries reported that automatic Kiro MCP lifecycle requires an ACP duplex
runner; these are the generic preparation failures addressed above. The Git
entry separately failed with `initialize private Git transport workspace
failed`; that log does not expose its underlying cause. A separate diagnostic
with the unchanged original Git runner confirms that this container cannot
start its contained Git child because the required process-children interface
is absent. This is separate environment evidence, not a successful Git test. The
original 15,116-byte log has SHA-256
`249660b1776f7a9aa05b2aaf9cdc0d705aecfc357c7ee3a54112b97076030a33`.
That failed run is retained and is not relabeled as a pass.

The corrected nine-path candidate ran all 25 SDK cases successfully, with no
failures or skips. Its local CLI run executed 14 named cases: 12 passed; the Git
entry and its parent failed during the same private-transport initialization,
before the nested Git/User case could start. ZIP/User, continuity and both
authored-byte controls passed. These are partial local CLI results, not the
required 15-case acceptance.

Two independent scratch overlays restored only the old Apply-intent forwarding
or the old locked confirmation. Each compiled and failed its intended semantic
assertion: `apply changed agreed preparation:` and
`locked fresh-only refusal failed:` respectively. The candidate itself passed
both corresponding SDK tests. The controls do not modify candidate source.

`make lint`, `make test-core` and final-source CI remain required validation.
The Git acquisition failure remains a separate required gate. Hosted CI must
execute the exact 15 CLI and 25 SDK cases on all three OS; queued, skipped and
unexecuted cases cannot satisfy that requirement.
Actual native Kiro loading, authentication, agent selection, MCP initialization
and model calls are not claims or merge requirements of this installation
slice.
