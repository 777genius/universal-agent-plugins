# Preserve the portable Grok stdio working-directory contract

Status: plan before implementation. Production baseline is main
`ba64b3feda24f160339fde484ed365a7080c3c75`. This is a separate Grok-only
fix workstream. Do not edit, overwrite or discard concurrent PR #883 commits.
No credentials, vendor model, login, trust automation, public capability
promotion, generated files or frozen VS Code changes are authorized here.

## Concrete defect and source of authority

Actual pinned Grok 1.0.70 run 43259198583, attempt 1, on source
`c5f159aa5f9405596d4d6964b33653f2ffabcd45`, tree
`d4dc05fd6a13df039f7709fede6bccac3cfe9a40`, performed real same-session MCP
initialize, tools/list and tools/call. The exact original tool facts reported
Project A as cwd while PLUGIN_ROOT pointed to the installed package and
PLUGIN_DATA to the correct retained data directory. The provider rejected
the wrong cwd and the actual test failed. Preserve that failure and original
artifact 13961988810; do not classify it as a flake or successful lifecycle.

The source fixture is schema-tagged Agent Plugins 1.0 stdio and omits cwd.
Published normative specification section 7.2.1 requires plugin-root cwd when
omitted; section 11.1 repeats that default. This is a product requirement,
not a direct test client's preference. UAP conformance.ParseCWDPath(empty)
selects the plugin anchor; shared.ResolveStdioPaths/ApplyStdioDataContract
resolve and emit the active-package cwd. Grok.Project currently passes that
entry through MCPDialectGrok without a process launcher.

Official upstream source at 2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8
corroborates the mechanism: mcp_server_config.rs reads cwd;
mcp_servers/server_details.rs discards it while converting stdio to ACP;
xai-grok-mcp/src/servers.rs constructs the child command with args/env and
no current_dir. This upstream commit is not claimed to be the source of
release build f50904eaee6c. The immutable 1.0.70 original is decisive evidence.

The failed artifact did not retain original installed .mcp.json bytes before
the native attempt. Its successful strict installed_server check establishes
that projected cwd equalled active, but does not provide rehashable raw config.
The new evidence must retain that exact config before every attempt, including
failed ones. Never replace it with synthesized probe configuration.

## Smallest production change

For Grok Project stdio only, reuse trusted launcher delivery and existing
managedstdio.EncodedArguments, pathcontract and strict JSON authorities.
After ordinary shared projection, replace each selected stdio command with
the exact trusted launcher under the future owned active package. Bind
plugin/data roots, the resolved portable cwd and its original anchor, and the
original bare/bundled command plus opaque expanded arguments. Remove the
ignored native cwd field. Preserve remote MCP entries and User layout.

Implement the existing ActiveNativeProjector contract to validate active
launcher/root/data/cwd/encoded-child bindings against the exact selected plan
and envelope. Old direct or stale projections must require refresh; read-only
checks must not silently rewrite active bytes. Reuse existing trusted helper
delivery, content hashing, ownership, receipt, rollback and removal behavior.
No generic helper, schema, state engine, resolver or registry change is needed.

## Exact bounded file list and estimates

| File | Intended change | Estimated delta |
| --- | --- | ---: |
| install/integrationctl/agentplugins/clients/grok/project.go | Call Project-only stdio binding and preserve current User path | 10–20 LOC |
| install/integrationctl/agentplugins/clients/grok/project_stdio.go (new) | Trusted launcher projection and active-binding validation through existing interfaces | 160–220 production LOC |
| install/integrationctl/agentplugins/clients/grok/project_stdio_test.go (new) | Projection, stale-authority, path and User noninterference contracts | 220–320 test LOC |
| cli/cmd/agentplugins/project_grok_e2e_test.go | Exercise ordinary composed trusted delivery and exact resulting bindings | 20–40 test LOC |
| scripts/grok_project_protocol_client.py | Strict actual helper/encoded-argv/hash admission; direct caller cwd is explicit; never modify decoded config | 50–80 LOC |
| scripts/grok_project_protocol_e2e.py | Supply source-built helper hash/caller scope; retain original installed config before attempts | 10–30 LOC |
| scripts/test_grok_project_protocol.py | Actual-helper admission and cwd/data/hash/opaque-argument negatives | 70–100 test LOC |
| docs/plans/2026-11-12-grok-stdio-cwd-native-fix.md | This plan, review findings and measured final validation | Documentation |

