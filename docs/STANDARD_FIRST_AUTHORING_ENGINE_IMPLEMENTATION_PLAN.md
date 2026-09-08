# Standard-First Authoring Engine Implementation Plan

## Owner clarification: preserve legacy capabilities (2026-09-06)

This clarification controls every phase, inventory, worker assignment and
acceptance check below. Standard-first command retirement is not permission to
delete the corresponding implementation. A capability missing from plugin.json
or Agent Plugins 1.0 must not be discarded for that reason.

Preserve useful plugin.yaml implementation, necessary dependencies, tests and
design documentation outside the standard authoring dependency graph. Reuse or
adapt it through a narrow neutral interface when a planned consumer needs it;
do not build a second engine or move files merely to create an archive.

Before any legacy deletion, record each capability's implementation, tests,
consumers, exact standard mapping if one exists, preservation destination and
support status. Classify it as reuse, adapt, preserve/defer, or explicitly
approved removal. Default unresolved cases to preserve/defer. Neither lack of a
standard field nor lack of a current standard-first caller proves dead code.
Deletion requires the owner's explicit acceptance of that capability's removal.

Normal standard authoring still reads only plugin.json and its standard
components, with no YAML fallback or legacy model dependency. Keeping source
does not advertise a supported v2 command. A maintained separate YAML entrypoint
is not implicitly authorized and remains a separate product decision.

Historical v1 binaries alone do not satisfy code preservation. See
[the preservation contract](./AUTHORING_CAPABILITY_PRESERVATION.md) and the
Phase 11 inventory gate. Current standard CLI/npm work may continue; blanket
legacy deletion is not authorized.

## Owner decision: early public wording and site checkpoint (2026-09-08)

The owner explicitly authorizes prioritizing a stable intermediate PR merge and
public site update before the full v2 CLI release and phases 7-11. This is a
partial delivery of this plan, not a smaller replacement for its full objective.
It supersedes preparation-document rules that require D5 merely to publish
truthful public wording; full executable release qualification still requires D5.

Scope: replace obsolete primary positioning with clear Use plugins / Build
plugins journeys, consistent maintained locales, canonical links and explicit
availability labels. Describe unreleased standard authoring commands as
preparation, never as installed or currently executable features. Preserve
accurate instructions for currently available products and the historical v1
reference, including redirects. Verify actual published versions before claiming
availability; do not relabel an unreleased candidate as the current version.

Implement a minimal explicit public-documentation boundary. The existing
DOCS_PREPARATION_PREVIEW flag remains restricted to disposable non-published
previews: do not enable it in production, remove the check without a replacement,
or use noindex alone as proof of safe publication. Update conflicting preparation
docs and their tests in the same bounded PR so subsequent agents see this owner
decision. Do not create a second docs engine or a generic release platform.

Checkpoint acceptance:

- Every maintained locale distinguishes available installation from unreleased
  authoring; no runnable future command is presented as current.
- All Use/Build/history links and redirects resolve in the actual built site;
  canonical-English fallback is explicit where a translation is absent.
- Counter, geometry, catalog and affected navigation E2E pass with genuine
  verified feeds, original assertions/timeouts and no masked retries.
- The production-mode docs/landing build succeeds without the preview flag;
  actual Pages assembly and public-boundary checks pass on the merge candidate.
- Independent review and required CI cover the exact merge candidate. Merge a
  dependency-safe, reversible checkpoint and verify the resulting deployed site.

Do not hold this checkpoint for unrelated runtime, migration, export, publish or
legacy-isolation phases. Do not merge the current preparation stack unchanged:
main/master auto-deploys Pages, so establish and verify the boundary first.
CLI asset release, npm/PyPI tags, Homebrew and native qualification remain
separate gates; this checkpoint does not attest them or activate future commands.
Useful YAML capabilities and all preservation constraints above remain intact.

The preliminary 100-500 changed-line estimate is a target, not a guarantee.
Re-estimate after bounded intake against the actual merge base; do not weaken
acceptance or expand scope just to satisfy that number. Hosted workers use
`gpt-6-astra`, reasoning `medium`, service tier `default` (no fast).

Checkpoint receipt: PR #190 merged as `dc28313ab6f567eb86eecac3ef903f79b584d4c3`.
Pages deployment 34170488596 passed on descendant
`758c1656e1d3e6f1783b96638839486416921453`. Public HTTP readback verified
five quickstarts, all 62 historical anchors and the Use/Build/history entrypoint.
Browser evidence remains composite: 34 passing scenarios followed by the complete
affected tooltip scenario passing after a positioning-only test fix, with no
assertion/timeout weakening or retry. Full deployed-browser validation remains
pending. This is partial delivery; phases 0-11 and executable release gates remain.

## Status

