# OpenCode project implementation evidence

Status: scoped adapter, catalog and public composed lifecycle checkpoint,
2026-10-23.
The original all-client plan and its two independent reviews remain
authoritative. Both independent implementation reviews are GO at corrected
checkpoint `90fcc0af`; hosted native qualification remains a separate pending
gate.

## Native layout and authority

The accepted native paths are root `opencode.json` or `opencode.jsonc` for MCP
configuration and `.opencode/skills/<name>/SKILL.md` for skills. The original
detected user `ConfigRoot` is preserved. The internal resolved layout carries
both native locations through projection, inspection, apply, verification,
removal and the existing dialect transaction/recovery code. Legacy user helper
entrypoints continue to select their original `<user-config>/skills` layout.
No package path, changed ambient cwd or fake user profile selects project scope.

The existing namespace preflight validates the explicit selected target context
and uses its exact project config candidates, without reaching into a concrete
adapter from the generic provider. A disabled same-name server in the selected project is a
collision; the distinct preserved user config is not mistaken for the project
registry. Exact project object IDs, names, absolute paths and containment are
checked before native writes.

The public current documentation was checked on 2026-10-23:

- [Configuration](https://opencode.ai/docs/config/) documents JSON/JSONC,
  root project config, upward discovery to the Git boundary and merging of
  user, project, custom-directory, inline and managed settings.
- [Skills](https://opencode.ai/docs/skills/) documents `.opencode/skills`,
  compatible `.claude`/`.agents` paths and their ancestor discovery.
- [MCP](https://opencode.ai/docs/mcp-servers/) describes native local/remote
  configuration; transport/dialect eligibility still comes from the existing
  immutable prepared host authority, rather than guessing from config content.

The existing hosted native matrix pins OpenCode 1.18.34 with per-platform
artifact integrity. This checkpoint does not change that pin or execute the
hosted runner locally.

## Existing durable transaction

No new journal schema or native mutation engine was added. Version1 private
OpenCode records already bind complete old/target state, including each
ClientBinding.Scope and NativeProfileRoot. Recovery resolves the split layout
from that binding authority and requires the selected ProjectRoot to agree.
A project recorder with missing or different context refuses discovery and
reconciliation before native mutation. All production recorder constructions
receive the selected root, including CLI lifecycle and installer observation,
service and recovery.

The existing exclusive skill moves, receipt-bound MCP dialect transition,
state disposition, uncertainty, retained backups and committed-cleanup behavior
remain in use. The legacy skill kind spelling is retained as a receipt codec
identifier; scope and the exact native path determine project ownership.

Successful project file configuration is conservatively prepared activation
with installed verification. Generic activation attestation cannot establish
native discovery and is refused before the provider shortcut and on direct
adapter reuse. Effective config, permissions, native skill discovery and MCP
initialization remain unobserved. User activation behavior is preserved.

## Executed contract evidence

Final complete OpenCode package: PASS1.035s. After adding scoped contracts,
planner/stager qualification and the namespace correction, the focused run passed:

```sh
go test -p=1 ./install/integrationctl/agentplugins/clients/opencode \
  ./install/integrationctl/agentplugins/providers \
  ./install/integrationctl/agentplugins/domain \
  ./install/integrationctl/agentplugins/planner \
  -run '^TestProject|^TestNativeTransition|^TestCompatibilityGolden|^TestPlanGolden' \
  -count=1 -timeout=2m
```

Final focused results: OpenCode PASS1.197s, providers PASS0.181s, domain
PASS0.004s and planner PASS0.016s. These are production provider,
native-kernel, filesystem and compiled inert TEST-process contracts, not real
OpenCode runtime evidence. They cover:

- root JSONC comments/foreign disabled entries with skills in `.opencode`,
  repeated automatic/verify-only configuration and same-pin missing native
  skill/MCP repair;
- exact repeat removal, untouched user/project-B inventories and refusal of
  tampered IDs/names/paths, wrong selected roots and activation attestation;
- selected-project namespace collision checks, exact slash/space MCP keys
  without filename normalization, and contained native alias refusal;
- production loader/planner/command-renderer/stager/activator composition,
  rendered command skill ownership and read-only active native reprojection;
- six actual abrupt TEST-process crash boundaries for both existing user and
  project v1-to-v2 transitions: prepared record, backup, skill publication,
  native config, state and cleanup, followed by reopened idempotent recovery;
- selected project context refusal before recovery, with native bytes/state
  unchanged and durable pending authority retained.

The installer and production CLI composition packages compiled successfully at
the new context seams. The focused regex had no matching tests there, so this
is not a claim of installer/CLI lifecycle execution at this checkpoint.

## Scope and gate assessment

The OpenCode catalog row adds only its qualified project native-config policy.
The four compatibility golden changes are exactly OpenCode's Scopes array from
[user] to [user, project]; a deep comparison checked that all other values are
unchanged. User planning goldens pass unchanged.

Exact golangci-lint2.13.2 changed-code and whole-file size/complexity/architecture
gates both report zero issues. The initial concrete-adapter import violation
was removed from the generic namespace provider. One complexity finding was
fixed by extracting unchanged managed-context validation. No baseline,
architecture exception, hosted guard or native artifact pin was changed.

## Remaining checkpoint work

- Hosted native effective-config/skill/MCP discovery and harmless marker proof,
  with A/B/user precedence, fresh restart and removal observations. Higher
  priority inline/managed/custom directory and ancestor behavior must be
  observed by the real pinned host; file presence cannot establish activation.

## Public composed saved lifecycle

The production public CLI composition now has a dedicated OpenCode saved
project journey. Real TLS Git acquisition, standard loading/planning/staging,
immutable storage, installer, native kernel and receipt/state publication are
used throughout. The native version-contract boundary is the existing compiled
inert TEST process, limited to `--version`; it supplies no real OpenCode native
discovery or runtime evidence. Production host preparation and exact executable
revalidation still execute against those fixture bytes.

The test installs saved dependencies into projects A/B and preserves the
distinct user config. Unchanged locked sync must retain exact state bytes.
Managed package drift and a missing native skill are repaired from the old
immutable pin even after the Git source advances. Durable interrupted directory
recovery and repeat recovery preserve committed native config and caller-owned
staging. Moving-source update must preserve source/binding/data authority;
saved status reports project/prepared. Removal consumes exact owned skills and
MCP keys, retains foreign disabled configuration/comments/skills and persistent
data, and repeat removal has no further project effects. User and all project-B
files remain byte-identical.

Final dedicated public composition contract: **PASS 3.345s**, Go 1.26.10,
GOMAXPROCS=3, `-p=2`, count1. Exact golangci-lint2.13.2 changed-code and whole-file
size/architecture gates on the affected public CLI package both report
**0 issues**. No adapter production code, native guard or lint policy changed
for this proof. The generic source-core Git/ZIP cold-restore/transfer suite
remains separate; this is not a transport-by-client cross-product claim.

## Independent R2 correction: active projection data authority

R2 identified that project ProjectActiveNative ignored the selected dataPath
supplied by core after validating the persisted data receipt. The existing
stored projection could therefore retain its old DataRoot while presenting
desired receipts for a different selected locator.

The project-only active check now validates matching target scope/layout,
exact active package root, containment, native config roots and stored
PackageRoot before deriving receipts. For stdio, the selected data locator must
be nonempty, clean and absolute, and equal the stored immutable DataRoot. A
mismatch returns ErrProjectionRefreshRequired without rewriting package bytes.
The existing user projection path is preserved.

The focused regression includes a correct selected locator plus missing,
relative, another-project and different-receipt locators, and a mismatching
active package root. All failures must leave package/native/user/project-B
inventories unchanged. Project provider contracts pass **0.022s**; the public
saved project lifecycle passes **3.395s** with its real stdio data journey. Both
exact golangci-lint2.13.2 changed-code and whole-file size/architecture gates
report **0 issues**. Both independent reviewers inspected this correction and
closed GO at `90fcc0af`; their reports are recorded alongside this evidence.
No native host or model execution was added.
