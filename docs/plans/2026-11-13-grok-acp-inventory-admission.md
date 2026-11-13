# Grok 1.0.70 isolated ACP inventory admission

Status: root and independent plan reviews authorize ONLY this metadata admission.
No local vendor/privileged execution, production enablement or remote publication
is authorized by this file. Restored base after environment loss:
`76f2a8a35bcffeb330409353f80bf57e94a77759`; preserve external Hermes corrections.
Root reviews the coherent implementation and gates before publication to PR878.

## Question and sources

Actual pinned CLI listing omitted direct Project plugins; absolute-path enable
returned NotFound. Public Grok source `2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8`
contains a sessionless extension listing full ID/root/scope/trusted/enabled. This
source guides a hypothesis, not proof of published1.0.70 behavior. Reuse exact
`scripts/project_native_collectors/grok.py:ACQUISITION` via
`scripts/project_native_research.py:acquire`. Binary SHA256:
`22d281c5964c5444147328096ba2a221f3f0874c6858e27c825656a80f6e58e9`;
archive SHA256 `11d27a30adfb75c4b9b180072ab27412a1916f054ff5497ab70fb9a77d304bcc`.
No production pin bump or actual model/key/login.

Public extensions/plugins.rs129-157 accepts camelCase sessionId. An unknown
session lazily builds the launch-cwd registry. agent/app.rs279-290 starts auth
refresh, managed policy and settings/model-catalog work before dispatch, so this
is not claimed side-effect-free or network-idle. acp_agent.rs534 also starts
launch MCP configuration discovery. Fixtures therefore contain ZERO MCP/hooks/
LSP/envrc/executable components. Empty MCP discovery may emit the one reviewed
notification below; it is not an MCP tool call.

folder_trust.rs294-308 maps allow_prompt=false to FailClosed;131-135/259-284 record
verdicts in the process-local DECISIONS map. "Durable" here describes caching,
not a disk grant. No trust store/grant/prompt response is authored. Actual writes
must still prove that no persistent trust/config projection was created. The
PluginInfo trusted boolean is deprecated and is not workspace-trust qualification.

## Scope, fixture and exact privilege boundary

Genuine GitHub-hosted Linux x64 with existing disposable opt-in only. Preserve
source/tree/run/attempt, original nonroot UID/GID and actual immutable pins.
Fresh private HOME/XDG/TMP; real Git User-cwd/A/B. Same plugin name in native
User/A/B has three physical roots, plus distinct foreign plugin names in each.
Only manifests and inert SKILL.md files; no copied real user auth/history/model
state, inherited provider keys/proxies, client imports or trust files.

Exactly THREE fixed scope invocations, each with fresh collector/native process
and dedicated strict cgroup, use the SAME physical User/A/B fixtures. Hold one
original content/identity baseline. AFTER EACH strict return, take authoritative
parent post-ledgers and validate against that original baseline BEFORE nextscope.
Child snapshots/group exit cannot establish post-state. Never renew the baseline.

Reuse Qoder's fixed launch pattern and isolation semantics:
`scripts/research-qoder-populated.py:wrapper_command,isolation_facts`, not the
Qoder-specific callable unchanged. The reviewed sealed Go controller runs as root
ONLY for fixed containment/launch. `/usr/bin/unshare --net -- /usr/bin/setpriv`
drops to original UID/GID, clear groups, zero inherited/ambient/bounding caps,
NNP1 BEFORE unprivileged collector/publisher code. Fixed `/usr/bin/env -i` and
`/usr/bin/python3 -I` are allowed verified system operands; arbitrary caller argv,
env, executable or outside-root paths are forbidden. No shell, generic sudo
helper, interface/routing change, policy load/sysctl/mount/setuid workaround.
Proof must show distinct namespace, only lo, original real/effective UID/GID,
groups=[], all five capability sets zero and NNP1. Collector/native share this
inherited namespace/privilege boundary; it is not a hostile-process sandbox.

## Strict existing process and sealed-directory authority

