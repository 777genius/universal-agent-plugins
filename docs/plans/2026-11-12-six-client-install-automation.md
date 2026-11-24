# Six-client installation automation — one PR

Date: 2026-11-12. Baseline: `071026c96fe0e937b6e8716037b1b08473e4a5db`.
Status: three design review passes recorded below; native admission and production
implementation are not yet accepted. A source review is not an executed E2E.

## Owner scope

One Draft PR contains the plan, corrections, implementation, genuine E2E,
merge and postmerge checks. Push each coherent commit immediately. Do not
split the clients into separate PRs or publish a release within this request.

**VS Code: excluded completely.** No adapter, config, protocol, dedicated test,
UI behavior, dependency qualification or native run changes.
**ChatGPT Web: excluded completely.** No new command admits it. Do not expand
this into ChatGPT Desktop/Codex, Kimi or other agents. Shared seams must preserve
excluded-target behavior byte-for-byte where possible and semantically always.

Included: Cline, Grok Build, Google Antigravity, Windsurf/Devin, Kiro, Hermes.
Reuse merged Grok #810/#869. Do not modify the branches or evidence of separate
Kiro authentication research #686 or Kimi #877.

## Required outcome

Automate safe, documented client steps after ordinary UAP delivery instead of
asking users to copy environment variables, edit JSON or repeat a verifiable
native action manually. Distinguish installed files, native discovery, enabled
runtime and provider authentication. Prepared remains valid; a badge change is
not implementation. A list/config response is not a successful MCP tool call.

Baseline qualification uses no purchased subscription, provider key or model
request. A native authentication requirement stays a separate external boundary;
never fabricate an account, bypass login or copy a real user's credentials.

## Native admission before mutation

For every new operation record the exact official version/hash, help syntax,
selected original profile, scope, target identity and actual effects. Use fresh
private HOME and independent Git projects A/B. Current documentation alone is
not proof that an older pinned executable implements the same interface.

| Client | Existing delivery | Work to qualify and implement | Boundary |
| --- | --- | --- | --- |
| Cline | User native Skills/MCP; Project Skills and `.uap/clients/cline/mcp.json` | Select the installed Project MCP source for an explicitly requested CLI process and verify effective configuration; eliminate manual environment assembly. | The override selects the whole file; do not merge/copy User servers. IDE and normal unwrapped startup remain separate. |
| Grok | User lifecycle; Project package; real no-model protocol proof in #869 | Explicitly consented exact-owned plugin enablement and native read-back. | Prove stable full identity, not name-only selection. Do not grant workspace trust or override prior disablement. |
| Antigravity | Native User/Project bundle, validation and isolated registration | Register/enable only through a proven scope- and source-specific native interface; verify inventory. | CLI profile and IDE profile may differ. No second copy, global registration or enabled IDE claim from file presence. |
| Windsurf/Devin | Devin User/Project native components; separate Cascade route | Native selected inventory verification and precise remaining actions after delivery. | Qualified Devin is not legacy Windsurf. Preserve coexisting profiles, local overrides and cloud/team policy. |
| Kiro | Automatic native config/Skill mutation; auth-gated runtime | Automatic file/selection verification and genuine read-only observation when supported without new login. | Do not rewrite custom-agent resources or includeMcpJson globally; never borrow #686 authentication evidence. |
| Hermes | User Skills/MCP and Project Skills; Project MCP refused | Qualify an official explicit profile/session route for isolated Project MCP, then integrate it with existing ownership. | No Project-to-User YAML fallback, copied login/history/model settings or claim that plain `hermes` loads a UAP profile. |

If a native action is not qualified, keep that row incomplete and preserve its
previous behavior. Do not replace an absent API with a fake native executable,
a general wrapper platform or a misleading Automatic label.

## Architecture and consent

Reuse the existing registry, plan/refinement, lifecycle, process supervisor,
native-config kernel, receipts, locks, transactions and rollback. Client logic
stays in `clients/<id>`; generic providers do not switch on client IDs.

1. Prefer existing interfaces. Add one narrow optional interface only after an
   actual native action needs it. No framework, service locator, parallel
   installer/resolver, state store, feature flag or duplicate capability matrix.
