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