Dedicated parent-module `research/grokacpinventory` composes existing
`adapters/process.OS.DuplexCapability` and RunDuplexWithPlannedShutdown in
`install/integrationctl/adapters/process/osrunner.go:155-187`. Existing Linux
osrunner_tree_linux.go owns atomic CLONE_INTO_CGROUP, pidfd/proc identity,
cgroup.kill/events/empty proof and sole reap. No shared runner modification, new
supervisor, bwrap/PID-namespace/timeout framework or VS Code import/dependency.
Require capability before vendor spawn; unavailable cgroup prerequisite refuses,
without enabling delegation or changing host settings.

The WHOLE collector and every native fork belong to the strict cgroup. Sealed
source/runtime copies are held and verified before launch and after return.
Every sealed cleanup, including prepare failure, requires the held original
root directory's device/inode/owner/mode and mount identity; substitution leaves
it untouched. Parent EOF, SIGTERM/SIGINT and the original deadline cancel strict
supervision. Any error conservatively withholds raw publication and recursive
fixture deletion; no leader exit, group absence or EOF invents tree cleanup.

Root helper records actual held canonical system executable paths+SHA256 for
unshare/setpriv/env/python3. No claim equates stock path to immutable identity.
The original-host monotonic clock is shared by Python monotonic_ns and Go
CLOCK_MONOTONIC. One scope has original30s INCLUDING help, with explicit5s cleanup
reserve. Outer acquisition/build/collection deadline150s is never renewed.

## Frozen closed wire

Shared input `/tmp/uap-grok-acp-input-<nonce>` fixed leaves helper,grok,collector.py,
request.json; dirs home,user,a,b,tmp,xdg-config,xdg-data,xdg-cache,xdg-state. Scope
selects user/a/b. Frontend changes ONLY known owned request.json between scopes,
after strict return+postledger, and records that control mutation explicitly.

Frontend request exact fields: schema_version1, source_commit, source_tree,
uid,gid,nonce,scope,start_monotonic_ns,helper_sha256. Fixed helper argv:
`--input-root <nonce-derivedroot>`. Root checks genuine SUDO_UID/GID and compiled
sourceCommit/sourceTree/collectorSHA; native binary has the constant pin above.
Root creates an independent root-owned sealed input directory. Collector request
is authored there by Go, not consumed as arbitrary user code: schema_version,
source_commit,source_tree,uid,gid,nonce,scope,input_root,parent_network_namespace,
start_monotonic_ns. Collector/native paths derive from sealed __file__.

Collector result exact fields: schema_version,source_commit,source_tree,scope,
nonce,semantic_complete,reason,isolation,commands,rpc; <=2MiB JSON+ONE LF.
Reason enum: complete/protocol-refused/help-refused/isolation-refused/collector-refused.
Commands max5 fixed order version/help/agent-help/stdio-help/acp, each label,argv,
exit,error,merged_base64,merged_bytes,merged_sha256. Error is null or deadline/
stream-bound/process-refused/protocol-refused. Aggregate ALL original command
streams <=1MiB; rpc<=128KiB with initialize/plugins/notifications only. Each
preflight has its OWN actual process argv/exit/stream hash, never synthetic merging.

Every native leader and its full pipe stream must settle before result emission.
On semantic error, kill/reap/drain still requires actual pipe EOF under original
deadline. Overflow/incomplete wait/drain emits NO result, not clipped raw proof.
A command_in_flight fence is set BEFORE EVERY command and cleared ONLY normal
return; semanticexcept rethrows if still in flight. Fully drained negative command
records may be retained. Collector flushes ONE JSON+LF, closes stdout/stderr,
then waits alive for strict parent shutdown. Go reads complete stream to EOF and
rejects any trailing byte/frame/whitespace before callback nil and planned stop.

Helper receipt exact fields: schema_version,source_commit,source_tree,scope,
nonce,cleanup_complete,sealed_cleanup_complete,system_executables,result. Both
cleanup booleans and helper exit0 are mandatory before frontend uses raw bytes or
takes post-ledgers. system_executables exactly unshare/setpriv/env/python3, each
path+sha256 from held FD; Ubuntu24 canonical paths /usr/bin/name, python3.12.
Frontend control stdin remains open; EOF cancels root helper. Do NOT use
native_client_research.run_probe stdinDEVNULL for this helper. Its groupkill is
not strict tree authority. Semantic failure stays failure despite clean tree stop.

## Native help and exactly two ACP frames