2. Any selected-session operation is explicit: no interactive process on normal
   install and no alteration of the parent shell. First verify the existing
   runner's stdin/stdout, cancellation and child-lifetime contract. Captured
   noninteractive ProcessRunner is not automatically an interactive TTY runner.
   Do not hide this mismatch behind `sh -c`, a detached child or a global shim.
3. Show exact native action, client, scope, original profile, project and owned
   package. Obtain action-specific consent. Existing `--yes`,
   `--activation-complete` and `--auth-complete` are not new enable/trust consent.
4. Revalidate exact receipt, selected executable/version, full native identity,
   source path and absence of shadowing under the current mutation lock. Stale,
   foreign, ambiguous, unsupported or disabled-by-user targets refuse before
   mutation. A plain name cannot select across User and Project A/B.
5. Never auto-grant trust, change enterprise policy, overwrite disablement,
   invoke a model prompt or use all-server startup as a verification shortcut.
   Probe only a reviewed owned target through a selective native interface.
6. Before any native command that writes beyond the owned leaf, require a
   verified exact recovery path using the existing kernel. **If that recovery
   cannot be verified, refuse before mutation and leave the action incomplete.**
   Never adopt a whole foreign file, use broad force flags, or restore a stale
   full-file snapshot over a concurrent user edit. Rollback is ownership-aware.
7. No persisted JSON/lock/wire/schema migration. Use existing truthful status
   facts. A failed native follow-up leaves delivery resumable, not falsely Active.
   Existing cancellation/rollback rules remain; no widened security exemptions.

## Implementation order within this branch

A. Qualify native commands and selected-session mechanisms using existing
   pinned acquisition and bounded subprocess helpers. Keep research evidence
   explicitly separate from actual UAP-product acceptance.
B. Cline selected-process path, with no User/IDE accidental rewrite.
C. Grok exact enable/verify, reusing installed-config evidence from #869.
D. Antigravity exact-scope registration/enablement and selected inventory.
E. Devin observation and legacy coexistence, then Kiro selection/file checks.
F. Hermes Project MCP only after a genuine isolated route is established.

Reuse current tests/collectors instead of writing a second installer or broad
new test platform. Consolidate actual corrections before another native CI
push; do not cancel queued jobs for cosmetic edits. No separate client PRs,
release, registry-signature changes or public capability promotion.

## Required acceptance

Every changed client has User and Project A delivery with independent Project B
and original User sentinels. Product tests use the ordinary public UAP CLI.

- Dry-run, missing/declined action consent: no native process or installed effect.
- Installation explicitly confirmed (including `--yes`) while separate native enablement consent is absent or declined: installation stays valid and resumable, with no enablement action, new trust, or unrelated profile change.
- Add/repeat/update/missing-package repair/cold locked restore/remove; same
  installation/source/binding/data authority across lifecycle and retained data.
- Exact-owned native action and read-back; same-name User/A/B collision, stale
  source, unknown version, foreign path, symlink, config shadowing and deliberate
  disablement negatives. All refuse without unowned changes.
- Trust/login remain unchanged. Partial user edits are not overwritten to make
  repair pass. Failure, cancellation, timeout and interrupted recovery preserve
  ownership; no surviving detached native process.
- Session selection changes neither parent environment nor unrelated profiles;
  startup behavior without the explicit selection is not misrepresented.
- Real local stdio/HTTP initialize/list/call only where admitted. Distinguish
  direct installed-config calls, native client calls and model authentication.

Record exact source SHA/tree, genuine executable identity, bounded original
stdout/stderr/exit, effect and isolation assertions, nonce/session correlations
and retained receipts. Check original artifact digest/CRC. Mocked, queued,
skipped, cancelled or research-only runs are not successful native acceptance.

Run applicable Linux/macOS/Windows coverage where the selected host is available,
without inventing unsupported-platform passes. Reuse affected lanes; a new
path-scoped lane is permitted only for an actual new behavior. For Go changes,
run make lint, test-core, affected rollback/install/polyglot checks and full
Required/security checks. Keep unrelated clients and their contracts intact.

Before merge inspect the final file allowlist, excluded boundaries, every
applicable exact-head check and actionable review. Use SHA-guarded merge, then
verify actual postmerge native and shared checks. Do not call all six complete
when a required operation is still absent or an external limit is only described.

