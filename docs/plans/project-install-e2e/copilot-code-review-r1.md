# Copilot project implementation independent review R1

Reviewer: codex_claude, 2026-10-23. Reviewed cumulative adapter
`5103b9d0..9542b52d0b109acda2883752dfc0748548db2011` and corrective frozen
checkpoints `9129c03f4e932edc52acae860e18e1ec13e0be9b` and
`b98f0c57983433e5c7f2cde76255f94f4da1a079` in work/uap-project-copilot.
The reviewer did not author implementation or edit the author's worktree.

## Decision: GO at b98f0c57

Two selected-package consistency findings are independently verified closed.
No remaining implementation blocker found in the reviewed configured project
component lane. This is not genuine Copilot native discovery, trust, session,
MCP initialization/tool use or release qualification.

## Findings and closure

R1-1: original copilotPlanRoot validated project/native/user context but did
not require ActivePath to be the selected physical artifact's exact package.
The shared component helper intentionally validates only project containment;
it cannot infer Copilot's client/artifact namespace. An existing different
package child within the same project therefore needed adapter admission.
Closed in `9129c03f`: validate the physical leaf and literal
.uap/managed/clients/copilot/<artifact> path plus containment wherever a resolved
plan is consumed. The early planner precondition remains context-only until
physical target resolution. New regressions cover changed in-project package
and artifact, composed/direct activation, projection and active reuse, with
complete unchanged inventories on refusal.

R1-2: generic provider activation already required Delivery.ActivePath equal to
Plan.ActivePath, but direct Copilot adapter activation did not repeat that
consistency requirement. Closed in `b98f0c57`: project preflight refuses a nonempty
delivery mismatch while allowing legitimate empty pre-staging delivery; direct
project Activate requires literal equality, including refusal of empty delivery.
An existing other child and empty-delivery regression covers direct activation
and whole-project/user/B unchanged inventories.

## Reviewed production boundaries

- Project TargetContext retains the original detected user ConfigRoot and
  selects exact project authority. All scope/root mismatches refuse; packages,
  state and data remain under selected .uap. Scoped domain policy uses the
  existing native-config effect/recovery lifecycle and a distinct Copilot
  project backend; user marketplace/global CLI and shared user backend behavior
  remain intact. No project operation invokes global plugin or user MCP CLI.
- Exact root .mcp.json is selected when present, otherwise .github/mcp.json;
  .github/skills holds native skills. Path containment/alias checks precede
  effects. Recorded exact receipt paths fence later effective-config selection
  changes instead of silently moving ownership. Existing bare maps, malformed,
  duplicate/ambiguous or foreign disabled members refuse without mutation.
- The narrow CodecCopilot reuses the existing kernel, emitting explicit
  stdio/http/sse tags and tools:["*"]. Exact native member receipts and shared
  component skill swaps/conditional writes own only their selected entries.
  Foreign settings/members/skill directories are retained. Unknown/duplicate,
  stale and copied binding kinds/paths/artifacts cannot be adopted or removed.
  No new configuration transaction engine is introduced.
- Stdio delivery uses the existing trusted launcher with absolute selected
  package/data/cwd. Read-only active reprojection recomputes the full selected
  source contract against immutable bytes; empty, relative or changed selected
  data returns ErrProjectionRefreshRequired without replacing old locators.
- Preflight rejects ActivationComplete before the generic shortcut, including
  direct adapter reuse and prepared components. Installed-file success remains
  prepared plus installed, without attestation. Trust, native selection,
  plugin/session/admin precedence, disabled policy and initialization are
  explicitly unobserved.

Independently reread current primary configuration reference:
https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-mcp-servers
It supports the selected repository paths/precedence, explicit transports and
all-tools shape, and distinguishes trusted native loading from file delivery.
The adapter conservatively refuses bare-map mutation rather than silently
changing its native meaning.

## Public composition and independent execution

The saved production CLI fixture uses real loopback TLS Git acquisition,
standard loading/storage/planning/staging, installer/state/native transactions,
A/B install, locked no-op sync, read-only views, installed-pin repair after
source advance, pin update with retained binding/data authority, interrupted
bounded directory recovery, owned remove and repeated absent removal. It
preserves user/B configuration/state, foreign disabled entries and skills,
and persistent data. Detection/security seams are inert; no real native client
or configured server executes. The provider stdio fixture independently checks
the trusted managed launcher and exact selected data receipt.

Go 1.26.10, GOMAXPROCS=3, -p=2/-count=1/-timeout=3m:

- Initial full selected `^Test(ProjectCopilot|Copilot)` checkpoint: native
  kernel codec **PASS 0.003s**, client **PASS 0.005s**, scoped domain
  **PASS 0.003s**, providers **PASS 0.069s**, public saved CLI **PASS 5.652s**.
- Final corrected b98f0c57, `^TestProjectCopilot`: composed providers including
  both new no-effect refusal regressions **PASS 0.079s**; complete public saved
  CLI journey **PASS 5.378s**. No further optional broad compilation was needed.

Pinned help/skill-list observations recorded by the author are separate from
this review and do not become new project MCP or runtime evidence. No Copilot
binary, launcher, configured MCP process, account/model/trust operation or
hosted native suite ran in this review. Genuine approved-host project
observations and the unchanged disposable guard remain independent gates.