No retained actual1.0.70 help yet proves the route. INSIDE each strict cgroup and
original30s budget capture --version, --help, agent --help, agent stdio --help,
<=5s each. Require exact1.0.70/f50904eaee6c and real help advertising agent/stdio/
--no-leader. Only then fixed `[binary,agent,--no-leader,stdio]`. No fallback,
reauth/always-approve/yolo/plugin-dir/trust flag or model configuration.

Send id1 initialize: integer protocolVersion1, clientCapabilities{}, fixed
clientInfo. After matching valid result send id2 `_x.ai/plugins/list`, params
sessionId=uap-admission-<nonce>-<scope> which was never created. No MCP initialized
notification, authenticate/session/new/load/prompt/action/permission/client writes.
Wire extensions use underscore; internal handler names omit it. Initialize version
is result._meta.agentVersion, NOT invented agentInfo. Bound actual source-defined
capabilities, auth-method declarations, modelState/publiccommands and ephemeral
CI hostname/agentInstanceId; these are not auth/inference proof or private profiles.

Only unsolicited frame admitted, at most ONCE/process before either response:
`{"jsonrpc":"2.0","method":"_x.ai/mcp/servers_updated","params":{"mcpServers":[]}}`.
No id/extra fields. Source McpServersUpdated uses camelCase, empty build_mcp_catalog
has no managed catalog. Nonempty/duplicate/unknown notification or inbound request
refuses. No broad notification ignoring or unreviewed retry. Inputframe4096bytes,
outputframe64KiB, <=64frames, UTF8/unique keys/finite numbers/depth16/items512,
exact numeric IDs/result-or-error. Listing must contain stable full ID, canonical
physical root, exact scope and boolean state; reject collisions/shadowing/siblings.
FIRST actual schema capture remains unqualified even if collection completes.

## Effects, raw evidence and recovery

Reuse metadata_inventory from scripts/devin_structured_inventory_admission.py,
physical/content snapshot+grok_profile_bootstrap from client_automation_native_probe,
and descriptor-safe archive helpers in native_client_project_artifacts.py.
Bound entire owned outer entry set4096/depth32; each ordinary file/evidence1MiB,
aggregate16MiB. Verify already-admitted large executable hashes separately.
Reject aliases/specials/replacement/outside paths/source or sibling changes.
Only already measured1.0.70 docs/user-guide named Markdown and unified log bootstrap
allowance can be reused; preserve all original bytes/physical identity. Any other
new config/trust/provider state is unadmitted. Unknown writes are recorded then
fail, never restored or adopted to disguise them.

Closed artifacts: admission.json/acquisition.json, three scope RPC and ledger
JSONs, up to15 scope-command merged logs. Summary contains isolation/system/cleanup
receipts and stream lengths+SHA joins. Task fixture data plus bounded public native
capabilities/ephemeral CI identity only; no credentials or arbitrary host dumps.
Use private pending ZIP then atomic upload rename; late failure never leaves a
partial canonical upload. Unknown cleanup emits only a fixed failure, no raw or
recursive deletion. All initial qualifications FALSE: inventory_admitted,
native_enablement,workspace_trust,native_mcp_call,provider_authentication,
real_model_inference,production_project_activation. A diagnostic is not product PASS.

Enablement remains separate: enable adds enabled/removes disabled with two locks;
disable adds disabled, so it cannot restore prior absence from BOTH lists. No
enablement, rollback mutation or production API is authorized here.

## Files, review and estimate

Exactly seven paths: this plan; scripts/grok_acp_inventory_admission.py;
scripts/test_grok_acp_inventory_admission.py; existing client-automation-native.yml;
parent research/grokacpinventory/{main_linux.go,files_linux.go,main_linux_test.go}.
Root-approved cap2,100 runtime/test/CI lines excluding plan, 4-6h composition/review,
no queue estimate. Extra size funds sealed directory/mount cleanup authority,
sealed copied-input pins, SIGTERM cleanup and actual EOF/capture-fence negatives;
no additional product feature, dependency, generic framework or excluded-client edit.