## Size and uncertainty

| Client | Production changed/new LOC | Test/E2E LOC | Complexity / confidence |
| --- | ---: | ---: | --- |
| Cline | 120–240 | 220–420 | 3/10 / 6/10 |
| Grok | 100–220 | 180–350 | 4/10 / 6/10 |
| Antigravity | 140–300 | 200–380 | 4/10 / 5/10 |
| Devin / legacy preservation | 150–350 | 250–500 | 5/10 / 6/10 |
| Kiro | 120–300 | 220–450 | 6/10 / 5/10 |
| Hermes Project MCP | 350–900+ | 400–850+ | 8/10 / 3/10 |

Estimates exclude vendor-side work and must be revised after native admission.
Do not compensate for a missing safe API with a large new framework.

## Three design reviews and concrete corrections

**R1 — native API/scope feasibility (source and retained-evidence review).**
Grok's documented full plugin identity must be checked against pinned 1.0.70;
that binary's `inspect` does not expose a stable ID, and its build hash is not
an upstream Git reference. Project enable cannot be implemented as name-only
`enable`. Cline's whole-file override applies to one process, not its IDE or
later ordinary startup. Antigravity's CLI and direct User/Project bundle paths
must be compared before registration; native install may duplicate content.
Hermes documents User MCP authority; an isolated profile is a distinct session
route, not native default Project support. **Correction:** retain these as
runtime-admission gates, not assumed feasible implementations.

**R2 — architecture/minimality.** Existing lifecycle/preflight/verifier interfaces
already cover configuration delivery and observation. The available captured
ProcessRunner has no interactive stdin/TTY contract. **Correction:** do not add
a generic action bus or session framework; first establish whether a narrow
explicit selected invocation can reuse a supported supervisor. No command or
public status is added merely to wrap a manual instruction.

**R3 — consent, recovery and falsifiability.** Name-only enablement can affect a
same-name foreign plugin. Native doctor may start every server; a probe is not
read-only merely because its name suggests diagnostics. A whole-file rollback
can overwrite concurrent foreign edits. **Correction:** require exact identity,
selective consent, no trust writes, verified ownership-aware recovery before
mutation, and collision/no-effect tests. CodeRabbit finding 4550436510 prompted
the explicit hard refusal in architecture item 6; it is accepted, not waived.

These three passes are design self-review, not independent executed proofs.
CodeRabbit independently reviewed the plan; production and native acceptance
remain pending. Subsequent findings are recorded here with their actual source.

## Primary references

