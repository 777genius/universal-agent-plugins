# Cline measured installed ACP grammar follow-through

## Exact genuine observation and scope of proof

Current PR525 source 8d278b1d0766931be192447733e1e8ffbd0167b5, tree
e1d976688c72828fbb6a1bc38843e4a14348641b, has genuine hosted runtime run
40453454042, native job 122021475603. The test passes its existing source,
public project installation, receipt/ownership, original-user custom-provider,
actual ACP permission and provider/probe checks, then deliberately fails at
agentplugins_cline_project_native_attempt_test.go:704 to await capture review.
It does not fail a native transport, model/provider, or cleanup assertion.

Native artifact 12711941321 is 29608 bytes, outer SHA256
15fa9c5af0385974a52abd567e92263aae10f163a5fb583b4e74348b98e75a36;
its runtime.zip SHA256 is
47f125ce97d47cd820704580f3a9349a22f9226979eeebc94ab3b56f9b10f39a.
Frontend artifact 12711871327 is 22999 bytes, outer SHA256
b915029a0db000128bfd0f9c52cf553b3976c86d2d00d51c6e8be483b26a696e.
CLI independently verified outer digests, canonical unique ordinary flat member
inventories, CRCs, exact source identity and frontend command-stream digests.
The frontend delegated service is closed, forced_stop=false, forced_kill=false,
under original UID/GID1001. The native process used existing planned duplex
shutdown, with auto_approve=false and injected_mcp_servers=[].

The ten measured native ACP frames establish initialize protocol1, agent name
cline/version3.0.73, native session1793077736317_iF4rU_cli, and the existing
user-catalog model/provider. The single installed uapclinea__inspect_runtime
call has pending tool_call_update, one exact session/request_permission,
in_progress tool_call_update, pending tool_call, completed tool_call_update,
the exact final agent message, session_info_update, and end_turn result.
Only allow_once is selected by the existing narrow client permission logic.

Native rawOutput, second provider-request tool history and independent probe
tools/call facts are equal. They join the installed package/data paths, argv,
nonce, revision a-one, marker, PID6091 and probe process session. The three
probe events initialize/tools-list/tools-call are ordered inside the measured
native interval. Provider metadata has error=false, two requests/two responses,
exact_call_issued=true and single_call_approved=true. Raw native-acp.jsonl SHA256
is 1a4e9a20fe5958b9232bd8d3f1e46c42382c66c61807a91ee133bb39b2a3c8af;
probe-events.jsonl SHA256 is
40f6f77054109fc93e141d1721efd9d50a32676c2631eff1323670fbf1c97284.
The prior genuine source6b76908e/run40397574726 separately captured and passed
the same ten-frame grammar and independent join before the same deliberate
failure. Retain that earlier evidence as history, without relabelling either run.

This observation proves one source-bound installed ACP exchange. All five
qualification flags remain false, as do vendor authentication and real model
flags. It does not prove the full A/B/user lifecycle, update, repair, retained
data across restart, removal, or successful post-removal discovery. Those remain
mandatory subsequent E2E work; this slice removes only the measured capture gate.

## Baseline and implementation boundary

Work starts in an isolated checkout of the exact current commit8d278b1d.
The actual GitHub current tree was independently read and fetched read-only:
the two Cline Go proof files, existing Cline contracts and two shared runtime
Python files match blobs71071c17,ca7d41dc,3502c1fd,854cef3b,dfbb0c86.
Preserve both externally published lint corrections dab49d09/8d278b1d, their
existing exact wire spelling, CI-only overlay and ancestry; do not add lint
suppression or replace an external file with the stale local version.
VS Code is explicitly out of scope.

Obtain two independent plan GOs before editing implementation. Scope is exactly:
repotests/agentplugins_cline_project_native_attempt_test.go;
repotests/agentplugins_cline_project_native_provider_test.go;
repotests/agentplugins_cline_project_native_contract_test.go for the narrow
existing permission-protocol fixture adjustment described below;
one new bounded Linux grammar-replay Go test file;
two exact nonsecret measured JSONL testdata files;
scripts/project_native_runtime.py; scripts/test_project_native_runtime.py;
and this plan/validation record. Preserve all existing Cline contract cases and
assertions; no other change to that existing contract file is in scope.

### Freeze the measured proof grammar