- Decision: approved for planning.
- Implementation: not started by this document.
- Baseline inspected: `b0b4268e964fa5808debbcc998bd174670faeb6e` on 2026-09-06.
- Product repository: `777genius/universal-agent-plugins` (the former `plugin-kit-ai` URL redirects to it).
- Directory repository: `777genius/universal-agent-plugins-registry`.
- Target standard: Agent Plugins 1.0.0.
- Current published baselines checked on 2026-09-06:
  [`universal-agent-plugins` npm](https://www.npmjs.com/package/universal-agent-plugins)
  `0.1.51`; [`plugin-kit-ai` npm](https://www.npmjs.com/package/plugin-kit-ai)
  and [PyPI](https://pypi.org/project/plugin-kit-ai/) `1.2.4`.

This plan supersedes the open decision in
[`TODO_AGENT_PLUGIN_AUTHORING.md`](./TODO_AGENT_PLUGIN_AUTHORING.md). It does not
silently rewrite ADR 0005. Phase 0 adds a focused authoring ADR that extends the
existing standard-first installer decision without weakening its invariants.

## Executive summary

Build one standard-first authoring engine in Go and expose it through two thin
entrypoints:

```text
plugin-kit-ai <command> -----------+
                                      +--> shared authoring CLI and services
agentplugins author <command> ------+             |
                                                   v
                                    plugin.json + mcp.json + skills/
```

`plugin-kit-ai` becomes a standard-first authoring utility, not a legacy
`plugin/plugin.yaml` product. `agentplugins author` exposes the same implemented
authoring commands from the main installer binary. Both entrypoints must call
the same Go packages in process. Neither entrypoint may shell out to the other
binary. `plugin-kit-ai` may additionally contain bounded v1 migration error
shims, but no second implementation of successful authoring behavior.

The existing Agent Plugins loader, schemas, `PackageEnvelope`, diagnostics,
path policy, process supervision, scaffold infrastructure, archive safety,
runtime checks, command rendering, and test helpers should be reused wherever
their contracts do not depend on `plugin/plugin.yaml`.

The migration is a strangler refactor:

1. Put a new standard-first service behind the existing small command runner
   interfaces.
2. Move reusable command construction out of `package main`.
3. Convert commands by user job, starting with a complete `init -> validate ->
   inspect -> test` vertical slice.
4. Preserve useful command names and implementations. Keep YAML-specific
   generation outside standard authoring; classify it for preservation or
   adaptation instead of deleting it because the portable standard is narrower.
5. Provide one explicit, non-destructive legacy project importer.
6. Migrate first-party examples and documentation.
7. Detach legacy wiring from normal standard authoring after the migration
   gate. Retain the isolated importer and useful legacy implementations with
   their tests; deletion follows the explicit capability-preservation gate.

Expected implementation size:

- first useful authoring slice: approximately 2,500-4,500 production lines and
  3,500-6,000 total changed logical lines with tests/docs;
- useful command parity and migration: approximately 5,000-8,000 production
  lines and 7,000-11,000 total with security/cross-platform fixtures;
- first-party example conversion and generated documentation are tracked
  separately because much of that diff is mechanical.

## Critical path and delivery gates

The shortest useful path is:

```text
ADR/import boundaries
  -> reusable command constructors
  -> standard project snapshot + validation policies
  -> init templates
  -> inspect/compat/doctor
  -> offline test + Skills
  -> stable dual-entrypoint release
```

Migration, runtime execution, client previews, archive tooling, and remote
publication are follow-on capabilities. They must not delay the first release
once the offline vertical slice is proven.

Go/no-go gates:

1. **Format gate:** normal authoring accepts only root Agent Plugins 1.0
   packages; OpenAI compatibility and `plugin/plugin.yaml` require explicit
   import or migration paths.
2. **Reuse gate:** shared command construction is behavior-neutral and current
   installer tests remain unchanged.
3. **MVP gate:** generated Skill and MCP packages pass equivalent offline flows
   through both entrypoints in disposable roots.
4. **Release gate:** native assets and wrappers bind the same authoring engine
   revision and current documentation teaches no legacy format.
5. **Preservation gate:** after consumer inventory, first-party migration and
   released migration tooling, detach legacy standard-command wiring. Preserve
   useful implementations and tests; each deletion needs an inventoried
   capability decision explicitly accepted by the owner.

## Highest-risk assumptions

| Risk | Why it matters | Required control |
| --- | --- | --- |
| Three formats become one implicit fallback | A valid-looking project could be interpreted differently by install, author, and publish | Exact format ID at every entrypoint; no fallback |
| Existing Cobra globals leak across roots | Persistent installer flags and package-level variables can change author behavior or tests | Fresh command tree and explicit flag adapter |
| Reuse means preserving the old domain | Calling old `app.PluginService` would keep `plugin/plugin.yaml` authoritative behind a new name | Reuse infrastructure below the manifest boundary only |
| Migration blocks delivery | Full legacy parity is much larger than the new user MVP | Release offline core before migration/runtime/publish |
| Runtime test executes package code | A convenience command can become arbitrary local execution | Explicit post-MVP opt-in, displayed plan, disposable roots, bounded process/network |
| Two product versions drift | Users could run different authoring behavior under each command | One embedded engine revision and parity tests |
| Agent Skills changes independently | Old and new binaries could disagree on `SKILL.md` | Pin embedded validation profile and expose its identity |
| Repository and Go module names differ | A broad rename could break consumers without improving authoring | Keep Go module paths stable; separate future ADR |
| Strict release policy is reported as standard failure | Users lose trust in conformance results | Separate loadable, conformant, authoring-ready, and release-ready states |

## Goals

1. Make root `plugin.json` the only portable manifest and the only authoring
   source of identity and metadata.
2. Let authors create, validate, inspect, test, package, and publish Agent
   Plugins 1.0 without understanding client-native layouts.
3. Reuse the mature parts of `plugin-kit-ai` rather than writing a second
   scaffold, process runner, archive implementation, or diagnostics stack.
4. Keep installation and authoring in one source repository and one domain
   model while preserving separate command responsibilities.
5. Keep both public entrypoints behaviorally identical for authoring commands.
6. Stop teaching `plugin/plugin.yaml` as the current standard authoring path
   after first-party migration. Preserve historical design docs, useful templates
   and implementation in explicitly labelled legacy boundaries; historical
   release artifacts remain immutable.
7. Preserve installer security, lifecycle state, rollback, Directory trust,
   and client adapters without coupling them to authoring concerns.
8. Remain forward-compatible with later Agent Plugins specification versions
   through the existing versioned schema registry and lossless package model.

## Non-goals

- Define a competing Agent Plugins specification.
- Add authoring, build, publication, OAuth, client selection, or marketplace
  fields to the core `plugin.json` schema.
- Introduce a required `plugin-kit.yaml`, `agentplugins.yaml`, or other sidecar.
- Convert `PackageEnvelope` into the legacy `IntegrationManifest`.
- Make every client-native feature portable. Agent Plugins 1.0 standardizes
  Skills and MCP servers; other behavior remains explicitly client-specific.
- Run agent or model commands during ordinary validation.
- Test authoring or runtime behavior in a real user project.
- Automatically publish packages or create external effects without an
  explicit command and a reviewed plan.
- Rewrite the stable installer lifecycle while adding authoring.
- Preserve obsolete command semantics merely to keep a command name.

## Normative references

- Agent Plugins 1.0 specification:
  <https://agent-plugins.org/specification>
- Author guide:
  <https://agent-plugins.org/plugin-authors/build-an-agent-plugin>
- MCP component guide:
  <https://agent-plugins.org/plugin-authors/mcp-servers>
- Skills component guide:
  <https://agent-plugins.org/plugin-authors/skills>
- Agent Skills specification, which is authoritative for each discovered
  `SKILL.md`:
  <https://agentskills.io/specification>
- Client extensions guide:
  <https://agent-plugins.org/plugin-authors/client-extensions>
- Existing installer architecture:
  [`adr/0005-standard-first-agent-plugins-installer.md`](./adr/0005-standard-first-agent-plugins-installer.md)

The specification text remains authoritative over this plan. A future spec
revision must be integrated through a new schema adapter and compatibility
decision, not by silently changing the meaning of the 1.0.0 schema identifier.
Because Agent Plugins delegates Skill validation to a separately maintained
Agent Skills specification, every released binary also records the exact
embedded Agent Skills validation profile revision/digest. Updating that profile
requires fixture review and a release; validation never fetches changing rules
from the network.

## Locked decisions and invariants

### Package source of truth

The publishable package root has this shape:

```text
my-plugin/
├── plugin.json
├── mcp.json                    optional
├── skills/                     optional
│   └── <skill>/SKILL.md
├── <reverse-domain-name>/      optional client extension
├── src/ or bin/                optional runtime implementation
├── package.json                optional language-native build input
├── pyproject.toml              optional language-native build input
├── go.mod                      optional language-native build input
├── README.md                   optional but generated by our templates
└── LICENSE                     optional; generated only by explicit choice
```

Invariants:

- `plugin.json` is required at the exact package root.
- Core identity and metadata are read only from `plugin.json`.
- `mcp.json` is the only portable MCP configuration source.
- `skills/` is the only portable Skills discovery root.
- No file may supplement, override, or repair invalid core fields in
  `plugin.json`.
- Unsupported `extensions` entries and extension directories remain opaque.
- Unknown future-compatible data must be preserved losslessly when a command
  rewrites a document.
- Build files may describe how runtime code is compiled, but may not redefine
  plugin identity, portable component discovery, or install policy.
- Generated client projections are derived artifacts. They are never authoring
  inputs and are never allowed to override the standard package.

### OpenAI compatibility format is a separate input format

The current code has an explicit `OpenAILoader` for the OpenAI compatibility
layout rooted at `.codex-plugin/plugin.json`, with optional `.mcp.json` and
`.app.json`. It is intentionally separate from the Agent Plugins 1.0 `Loader`,
which reads root `plugin.json`, `mcp.json`, and `skills/` and never falls back to
the OpenAI layout.

The standard-first authoring path must preserve this separation:

- it accepts only `FormatIDAgentPluginsV1` as a normal authoring project;
- it never treats `.codex-plugin/plugin.json` as root `plugin.json`;
- default templates do not create `.codex-plugin/`, `.mcp.json`, or `.app.json`;
- those files never supply or override standard manifest/component fields;
- OpenAI compatibility inspection or import is always explicit and labelled
  non-portable;
- OpenAI app binding or runtime evidence is never counted as Agent Plugins 1.0
  package conformance;
- if an OpenAI projection is needed, it is derived by the client provider or a
  supported namespaced extension, not added as another portable component.

### No second required config

The MVP introduces no authoring sidecar.

Repeatable commands derive their inputs from:

1. the standard package;
2. language-native files such as `package.json`, `pyproject.toml`, and `go.mod`;
3. explicit CLI flags;
4. immutable Directory or release metadata when publication requires it.

If a future use case genuinely requires persistent build configuration, it
needs a separate ADR after a second concrete consumer exists. Such config must:

- live outside the published package root, or in a clearly tool-owned
  non-portable repository envelope;
- never be required to install or validate the Agent Plugin package;
- never duplicate core manifest fields;
- never be copied into the release package by default.

### Two entrypoints, one implementation

Authoring behavior must be constructed by a reusable command factory. Cobra
commands are mutable and acquire parent pointers, so each entrypoint must build
a fresh command tree from the same factories. A single `*cobra.Command`
instance must not be attached to both roots.

Illustrative shape:

```go
// Illustrative only. Exact package names may change during implementation.
type AuthoringApp struct {
    Projects ProjectReader
    Init     InitService
    Inspect  InspectService
    Test     TestService
    Pack     PackService
}

func NewAuthorCommand(app AuthoringApp) *cobra.Command
func NewPluginKitRoot(app AuthoringApp) *cobra.Command
```

The `plugin-kit-ai` entrypoint mounts the authoring commands at its root. The
`agentplugins` entrypoint mounts the same factories under `author`.

```text
plugin-kit-ai init demo
agentplugins author init demo
```

For equivalent arguments, both must produce equivalent exit status,
machine-readable result, diagnostics, and filesystem effects. Branding and the
displayed invocation prefix may differ. Product-facing fields such as
`entrypoint`, invocation text, and wrapper version may differ; the result must
also expose one identical authoring engine revision so parity is testable.

The current `agentplugins` root already owns persistent installer flags. The
authoring subtree must not redeclare colliding Cobra flags or reuse the mutable
installer `options` object as its domain configuration. During the pre-1.0
compatibility period:

- inherited `--format`, `--no-color`, `--dry-run`, and `--target` are mapped into
  a fresh authoring options value when the selected author command supports them;
- explicitly supplied installer-only `--scope`, `--accept-security-risk`, and
  `--security-details` are rejected before any authoring effect;
- author commands do not relocate existing root flags in the same PR, because
  that could break scripts that place persistent flags before the subcommand;
- help labels inherited installer-only flags and gives the valid authoring
  alternative;
- a later root-flag cleanup requires a separate compatibility decision.

### Existing Go module identity is out of scope

The repository has a canonical public product URL while current Go modules and
internal imports still use `github.com/777genius/plugin-kit-ai/...`. Renaming the
Go module is not required to deliver standard-first authoring and would create
an unrelated ecosystem-wide compatibility migration. This program keeps those
module paths stable. Any future module rename requires its own ADR, consumer
inventory, forwarding strategy, and release plan.

### No subprocess wrapper around the old engine

The new path must not:

- create a temporary `plugin/plugin.yaml`;
- call `plugin-kit-ai` from `agentplugins` or the reverse;
- serialize a `PackageEnvelope` into `IntegrationManifest`;
- depend on both binaries being installed;
- parse human output from another command;
- hide format conversion from the author.

These shortcuts would preserve the old domain model and introduce version,
error, signal, quoting, and platform drift.

### Separation of authoring and installation

Authoring and installation use the same standard package model but have
different responsibilities:

```text
authoring: mutable working tree -> plan -> explicit local authoring mutation
installer: immutable snapshot -> client plan -> transactional client mutation
```

The authoring service may create or edit a package selected by the author. It
must not read or change installer state, client configuration, Directory cache,
OAuth state, or managed plugin data.

The installer may project a validated immutable package into clients. It must
not modify the source package or invoke authoring commands.

## Current implementation assessment

### Already suitable for direct reuse

- The Agent Plugins 1.0 schema registry and pinned schemas.
- `loader.Loader` and `domain.PackageEnvelope`.
- Component diagnostics and failure boundaries.
- Source acquisition and immutable package snapshots for read-only validation.
- Path containment, file type, symlink, and executable-mode policy.
- Cobra command-level runner interfaces.
- JSON output envelope and exit-code conventions where already public.
- Process supervision and bounded execution primitives.
- Runtime discovery for Node.js and Python where it does not assume
  `plugin/plugin.yaml`.
- Archive header/path safety utilities after package-content policy is
  separated from old export metadata.
- Documentation export tooling after command factories are moved from
  `package main`.
- Existing client planners/providers for compatibility reporting and
  install-time projections.

### Reusable after adaptation

- Scaffold planning, template embedding, rendering, and filesystem writes.
- `doctor`, `dev`, `test`, `export`, `bundle`, `inspect`, and `compat` command
  renderers.
- Native import helpers.
- Fixture-driven runtime tests.
- Publication diagnostics and GitHub release plumbing.
- Skills authoring helpers.
- npm and PyPI binary bootstrappers.
- VitePress documentation and generated CLI reference.

The adaptation boundary is the project model. Reused services must accept a
standard-first project or narrower capability interface, not
`pluginmanifest.Manifest`.

### Excluded from the standard domain; preserve implementation pending review

- `plugin/plugin.yaml` as the authored root.
- `targets` stored inside the authored manifest.
- `launcher.yaml` as a required portable concept.
- Generation of root portable manifests from YAML source.
- Old `pluginmanifest` graph as the domain passed into new authoring commands.
- Legacy lifecycle commands duplicated by `agentplugins add/update/repair/remove`.
- Publication models that require old `publish/*` YAML to determine portable
  package identity.
- Templates and examples that teach generated root files as the source of
  truth.

### Measured impact surface

The current authoring implementation spans hundreds of Go files. Exact search
found `plugin/plugin.yaml` and historical `plugin.yaml` references across
dozens of docs/examples/test files and
direct `pluginmanifest` or `pluginmodel` coupling in the command rendering and
application layers. This means a full mechanical rename is unsafe, but the
small command runner interfaces provide a viable seam for incremental
replacement.

### Known policy conflation to remove

The current standard loader is optimized for safe installation, not yet for a
complete author-facing conformance explanation. For example, it applies
`pathpolicy.ValidateLeafID` to the manifest name after schema validation, and
the snapshot/archive layer enforces cross-platform physical path restrictions.
Those controls are useful, but some are stricter than the portable
specification itself.

The refactor must not solve this by weakening installer safety. Instead it
separates:

1. bounded parsing and normative Agent Plugins/Agent Skills facts;
2. installer loadability and security policy;
3. cross-platform authoring/release hygiene;
4. destination-specific publication policy.

Illustrative outcome: a name that satisfies the official schema but is unsafe
as a Windows directory can be `conformant: true`, `authoring_ready: false`, and
`installable` only where a provider can map it to a safe physical identifier.
Diagnostics name the policy layer. They never call the package non-conformant
unless the standard itself says so.

Before implementation, audit every fatal error in the loader, package digest,
and source acquisition paths and classify it as normative, host-safety,
installer-policy, or release-policy. This classification becomes a table-driven
test fixture, not prose-only intent.

## Target module architecture

Suggested package boundaries:

```text
cli/plugin-kit-ai/internal/
├── authoringcli/          reusable Cobra factories and renderers
├── authoring/
│   ├── project/           mutable project root and CAS identity
│   ├── init/              standard templates and scaffold plans
│   ├── inspect/           component and compatibility reports
│   ├── test/              static and explicit runtime tests
│   ├── pack/              deterministic package archive
│   ├── import/            explicit native/legacy import plans
│   └── publish/           reviewed release and Directory handoff
├── scaffold/              reusable low-level template/file primitives
├── runtimecheck/          reusable language/runtime checks
└── legacyimport/          isolated read-only converter, never normal authoring

install/integrationctl/agentplugins/
├── domain/                canonical standard package envelope
├── conformance/           bounded decoding and normative spec facts
├── adapters/loader/       standard loader
├── adapters/specregistry/
├── policy/                installer loadability/security evaluation
├── planner/               client compatibility/install planning
├── providers/             client projections and activation
└── usecase/               installation lifecycle
```

The exact directory names are less important than these dependency rules:

```text
authoringcli -> small authoring service interfaces
authoring services -> conformance facts + author policy + low-level infrastructure
installer loader/policy -> conformance facts, never authoring packages
installer CLI -> installer services + shared conformance facts
standard domain/conformance -> no Cobra, mutation, client, or publication code
legacy importer -> standard authoring plan
standard authoring -> never imports legacy manifest packages
```

Shared decoding and normative diagnostics live beside the canonical standard
domain, not under `cli/.../internal/authoring`. Otherwise the installer would
need to import an authoring layer and the dependency direction would invert.
The existing loader remains the installer-facing facade while its bounded
decoder is extracted underneath it; this lets installer behavior remain stable
during the refactor.

Avoid a single large `AuthoringBackend` interface. Keep command-specific ports
so `init`, `test`, and `publish` do not depend on methods they do not use.

## Standard project model

Introduce a thin authoring value around the canonical `PackageEnvelope` rather
than a second manifest model. The value must distinguish the mutable source
tree from the sealed snapshot that was actually parsed.

Illustrative shape:

```go
// Illustrative only.
type Project struct {
    SourceRoot       string
    SnapshotRoot     string
    Package          domain.PackageEnvelope
    SourceTreeDigest string
    Git              GitIdentity
}
```

Rules:

- Read-only commands acquire a private snapshot and use the existing loader.
- The reader requires `Package.FormatID == FormatIDAgentPluginsV1`; an OpenAI
  compatibility or legacy envelope is never accepted through this path.
- A local authoring snapshot carries no Directory trust claim and no catalog
  evidence. Remote/catalog evidence cannot make a mutable project authoritative.
- `SnapshotRoot` is private and immutable for the operation; reports expose
  package-relative paths and digests, not the temporary path.
- Mutating commands first resolve and validate the real package root, then
  compute a plan against exact source digests.
- Before applying a mutation, compare the affected source files with the
  digests in the plan.
- Refuse the mutation if any affected input changed.
- Never recursively guess a package root. The positional path is the package
  root. Monorepo users pass the plugin subdirectory explicitly.
- Never follow a symlinked package root for mutation.
- Reject files whose resolved path leaves the project root.
- Do not fetch a schema at runtime. Use the embedded schema registry.
- Always close file descriptors and remove the exact private snapshot on
  success, validation failure, cancellation, panic recovery, or disk error.

`PackageEnvelope` remains the parsed standard/component truth, but it is not a
complete editable filesystem model. Archive selection, tracked/untracked file
inventory, file modes, and mutation preconditions stay in narrow authoring
types rather than being added to the installer domain.

## Public command model

### Primary end-user lifecycle

Unchanged:

```bash
agentplugins search <query>
agentplugins add <name-or-source>
agentplugins update <installation>
agentplugins repair <installation>
agentplugins remove <installation>
agentplugins switch <installation>
agentplugins rebind <installation>
agentplugins migrate-format <installation>
agentplugins migrate-state
agentplugins list
agentplugins info <installation-or-package>
agentplugins validate <local-or-full-sha-source>
agentplugins outdated
agentplugins doctor [installation]
agentplugins version
```

### Standard-first authoring

```bash
agentplugins author init <name>
agentplugins author validate [path]
agentplugins author inspect [path]
agentplugins author compat [path] --target <clients>
agentplugins author doctor [path]
agentplugins author bootstrap [path]
agentplugins author test [path]
agentplugins author dev [path]
agentplugins author generate [path] --target <clients> --output <dir>
agentplugins author normalize [path]
agentplugins author skills <subcommand>
agentplugins author import native <source-path> --from <client> --output <new-path>
agentplugins author migrate project <legacy-path> --output <new-path>
agentplugins author export [path]
agentplugins author bundle <subcommand>
agentplugins author publish [path]
agentplugins author capabilities
agentplugins author version
```

The same authoring subcommands are exposed at the root of `plugin-kit-ai`:

```bash
plugin-kit-ai init <name>
plugin-kit-ai validate [path]
plugin-kit-ai inspect [path]
plugin-kit-ai migrate project <legacy-path> --output <new-path>
```

Every command in the authoring list, including migration, is built from the
same command/service factory for both entrypoints. The shorter excerpt above is
illustrative, not a smaller `plugin-kit-ai` feature set.

### Command parity decisions

The release surface follows the phase gates below. A deferred command is absent
from current help; it is not shipped as a success-shaped roadmap stub.

| Existing command | Standard-first result | First availability |
| --- | --- | --- |
| `init` | Create root `plugin.json` and selected standard components | MVP, Phase 3 |
| `validate` | Use the shared standard decoder and authoring policy | MVP, Phase 2 |
| `inspect` | Show manifest, components, diagnostics, runtime requirements, and target compatibility | MVP, Phase 4 |
| `compat` | Plan support for explicit clients without touching their configuration | MVP, Phase 4 |
| `capabilities` | Report engine/schema/client capabilities from generated source metadata | MVP, Phase 4 |
| `doctor` | Inspect project toolchain and local runtime readiness without mutation | MVP, Phase 4 |
| `test` | Run static tests by default; add explicit runtime modes later | Static in MVP Phase 5; runtime in Phase 7 |
| `skills` | Create and validate Skills under standard `skills/`; remove external npm pass-through lifecycle from authoring | MVP, Phase 5 |
| `dev` | Watch relevant package files and rerun the standard cycle | Phase 7 |
| `bootstrap` | Install only dependencies of recognized generated templates using lockfiles | Phase 7 |
| `generate` | Render disposable client projection previews; never create an alternate source manifest | Phase 9 |
| `normalize` | Canonically format supported JSON while preserving values and unknown data | Phase 8 |
| `import` | Convert an explicit native source path into a separate standard package plan | Phase 8 |
| `migrate project` | Convert only the explicit canonical v1 legacy source into a separate standard package | Phase 8 |
| `export` | Create a deterministic standard package archive | Phase 9 |
| `bundle` | Keep safe archive inspection/fetch only where it adds value; move publication under `publish` and retire the separate install lifecycle | Phase 9, narrowed |
| `publish` | Validate and package, then perform one explicitly selected publication destination | Phase 10 |
| `publication` | Fold useful diagnostics into `publish inspect`; retire old marketplace materialization model | Replaced in Phase 10 |
| `install` | Current verified third-party binary downloader is not portable plugin installation; retire from authoring or move to a separately named developer utility | Removed from v2 current help |
| `integrations` and root lifecycle aliases | Remove from authoring; use `agentplugins` lifecycle | Removed from v2 current help |

Preserving a command name does not justify preserving old target-oriented
semantics. Removed behavior must fail with a precise replacement command, not a
silent alias that changes unrelated state.

Parity means parity of useful author jobs, not preservation of every flag or
pass-through. In particular:

- `skills init` and `skills validate` move to standard package services;
- old `skills generate` becomes part of explicit client projection preview;
- old external `skills install/list/update/remove` wrappers do not belong under
  authoring and must not pull an npm CLI into the new engine;
- old `install [owner/repo]` downloads release binaries and is not equivalent to
  `agentplugins add`; it must not be relabelled as Agent Plugin installation;
- removed commands remain documented in the v1 migration page, while current
  help points to the exact replacement where one exists.

## Template contract

Initial templates should be job-first and standard-first:

```text
skill
mcp-remote
mcp-stdio
hybrid
```

Example:

```bash
plugin-kit-ai init docs-skill --template skill
```

For a complete non-interactive invocation, component-specific inputs are
explicit:

```bash
plugin-kit-ai init docs-helper --template mcp-remote \
  --url https://docs.example.com/mcp
plugin-kit-ai init local-helper --template mcp-stdio --runtime node
plugin-kit-ai init hybrid-helper --template hybrid \
  --mcp-template mcp-remote --url https://docs.example.com/mcp
```

Generated files:

- every template: `plugin.json`, `README.md`, `.gitignore`;
- `LICENSE` and manifest `license`: only when an explicit `--license` choice was
  made; the tool must not assign copyright terms on the author's behalf;
- `skill`: `skills/<name>/SKILL.md`;
- `mcp-remote`: requires an absolute supported `--url` in non-interactive mode
  and creates `mcp.json` with one streamable HTTP server;
- `mcp-stdio`: requires an explicit supported runtime in non-interactive use and
  creates `mcp.json`, a language-native minimal server project, locked
  dependency inputs, and a package-relative launcher when bundled;
- `hybrid`: one Skill and one explicitly selected remote or stdio MCP template
  without client-native projections.

Template rules:

- use the canonical 1.0.0 `$schema` identifiers;
- default plugin version to `0.1.0` even though only `$schema` and `name` are
  required by the core schema;
- never silently normalize identity: suggest a valid lowercase name, require
  confirmation interactively, and fail with the suggestion non-interactively;
- do not copy Git user name/email into `author` without explicit flags or
  confirmation;
- never embed credentials, tokens, real account IDs, or OAuth config;
- never create a fake endpoint or unresolved template token and then report the
  package authoring-ready; missing required template values prompt only on a
  TTY and fail with the exact flag in non-interactive/JSON mode;
- use `${PLUGIN_ROOT}` and `${PLUGIN_DATA}` only in fields allowed by the
  specification;
- do not assume ambient environment variables unless `mcp.json` supplies them;
- do not emit root `hooks/`;
- keep client-specific examples behind an explicit namespaced-extension flag;
- ensure a newly generated package passes the same shared standard decoder and
  authoring policy shipped in the binary.

For the MVP, standard-first `init` writes only to a destination that does not
exist. Supporting an already-created empty directory would require a separate
cross-platform transaction/lock contract and is not worth complicating the
first slice. The old broad `--force` overwrite behavior is removed. If a script
passes it, fail with a migration explanation; do not keep a misleading flag
that can overwrite content. A future explicit in-place mode needs its own
atomicity tests before release.

## Validation contract

`agentplugins author validate`, `plugin-kit-ai validate`, and the existing
read-only `agentplugins validate` must share one fact-gathering loader and
diagnostic model, but they do not use the same success policy:

- installer validation answers whether the package can be safely loaded and
  what components remain usable under the specification's failure boundaries;
- authoring validation answers whether the package itself conforms and is ready
  for the selected authoring operation;
- strict release validation adds deterministic packaging and repository policy
  without redefining standard conformance.

The result model exposes these states separately:

```text
loadable -> conformant -> authoring_ready -> release_ready
```

A later state implies the earlier states. The reverse is never assumed.

Illustrative result shape:

```go
// Illustrative only.
type ValidationReport struct {
    Format         string
    Package        *domain.PackageEnvelope // absent after a fatal core parse
    Loadability    PolicyResult
    Conformance    PolicyResult
    Authoring      PolicyResult
    Installability map[domain.ClientID]PolicyResult
    Release        *PolicyResult // only when requested
}
```

Policy results reference one deduplicated diagnostic list by stable IDs. A
stricter layer may add a diagnostic, but cannot change the severity/category of
a normative diagnostic produced below it.

Validation layers:

1. Package root and filesystem containment.
2. `plugin.json` parsing and schema identity.
3. `mcp.json` parsing, version match, and per-server diagnostics.
4. Skills discovery and Agent Skills validation.
5. Extension container/value shape required by the core schema, without
   interpreting unknown namespace contents.
6. Authoring hygiene warnings that do not alter conformance.
7. Optional explicit target compatibility reporting.

Strictness:

- core manifest errors reject the plugin;
- an invalid MCP document disables MCP but does not erase valid Skills;
- an invalid MCP server skips that server but preserves other servers;
- an invalid Skill skips that Skill but preserves other components;
- unsupported extension namespaces remain opaque;
- unknown top-level fields follow the exact spec behavior and remain available
  in lossless raw data;
- JSON duplicate keys are rejected before map decoding;
- legacy YAML content is never opened or parsed during standard validation;
- the positional path is always the exact standard package root; validation
  never searches parents, descendants, or sibling repositories for a manifest;
- if the exact selected root contains both root `plugin.json` and the canonical
  v1 legacy path `plugin/plugin.yaml`, authoring reports
  `legacy_manifest_ignored`; strict release policy fails so the legacy source is
  not accidentally shipped;
- if root `plugin.json` is absent but canonical `plugin/plugin.yaml` exists,
  validation still fails `missing_standard_manifest` and adds the exact
  migration command as a next action. File presence may be checked, but legacy
  YAML bytes are not read by the standard path.

The core specification calls for reverse-domain extension namespaces but does
not define a stricter portable regex or central registry. The authoring hygiene
layer may warn about implausible names; it must not invent a fatal naming rule.
Top-level directories are treated as extension directories only when they match
an extension key or a client namespace we explicitly implement. Other package
files remain ordinary opaque package content.

Unknown top-level fields and a non-object `extensions` value are important
special cases: the Agent Plugins client must report/ignore them and may continue
loading valid components, but the authored package is not schema-conformant.
Likewise, invalid `mcp.json`, one invalid MCP server, or one invalid Skill follows
the standard's narrow loading boundary while still making the corresponding
authoring conformance check fail. Human output and exit codes must not collapse
"loaded with skipped content" into "fully valid".

Plugin `version` is optional and Semantic Versioning is recommended, not
required, by Agent Plugins 1.0. General validation therefore never rejects a
non-SemVer string solely for its syntax. A publication destination may impose a
stricter version/tag policy, but reports that as publication policy rather than
standard conformance.

Human and JSON output must distinguish:

- standard conformance;
- authoring hygiene;
- client compatibility;
- local toolchain readiness;
- runtime test evidence.

Passing one category must never imply another.

## Inspect and compatibility contract

`inspect` is read-only and reports:

- package name/version/schema;
- exact package root and tree digest in JSON output;
- Skills, MCP servers, transports, and invalid components;
- extension namespaces without dumping sensitive values;
- runtime executable requirements;
- warnings for visible credential-like data;
- compatibility per requested target;
- which behavior is standard, projected, manual, unsupported, or untested;
- recommended next commands.

`compat` uses the existing client registry and planning capabilities but does
not require installed clients and never writes client configuration. An
explicit `--target` means "evaluate these clients", not "install into these
clients".

Compatibility must be derived from component requirements and adapter
capabilities. It must not be stored as a target list in `plugin.json`.

## Doctor and bootstrap contract

Authoring doctor is separate from installation doctor:

```text
agentplugins doctor               installation/client health
agentplugins author doctor .      source project/toolchain health
plugin-kit-ai doctor .            same authoring health
```

Authoring doctor is read-only. It may inspect:

- supported schema version;
- language-native project files;
- required executable availability and version;
- lockfile presence;
- bundled launcher existence and executable mode;
- local path containment;
- expected build outputs for templates generated by the tool.

It must not:

- install dependencies;
- launch an agent;
- connect to remote MCP services;
- read credentials from client stores;
- mutate caches or the project.

`bootstrap` is explicit mutation. It is supported only for recognized
standard templates and deterministic language-native managers:

- Node.js: use the existing lockfile and a frozen/clean install mode;
- Python: create a project-local isolated environment from a locked input when
  the generated template provides one;
- Go: download/build through the declared module and pinned toolchain policy;
- unknown/custom projects: print the detected native command and stop unless
  the user explicitly selects it.

Recognition must not depend on a hidden tool manifest or template marker.
Bootstrap may proceed automatically only when standard component references,
language-native files, and a supported lockfile form one unambiguous plan. If
multiple runtimes or package managers are plausible, require an explicit
`--runtime`/`--manager` choice and show the exact command before mutation.
Dependency-manager cache/config homes are operation-local or project-local;
bootstrap must not read a user's npm, pip, Go, agent, or credential config by
default. It may use the network only as the visible consequence of this
explicit command, under the manager's locked/frozen mode, and reports the exact
manager/runtime selected.

Generated dependencies must be selected from a currently supported stable
release, checked against the dependency's official source when the template is
updated, and committed through the language-native lockfile or checksum policy.
Runtime commands use locked versions and never float to `latest` during normal
bootstrap.

Never infer and execute arbitrary scripts merely because a file is named
`package.json`, `Makefile`, or `pyproject.toml`.

## Test and dev contract

Default `test` is safe and offline:

- run validation;
- validate all Skills;
- validate MCP server configuration and local executable containment;
- compare generated template fixtures where present;
- run no agent, model, OAuth, browser, or remote service.

Runtime testing is opt-in and server-specific:

```bash
plugin-kit-ai test . --mcp-server local-docs --handshake
plugin-kit-ai test . --mcp-server remote-docs --handshake --allow-network
```

Runtime test rules:

- use a new temporary `PLUGIN_DATA` directory;
- use a minimal credential-free environment by default;
- never inherit client auth stores;
- never run inside a real user project;
- supervise child processes with deadlines and complete cleanup;
- distinguish process start, MCP initialize, tool listing, and actual tool
  execution as separate evidence levels;
- do not call tools unless an explicit fixture requests them;
- require an explicit network flag for non-loopback endpoints;
- redact headers and environment values from diagnostics;
- retain sanitized failure metadata, not raw secrets.

`dev` reuses the same validation and test services. It watches only the selected
package root, uses bounded debounce, ignores its output directory, and cancels
the previous cycle before starting another. It must not watch the whole home or
workspace recursively when a narrower package root is available.

## Generate contract

Agent Plugins packages do not need client manifests checked into the source
tree. `generate` is retained for authors who want to inspect derived client
layouts.

```bash
plugin-kit-ai generate . --target claude,codex --output ./dist-preview
```

Rules:

- explicit targets are required in non-interactive use;
- output must not be the package root or any ancestor of it;
- generation uses the same projection code or pure renderers as installation;
- no installed client detection is required;
- no client config or installer state is changed;
- output is marked derived and includes source tree/schema identity;
- output is reproducible for the same package and target capability profile;
- generated projections are excluded from `pack` by default;
- preview success is not runtime or activation evidence.

Do not create fake `DetectedClient` values merely to drive production mutation
providers. Extract or reuse a pure projection boundary. If a provider cannot be
made pure in the bounded PR, `generate` should report that target as
`preview_unavailable` rather than writing through a fake user environment.

## Normalize contract

Normalization is optional and explicit. It may format:

- `plugin.json`;
- `mcp.json`;
- generated template-owned JSON files.

`normalize` does not rewrite `SKILL.md` in the first implementation. Preserving
YAML comments, quoting, key order, line endings, and Markdown boundaries is not
worth a second round-trip document model. `skills validate` reports actionable
format problems; a later formatter needs its own byte-preservation fixtures.

Rules:

- plan and show affected files before mutation in interactive mode;
- never remove unsupported extension data;
- never repair an invalid schema by guessing;
- never change semantic JSON value types;
- reject duplicate keys rather than choosing a winner;
- use compare-and-swap against the read digest;
- write through a same-directory temporary file, sync, and atomic rename where
  supported;
- preserve permissions unless the standard requires tightening them;
- fail without partial writes when one planned file changes concurrently.

## Import and migration contract

Two explicit import modes are allowed:

```bash
plugin-kit-ai import native ./claude-config.json --from claude \
  --output ./standard-project
plugin-kit-ai migrate project ./legacy-project --output ./standard-project
```

Native import:

- requires an explicit source path, client format, and missing output path;
- never discovers or opens an ambient client profile merely from `--from`;
- recognizes documented MCP and Skills surfaces only;
- generates a reviewable plan before writing;
- does not read OAuth credentials, tokens, cookies, or secret stores;
- replaces secret values with named placeholders only when the author confirms
  the mapping;
- otherwise skips the credential-bearing entry and reports it; it never copies
  a literal secret, fabricates a portable credential reference, or silently
  removes authentication requirements;
- records unsupported native behavior as diagnostics;
- never claims portable support for hooks, rules, commands, agents, or other
  client-specific features unless represented by an explicit compliant
  extension namespace.

Legacy project migration:

- is the only new code allowed to read the current canonical legacy manifest
  `<legacy-project-root>/plugin/plugin.yaml`;
- lives behind a narrow migration-only package boundary;
- requires the positional argument to be the legacy project root and does not
  recursively search for YAML manifests;
- writes to a separate destination that must not exist; it stages and renames
  exactly like standard-first `init`;
- never deletes or rewrites the source project;
- copies identity/metadata only when valid under the Agent Plugins schema;
- converts portable MCP and Skills when the mapping is exact;
- reports every unconverted target, hook, launcher, publish channel, and
  runtime assumption;
- never silently drops unsupported behavior;
- validates the generated package with the standard loader;
- emits a machine-readable migration report with source and output digests;
- does not install the result.

If root `plugin.json` and `plugin/plugin.yaml` both exist, migration still
requires an explicit `--from legacy-plugin-yaml` source choice. Normal standard
authoring never falls back. Historical layouts outside the canonical v1 path
are unsupported until a real fixture justifies an explicit importer; no broad
recursive YAML discovery is added.

## Export, bundle, and archive contract

`export` creates a deterministic archive of the standard package root.

Safe default:

- use Git-tracked content from an explicit commit when the project is in Git;
- reject a dirty tree unless `--from-working-tree` is explicit;
- exclude `.git`, editor state, dependency caches, and preview output;
- fail closed when tracked package content matches credential/secret policy;
  do not silently omit a tracked source file and publish a broken package;
- ignore untracked local secret files, but report their paths in redacted form
  when `--from-working-tree` would otherwise make package selection ambiguous;
- fail on symlinks or entries that resolve outside the package root;
- normalize archive paths, timestamps, owners, and ordering;
- preserve only required executable bits;
- emit checksums and provenance beside the archive as release metadata, never
  as another manifest inside the package root or portable archive;
- validate the extracted archive again before success;
- produce the same digest for identical package content across supported OSes.

The specification permits clients to resolve package symlinks that remain
inside the plugin root. Our deterministic cross-platform archive policy may be
stricter and reject all symlink archive entries, but that is a release-policy
diagnostic, not a claim that the source package violates Agent Plugins 1.0.
Source validation must still report an actual containment violation separately.

`bundle inspect` may inspect a local archive without extraction outside a
private temporary root. A narrowed `bundle fetch` may download a checksum-bound
archive into an explicit output path and inspect it, but does not install it.

The old `bundle install` and `bundle publish` workflows are not copied into the
new engine:

- `bundle publish` becomes `publish github` when the new publication contract is
  available;
- `bundle install` fails with an exact migration message until `agentplugins
  add` itself supports that archive/source type through the one transactional
  lifecycle;
- no authoring command implements a parallel install state, update path, or
  removal path.

## Publish contract

Publication is outside the portable package standard, so it remains an
explicit authoring workflow rather than manifest metadata.

Initial destinations:

1. GitHub release attached to an exact repository commit;
2. prepared submission to Universal Agent Plugins Directory.

Local packaging remains `export`; `publish` does not add a second name for a
filesystem-only operation.

Destination version policy is explicit:

- local `export` does not require `plugin.json.version`;
- GitHub release publishing requires an explicit immutable tag; it may derive a
  tag from `version` only when the value is valid SemVer and the user selected
  that behavior;
- Directory submission binds the exact repository revision and package digest;
  an optional plugin version remains metadata unless Directory policy states a
  stronger requirement.

Every publish command follows:

```text
validate strict -> test required static gates -> pack -> verify extracted pack
-> display destination and immutable identity -> explicit confirmation -> act
-> verify resulting remote identity
```

The displayed plan has a canonical digest bound to the source commit, package
tree/archive digest, destination, tag or submission identity, and provider.
Interactive confirmation accepts only that current plan. Non-interactive/JSON
execution requires an exact `--confirm-plan <digest>` value rather than a broad
`--yes`. The service rechecks source and destination preconditions immediately
before the first effect.

Rules:

- no credentials in `plugin.json` or `mcp.json`;
- `--dry-run` performs no remote mutation and returns the confirmation digest;
- no hidden default repository inferred from unrelated Git remotes;
- do not publish from a dirty tree by default;
- do not overwrite an existing version/tag/artifact;
- run the existing package security scan as a publication-policy gate, separate
  from standard conformance, and bind its result to the exact package digest;
- the authoring command may open/update one deterministic Directory submission
  PR but never merges it directly;
- registry-owned protected automation may conditionally auto-merge that PR only
  under its independently reviewed trust/policy gates; suspicious or incomplete
  evidence always requires manual review;
- do not claim Directory review, client runtime, OAuth, or upstream ownership
  based only on a successful upload;
- use existing protected release and provenance infrastructure where its
  identity model matches the standard package;
- keep registry-specific compatibility and trust metadata in the registry,
  not in the package manifest.

## Distribution and versioning

### Product versions

- `agentplugins` continues its own pre-1.0 release sequence.
- `plugin-kit-ai` changes authoring format and command semantics, so the first
  standard-first release is `2.0.0`.
- Both release artifacts record the exact source commit and authoring engine
  build identity.
- The two product versions need not be numerically equal.
- Historical release metadata that names `777genius/plugin-kit-ai` remains
  immutable. New release producers use the canonical
  `777genius/universal-agent-plugins` identity. Changing repository identity in
  checksums, launchers, or provenance is a reviewed release migration with an
  isolated install/update/uninstall smoke, never a string replacement across
  historical artifacts.

### Binary strategy

Final distribution uses two tiny native Go mains linked against the same shared
authoring packages:

- `agentplugins` includes the authoring tree under `author`;
- `plugin-kit-ai` mounts that tree at its root;
- both binaries are built from one source commit and embed the same authoring
  engine revision;
- neither binary invokes, downloads, or parses output from the other;
- release wrappers download only the native asset matching their product name
  and verify its checksum/provenance.

This removes an unnecessary runtime wrapper decision while preserving separate
product versions and packaging channels. A filesystem symlink or argv-name
dispatch may be used only as a platform packaging optimization after tests prove
install, upgrade, uninstall, help, signals, and product-version reporting; it is
not the baseline architecture.

### npm, Homebrew, and PyPI

- `universal-agent-plugins` remains the main npm installer/manager package.
- `plugin-kit-ai@2` becomes the standard-first authoring package.
- npm wrappers share download, cache, checksum, and provenance verification
  code.
- Homebrew may expose both executable names from the same formula only after
  an install/uninstall collision test.
- The existing `plugin-kit-ai` PyPI launcher must not silently switch to a
  different binary without a major release and matching provenance metadata.
- `plugin-kit-ai-runtime` is audited separately. It is not part of the Agent
  Plugins portable standard and must not be required by default templates.
- Do not create a new Python SDK merely to mirror Go code. Standard plugin
  runtimes should use language-native MCP SDKs unless our helper provides a
  distinct, tested capability.

Release cutover is explicit:

1. build both native binaries once from the same exact source commit;
2. verify checksums/provenance and run packed npm/PyPI/Homebrew launcher smoke
   in fresh disposable homes before publication;
3. publish the first complete standard-first release directly as stable
   `plugin-kit-ai@2.0.0`; no public beta is required, but the stable tag is not
   moved until all supported asset lanes pass;
4. publish the compatible `universal-agent-plugins` release from the same
   engine commit without forcing its product version to equal `2.0.0`;
5. read back registry metadata and install by exact version before changing
   npm `latest`, Homebrew metadata, PyPI guidance, or site examples;
6. if a launcher channel fails after publication, restore only its mutable
   channel pointer or formula to the last proven version. Never delete or
   overwrite an immutable package/version.

### Prepublication provenance and qualification order

This order clarifies the cutover above and governs subsequent worker contracts.
It specifies work to implement and verify; it is not evidence of release readiness.
Distinguish frozen-input provenance from release qualification:

1. Freeze both native products at the final accepted integration commit. A
   separate protected job may attest the exact prepared inputs and a fixed
   provenance-only record before execution. This grants no platform acceptance,
   publication permission, or channel eligibility. Preparation stays read-only;
   its candidate/projection/pair byte contracts remain unchanged.
2. Authenticate those inputs and package custody before prepublication execution.
   A reviewed production launcher may accept an explicit checksum-bound local
   asset source for this purpose. It must verify the selected product, target,
   source, outer asset and extracted binary through the normal checked cache
   path. Expected digests come from authenticated input provenance in the verified
   package binding, never from a caller-supplied asset-and-digest pair. Reject
   malformed inputs without fallback, and never execute the supplied
   file directly. This capability must be implemented and reviewed first;
   ordinary null-qualified preparation remains non-executable.
3. Pack the final npm tarballs once with the authenticated input binding. Keep
   final qualification and execution receipts outside those immutable bytes:
   frozen inputs -> input provenance -> tarballs -> stage receipt -> execution
   receipts -> release qualification -> distribution readbacks -> channels.
   Do not embed a future qualification digest in a tarball whose execution
   receipt must itself be included in that qualification.
4. Require all twelve native product/target lanes and authenticated public-packed
   acceptance before qualification signing or native release publication. Public
   acceptance includes actual installed launchers, declared host/Node support,
   genuine production installer dry-run and lifecycle, preservation/parity, and
   the complementary ten-project/thirty-plan gate. Help or injected planner
   success cannot replace distributed installer success. Missing lanes remain
   unresolved; they never become skipped successes.
5. After qualification and both native release readbacks, install those identical
   npm tarballs from authenticated staging artifacts into fresh disposable homes,
   checking their recorded digests without packaging from a checkout. Before
   publishing either npm package, run their production launchers without the local
   asset source and verify anonymous native downloads from the canonical public
   GitHub release URLs. This is not a prepublication npm-registry download. Both
   publishers verify the staged tarball SHA256, SRI and SHA1 and publish that exact
   file without repacking. Require both registry provenance/readbacks before pair
   channel promotion. Preserve the repository-wide latest release policy needed
   by historical kit launchers; a non-default release is still a public effect.

Every admitted receipt binds the exact source and signer workflow revision,
completed run/attempt, artifact identity/digest, subjects and evidence closure.
Extract the same checked archive bytes. Provenance signatures authenticate input
custody, not future test results. Cryptographic verifier compatibility requires
real positive and negative evidence, not supplied fixture JSON.

No synthetic transport, manufactured cache/assessment qualification, test trust
flag, command/network-policy relaxation or provider-control workaround satisfies
these gates. Required process observation and genuine installer services remain
execution prerequisites. Preserve existing private/v1 contracts and the explicit
limitations of offline fixture evidence. Native, npm, PyPI, Homebrew and final
clean-clone gates remain required; this clarification neither reduces scope nor
opens phases 7-11 before the stable MVP release.

PyPI remains a launcher/distribution surface, not a second Python
implementation of the authoring engine. A source distribution must not contain
a divergent authoring path.

### Command-name collisions

Top-level `agentplugins validate` remains a package/installability check for
users of the manager. `agentplugins author validate` and `plugin-kit-ai
validate` are the authoring view over the same conformance service and add
authoring hygiene. Likewise, top-level `doctor` diagnoses installations and
clients, while `author doctor` diagnoses a source project. Help, examples, JSON
operation names, and errors must preserve that distinction.

### Machine-readable compatibility contract

New authoring commands use the existing one-document JSON envelope shape but a
distinct stable command identifier such as `author.validate`. Both entrypoints
emit the same command identifier and data schema for the same authoring job.

The shared payload includes:

- `authoring_schema_version`;
- authoring engine version and source revision;
- requested operation and effect mode;
- standard/package identities and typed diagnostics;
- result state and sanitized next actions.

`plugin-kit-ai version` and `agentplugins version` remain product-version
commands and may differ. Authoring payloads expose the common engine revision
instead of pretending the wrapper/product versions match.

Rules:

- `--format json` writes exactly one JSON document to stdout and never prompts;
- progress and human hints do not contaminate stdout;
- diagnostic codes and fields are stable within an authoring schema version;
- adding optional fields is compatible, while removing/changing meaning bumps
  `authoring_schema_version`;
- path fields are package-relative unless the caller explicitly requests local
  absolute paths;
- exit behavior is identical between entrypoints and is frozen by golden tests
  before public release; implementation must not invent a second exit-code
  mapping beside the existing CLI conventions.

## Implementation phases

## Phase 0 - Architecture and contract freeze

### Summary

Record the approved standard-first authoring decision before behavior changes.

### Detailed implementation steps

1. Add an authoring ADR that references ADR 0005.
2. Mark `TODO_AGENT_PLUGIN_AUTHORING.md` as decided and link to this plan/ADR.
3. Mark `PLUGIN_STANDARD_AND_PUBLISH_PLAN.md` and
   `PLUGIN_YAML_V1_SPEC.md` as historical, not current Agent Plugins guidance.
4. Record the forbidden dependency directions and classify every current
   command as reuse, adapt, migrate, or retire from the standard interface.
   Track implementation disposition separately as reuse, adapt, preserve/defer,
   or owner-approved removal; retiring a command never implies code deletion.
5. Capture current CLI `--help`, JSON outputs, release artifacts, and test
   baselines for intentional-diff review.

### Edge cases

- Existing docs link directly to old pages.
- Generated CLI docs are derived from the root command.
- GitHub repository redirects may hide stale package metadata.
- Existing automation may still publish `plugin-kit-ai` v1 artifacts.

### Tests

- documentation link check;
- CLI surface snapshot baseline;
- current installer test suite unchanged.

### Rollback / kill switch

Documentation-only revert. No release behavior changes.

### Acceptance criteria

- one approved ADR defines the source of truth and entrypoint relationship;
- no plan or current guide calls `plugin/plugin.yaml` the universal standard;
- current installer behavior remains unchanged.

## Phase 1 - Shared command construction

### Summary

Move reusable authoring command factories out of `package main` without changing
their current behavior.

### Detailed implementation steps

1. Create `internal/authoringcli`.
2. Move command construction and human/JSON rendering behind exported factory
   functions with injected command-specific runners.
3. Keep `cmd/plugin-kit-ai/main.go` as composition only.
4. Add an `author` placeholder to the `agentplugins` root behind internal build
   wiring, but do not expose incomplete commands in a release.
5. Generate fresh command instances for both roots.
6. Add an explicit root-flag adapter: copy only supported inherited values into
   immutable authoring options and reject changed installer-only flags.
7. Keep existing installer persistent-flag placement unchanged in this phase;
   do not combine command extraction with a public parsing migration.
8. Add static import-boundary tests so new standard authoring packages cannot
   import legacy manifest/application packages.

### Edge cases

- Cobra parent reuse and persistent flag inheritance.
- Different stdout/stderr/input streams in tests.
- cancellation context and signal propagation.
- command usage prefixes in error/help text.
- hidden internal docs commands accidentally becoming public.
- installer-only persistent flags appearing in author help or silently changing
  author behavior;
- a failed command leaving option values in a reused command tree during tests.

### Tests

- old `plugin-kit-ai --help` parity before semantic conversion;
- fresh command tree can be constructed more than once;
- no shared mutable flag state between roots or tests;
- exit codes and JSON envelopes remain stable.
- legacy `agentplugins --target cursor add ...` parsing remains valid;
- explicit installer-only flags under `author` fail before runner invocation.

### Rollback / kill switch

Mechanical revert to the old command location. No manifest behavior changes.

### Acceptance criteria

- both roots can mount independently constructed authoring command trees;
- command code is no longer trapped in `package main`;
- installer lifecycle tests remain unchanged.

## Phase 2 - Standard project reader and shared validation

### Summary

Make `PackageEnvelope` the only model consumed by new authoring reads.

### Detailed implementation steps

1. Freeze current installer loader behavior with golden failure-boundary and
   installability fixtures before extraction.
2. Add a local project reader that creates a bounded private snapshot and
   invokes shared standard decoding. Reuse the existing local acquirer where its
   policy matches; do not let a stricter installer/archive rejection erase the
   normative conformance report.
3. Audit and classify current loader/acquisition failures by policy layer.
4. Extract bounded standard document/component decoding below installer policy
   so conformance and installability can share parsed facts without sharing
   success criteria.
5. Keep all current installer safety checks in an explicit installability
   policy before any planner/provider effect.
6. Extract the current `agentplugins validate` result construction into a
   reusable validation service with separate policy reports.
7. Add duplicate-key detection before standard JSON decoding if not already
   guaranteed by the shared decoder.
8. Add authoring hygiene diagnostics as a separate layer.
9. Wire the service into:
   - `agentplugins validate`;
   - `agentplugins author validate`;
   - `plugin-kit-ai validate`.
10. Keep installer source acquisition and remote exact-SHA validation unchanged.

### Edge cases

- missing/empty root;
- package root symlink;
- symlinked `plugin.json`, `mcp.json`, or `skills/`;
- unreadable files and permission changes during snapshot;
- malformed UTF-8 or JSON;
- duplicate keys;
- mismatched plugin/MCP schema versions;
- unsupported future schema;
- unknown fields and extensions;
- a spec-valid name or contained source link rejected by stricter install or
  cross-platform release policy;
- both root `plugin.json` and canonical `plugin/plugin.yaml`;
- package in a monorepo subdirectory;
- cancellation during snapshot.

### Tests

- existing official loader tests;
- entrypoint parity fixtures;
- table-driven classification of every fatal loader/acquisition diagnostic;
- spec-conformant but non-installable/non-exportable fixtures;
- exact failure-boundary fixtures;
- no network during local validation;
- no mutation of source, installer state, cache, or clients;
- JSON result stability and secret redaction.

### Verification commands

```bash
go test ./install/integrationctl/agentplugins/...
go test ./cli/plugin-kit-ai/internal/agentpluginscli/...
go test ./cli/plugin-kit-ai/internal/authoring/...
go test ./cli/plugin-kit-ai/internal/authoringcli/...
```

### Rollback / kill switch

Keep old authoring commands unreleased until standard validation passes both
entrypoints. Installer validate remains independently revertible.

### Acceptance criteria

- identical standard package produces equivalent results from all validation
  entrypoints;
- normative conformance and stricter install/release policy are independently
  visible and cannot overwrite one another;
- no standard validation code imports legacy manifest packages;
- component-level failures remain isolated correctly.

## Phase 3 - Standard-first init vertical slice

### Summary

Create valid standard packages without `plugin/plugin.yaml`.

### Detailed implementation steps

1. Split low-level scaffold planning/render/write utilities from old template
   selection.
2. Add the four standard templates.
3. Make scaffold output a pure plan before applying it.
4. Validate the plan paths and collisions.
5. Render into a private sibling staging directory and install the missing
   destination with one rename. If the destination appears before that rename,
   lose the race cleanly and leave it untouched.
6. Validate the generated package with the same shared decoder and authoring
   policy used by public `validate` before reporting success.
7. Print the shortest useful next steps.

### Edge cases

- invalid plugin/skill/server names;
- case-insensitive path collisions;
- Windows reserved names and separators;
- destination already exists;
- two concurrent init commands target the same destination;
- interrupted write;
- disk full;
- permission-denied parent;
- template rendering bug;
- executable modes on Windows versus POSIX;
- unavailable requested runtime;
- missing remote URL, runtime, or hybrid MCP choice in non-interactive mode;
- name normalization changing identity without confirmation.

### Tests

- golden tree for every template and supported runtime lane;
- every generated tree validates;
- no generated tree contains `plugin/plugin.yaml` or root `hooks/`;
- failure leaves no partial destination;
- any existing destination, empty or non-empty, remains byte-identical;
- concurrent destination creation/change loses cleanly without replacing it;
- cross-platform path and mode tests.

### Rollback / kill switch

Keep `init` absent from release builds until every generated template validates.
A revert removes only the new command/templates; it does not affect validation
or installation. Failed init removes only its exact private staging directory.

### Acceptance criteria

```bash
plugin-kit-ai init demo --template skill
plugin-kit-ai validate demo
agentplugins author validate demo
```

All commands succeed, both validators report the same normative package facts
and digests, authoring adds no blocking hygiene finding, and no legacy manifest
exists.

## Phase 4 - Inspect, compatibility, capabilities, and doctor

### Summary

Complete the safe read-only authoring loop.

### Detailed implementation steps

1. Replace old inspection renderers' direct `pluginmanifest` dependency with a
   standard inspection result.
2. Reuse client capability metadata to produce explicit compatibility results.
3. Separate standard conformance, compatibility, runtime readiness, and
   evidence in output.
4. Adapt reusable runtime checks to language-native files and standard MCP
   requirements.
5. Update generated command/support docs from the new command tree.

### Edge cases

- package with only Skills or only MCP;
- unsupported MCP transport for a requested client;
- missing local executable;
- remote MCP requiring OAuth;
- client extension unknown to our CLI;
- client installed locally versus merely supported by the adapter;
- compatibility changes with client version;
- sensitive fixed headers in malformed input.

### Tests

- matrix fixtures for Skills/MCP/transports/extensions;
- no installed client required for explicit compatibility;
- no secret values in human or JSON output;
- authoring doctor performs zero writes and process launches;
- help and docs generation parity.

### Rollback / kill switch

The read-only commands are independently removable from the public command
tree. Capability metadata remains shared with the installer; do not roll it
back by duplicating or freezing a second author-only registry.

### Acceptance criteria

- a new author understands what the package contains and where it can work;
- compatibility never claims runtime/OAuth evidence it does not have;
- read-only commands do not touch user client configuration.

## Phase 5 - Static test and Skills authoring

### Summary

Complete a useful offline authoring loop without executing package content.

### Detailed implementation steps

1. Split static component tests from executable runtime tests.
2. Make default `test` compose conformance, authoring hygiene, static Skills,
   and MCP configuration checks without resolving or starting executables.
3. Reuse Skills init/validate logic under the package `skills/` root.
4. Exclude external npm Skills lifecycle wrappers from standard authoring
   command wiring; preserve their implementation and tests under the capability
   inventory contract. Document an independent replacement only when it exists.
5. Record the embedded Agent Skills profile identity in JSON results.

### Edge cases

- Skill directory/frontmatter name mismatch;
- nested `SKILL.md` that is not an immediate child of `skills/`;
- malformed YAML, duplicate fields, aliases, and oversized frontmatter;
- scripts, references, or assets escaping the Skill/package root;
- experimental `allowed-tools` interpreted as portable authorization;
- a newer Agent Skills rule set than the one embedded in the binary;
- local executable missing while static MCP configuration remains valid.

### Tests

- canonical Agent Skills fixtures and failure-boundary fixtures;
- test does not invoke process, DNS, socket, browser, agent, or credential
  stores;
- symlink/containment and bounded parsing regressions;
- Skills init collision and atomic-write tests;
- identical static results through both entrypoints.

### Rollback / kill switch

Static test and Skills authoring are separate command registrations. Disable a
broken authoring mutation without disabling shared Skill validation or the
installer's ability to load otherwise usable package components.

### Acceptance criteria

- generated Skill and MCP templates pass the offline static test;
- static test remains process- and network-free;
- temporary snapshots are removed after success, failure, or cancel;
- Agent Skills validation provenance is visible and reproducible.

## Phase 6 - Dual-entrypoint MVP release and documentation

Delivery order: preserve the owner-approved early public wording checkpoint,
then qualify and publish the full dual-entrypoint MVP. Public wording alone does
not qualify the CLI, native assets, wrappers or later release-gated phases.

### Summary

Publish the proven core authoring slice through both command names before
building migration, advanced packaging, and remote publication features.

### Detailed implementation steps

1. Ship `agentplugins author` in native assets with only commands that meet the
   Milestone A gate.
2. Publish `plugin-kit-ai@2` as the standard-first authoring entrypoint from the
   same engine commit.
3. Do not register deferred commands in v2 help. Only names removed from v1 may
   have bounded `plugin-kit-ai` compatibility shims that fail before effects
   with an exact replacement or migration command. Never add generic roadmap
   stubs under `agentplugins author`.
4. Update Homebrew and supported launchers with shared provenance.
5. Rebuild docs around two first-level journeys:
   - Use plugins;
   - Build plugins.
6. Move old authoring docs into a historical migration section or remove them
   after redirects exist. Until Phase 8 ships, that page points legacy users to
   the exact last supported v1 command set (`1.2.4` at this plan's baseline) and
   says that v2 migration is not yet available; it must not document a future
   command as executable.
7. Update landing, README, package metadata, examples, and generated CLI docs.
8. Verify all public links use the canonical repository/site.
9. Treat English documentation as the canonical release surface. Update other
   maintained locales in the same release or replace stale command snippets
   with a clear link to the canonical page; never leave conflicting examples.

### Edge cases

- stale npm cache downloads the v1 binary;
- mismatched authoring engine versions between entrypoints;
- old PyPI package remains discoverable;
- Homebrew executable collisions during upgrade/uninstall;
- redirects hide broken docs links;
- generated docs expose internal commands;
- translated pages retain v1 commands after the canonical page moves to v2;
- Windows launcher quoting and signal forwarding;
- a removed v1 invocation lacks a useful replacement, while a truly deferred
  command is accidentally exposed before it works.

### Tests

- install both entrypoints in fresh isolated environments;
- run the same golden authoring flow through each;
- compare JSON output and resulting tree digests;
- upgrade/uninstall/reinstall smoke on supported OSes;
- npm provenance/checksum/cache tests;
- docs build, link checker, and landing smoke;
- v1-to-v2 invocation tests for every removed, renamed, or deferred command.

### Rollback / kill switch

Keep the last v1 artifacts immutable. If the v2 release or wrappers are broken,
move only the affected package channel/dist-tag back to the last proven release;
do not rewrite published artifacts or re-enable legacy parsing in a standard
package.

### Acceptance criteria

- both entrypoints execute the same authoring engine revision;
- Milestone A is usable without a registry, account, OAuth, or client install;
- the main docs contain no current workflow requiring `plugin/plugin.yaml`;
- a new user can distinguish install versus author in one screen;
- deferred commands are absent; bounded v1 error shims never claim success or
  mutate state.

## Phase 7 - Explicit runtime test, dev, and bootstrap

### Summary

Add process and dependency mutation only after the offline authoring MVP is
released and stable.

### Detailed implementation steps

1. Adapt existing process supervision to standard MCP server definitions.
2. Add explicit server, handshake, tool, network, fixture, and deadline flags.
3. Rebuild `dev` around the standard validation/test services.
4. Restrict `bootstrap` to unambiguous generated layouts and locked
   language-native dependency flows.
5. Keep runtime/dependency evidence separate from conformance and authoring
   readiness.

### Edge cases

- child process forks or ignores signals;
- stdout/stderr flooding, partial MCP frames, and hanging shutdown;
- remote redirects and credential forwarding;
- ambient proxy/auth variables and inherited client stores;
- dev watcher self-trigger loops and concurrent edits;
- two dev processes on the same project;
- package deletion while a cycle is running;
- multiple runtimes/package managers without an explicit selection;
- lockfile/runtime version drift and dependency install partial failure;
- ambient package-manager credentials, proxy settings, or global caches.

### Tests

- disposable local MCP fixture for initialize/list tools;
- timeout, crash, malformed output, signal, and cleanup regressions;
- network and tool calls denied by default;
- empty credential environment;
- watcher debounce/cancellation and single-project lock;
- Node/Python/Go bootstrap only in new isolated test projects;
- bootstrap uses disposable package-manager homes and never touches the real
  user cache or credential files;
- dependency failure leaves source files and lockfiles unchanged unless their
  exact planned update was explicitly selected.

### Verification commands

Use only repository fixtures and newly created temporary projects. Never point
these commands at a real user project or inherit a real agent profile.

### Rollback / kill switch

Runtime flags and `bootstrap` stay separately disableable. Disabling them does
not affect offline validate/inspect/test or installation lifecycle commands.

### Acceptance criteria

- generated stdio template passes an isolated MCP handshake;
- explicit network tests cannot forward credentials across origins;
- every process, process group, temp directory, and partial dependency staging
  root is removed after success, failure, timeout, or cancellation.

## Phase 8 - Normalize, import, and migration

### Summary

Add explicit controlled mutation and a one-way exit from legacy projects.

### Detailed implementation steps

1. Implement lossless JSON document editing helpers.
2. Add digest-bound mutation plans and atomic file replacement.
3. Adapt portable native MCP/Skills importers.
4. Add the narrow migration-only legacy reader.
5. Produce migration reports and unsupported-feature diagnostics.
6. Convert representative first-party legacy projects before broad migration.

### Edge cases

- mixed valid/invalid native entries;
- omitted, symlinked, or concurrently changed native source path;
- duplicate MCP names;
- secrets embedded in native config;
- absolute local paths;
- unsupported hooks and commands;
- two manifests present;
- source and output overlap;
- stale plan after concurrent edit;
- case/normalization collisions;
- partial cross-filesystem rename.

### Tests

- fixture corpus covering each old target type;
- explicit no-secret-copy assertions;
- unsupported behavior is never silently dropped;
- source tree remains byte-identical;
- generated standard package validates;
- rerun is deterministic or fails with a clear existing-output message.

### Rollback / kill switch

Keep normalize, native import, and legacy migration behind separate command
registrations. A failed or reverted mutation feature leaves standard read-only
authoring available; migration never modifies its source, so rollback removes
only the exact tool-owned output/staging root.

### Acceptance criteria

- every maintained first-party legacy example has either a valid migrated
  package or an explicit documented reason it cannot be an Agent Plugins 1.0
  package;
- normal authoring code does not import legacy packages;
- migration is the sole legacy reader.

## Phase 9 - Generate previews, export, and bundle

### Summary

Restore useful delivery workflows without reintroducing generated source
manifests.

### Detailed implementation steps

1. Extract a pure projection boundary from installer staging where practical.
2. Implement explicit-target preview generation.
3. Adapt safe archive inspection and deterministic export.
4. Reuse checksum and verified-fetch infrastructure.
5. Redirect archive installation to the one installer lifecycle.

### Edge cases

- target preview requiring unavailable client-version evidence;
- preview output inside source root;
- archive traversal, duplicate paths, device files, symlinks, hard links;
- archive bombs and unbounded metadata;
- dirty Git trees and untracked secrets;
- executable-bit differences across platforms;
- identical content producing different archives.

### Tests

- projection output matches installer projection for the same capability
  profile;
- zero user client/state mutations;
- deterministic archive digest on Linux/macOS/Windows where supported;
- malicious archive corpus;
- extracted archive validates with the standard loader.

### Rollback / kill switch

Preview and archive commands are independently disableable. No archive format
is accepted by `agentplugins add` until that installer path is separately
released and state/rollback tests pass; disabling author export never changes
existing installations.

### Acceptance criteria

- authors can preview supported clients without installing into them;
- exported bytes are reproducible and contain no legacy manifest;
- only `agentplugins` owns installation state.

## Phase 10 - Publish and Directory submission

### Summary

Connect standard packages to existing protected release and Directory workflows.

### Detailed implementation steps

1. Define a standard package publication plan/result contract.
2. Adapt GitHub release upload to exact commit and immutable asset identity.
3. Add Directory submission preparation using registry-owned metadata outside
   the package.
4. Reuse existing provenance, checksum, and fail-closed verification gates.
5. Keep remote mutation behind explicit confirmation and machine-readable
   dry-run.

### Edge cases

- version/tag already exists;
- package version absent or non-SemVer;
- repository metadata differs from actual origin;
- upstream fork versus publisher repository;
- package subdirectory in monorepo;
- source or destination identity changes after plan confirmation;
- GitHub API partial success or timeout;
- duplicate PR/submission;
- registry schema changes;
- publisher trust or ownership not established.

### Tests

- local fake release service and exact identity checks;
- external effects mocked in unit tests;
- one disposable repository E2E only after explicit approval;
- duplicate/idempotency and uncertain-result reconciliation;
- Directory submission validates in registry CI.

### Rollback / kill switch

Keep each remote publisher behind a destination-specific feature gate and
explicit confirmation. On uncertain remote results, disable new attempts and
reconcile by immutable identity; never retry a create blindly. Removing the
authoring publisher does not alter registry automation or already published
artifacts.

### Acceptance criteria

- a standard package can be packaged and submitted without adding registry
  metadata to `plugin.json`;
- remote uncertainty never reports success without reading back exact identity;
- the CLI performs no direct merge or ownership claim; any conditional merge is
  attributable to registry-owned protected policy and remains auditable.

## Phase 11 - Legacy isolation and capability preservation

### Summary

Detach the old model from standard authoring after parity and migration are
proven, while preserving useful legacy capabilities and their implementation.

### Preconditions

- first-party examples migrated;
- standard-first CLI and docs released;
- migration command released and tested;
- npm/Homebrew/PyPI transition documented;
- standard authoring/release CI does not require legacy manifests; isolated
  tests for preserved implementations may still use explicit legacy fixtures;
- repository-wide search classifies every remaining `plugin.yaml` reference as
  a preserved legacy implementation/design, canonical legacy path, historical
  fixture, or migration test;
- every capability has a reviewed preservation disposition before removal.

### Detailed implementation steps

1. Detach old templates and command wiring from standard authoring; preserve
   useful template source and tests in their documented legacy boundary.
2. Remove old lifecycle aliases from the authoring binary.
3. Complete the capability inventory with source, tests, consumers, mapping,
   preservation destination and support status. Preserve unresolved code.
4. Retain the isolated read-only importer plus useful legacy implementations,
   required dependencies, tests and design documentation outside the standard
   authoring graph. Remove only individually reviewed, owner-approved items in
   bounded PRs. No automatic deletion based on unused standard-first imports.
5. Mark `plugin-kit-ai` v1 npm/PyPI versions deprecated without deleting
   historical artifacts.
6. Audit old runtime consumers and preserve useful packages even when no
   maintained example currently calls them. Removal needs a recorded capability
   decision and explicit owner acceptance, not only an unused-import search.

### Edge cases

- hidden CI/generator imports;
- old examples referenced from external docs;
- Go module paths retained for compatibility;
- release scripts expecting both binaries;
- migration tests accidentally importing production legacy packages.
- removal of the migration reader while users still need to convert v1 projects.

### Tests

- repository-wide forbidden-import and forbidden-generated-file checks;
- clean build/test/package from a fresh clone;
- released standard-first smoke;
- standard authoring accepts legacy source only through explicit migration;
  preserved legacy tests remain isolated and do not create implicit fallback;
- old releases remain downloadable;
- retained implementations and their required tests remain in source;
- every deletion matches a reviewed inventory item explicitly accepted by the owner.

### Rollback / kill switch

Revert one bounded isolation or owner-approved removal PR. Do not restore the old authoring path inside a
new standard package. Historical binaries remain the fallback for an old
project while it is migrated.

### Acceptance criteria

- current source and releases contain no normal authoring dependency on
  `plugin/plugin.yaml`;
- explicit migration remains available through a narrow isolated reader until a
  future major version removes it under a separately announced support policy;
- every legacy capability has an explicit preservation/adaptation destination
  or owner-approved removal decision; unresolved cases remain preserved;
- `plugin-kit-ai` means standard-first authoring;
- `agentplugins` and `plugin-kit-ai` share one authoring implementation.

## Cross-cutting edge-case checklist

### Standard evolution

- Schema identifiers are exact and versioned.
- The loader registry chooses behavior by schema identity.
- A newer unknown schema fails as unsupported without network schema fetch.
- Unknown fields are not promoted into tool semantics.
- `normalize`, `pack`, and migration preserve supported opaque extension data.
- New spec support is added as a new loader/validator implementation, not a
  global conditional spread across commands.

### Filesystem and concurrency

- package root is explicit and real;
- no path traversal, symlink escape, junction, reparse point, device, or FIFO;
- case-insensitive path collision is rejected;
- read plans bind exact digests;
- writes use compare-and-swap and atomic replacement where supported;
- interrupted commands leave no partial success claim;
- output directories cannot overlap inputs;
- temporary roots use private permissions and are always cleaned;
- disk-full and permission failures preserve original files.

### JSON and text parsing

- duplicate keys rejected;
- invalid UTF-8 rejected;
- size/depth/count bounds applied before allocation-heavy parsing;
- number/string/bool types never coerced;
- schema and MCP version mismatch handled at the MCP boundary;
- Skill frontmatter parsing is bounded and does not execute content;
- diagnostics are stable and machine-readable.

### Secrets and network

- package validation never requires network;
- fixed MCP headers/env values are treated as visible package data;
- no ambient client credentials inherited by tests;
- output redacts values while retaining useful field locations;
- remote MCP testing requires explicit opt-in;
- redirects cannot forward credentials to a different origin;
- publish reads credentials only through the destination provider's supported
  auth mechanism and never writes them to package files.

### CLI behavior

- two entrypoints produce equivalent machine output;
- help text uses the correct invocation prefix;
- stdout is reserved for requested results and stderr for diagnostics;
- cancellation propagates to all child operations;
- JSON output is never mixed with prompts;
- non-interactive commands require explicit values instead of hanging;
- removed old behavior exits with a precise migration/replacement command;
- installer and authoring `doctor` remain unambiguous.

### Runtime and process safety

- default test launches nothing;
- explicit runtime tests use disposable directories and bounded processes;
- child process groups are reaped on timeout/cancel;
- stdout/stderr and MCP frames have hard bounds;
- executable resolution follows the Agent Plugins rules;
- bundled relative commands remain inside the package root;
- runtime evidence states exactly which stage passed.

### Client-specific extensions

- extension namespaces are reverse-domain names;
- unknown namespaces are ignored without deep validation;
- supported namespaces have isolated validators;
- namespace ownership is not inferred or claimed merely from its spelling;
- no stricter cross-client namespace regex is invented by the authoring tool;
- extension failure does not overwrite portable components;
- root `hooks/` is not generated as a portable component;
- client projections remain derived outputs.

### Migration

- no silent format detection or switching;
- source is never deleted;
- unsupported behavior is listed exhaustively;
- secrets and machine-local absolute paths are not copied silently;
- migrated output passes the current standard validator;
- migrated project is not installed automatically.

## Test strategy

### Unit tests

- command factories and flag isolation;
- standard project loading;
- template plans and rendered files;
- diagnostics and strictness;
- runtime requirement detection;
- deterministic archive generation;
- migration mappings and unsupported diagnostics;
- output redaction and JSON contracts.

### Contract tests

- official Agent Plugins schema fixtures;
- existing conformance adapter suite;
- both authoring entrypoints produce the same result contract;
- no standard authoring package imports legacy manifest code;
- no new template emits a legacy manifest;
- no authoring command mutates installer state or user client configuration.

### Integration tests

- `init -> validate -> inspect -> test -> export -> validate extracted`;
- `init -> agentplugins add . --target <fixture-client> --dry-run` against
  isolated homes and target roots;
- standard package compatibility across supported adapters;
- native import and legacy migration fixture journeys;
- npm/Homebrew/PyPI launchers in disposable homes.

### Runtime E2E

- only new sandbox/test projects;
- no real user project, task assignment, agent terminal, or model prompt;
- local test MCP first;
- remote/network test only with explicit approved fixture;
- proof includes exact binary, source commit, package digest, command, OS, and
  observed boundary.

### Cross-platform CI

At minimum:

- Linux amd64;
- macOS arm64;
- Windows amd64;
- focused arm64 build/package checks for other released assets;
- path/case/executable-mode tests appropriate to each OS.

## CI gates

Each bounded PR runs only the relevant focused suite plus required repository
checks. The final release candidate runs:

1. Go tests for standard domain, loader, authoring, installer, and command roots.
2. Conformance adapter suite.
3. Template golden and generated-package validation matrix.
4. Static forbidden-import/forbidden-manifest checks.
5. Cross-platform build and launcher smoke.
6. Deterministic archive comparison.
7. npm/Homebrew/PyPI package verification where affected.
8. Documentation build and link check.
9. One full clean-clone E2E of each public authoring entrypoint.

Do not block independent implementation work on long CI when focused gates are
already available. Do not rerun a fully proven exact head unless code,
environment, or required evidence changed.

## Observability

Machine-readable command results include:

- command schema/version;
- engine version and source revision;
- package schema and tree/manifest digest;
- operation mode: read, plan, local mutation, or remote mutation;
- component diagnostics grouped by failure boundary;
- compatibility and runtime evidence kept separate;
- affected relative paths for authoring mutations;
- sanitized next actions.

Never emit absolute user paths in public evidence by default. Local JSON output
may include the selected root only when explicitly requested.

## Rollback strategy

- Keep installer behavior independent so authoring can be disabled without
  affecting installed plugins.
- Hide incomplete `agentplugins author` commands from release builds until the
  first vertical slice passes.
- Use bounded, dependency-safe PRs that can be reverted separately.
- Never rewrite or delete historical npm/PyPI/GitHub release artifacts.
- Preserve exact v1 binaries for old projects while migration is available.
- If standard-first authoring release fails, roll back the launcher/dist-tag,
  not package contents or user projects.
- A failed migration leaves the source unchanged and deletes only its exact
  tool-owned staging output.

## Proposed PR sequence and review budget

Approximate implementation budget: Phase 0 counts handwritten ADR/plan lines;
Phases 1-10 estimate production code and exclude tests, generated fixtures, and
generated documentation.

| Phase | Scope | Approximate change |
| --- | --- | ---: |
| 0 | ADR and policy inventory | 100-250 lines |
| 1 | Shared commands and flag adapter | 300-600 lines |
| 2 | Standard decoder/policy separation | 500-900 lines |
| 3 | Safe init templates | 550-900 lines |
| 4 | Inspect/compat/capabilities/doctor | 350-650 lines |
| 5 | Offline test and Skills | 300-550 lines |
| 6 | Dual-entrypoint release/wrappers | 300-550 lines |
| 7 | Runtime test/dev/bootstrap | 700-1,200 lines |
| 8 | Normalize/import/migration | 800-1,400 lines |
| 9 | Preview/export/bundle | 700-1,200 lines |
| 10 | GitHub/Directory publication | 500-900 lines |
| 11 | Legacy isolation and preservation | Re-estimate from capability inventory; no assumed net deletion |

Re-estimate after Phases 2 and 6 using actual changed production/test lines and
reuse achieved. Do not protect an early estimate by hiding necessary policy
work or by counting moved legacy code as new capability.

Target approximately 2,000 changed logical lines per PR, excluding clearly
identified generated docs, golden fixtures, and mechanical file moves.

1. `docs: record standard-first authoring architecture`
   - ADR, historical markers, failure-policy classification contract.
2. `refactor(cli): share authoring command factories`
   - no semantic behavior change; root-flag adapter tests.
3. `refactor(agentplugins): separate conformance from install policy`
   - shared bounded decoder and explicit policy layers; no installer safety
     relaxation.
4. `feat(authoring): validate standard projects`
   - local snapshot/project reader, validation policies, entrypoint parity.
5. `feat(authoring): scaffold Agent Plugins packages`
   - standard templates and safe writes.
6. `feat(authoring): inspect compatibility and readiness`
   - inspect, compat, capabilities, doctor.
7. `feat(authoring): test standard components offline`
   - static test and Agent Skills init/validate.
8. `feat(cli): release shared authoring entrypoints`
   - native assets, launchers, v2 migration errors, README and generated command
     docs. Split packaging and public docs only if the review budget requires it.
9. `feat(authoring): test and develop MCP runtimes`
   - explicit MCP runtime test and dev watcher.
10. `feat(authoring): bootstrap generated runtimes`
   - deterministic recognized templates only.
11. `feat(authoring): normalize standard documents`
   - lossless CAS writes.
12. `feat(authoring): import and migrate projects`
   - native import and one-way legacy conversion; split native and legacy only
     if their shared plan/result contract remains stable independently.
13. `feat(authoring): preview client projections`
    - pure generate boundary.
14. `feat(authoring): export standard packages`
    - deterministic archives and bundle inspection.
15. `feat(authoring): publish standard packages`
    - exact GitHub release and Directory submission plan.
16. `refactor(authoring): isolate legacy capabilities`
    - preserve useful implementation and tests outside standard authoring;
      deletion PRs only for individually owner-approved inventory items.

The exact PR count may shrink when adjacent changes remain below the review
budget and share one invariant. Do not combine installer lifecycle changes,
authoring migration, runtime execution, and remote publication into one mega-PR.

## Delivery milestones

### Milestone A - Usable standard authoring MVP

Commands:

- `init`;
- `validate`;
- `inspect`;
- `compat`;
- authoring `doctor`;
- static `test`.

Expected size: 2,500-4,500 production lines and 3,500-6,000 total changed
logical lines including focused tests and public docs.

Exit criteria:

- one generated Skill package and one generated MCP package complete the local
  authoring journey through both entrypoints;
- each generated package reaches the existing installer planner through
  `agentplugins add <local-path> --target <fixture-client> --dry-run` without
  conversion or a second manifest; representative fixture clients cover Skill,
  shared MCP, and client-specific activation outcomes;
- no legacy manifest emitted or read;
- installer tests remain green;
- both native entrypoints and supported wrappers are released from the same
  engine revision.

### Milestone B - Useful parity

Adds:

- runtime test;
- dev;
- bootstrap;
- normalize;
- import/migration;
- client projection preview;
- export/bundle.

Cumulative expected size: 5,000-8,000 production lines and 7,000-11,000 total
changed logical lines including security/cross-platform fixtures.

Exit criteria:

- maintained first-party examples migrate;
- standard archives are deterministic;
- old authoring docs are no longer primary.

### Milestone C - Publish and capability preservation

Adds:

- GitHub release and Directory submission;
- legacy isolation and reviewed capability preservation.

Size depends on how much existing publication infrastructure can be cleanly
adapted. Re-estimate after Milestone B rather than inventing a large platform
up front.

## Final acceptance criteria

The program is complete when all of the following are true:

- [ ] `plugin.json` is the only current standard authoring manifest.
- [ ] No normal standard authoring command reads or creates `plugin/plugin.yaml`.
- [ ] Standard authoring accepts legacy input only by explicit non-destructive migration.
- [ ] `plugin-kit-ai` and `agentplugins author` invoke one Go implementation.
- [ ] Equivalent commands have equivalent JSON contracts, exit codes, and
      filesystem effects.
- [ ] New Skill, remote MCP, stdio MCP, and hybrid projects validate immediately.
- [ ] Standard conformance is separated from compatibility and runtime evidence.
- [ ] Default validation/testing launches no agent, model, OAuth, or network.
- [ ] Explicit MCP tests run only in disposable test roots with bounded cleanup.
- [ ] Generated client previews never mutate user client configuration.
- [ ] Exported packages are deterministic, safe to extract, and validate again.
- [ ] Publication metadata remains outside portable manifest fields.
- [ ] Installer lifecycle, state, rollback, and Directory trust are unchanged.
- [ ] Supported release assets and launchers pass isolated cross-platform smoke.
- [ ] Public docs have clear `Use plugins` and `Build plugins` paths.
- [ ] Current docs and templates do not teach the old format.
- [ ] First-party examples are migrated or explicitly classified as non-portable
      client extensions.
- [ ] Standard command wiring is independent of the legacy model after migration gates.
- [ ] Useful legacy implementation, dependencies, tests and design documentation
      are preserved; each deletion has an explicit owner-approved inventory decision.

## Decision summary

Chosen approach:

**Shared standard-first Go authoring engine with two thin entrypoints.**

- Confidence: 10/10.
- Reliability: 9/10.
- Complexity: 6/10.
- Expected useful parity: 5,000-8,000 production lines and 7,000-11,000 total
  changed logical lines, plus separately identified mechanical documentation or
  fixture migrations.

This approach maximizes reuse without preserving a second package model. It
keeps the existing command investment, gives authors a dedicated
`plugin-kit-ai` experience, exposes the same capabilities from `agentplugins`,
and leaves Agent Plugins 1.0 `plugin.json` as the sole portable authority.