- [Grok plugin commands and full identity](https://github.com/xai-org/grok-build/blob/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8/crates/codegen/xai-grok-pager/docs/user-guide/09-plugins.md)
- [Antigravity plugins](https://antigravity.google/docs/plugins)
- [Cline selected configuration](https://github.com/cline/cline/blob/main/docs/mcp/mcp-overview.mdx)
- [Hermes MCP](https://hermes-agent.nousresearch.com/docs/user-guide/features/mcp)
- [Kiro CLI](https://kiro.dev/docs/cli/reference/cli-commands/)
- Existing `clients/lifecycle.go`, `ports/interfaces_runtime.go` and native E2E
  helpers are the integration seams, not permission to bypass their boundaries.

## 2026-11-14 — dependency recovery fixture after parent-authority validation

Coverage run `43542221892`, job `132285145503`, and Core run `43542221685`,
job `132285147039`, both failed `TestDependencyEnvironmentCLIEndToEnd` at its
injected `Store.Save`: `standalone publication parent authority is required`.
Both checked out PR merge `afd3bfef5159bb3d85b5b451d9f066ff0d3bd443` for branch
`539e6a54a05cc297370f388116e681fbcdafa443` and base
`ee6bbb04c3bf1cf2981764e5093d803dc0b57aec`. This was a test-fixture failure before
the requested recovery, not an observed production publication failure.

The fixture had copied an ordinary binding replacement into standalone receipts.
Fresh standalone swaps now require original parent authority and cannot inherit
the replacement's prior digest or physical proof. The corrected foreign fixture
uses the existing pending standalone `directory_remove` role produced by grouped
removal after its binding disappears, matching the existing lifecycle recovery
fixture. The owned pending receipt remains unchanged. The mixed recovery must
reach the typed environment-authority refusal; state-byte preservation, repair
refusal, and subsequent owned-only recovery assertions remain in place. No
production guard, receipt schema, native admission, or public capability changes.

Local Go 1.26.11 validation compiled and started the focused E2E, but its first
`deps lock --dry-run` refused at Git `phase=init`, `class=ordinary_unsupported`,
`exit_status=unavailable`, before the changed fixture. The command exited 1 after
39.943 seconds; the underlying errno was not reported. This is not a local E2E
pass. `gofmt` and `git diff --check` completed; fresh hosted Core and Coverage
must execute the complete corrected recovery scenario. The separate generator
assertions and generated landing mapping remain outside this fixture correction.

### Composed refusal and cross-platform fixture checks

The reviewed integration keeps the corrected foreign standalone removal above.
Both public recover and repair now require the exact environment-authority
refusal and preserve the complete captured manifest, lock, state, managed/store
files, native registration map and native call count. The original owned receipt
is then recovered after only the injected foreign receipt is removed. This
supersedes the alternative draft that appended a foreign receipt to the binding;
it does not change production recovery authority.

Original transaction jobs on the same 539 source exposed two fixture boundaries:
raw temporary-directory aliases disagreed with the canonical captured authority,
and a production-held Windows base directory refused rename with sharing error
32. The common fixture now canonicalizes its original root before creating any
paths or authority. Only that exact Windows rename denial is an accepted
protection outcome, with original held/named identities, metadata, contents,
staging/marker bytes and an absent move destination checked. Actual POSIX and
child-substitution refusal paths remain intact; no handle is closed or rebound.
The identical real-symlink temporary-parent replay changed from two causal
failures to 18 affected tests passing in 0.490 seconds. Independent source review
accepted the three existing test-file changes. Genuine final-source Windows and
macOS execution remains required; neither local source review nor this Linux
replay replaces the hosted matrix or the native client evidence.

The completed e5 Required job 132297760690 also exposed a stale Cline cycle
configuration oracle. The real Project stager emits its owned managed-stdio
launcher and private protocol prefix; the cycle proof still expected the raw
authored child executable. Its installed-file proof already required the owned
launcher and metadata. The configuration proof now independently pins that exact
launcher, root/data/cwd/plugin fields and original child command/arguments, while
keeping the closed environment and source/ownership checks. Six controls reject
an unwrapped transport, a foreign launcher, wrong root/data, an altered child
command and an extra argument. This corrects test expectations for the current
transport; it adds no production command or native qualification.

## 2026-11-14 — independent retained native evidence review

This review records original executions with their own source identities. It
does not qualify subsequent changes or replace the final-head merge gates.

### Cline selected sessions and cold agreement restore

[Run 43588580177](https://github.com/777genius/universal-agent-plugins/actions/runs/43588580177)
executed source `1412f416b9bf70b227d8f517e1881075f2b05a38`, tree
`a42b970ebaf15e73a6dcbbd57f9c4c47ea1d4f57`. All three original artifact bytes were
downloaded and checked against GitHub's digest, ZIP CRC and exact single-member
closure. Each original contains the same 30-command lifecycle, three warm
inspection stages, and separate agreement-only cold restore/repeat.

| Host | Original artifact | Outer SHA-256 |
| --- | ---: | --- |
| Linux x86-64 | 14112068950 | `3a5ef19bd7ac8485db3584613b88726bb9a856d14913e7b6774a3e3ce8f50f08` |
| macOS ARM64 | 14113185358 | `6e7a6aaf5b9722614855517df6e925b2380330dd0db0f688de88d6c3ae9841fb` |
| Windows AMD64 | 14111523757 | `d03909ec8c4afab92239fca18bd91704d06d11aef0edfe6eefa6a3d5160de6f8` |

The review independently joined all 60 original stream encodings per host,
expected positive/negative exits, source/tree, six runtime pins and each command's
positive cleanup receipt. It also rehashed the event stream and passed the
existing consumer's nonce/PID/session/order checks: five distinct installed
transport sessions per host, each with initialize, tools/list and tools/call.
The real selected Cline process supplies configuration inspection; the five
protocol calls are direct installed-transport calls. All originals explicitly
retain `native_cline_tools_call=false`, no authentication and no model invocation.

### Hermes existing User and Project Skills lifecycle

[Run 43591958231](https://github.com/777genius/universal-agent-plugins/actions/runs/43591958231)
passed Linux, macOS and Windows on head `f665e332fa944e46e2942119036a076f0381268a`.
Its actual checkout `04da68fbf9cfe6435c3947d98fd52ae2c9853672` has the identical
tree `442adc55e588a6ac4b5b8c94ac85d13a1765f04b`, independently confirmed through
the original commit object. The pinned Hermes source remains
`2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19`.

| Host | Original artifact | Outer SHA-256 |
| --- | ---: | --- |
| Linux x86-64 | 14113275161 | `956861f941de9ab0b18122307fe94a2272abdba3a3ab69e97024bec7d41f6856` |
| macOS ARM64 | 14113092711 | `e2896f1fa25e367c22e1b7afac77e58a0862fc8354a41678b5822a20da1bae43` |
| Windows AMD64 | 14112668635 | `3862ef602953ce3eddde079e3610e1bf52f955df969c2ef3da66952e0fa849cd` |

All three downloaded original archives matched the API digest and passed CRC.
Their summaries retain lifecycle exit zero and no timeout. The observed native
MCP sequence is four initialize/initialized/tools-list exchanges; `tools_call`
and `model_turn` are false. Separate no-auth tool-call and isolated Project MCP
lanes still require their own original evidence. The existing Project MCP
refusal in this lane is preserved and is not a positive Project MCP result.

### Incoming corrections executed locally

The independently published `d9008f2c8bc19ee0b8a6cb62a6d50e25bf76ef91` was
reconstructed with its exact original tree and commit SHA before running its
unchanged tests. The environment suite passed all 217 tests in 36.434 seconds.
The Hermes profile suite ran 140 methods in 4.286 seconds: 139 passed and the
existing Windows-only case was skipped on Linux. That skip supplies no Windows
coverage. These are inert contract results, separate from native qualification.

The composition preserves the subsequent privacy correction
`bd63cdcae2b74998a13af7380ff998b29c5e0d51` in all three original paths. Its
closed diagnostic omits a foreign file-content digest; the private inventory
still verifies that digest. The composed Hermes suite ran 143 methods in
4.564 seconds: 142 passed and the same Windows-only case was skipped.

### Devin Global and Kiro execution boundaries

The original Devin Global job `132441892348` in
[run 43589882702](https://github.com/777genius/universal-agent-plugins/actions/runs/43589882702)
succeeded on `6d1f01deb47b05854a3a3ab7657889b44ba1cef0`, tree
`b818ff2bc551129fbfa17ba50882e1039e79756f`. Independent original review of
artifact `14112974433` joined outer SHA-256
`18b26baaf56ef587d9ef1308ad572ec07fe178317402ef4e05e261be9651ea3b`, both ZIP
CRCs, all 75 inner members, 74 native/public streams, four build streams,
234 acquisition/runtime pins, 37 ordered commands and 30 effects. Twelve
post-operation A/B inventories preserve foreign rows while the owned Skill
and two MCP entries appear and are removed. Cleanup and Global lifecycle
capture are complete; runtime initialize/list/call and Project qualifications
remain false. This is the actual Global inventory/public lifecycle boundary.

On `d9008f2c8bc19ee0b8a6cb62a6d50e25bf76ef91`, Kiro live audit
[run 43593070067](https://github.com/777genius/universal-agent-plugins/actions/runs/43593070067)
passed the corrected Docker preparation and the 30-Skill job. The plugin job
ran all 30 cases: 23 correctly retain incomplete-native-proof despite verified
prepared lifecycle/repeat/removal/sentinels, six pass, and one published
marketing-kit negative is unsupported without residue. These prepared cases
must not be relabeled native success. This acquisition selected Kiro 3.0.0;
the separate no-auth lane retains its own 2.34.0 pin and needs current-source
execution. Neither lane supplies evidence for the other's binary.