No other production paths are in scope. If a required fixture correction
outside this list is discovered, report it before expanding the patch.
Current main Grok adapter blobs match the failed source. PR #878 at
9bbf942d7532805a1ef98f71b1d2694804ec84d3 changes only two automation
workflows, its six-client plan and two Python probes; no adapter overlap.

Complexity 4/10, approximately 170–240 production LOC and 360–530 test/helper
LOC. Reliability confidence 8/10 for reused launcher/path authorities; 6/10
until actual pinned Grok executes the final installed projection. New
production functions stay under existing size/complexity limits. No lint
exemption, architectural budget, oracle or safety refusal is weakened.

## Meaningful verification and evidence

- Test omitted/default cwd, explicit plugin-relative cwd and PLUGIN_DATA cwd;
  bundled and bare commands; opaque/empty arguments; mixed remote entries.
- Refuse missing trusted delivery, wrong helper bytes/path, stale plugin/data
  root, wrong anchor/cwd/encoded argv and escaping/symlinked topology. Preserve
  Project B, User and foreign objects, selected receipts and retained data.
- The Python helper must read the actual installed .mcp.json, validate the
  expected launcher hash against the source-built installer and decode the
  private argv transport. It must execute those exact installed command/args
  and env from Project A, without constructing an alternate server document.
  Assert the reported portable cwd, package/data roots, nonce and marker.
- Preserve old protocol lane .github/workflows/grok-project-protocol.yml:
  genuine native doctor initialize/list plus separately labelled direct
  installed-launcher tool calls. Do not relabel direct calls as native calls.
- Run relevant Grok/managedstdio/CLI contracts, make test-core and make lint,
  affected install/rollback/polyglot checks and applicable ordinary CI. Run
  adapter/launcher contracts on Linux/macOS/Windows where available; actual
  vendor qualification remains Linux x64 until another native pin is tested.
- Actual final-source hosted execution must prove default/plugin/data cwd,
  fresh initialize/list/call, exact original config/helper/stream hashes,
  lifecycle update/repair/restart/removal, data retention and foreign isolation.
  Review original artifact API digest/CRC/source bindings before acceptance.

## PR #883 dependency and merge order

Concurrent PR #883 moved through f1f7da85aaa4794f61184e8bbab69722915ac21d
to 473d11f09b1b3027ceaebe3918370a92d2f38699 and changed its expected native
cwd to Project A. Preserve those commits and their provenance. Their raw
native tools-call observation does not establish portable cwd conformance.

After this product fix is reviewed, its owner must reconcile PR #883 with the
new actual helper admission and restore the portable package/data cwd oracle.
That follow-up belongs to PR #883's owner; this workstream does not edit its
branch or new grok_noauth_call_e2e.py/test/workflow files. Both old direct-helper
and new native-call lanes must pass on their actual final combined source.
Only guarded final-source merge and genuine natural postmerge execution can
complete acceptance. Source GO or a plan is not executed product acceptance.

## Primary sources and retained originals

- https://agent-plugins.org/specification section 7.2.1 and section 11.1
- https://agent-plugins.org/plugin-authors/mcp-servers
- https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-config/src/mcp_servers/server_details.rs
- https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-mcp/src/servers.rs
- Original run https://github.com/777genius/universal-agent-plugins/actions/runs/43259198583
- Original local retention: grok-noauth-call/.review-hosted/first-original.zip
  and first-evidence.json; parent independently verified API/SHA/CRC.
