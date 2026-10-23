# Copilot CLI project adapter independent code review R2

Reviewer: cursor_clients, 2026-10-23. Frozen implementation:
`9542b52d0b109acda2883752dfc0748548db2011` in work/uap-project-copilot;
reviewed the complete client delta from `5103`, narrow native codec, scoped
domain declaration, composed fixtures and public TLS Git lifecycle. Read the
pre-code pin/help evidence and the original two-GO client plan. Rechecked the
current official GitHub MCP/skills documentation linked in that evidence.

## Decision: GO for the conservative project component adapter

No code blocker found. The project lane has a distinct exact filesystem
contract and preserves the user marketplace/CLI and unsupported command policy.
Configured files remain prepared with installed-file verification. This review
does not qualify native project discovery, MCP initialization or release.

### Native format and effective path

- Copilot's narrow codec maps portable stdio and streamable HTTP/SSE to explicit
  native stdio/http/sse entries, with documented tools:["*"]. It delegates
  existing field validation and placeholder handling to the current kernel,
  refuses unqualified remote transports and native cwd, and leaves the other
  codecs unchanged. The tools member selects available tools; this adapter
  does not add trust, authentication or permission grants.
- A regular selected-root .mcp.json takes precedence. Otherwise the adapter
  selects .github/mcp.json. Root/child aliases refuse through the existing
  no-follow containment checks. The .github/skills directory is the sole new
  skill lane, with no .vscode config or user registry writes.
- Existing nonempty bare maps refuse before activation instead of being
  silently wrapped into a different native meaning. Strict malformed,
  duplicate and wrong-shaped documents remain failures. Empty objects can be
  initialized. Exact native receipt paths fence a changed selected config;
  staged fallback receipts cannot silently become root-config authority.
- Current official MCP documentation confirms repository paths, same-directory
  root precedence, nearer-cwd precedence, nested/bare support and the folder
  trust boundary. The implementation deliberately supports a narrower safe
  write contract and does not infer effective descendant/session/plugin/admin
  selection from successful file delivery.

### Scope, ownership, lifecycle and selected data

- Original detected user config remains in TargetContext. Project registry is
  the selected project root, while managed package/state/data stay under .uap.
  Scoped backend identity is Copilot rather than the shared user editor family;
  existing user sharing is retained. VS Code implementation is untouched.
- Binding inspection validates project scope, selected artifact/package/native
  root and exact native object identities/paths/digests. Unknown ownership is
  not filtered into permission to mutate. The only excluded receipt kind is
  the core's managed_package_directory. Foreign skills and disabled same-name
  entries are collisions; stale managed bytes cannot be overwritten/removed.
- The adapter reuses the reviewed shared skill transaction and ownership-aware
  kernel batch. New server values are checked against staged exact receipts;
  writer failure rolls back skills, and committed cleanup retains receipts.
  Update, missing owned components, dropped MCP and repeated removal preserve
  foreign objects and other project/user state.
- Stdio uses the existing trusted managed launcher. Read-only active reuse
  recomputes the complete authored stdio contract against the selected package
  and data locator, including environment, launcher command/args and cwd. Full
  equality with immutable stored projection is required. Missing, relative or
  different selected data refuses with projection-refresh-required; inspection
  never repairs the mismatch by changing digest-covered package bytes.
- Project attestation is rejected by preflight before automatic/manual fallback.
  Project inspection/activation/removal do not invoke a global Copilot command.
  Successful native file verification is downgraded to prepared and remains
  un-attested. Genuine pre-code skill listing and absent untrusted workspace MCP
  evidence are kept separate from the new configuration fixtures.

### Production composition and evidence limits

The public saved CLI fixture retains production scoped composition, solver,
store, planner, installer and native filesystem transactions over real loopback
TLS Git. It covers A/B add, exact locked no-op, installed-pin repair despite a
moving source, bounded pending directory-journal recovery and repeated recovery,
moving update, retained source/binding/data authority, conservative saved status,
exact removal and absent repeat without effects. Complete user/B inventories
remain unchanged. Its detector/security seams are explicit, and trusted helper
bytes are copied without execution. The journal fixture is not relabeled as a
genuine native process restart. No export/ZIP/native-runtime claim is inferred
from this particular fixture.

## Independent bounded verification

- Project providers: **PASS 0.182s**; narrow Copilot native codec: **PASS 0.010s**;
  scoped domain policy: **PASS 0.003s**.
- Full existing Copilot client suite: **PASS 0.006s**.
- Public saved production CLI TLS Git lifecycle:
  TestProjectCopilotComposedSavedLifecycle — **PASS 5.529s**.
- Complete delta whitespace check and clean frozen worktree verified. Read the
  shared component request/commit path to confirm selected receipt enforcement,
  no-write missing removal and rollback semantics are actually composed.

No native Copilot client, launcher, MCP, model, trust or account process was
executed by this review. Genuine hosted native and cross-platform qualification
remain pending; this GO supports publishing the reviewed project client draft.

## Final R1 correction refresh: GO at `9129c03f4e932edc52acae860e18e1ec13e0be9b`

Reviewed the narrow final correction in scope.go/project.go and its composed
regression. The full project plan now requires a valid selected artifact ID and
literal `.uap/managed/clients/copilot/<id>` ActivePath with no-follow project
containment. The earlier planner precondition checks only scope/root because
generic target resolution has not populated ActivePath at that point. Direct
active projection validates the full plan before data/server reads.

The new regression rebinds an existing in-project package or changes the
physical artifact ID, then requires refusal through composed/direct activation,
direct projection and direct active reuse, with the complete native/package/
user/B inventory unchanged. Independently executed that exact regression:
**PASS 0.036s**. This closes the R1 exact-artifact gap and preserves this R2 GO.
Other changes in the final leaf are previously reviewed core/source/CI ancestry;
this refresh introduces no broader native/runtime qualification claim.

Final adjacent direct-delivery closure: **GO** at
`b98f0c57983433e5c7f2cde76255f94f4da1a079`. Preflight accepts an absent delivery
only during the real pre-staging phase and rejects any supplied different
ActivePath. Direct activation requires literal Delivery.ActivePath equality
with the already validated exact project artifact. The regression covers an
existing other child and empty delivery, and preserves the complete base
inventory on refusal. Independently ran both exact-artifact/direct-delivery
regressions: **PASS 0.033s**. All earlier R2 conclusions remain in force.