Retain the current strict JSON decoder, closed field sets, response order,
native session format, provider-issued call binding, single permission, exact
tool/title/input, original project cwd and empty session/new mcpServers. Tighten
agentInfo.name to the measured exact cline value. Add a small finite progress
state for the measured tool notifications: pending-update before permission,
in-progress only after that approval, the measured pending tool_call, then one
completion. Reject repeats, missing transitions, out-of-order tool events and
events outside the prompt stage. Final text may arrive in bounded chunks only
after completion and must concatenate to the existing exact nonce sentinel;
retain strict session-info timestamp shape and require the measured final
session-info notification before end_turn. Do not accept new protocol fields,
tools, permission choices, sessions, model/provider IDs or source injection.

The current installed source/receipt/tree checks, provider validation and
native/provider/probe/marker join remain decisive. After all of them succeed,
replace the deliberate fatal capture gate with runtime_attempt_complete=true
and native_exchange_observed=true. Keep every qualification entry false.
Deferred provider closure and original total-budget checks must run before the
final result is written; if any deferred check marks the test failed, reset
runtime_attempt_complete=false so cleanup failure cannot emit success.

### Meaningful inert replay of actual measured bytes

Preserve the exact hashed native ACP and probe JSONL streams as small testdata;
they contain disposable fixture paths/nonces and no credentials. Re-read them
with fixed digest assertions. Feed the actual transcript through the existing
f.acp parser using inert readers/writers and the recorded call/session/nonce
inputs, then compare the returned native facts to the recorded probe call and
exercise existing clineAttemptEvents on its recorded interval. Verify outgoing
initialize/session-new/prompt/allow-once messages remain exact, including no
injected MCP servers. No vendor, listener, model, installer or subprocess runs.

Use differential malformed transcripts from those actual bytes to reject wrong
agent/session/tool/call/arguments, duplicate or reordered permission/progress/
completion, missing completion/sentinel/final result, unknown fields/methods,
failed tool output and mismatched probe facts/events. Keep existing hostile
provider/installed-authority/session/permission/event contracts enabled. The
existing unknown/duplicate-permission ACP fixture currently sends permission
immediately after session/new. Add the actually measured pending
tool_call_update before that permission, so its first valid allow-once and
second duplicate cancellation assertions continue to exercise their intended
boundary after ordering is tightened. Preserve those assertions and all cases.
Put all new replay tests in a bounded file rather than expanding existing large
contract files.

### Preserve closed artifact admission for both outcomes

Retain the current failure diagnostic branch: failed captures still have
runtime_attempt_complete=false/test_failed=true and all qualifications false.
Add a strict success pair true/false; reject mixed contradictory pairs. Success
requires the existing exact complete 21-member closure: all four public command
triples, both ACP members, both provider request/response pairs, provider receipt,
probe events and summary; no ACP error member. Require public command exit0,
error=false/deadline=false, ACP error=false, exact two/two provider counts,
issued/approved=true/error=false, measured attempt stage/session identity and
native_exchange_observed=true. Preserve exact source identity, version, Prepared
activation, secret rejection, member bounds and every qualification=false guard.

The shared Python frontend remains an envelope/closure validator, while the
reviewed Go proof owns the native protocol and independent authority joins.
Do not add a second native protocol engine or trust an incomplete summary-only
success. Extend existing Python contracts with complete success and missing/
contradictory/forged member and receipt cases. Other clients' admission behavior
must remain unchanged. collection_complete then describes this one measured
attempt only; it never implies full lifecycle qualification.

## Checks, publication and remaining E2E

After implementation review, use the exclusive Go/lint slot for focused existing
Cline plus new grammar-replay inert contracts and affected all-enabled and
whole-file architecture/size lint. Run existing shared Python runtime-admission
contracts, gofmt/diff checks and exact source/fixture hash verification. No local
vendor or SDK execution, host spoofing, pin change, policy bypass or suppression.

Root alone publishes reviewed commits to existing PR525. Fresh genuine hosted
source-bound execution must observe the measured exchange and ordinary delegated
cleanup on the new head before the single-attempt gate is considered closed.
Do not relabel the earlier deliberate failure. Then implement and independently
review the remaining full project lifecycle/native restart/removal proof, keeping
user authority, retained data, source movement and authentication as their own
observed gates. No release or new PR is authorized by this plan.

## Plan review

Final plan SHA256 e06ee55bd8e1a3010748cae724ded5cd807c06211700ca069b742a7f1dd887eb
received independent desktop R1 GO and shell R2 GO before implementation.
