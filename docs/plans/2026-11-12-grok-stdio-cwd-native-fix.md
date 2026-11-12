# Preserve the portable Grok stdio working-directory contract

Status: implemented locally; independent review and actual final-source CI are
required before acceptance. Production baseline is main
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
No new generic helper, schema, state engine, resolver or registry is introduced.
The owner-authorized scoped declarative/read-only corrections below are needed
to reach the existing public lifecycle paths; a new adapter hook alone was
proven insufficient.

## Exact bounded file list and estimates

The original eight-path plan expanded after independent review and actual
composed regressions proved two existing bypasses. The owner explicitly
accepted minimal generic use of the existing declarative traits; no client-ID
branch or alternate installation engine is added.

| File | Final bounded purpose |
| --- | --- |
| install/integrationctl/agentplugins/clients/grok/project.go | Project-only trusted delivery before any projection writes; preserve User |
| install/integrationctl/agentplugins/clients/grok/project_stdio.go (new) | Encoded launcher projection and receipt-bound read-only validation |
| install/integrationctl/agentplugins/clients/grok/project_stdio_test.go (new) | Three cwd anchors, opaque argv, legacy/wrong authorities/helper refusals |
| install/integrationctl/agentplugins/domain/project_policy.go | Existing scoped Project flags for helper readiness and active projection |
| install/integrationctl/agentplugins/domain/clients_catalog.go | Declare those flags for Grok Project only |
| install/integrationctl/agentplugins/domain/project_grok_policy_test.go (new) | Prepared/no-effects/User and other prepared-client policy invariants |
| install/integrationctl/agentplugins/usecase/prepared_project_verifier.go | Honor required active projection after unchanged fail-closed legacy branch |
| install/integrationctl/agentplugins/usecase/prepared_project_verifier_test.go | Grok/Pi/other prepared and mismatched legacy classification |
| install/integrationctl/agentplugins/usecase/observe_owned_binding.go | Reuse guarded projection-only observation separately from activation proof |
| install/integrationctl/agentplugins/usecase/projection_refresh.go | Existing unsaved update refresh includes required active projection |
| install/integrationctl/agentplugins/usecase/refresh_projection.go | Command-skill equality cannot bypass required native projection refresh |
| install/integrationctl/agentplugins/installer/verify_prepared.go | Sealed-handle projection-only readback; shared state/profile/cancellation guards |
| install/integrationctl/agentplugins/installer/verify_prepared_projection_test.go (new) | Actual common sealed guard under cancellation/state/profile races |
| cli/internal/agentpluginscli/deps_sync.go | Inspect only declaratively required unchanged projections before no-op |
| cli/cmd/agentplugins/project_grok_e2e_test.go | Actual composed saved/unsaved old-direct migration, pin rules and readiness |
| scripts/grok_project_protocol_client.py | Admit exact emitted helper/config/hash/argv and explicit caller cwd |
| scripts/grok_project_protocol_e2e.py | Raw config before attempt; actual default/plugin/data cwd update/restart stages |
| scripts/test_grok_project_protocol.py | Strict helper/data/cwd/source-hash rejection contracts |
| docs/plans/2026-11-12-grok-stdio-cwd-native-fix.md | This plan, findings and measured validation |

Current main Grok adapter blobs matched the failed source. PR #878 at
9bbf942d7532805a1ef98f71b1d2694804ec84d3 changed only two automation
workflows, its six-client plan and two Python probes; no adapter overlap.

Complexity 6/10 after the public-path findings, approximately 360–430 added
production LOC and 500–650 test/helper LOC. Reliability confidence 8/10 for
reused launcher/path/state authorities; 6/10 until actual pinned Grok executes
the final installed projection on hosted Linux. Production functions retain
existing size/complexity limits. No lint exemption, architecture budget,
oracle, trust or safety refusal is weakened.

### Public-path findings and authorized correction

Grok's prepared Project originally had neither helper-readiness nor active
projection traits. The generic prepared-file shortcut therefore bypassed a
new adapter hook. The scoped policy now declares both while retaining
LifecyclePrepared, no TracksNativeEffects, and unchanged User traits. The
legacy Qoder malformed-receipt classifier remains before the new flag check.

