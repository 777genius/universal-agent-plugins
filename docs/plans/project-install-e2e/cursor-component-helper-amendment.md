# Cursor native component helper amendment

## Scope

Cursor project components are already approved in both client plan reviews. The
existing user Cursor manual package preparation stays unchanged. This amendment
adds only new `clients/shared/project_components*.go` helpers consumed by Cursor and the concurrently reviewed Codex project adapter;
no existing Gemini/Cline/OpenCode implementation or shared nativeconfig codec is
changed. There is no client discovery, settings engine, native executable or
new source engine. The helper uses existing `nativeconfig.Kernel` and
the caller-selected existing/independently reviewed codec for native MCP. Cursor
selects `CodecMCPServers` (command/args/env/cwd or url/headers); Codex selects its
separately reviewed native TOML codec. Future clients require their own reviewed
qualification before reuse.

## Exact inputs and preflight

A small explicit layout selects project boundary, skills root, exact native
config Paths, codec, and skill/MCP ownership kinds. Cursor supplies selected
ProjectRoot, `.cursor/skills`, `.cursor/mcp.json` and distinct project kinds.
Codex supplies selected ProjectRoot, `.agents/skills`, `.codex/config.toml`,
its independent codec and distinct project kinds. Every supplied filesystem
root/config is an exact no-follow strict child of the selected project boundary. Adapter validates matching ScopeProject/TargetContext,
original user ConfigRoot, exact planned NativeRegistryRoot before any helper.
Project artifacts remain exact `.uap/managed/clients/cursor/<physicalArtifact>`;
user ConfigRoot is never substituted. Shared helper requires clean absolute
project boundary and child roots/config paths, exact owned native paths, leaf names, no-follow containment, nonempty
digests, unique object IDs/names and only its declared kinds; unknown prior or
desired ownership fails closed. Project binding validator checks these paths
plus exact recorded native root; core independently validates selected target,
namespace, scope and project-state identity.

Adapters retain projection and server-dialect responsibility. Cursor derives
skill digests from already-rendered staged skills and MCP receipts from its
already-resolved staged `mcp.json`; Codex projects its own native codec. Each
adapter reads its immutable package with strict unique-key JSON/no-follow
containment, preserving existing managed stdio launcher, package/data/cwd
binding. The shared mutation accepts prepared native servers keyed by logical
name and exact desired ownership receipts, then compares kernel DesiredReceipt
before mutation. It does not load a descriptor, expand placeholders, implement
portable/native transport mapping, or execute code. Active reprojection reads
digest-covered files without writing the package. Caller native-root/scope
validation remains separate from helper's boundary containment validation.

## One mutation and rollback

Before creating a transaction: require injected kernel file IO, validate all
prior+desired identities/paths, verify existing prior directory digest/exact
whole-entry MCP receipt (allow missing for repair/remove), and reject a newly
requested object that already exists without prior ownership. Build all MCP
requests and Desired receipts from caller-prepared native servers; receipt/path/digest must
match staged desired ownership before native changes. Same-name foreign and
disabled entries collide; never enable, adopt or remove them.

When skills change, create a private same-filesystem transaction under selected
explicit skills root; stage desired copies and verify digests before native writes.
Move previous owned skills to private backups with exclusive rename, reverify
isolated backup digest, then exclusively install desired directories. Identical
prior/desired skill digests may stay in place after verification. Recheck
installed desired skills and cancellation before the one kernel `ApplyBatch`
for MCP. Kernel retains its existing lock, exact-entry preconditions, desired
receipt check, atomic replacement, rollback and postwrite verification.

If skill or MCP operation fails before confirmed config commit, remove only
newly installed skill directories that still match their digest, restore exact
backups without overwriting a foreign destination, and report native effect
unchanged only when the observed rollback plus kernel failure supports it.
Unknown/partially committed MCP errors remain uncertain even if skills restore;
core retains its existing pre-mutation attempt/pending ownership/recovery path.
If rollback cannot remove an externally changed install or restore a backup,
return a recovery error naming the private transaction and retain it. Never
remove that only recoverable copy. Cancellation before mutations is unchanged;
after mutation begins it follows the same observed rollback rules.