Review1 root+independent plan GO recorded before code; review2 coherent source,
fixed privileged launch/cleanup/effects/closed artifacts before publication;
review3 original genuine hosted bytes/hash/CRC/source/transcript/ledger/tree proof.
Inert contracts cover isolation refusal, pre-cancel, command failures at each
position, actual no-EOF/flood drain, retained fully-drained negative, unknown wire,
wrong IDs/version/source/root, unknown trust/write, directory replacement, cleanup
refusal and late staging failure. Go focused tests+lint and coherent applicable
ordinary gates required. No vendor or privileged execution locally. No plan-only
publish or manual queue cancellation. After environment loss all tests rerun from
restored actual files; no failed patch is treated as applied.

## Exact primary source references

All Grok URLs refer to public snapshot2bdd, not shipping-source provenance:

- https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-shell/src/extensions/plugins.rs (blob5a57cd7ddf7cbf345ad2953dbc03a60c8ad8b4a5)
- https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-shell/src/agent/app.rs (blobdcacea522f0e3772962192bf3165cdb49b5f457c)
- https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-shell/src/agent/mvp_agent/acp_agent.rs (blobd689b2a41949cbc5fa7f35c1c94eb614c8b3fe42)
- https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-shell/src/agent/mvp_agent/agent_ops.rs (blobfa77f1ec809cf3c944ba91895a73ba422a24054a)
- https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-shell/src/agent/folder_trust.rs (blob8179f315d1d39a347bb98b70e1463aa4a3627929)
- https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-shell/src/extensions/mcp.rs (blob076614ec6652b45bcc9b1a0b9cd9635e45d752b9)
- https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-pager/docs/user-guide/15-agent-mode.md
- https://agentclientprotocol.com/protocol/v1/initialization
- https://agentclientprotocol.com/protocol/v1/extensibility

### Reviewed artifact publication correction

Root approved cap2,100 code/test/CI lines, still exactly seven paths, to address
independent review of original output ownership and stale upload permission.
Frontend holds the original output directory from creation. One inline publisher
uses that FD, descriptor-anchored staging, existing write_zip and Linux atomic
renameat2 NOREPLACE; it never retries after a publication exception. Only complete
publication returns source/nonce/original directory IDs/final ZIP identity/bytes/SHA
in ONE closed parent stdout frame. A late exception yields no upload permission.

The workflow parent uses existing run_probe memory_only to establish bounded
frontend completion. It accepts the exact single frame only after error=None and
normal reaped return, validates source and original output/ZIP via NOFOLLOW held
identities and SHA, then writes/verifies a fresh private admitted copy. The upload
action uses ONLY this step output and is skipped when permission is absent; it
never scans an old output prefix on always(). No foreign outer replacement is
quarantined or deleted. These are fixed Grok-only functions, not a generic exporter.

Workflow upload additionally requires the parent admission step's SUCCESS
conclusion, so a partial GITHUB_OUTPUT write is inert. A fully validated negative
frontend exit1 may stage evidence through successful artifact admission, but a
separate observation-status step fails the job. It cannot become native PASS.
Any unexpected exit, incomplete parent capture, missing receipt or failed second
output write leaves no upload permission.

Root approved the final2,100 physical code/test/CI-line cap (actual2,062 excluding
plan) for two publication-review corrections: workflow independently derives exact
HEAD^{tree}, and the ZIP descriptor/full pre-open metadata stays held across
read/hash/rename/admitted-copy with final root/upload/ZIP path+FD rechecks. Same-byte
archive and upload-directory replacement negatives preserve the foreign sentinel
and refuse permission. No generic abstraction or new target/operation was added.


## First hosted entrypoint correction

Exact38472 native job131779695630 passed the inert contracts but produced no
admitted stdout handoff; its log reports the parent's empty-document refusal,
not the child's underlying stderr. Independent source review identified a
deterministic missing required target argument in require_hosted. The explicit
linux-amd64 argument now matches the genuine dedicated lane. An inert test calls
the real entrypoint with a required-argument guard and proves no output directory
or acquisition occurs before that authority check. It does not spoof host facts.

Nested publication identity dictionaries now require exact keys and literal
integers; booleans cannot pass integer equality. Archive digest spelling is closed.
The previous continuously held file/directory authority and cleanup fences remain.
Root approves the additional22 targeted guard/test lines beyond the2100 estimate;
18 Python and nine helper Go contracts pass. Fresh hosted metadata, then a separate
exact-selector enable/recovery admission, remain required before production work.