A genuine old direct tree seeded through an explicit predecessor Projector
seam has its original valid artifact receipt. An unsaved repeated add must
refuse it with ErrProjectionRefreshRequired. Existing explicit update can
refresh that tree at the same source revision through its existing transaction
and data authorities. A command-skill equality shortcut must not bypass this
native projection check. Repair continues to preserve the recorded projection.

Saved add/sync previously returned unchanged solely from source digest and
lifecycle facts, without invoking projection readback. The new read-only
unchanged preflight is gated only by RequiresActiveNativeProjection. It uses
the sealed repair handle and exact source/receipt/profile state, asserts
projection completeness, and never claims activation. Existing saved pin
policy remains intact: a stale projection gets ErrProjectionPinChangeRequired;
a source pin change applies the current projection. No silent saved refresh
or receipt mutation is authorized.

### Local validation so far

With official Go1.26.11, the three composed ZIP tests pass: same-revision
unsaved repeat refusal and explicit update refresh; saved old-pin refusal
with package/state unchanged, new-pin update, then current-pin repeat with
package/state unchanged; missing helper planning refusal before A/B/User
effects. Adapter stdio (including bare and bundled commands), scoped policy and
existing/new prepared classifier tests pass. All five existing Pi facade
tests and the nine-case sealed-handle cancellation/state/profile/invalid-handle
regression pass without skips. Nineteen inert Python contracts pass.
Complete make lint with LINT_BASE=HEAD (the exact ba64 local baseline) passes
all four whole-file gates, four changed-line passes and unchanged exclusions.
These are local contract observations, not hosted/native qualification.

Full make test-core actually failed in this container, including unavailable
/proc containment and malformed-or-duplicate Codex environment refusals.
The latter Codex refresh callback failure was independently reproduced on
unchanged exact ba64 baseline source. Preserve the actual failed core log;
this is not a full-core pass or a waiver. The CLI phase is not credited when
make stopped after the core failure. Actual hosted full core, cross-OS,
rollback and genuine native results remain mandatory.

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
That follow-up belongs to PR #883's owner and the root integrator; this
workstream does not overwrite its branch or concurrent commits. The tiny
integration delta must capture inherited original config before admission,
use inherited expected_runtime_cwd(active,data), pass source-built App hash
and expected cwd to installed_server, and assert that explicit expected cwd
in native tool facts. Both old direct-helper
and new native-call lanes must pass on their actual final combined source.
Only guarded final-source merge and genuine natural postmerge execution can
complete acceptance. Source GO or a plan is not executed product acceptance.

## Follow-up independent review before hosted acceptance

Review of the exact `30389a7757c0e1b81d6a7c3939270e6ffaaed549`
tree found two bounded corrections. Helper digest readback now reuses Grok's
existing anchored `openProjectHandle(path, false)`: its Unix nonblocking,
no-follow walk and Windows no-reparse walk prevent a regular-file-to-FIFO or
ancestor-alias replacement from hanging or following another file at open.
The existing handle race regressions remain the authority; no new opener,
test-only production hook or weakened path check is introduced.

The existing composed MCP-only retained-data test still expected direct `go`
execution. It now admits only the installed managed launcher, exact
package/data/default-cwd authority, explicit encoded-argv separator, original
decoded `go version` child and retained `DATA` expansion. Its existing
installation, removal and foreign-state checks remain intact.

Independent source review and seven focused Go test groups (including thirty
subcases) passed, covering anchored replacement refusals, cwd/opaque arguments,
stale projections, helper readiness, scoped policy and sealed observation
guards. Full `make lint LINT_BASE=HEAD` passed both passes for all four modules
with the existing baseline. The local composed Git test cannot reach its changed
assertion because this host refuses private Git process containment. Full local
`make test-core` has likewise reported missing `/proc/.../children` prerequisites;
neither is a full local E2E PASS. Current-source hosted Required, cross-platform
contracts and combined #883 genuine native lifecycle remain mandatory.

## Primary sources and retained originals

- https://agent-plugins.org/specification section 7.2.1 and section 11.1
- https://agent-plugins.org/plugin-authors/mcp-servers
- https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-config/src/mcp_servers/server_details.rs
- https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-mcp/src/servers.rs
- Original run https://github.com/777genius/universal-agent-plugins/actions/runs/43259198583
- Original local retention: grok-noauth-call/.review-hosted/first-original.zip
  and first-evidence.json; parent independently verified API/SHA/CRC.