On successful ApplyBatch, or its existing CommittedCleanupError, native commit
is authoritative: keep installed skills, drop private backups, return committed
outcome; lock cleanup degradation uses existing lifecycle warning. If private
transaction cleanup itself fails, report committed cleanup degradation rather
than rollback committed config. No persistent application data directory is
removed. Empty transaction/skills parents may remain; foreign file bytes/modes,
user/project-B configs, hooks and skills are untouched. No copying of opaque
native state, HOME redirection or session/global MCP fallback.

## Contract validation and evidence

Tests through actual Cursor provider/loader/planner/stager, plus Codex consumer
contract fixtures for its distinct root/codec: project skill+MCP
add, repeat verification/noop, changed skill/MCP update, removed component,
exact-pin ownership repair, repeat remove, user/A/B byte+mode inventory,
foreign disabled collision/malformed config and symlink/path/root refusal.
Inject kernel failure after skill installation to prove exact restoration;
change an installed skill at failure boundary to prove preservation+retained
recovery. Exercise staged-receipt tamper and config-only/no-skill requests.
Runtime outcome is always project prepared+installed; generic attestation is
refused in preflight and direct adapter reuse. Tests are inert contract evidence,
not real Cursor discovery or MCP execution. Current official artifact/help
research and hosted native qualification are recorded separately.

## R1 applied clarification

A same-ID/same-digest skill no-op requires the destination to be observed
present as an exact real directory with the expected digest. `allowMissing`
verification alone never qualifies a no-op: a missing same-pin skill is staged
and recreated. Dedicated repair proof removes the owned skill without changing
its desired receipt, then proves restoration. All existing symlink ancestors of
skills/config are refused even when their target stays inside the selected
project; tests cover contained aliases as well as external escapes.

Frozen public helper signatures:

- `ProjectComponents{ProjectRoot, SkillsRoot string; ConfigPaths nativeconfig.Paths; Codec nativeconfig.Codec; SkillKind, MCPKind string}`
- `(ProjectComponents).ValidateObjects([]domain.NativeObjectOwnership) error`
- `(ProjectComponents).SkillObject(name, sourceRelative, stagingRoot string) (domain.NativeObjectOwnership, error)`
- `(ProjectComponents).MCPObject(name string, server nativeconfig.Server, resolvedPath string) (domain.NativeObjectOwnership, error)`
- `(ProjectComponents).Verify(objects []domain.NativeObjectOwnership, allowMissing bool, kernel nativeconfig.Kernel) error`
- `(ProjectComponents).Inspect(components []domain.ComponentDecision, managed *domain.ClientBinding, kernel nativeconfig.Kernel) (clients.RegistryFinding, error)`
- `ProjectComponentMutation{Layout ProjectComponents; ActivePath string; Previous, Desired []domain.NativeObjectOwnership; Servers map[string]nativeconfig.Server; Kernel nativeconfig.Kernel}`
- `(ProjectComponentMutation).Apply(ctx context.Context) error`

`MCPObject.resolvedPath` is the explicit selected config path, constrained to
one of the layout's declared config paths. Native object IDs are exactly
`kind + ":" + name`. Prepared server values are the adapter's already-resolved
native codec inputs; helper does not expand placeholders a second time.

## R2 applied clarification

`ActivePath` and projection `stagingRoot` must be explicit clean absolute
no-follow strict children of the selected project boundary. Skill
`sourceRelative` is a clean portable child directory with no empty, dot,
traversal, absolute, drive, backslash or control segment; source containment
rejects any symlink ancestor. Before any skill/config effect, the prepared
Servers key set must equal desired MCP logical names exactly; both missing and
extra entries fail. Contract tests tamper source and server-map inputs before
write and assert native user/A/B inventory remains unchanged.

Private transaction cleanup never recursively removes an unchecked recovery
path. Record its physical directory identity at creation and exact staged and
backup skill digests. Before deletion, reject any replaced transaction root,
unknown entry, symlink or changed backup/staged directory. Cleanup removes only
still-identified owned material, preserving changed or unknown recovery copies.
A postcommit cleanup refusal returns CommittedCleanupError and retains the
transaction with an actionable recovery location; committed native skills/config
are never rolled back because cleanup degraded. A focused fixture changes a
private backup at the successful MCP boundary, then proves committed native
objects remain installed, the modified backup is retained and warning semantics
are preserved.
