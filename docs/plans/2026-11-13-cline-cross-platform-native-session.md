# Cline native managed-session coverage on three platforms

## Observed Windows import and macOS cold-install corrections

The original b378190d Windows [job 131910356071](https://github.com/777genius/universal-agent-plugins/actions/runs/43432575104/job/131910356071)
passed its Go packages but failed while importing the Python contracts:
`cline_managed_session_e2e` imports portable acquisition helpers, which imported
the POSIX-only `resource` module at module scope. No vendor/native action ran.
Root R1 approves moving that import into the existing `child_limits` function
and an explicit POSIX-only refusal at `run_probe` entry, before opening capture
files or starting a process. Core limits, selectors, process-group cleanup and
existing POSIX execution stay unchanged. There is no Windows process fallback;
the Cline journey continues to use its existing Go bridge.

The original b378190d macOS job 131910354493 reached native installation, update
and repair, then refused cold installation because the bridge appended
`--target cline` to `install --locked`. The ordinary install command takes its
targets from `uap.toml`; explicit Project scope/root remain valid. Root R1 also
approves omitting only `--target` for the fixed install verb, preserving the cold
working directory, private HOME/state and all agreement/receipt checks.

This correction is limited to five existing files: this plan,
`scripts/native_client_research.py`, `scripts/test_native_client_research.py`,
and the `clinesession/main.go` and `clinesession/main_test.go` research files.
Estimate: 6–10 Python runtime lines, 20–35 Python contract lines, 5 Go runtime
lines and 25–40 Go contract lines. Tests must reproduce the missing-resource
import without claiming Windows execution, refuse unsupported probe execution
before effects, and verify exact cold-install/repeat invocation through the
bridge with a targeted native-inspection control. No product parser change,
host-flag spoofing, new downloader or qualification promotion is included.
Independent source review and fresh exact-head native results remain required.

Correction validation: the new missing-resource/refusal contracts and both cold
invocation cases failed against the original 3718c139 source. After correction,
45 relevant Python contracts and all eight Go helper tests pass; the complete
Cline acquisition import also passes with `resource` unavailable. Scoped Go lint
reports zero issues, Windows amd64/macOS arm64 test cross-compiles pass, and
root's independent R2 accepted the change. These local contracts and compiles do
not qualify the pending genuine final-source native runs.

Base: ff0be246091497b031eb0b5150803d755cbd5338. This extends the existing no-account managed-session journey, not production client behavior. Read together with the six-client installation automation plan, ARCHITECTURE.md and CONTRIBUTING.md. The current Linux journey uses POSIX-only command, file and protocol helpers; merely adding matrix hosts would not qualify Windows or macOS.

## Allowlist and size

Only these eleven paths: scripts/cline_managed_session_e2e.py; scripts/test_cline_managed_session_e2e.py; scripts/project_native_collectors/cline.py; .github/workflows/cline-managed-session.yml; new install/integrationctl/agentplugins/research/clinesession/main.go, protocol.go and main_test.go; this plan; and new plugin.json, mcp.json, skills/check/SKILL.md alongside repotests/testdata/agentplugins_native_probe/main.go. Do not change that shared probe main.go. Initial estimate: 950–1150 runtime/test changed lines, excluding this plan; report a concrete overrun before expanding. New Go production files remain below 500 effective lines. No shared process, native adapter, parser, module edge or security exemption changes.

## Verified acquisition

All three targets retain Cline 3.0.73. Existing Linux x64 pin is unchanged. The official cline@3.0.73 optionalDependencies identifies @cline/cli-darwin-arm64 and @cline/cli-windows-x64, not cli-win32-x64. These registry archives were downloaded and inspected as bytes without executing the vendor payload:

| Target | Archive bytes / SHA256 | Entrypoint / bytes / SHA256 |
| --- | --- | --- |
| darwin-arm64 | 33810336 / 3564eb2249594e0d29a606daef4bb5f0f7ae6e08701d41171e0b20f7107b3f32 | package/bin/cline / 90264306 / 1ddd1a076cb333b27f61b172ae07611dcfe743218dc2547bda22a7b1e2722358 |
| windows-x64 | 47664377 / bca9c4be2dcacae2886b27a2426dbed324ca1f54644ead519e00a680c3575234 | package/bin/cline.exe / 113940152 / ebf74e8bb04a99d0098ccc446f9b11cf50e6c6ff994c4d1e87aa57e22bd8e051 |

Freeze the corresponding exact publisher SHA512 integrity strings in the existing collector platform table. Use existing acquire/materialize archive validation; verify all materialized member hashes through the portable held-file reader, before and after execution. Refuse unknown OS/architecture, archive size/digest/member/type mismatch or changed native bytes. No latest lookup or arbitrary executable discovery. Official sources are https://registry.npmjs.org/cline/3.0.73 and the platform package/version endpoints; full member receipts are local analysis inputs, not native acceptance.

## Fixed bridge, not a generic executor

The new Go executable is source-commit/tree pinned. Each action receives one bounded JSON object plus LF; Python keeps stdin open until the bridge exits. One shared buffered reader retains prefetched extra bytes. Parent EOF, any extra byte, SIGINT/SIGTERM or the original request deadline cancels the same context. Do not read the request until EOF. Command time is at most 120 seconds, with five seconds reserved for existing descendant-exit grace, all inside the existing 900-second journey budget. Protocol actions use at most 15 seconds inside that original bound. Bound request to 16 KiB and ordinary stdout to 1 MiB; stderr has the existing 32 KiB runner bound. Truncated, malformed or incompletely settled capture cannot authorize another action or evidence success.

Closed request fields identify schema, source commit/tree, absolute canonical disposable root, action, and original deadline. Runtime binary identities are task-authored once under that root and compiled/recorded SHA256s are rechecked through held file identities before launch and after settlement. No request-supplied arbitrary argv, environment, cwd, URL, or target path. Root has the fixed uap-cline-session- prefix and only the fixture-created root may be used. Pin native, UAP, probe and helper; Git is the explicit resolved hosted executable, checked and recorded before commands. Executable suffix and path separator are platform-derived, never guessed from an untrusted string.

Fixed action table: native-version; project-init; install/repeat-install/update/repair/remove; declined-start/install-yes-is-not-start-consent; preflight-list/preflight-doctor/preview; session-inspect; missing-package-refusal/removed-package-refusal; ordinary-native-user-source; direct-session; saved-add/cold-locked-install/cold-repeat/cold-session. Each maps to literal reviewed arguments and fixture-derived selectors/paths. Expected nonzero exit is admitted only for the corresponding refusal action and still requires positive cleanup proof. IDs needed for repair come from the exact selected state, not caller argv. All other actions or unknown wire fields/types/duplicates refuse before launch.

Use process.OS.RunWithDescendantExitGrace for every command. Require CleanupComplete true and NoChildStarted false before a completed-action receipt; no nil/zero-PID inference. Retain actual exit and semantic refusal separately from cleanup. The runner owns Windows Job creation/assignment/resume/termination and macOS/Linux ordinary containment. These are existing trusted-child authorities, not a new hostile-process sandbox.

Important: OS.RunDuplex and RunDuplexWithPlannedShutdown intentionally refuse their stronger prerequisites on Windows and macOS. Do not bypass or weaken them. For the exact task-owned direct MCP exchange only, an ordinary supervised invocation of this same bridge enters fixed protocol-client mode. It launches ONLY the exact installed managed helper selected from the current owned Cline transport, using existing managed-stdio arguments/environment/cwd. The outer ordinary runner retains descendant cleanup authority throughout. The inner protocol client is not another supervisor and its leader exit is not tree proof.

The fixed client uses bounded Go pipe reads, preserving MCP sequencing: send initialize; validate its exact correlated response and negotiated protocol/server identity; only then send initialized, tools/list, valid tools/call and wrong-nonce tools/call, validating each response before the next request. Require exact IDs, bounded JSON-LF frames, no extra responses, expected nonce/revision/cwd/plugin-root/data/marker and ordered same-session probe events. Close stdin, drain bounded stdout/stderr to EOF and wait once. On any error, preserve it and let outer cancellation/cleanup finish; no success result before outer settlement. Do not send an entire pipelined transcript before initialize is validated. No session/new, model, auth, trust, auto-approval or arbitrary server selection.

## Portable filesystem and environment

Keep all profiles independent of real HOME. Set HOME and USERPROFILE where applicable; task-owned temp/XDG/cache roots; platform-correct PATH using os.pathsep; updater disabled, auto-approval false, Git prompting/hooks/system/global config suppressed using os.devnull. Preserve only explicitly required Windows SystemRoot/SystemDrive infrastructure, no credential/provider variables or ambient tool args. Require original User settings bytes and independent Project B/foreign-control snapshots unchanged at every stage.

Replace this journey's uses of POSIX-only read_owned/verify_files with existing portable artifact open/identity authorities and bounded held reads. Do not change shared Grok helpers. Require canonical root and contained components, ordinary single-link files, bounded bytes, no symlink/reparse/special node, stable same-domain pre/open/post identities and hashes. Preserve original root/foreign snapshots; never adopt a new baseline after a native write. Unknown source/profile/foreign change refuses the next action. Use .exe for authored fixture binaries on Windows. Deliberate missing-package fault preserves the exact owned package by moving it into an independently owned quarantine; do not recursively delete a pathname after closing authority. Retain disposable roots on failure rather than introducing a cleanup engine. Filesystem retention is distinct from mandatory process-tree settlement.

## Lifecycle and actual cold locked restore

Retain the current task-authored package's install, repeated install, missing consent, exact native source selection, update, missing-package refusal, repair, remove and retained-data assertions. Actual Cline config inspection is invoked through ordinary UAP selected-process startup; ordinary unwrapped Cline in independent B must still see only the original User source. Project and User foreign entries remain byte/identity preserved. Keep activation Prepared.

Cold restore is a separate explicitly identified immutable public fixture, not an improvised re-add. Add only three tiny manifest/Skill/MCP files alongside the existing stdlib probe source. Its go run argument uses the verified absolute PLUGIN_ROOT/main.go token, never an assumed current directory; use pinned Go 1.26.11 with GOTOOLCHAIN=local/GOWORK=off. Fresh task environment supplies only task-owned probe parameters; immutable source bytes do not change for each nonce.

Acquire this package through the repository's exact current source commit and exact fixture path, using the existing saved-install CLI. Keep the resulting actual uap.toml/uap.lock bytes. Create independent project C, HOME, profile, cache and state, initially empty; transfer ONLY those two agreement files. Execute install --locked --yes with the existing saved-install authority. Validate exact source commit/tree/manifest/package/binding agreement, unchanged agreement bytes, native selected configuration and direct installed probe handshake. Repeat locked install and prove stable package/state/data/foreign controls. Do not copy .uap state, store or native configuration from the original project. HTTPS-only acquisition remains unchanged; no HTTP loopback bypass, custom CA or local archive masquerading as portable agreement. Native run must report any acquisition/prerequisite refusal honestly.

## Validation and evidence

Add inert contracts for closed action/wire decoding, parent EOF/extra input cancellation, unknown action no-child-start, nonzero/cleanup-incomplete/truncated output no next action, pinned executable replacement, selected helper escape/unknown transport, exact native platform selection, Windows path/env suffix behavior, sequential initialize gating, bounded protocol/error output and genuine agreement-only cold setup. Reuse real process adapter/managed-stdio platform contracts. Run focused Go tests plus GOWORK=off parent build, module lint/size, Python contracts and Go cross-compiles; root runs coherent lint/core.

Workflow expands genuine native journey to Linux x64, macOS arm64 and Windows x64, with exact checkout source and pinned setup-Go, while retaining existing platform managed-session tests. Publish per-platform source/runtime/acquisition/action/protocol evidence only after positive settlement, with all hashes/stream joins. Evidence retains model_invoked=false, vendor_authentication_used=false, native_cline_tools_call=false; direct-installed-MCP evidence is explicitly labelled. Queued/skipped/cancelled/refused/evidence-only runs are not native lifecycle PASS. No production label or account/model inference changes.

Root design review R1 accepted the eleven-path scope and separate protocol.go responsibility. Preserve read-only marker checks after initial creation, source/receipt-bound cold restore, ordinary-runner cleanup and the original deadline. Complexity 6/10, reliability 8/10, confidence 7/10 before hosted execution. A second independent source review and native execution remain required.

Implementation clarification: the parent Python original 900-second fence uses its own monotonic Budget. Each request carries an absolute UTC deadline, computed once from current wall time plus at most 120 seconds and the remaining original budget. Go checks the bounded wall-time interval and creates its own cancellation timer; it never compares Python and Go monotonic epochs. The bridge lives in the child agentplugins module to reuse managedstdio argument and path authorities without a reverse module dependency.

## Final source review and validation checkpoint

The frozen eleven-path patch changes 1407 runtime/test lines including additions and removals (plan excluded). Production adapters, process authority and public capability labels are unchanged. The increase over the initial estimate is the explicit portable bridge, cold source/receipt joins, and the required negative contracts; no additional product feature or abstraction was added. Root reviewed the scope increase, and a second independent reviewer checked the final behavior and size.

Independent R2 findings were corrected: embedded bytes.Buffer no longer exposes an unbounded ReaderFrom path; an actual 64 KiB stderr child must fail. Event proof requires exactly five independent owned sessions and fifteen joined events, refusing missing/extra/foreign sessions. Clean source checks run before compilation and after native observation. Explicit null disablement/auto-approval refuses. A positive read-only protocol contract found and fixed JSON number identity handling. Final success checks the original 900-second budget after evidence and Git joins.

Local focused Go: seven top-level tests plus meaningful subcases PASS. Python: six contracts PASS. Scoped lint: zero issues. Windows amd64/macOS arm64 cross-compilation passed; cross-compilation is not native execution. Genuine native lifecycle/cold restore on the final published commit remains required in the three-platform hosted workflow. The local host is not falsely advertised as GitHub-hosted.

## Concurrent implementation union

The reviewed union keeps the closed wire schema, runtime/source pins, exact operation/event joins, receipt-bound cold restore and composition-based stderr bound above. It additionally isolates cold Go caches and working directory, merges environment keys through the existing session authority, and compares managed argv as individual arguments. Each direct invocation preserves installed configuration, state, helper and probe identities/bytes plus the previous event prefix before another action may start. Cold native inspection must leave state unchanged. Thread joins share one cleanup deadline, and the workflow uses explicit platform Python selection and the child Go module.

Additional contracts execute the real owned Go probe through separate write/read sessions, verify cold environment independence, and refuse state mutation, identical-byte helper replacement, replaced event prefixes and foreign session events. The union remains inside the original eleven paths; genuine three-platform native results and all original false qualification flags remain unchanged pending the published-head run.

Union validation: nine top-level Go tests and seven Python tests pass, full-rule scoped helper lint reports zero issues, and the child module builds with GOWORK=off. Windows amd64 and Darwin arm64 test binaries compile; these compile checks do not claim native execution. The workflow YAML parses with explicit PowerShell/native Python selection and retains cancel-in-progress=false.

## Deterministic Windows pre-resume cancellation regression

The external 6ec9ad68/3718c139 correction checks cancellation after successful
Job assignment and before resume. Preserve those six production lines. A missing
PID file could also result from resume followed by fast termination, so the test
must observe the existing resumeSessionChild seam, which delegates to the real
resumeSessionProcess/ResumeThread path. Reuse 16bb6b0d without a second seam. Cancellation at the assigned-but-suspended boundary requires zero
calls; a paired real-resume control requires one call before cancellation.

The existing disposable executable remains the only child. Before real assignment,
retain non-inheritable duplicate process and Job handles through a synchronized
channel. Register teardown before launch: cancel, terminate the retained Job,
wait on the retained process with a direct fallback if necessary, join the sole
StartSessionContext goroutine, close handles, then restore hooks. Report every
teardown error; require the process already settled before rescue can mask a bug.
No PID reopening or PID-file absence supplies execution/cleanup proof.

A duplicate refers to the same object and retains its lifetime, so this regression
exercises successful explicit termination; the separate kill-on-last-close failure
contract stays unchanged. See Microsoft's [DuplicateHandle](https://learn.microsoft.com/en-us/windows/win32/api/handleapi/nf-handleapi-duplicatehandle)
and [Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects)
contracts. Root R1 initially scoped two Go files plus this plan; after the external
16bb6b0d seam, this follow-up changes only the existing test and plan. Linux
cross-compilation/lint can validate source, but actual Windows execution is still
required; no containment, consent boundary or platform support claim is broadened.

## Original empty-array evidence preservation

The b79c2ac0 Windows [job 131933636575](https://github.com/777genius/universal-agent-plugins/actions/runs/43439520201/job/131933636575)
completed the first direct exchange and all eleven recorded command cleanups,
but correctly refused its event join: the original tools/call and transcript
contained `argv: []`, while re-encoding the decoded report produced `argv: null`.
The ordered methods, session, nonce, PID and other facts matched. This failed
capture does not qualify the remaining native lifecycle or cold restore.

The concurrent cf454ea7/b4c6abe9 correction preserves a non-nil empty array in
the existing shared strict decoder and covers empty, nested and null values.
This follow-up changes only this plan and the existing clinesession Go test:
serialize both real owned-probe write/read reports through the final facts
field, decode them independently with encoding/json, and require complete
agreement with the original tools/call event, including its empty argv array.
No second decoder, normalization or protocol runtime change is added. The
strict Python comparison, original probe, process bounds and qualification
flags remain unchanged. Independent static review accepted this regression.
Local execution is unavailable: red/green, lint and cross-platform execution
have not been performed for this follow-up. The test is designed to detect
the historical mismatch; fresh exact-source CI remains required.

## Held runtime and portable import follow-up

Preserve the concurrent cold-environment, agreement-target, original-event and empty-array corrections through 64cf9251. The fixed bridge now holds the original root and all five runtime file descriptors across command settlement, joins held and named identities before invocation and afterwards, rechecks pinned bytes, and refuses success after final release if the original context expired. Exact stdout/stderr capture caps refuse conservatively. Same-byte replacement cannot replace original identity.

The proof is portable incarnation, mode, size and mtime continuity plus compiled SHA256 pins. It does not independently establish POSIX UID/GID/nlink or Windows SID/ACL continuity; the existing Python admission checks retain those narrower platform and single-link conditions. No generic ownership framework or production capability is added.

The shared research utility retains its lazy POSIX resource import. Its process entrypoint now refuses absent resource support before creating capture files or starting a child; bounded reads and the complete Cline frontend remain importable without resource. The isolated-import regression, held-root/runtime replacement and late-cancellation contracts accompany the change.

Before execution became unavailable, the corresponding 40692b0f union passed focused Cline Go tests, 59 shared Python contracts and 13 Antigravity contracts; full Core completed with the retained 25 host-restricted failures in seven packages. This source is reconciled with subsequent branch changes through 64cf9251 and requires fresh hosted lint, Core and native cross-platform checks. No pending or historical result is substituted for current-source acceptance.
