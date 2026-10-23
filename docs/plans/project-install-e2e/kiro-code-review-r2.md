# Kiro project implementation independent review R2

Reviewer: codex_claude, 2026-10-23. Reviewed the cumulative Kiro adapter delta
`426a82b3..9eeadc1a47650d95c803ae043fa9411ce51ae21c` and the final correction
`b3b10e7ac15b0c6d5049826b831c1f84d143513d` in work/uap-project-kiro.
The reviewer did not author this implementation or edit that worktree.

## Decision: GO at b3b10e7a

No remaining implementation blocker found. The independent R1 omitted-user
context finding is closed: project admission now requires literal equality to
the detected original user ConfigRoot, including empty/nonempty mismatch.
The explicit omitted-user regression independently passes alongside wrong-user,
wrong native root/scope, copied-project and native-root alias refusals. Both
empty contexts retain their supported file-only behavior. This GO covers the
configured project lane and its production fixture composition; it is not a
native workspace/agent/runtime or release qualification.

## Reviewed production boundaries

- Project TargetContext selects exact .kiro native authority while retaining
  the separate original user profile. Planning validates context before target
  resolution, then requires the exact selected physical-artifact package under
  .uap/managed/clients/kiro. Projection, inspection, lifecycle and binding reuse
  revalidate their selected project/native/package roots. Native skill/config
  paths remain .kiro/skills and .kiro/settings/mcp.json; package/state/data stay
  in the project .uap namespace. No user-profile surrogate is introduced.
- Project native receipts retain the existing Kiro kind/ID format and exact
  JSON object digest. The portable shared validator checks the normalized IDs,
  exact native layout, clean absolute paths, source-relative paths, digest and
  duplicate/unknown ownership. Only the core-managed package receipt is kept
  outside the native writer; foreign skill/config documents are never owned
  wholesale. Copied bindings must match scope/root/artifact/locator exactly.
- The existing Kiro JSONC/exact-file conditional transaction and exclusive
  skill-swap/rollback implementation remain the native writer. Project routes
  pass the explicit validated native root instead of calling legacy global
  ConfigRoot wrappers. Missing writer/launcher and malformed/foreign/disabled
  ownership refuse before effects. Visible-write failure rolls back the exact
  original document and staged skill effects; committed cleanup degradation
  retains exact receipts plus a conservative prepared result.
- Project MCP emits documented command/url configuration without portable
  type tags or an assumed native cwd. Stdio receives the existing trusted
  launcher, absolute package/data/cwd bindings and original source arguments.
  Active reuse checks the selected current data receipt, launcher path,
  environment and protocol root/cwd prefix without rewriting digest-covered
  bytes. Empty/relative/different data authority requires projection refresh.
- Activation preflight runs before generic attestation admission, and direct
  project activation repeats the guard. Project activation/verify-only remain
  prepared plus installed-file verification. Attestation cannot turn native
  policy, skill discovery or MCP initialization into an observed success.
  Existing user CLI/ACP behavior and engine selection are preserved; no project
  process invokes global ACP, migrates trust/account state or changes engine.
- Scoped domain policy selects the existing native-config effect/recovery
  lifecycle while user CLI-registry traits stay intact. The project preparation
  branch is explicit and does not loosen user preparation admission.

## Public composition and isolation evidence

The public installer SDK fixture uses real loader/planner/stager/state/native
transactions with inert launcher bytes. A/B have separate stdio/data locators;
unchanged update is a no-op, repair consumes the installed pinned bytes despite
moved source, a fresh installer instance performs update/status/remove, and
retained data markers, global configuration and B ownership remain unchanged.

The production CLI fixture exercises real loopback TLS Git and ZIP acquisition,
saved A/B installation, locked no-op sync, read-only views, missing-skill repair,
update retaining source/binding authority, exact portable export/cold import
and owned remove. It preserves user/B native bytes/state, foreign settings,
skills and hooks. TestProjectKiro naming is selected by the existing hosted
project-contract gate. Security/detection boundaries and native runtime seams
are explicitly inert; they are not relabeled as genuine Kiro proof.

## Independent bounded execution at corrected checkpoint

Go 1.26.10, GOMAXPROCS=3, -p=2/-count=1/-timeout=3m, selection
`^Test(ProjectKiro|Kiro)`:

- Kiro client project plus existing TestKiro contracts: **PASS 1.440s**.
- Composed providers: **PASS 0.084s**.
- Public installer SDK stdio/data/repair/restart/remove: **PASS 0.154s**.
- Production CLI Git/ZIP saved source lifecycle: **PASS 5.301s**.
- The same selection found no domain tests; no fresh domain execution is
  claimed by that command.

No actual Kiro binary, configured MCP process, model, login, account mutation,
auto-trust or hosted native suite ran in this review. Pinned successful help,
file receipts and these fixtures do not qualify project discovery, IDE/mobile,
active custom-agent includeMcpJson/resources policy, native initialization/tool
use, native restart or release readiness. Those remain separate genuine hosted
observations under the unchanged disposable guard.
