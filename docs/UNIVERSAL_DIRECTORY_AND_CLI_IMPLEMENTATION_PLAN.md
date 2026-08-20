# Universal Agent Plugins Directory and CLI - Implementation Plan

Status: implementation-ready plan extending the current released baseline

Last updated: 2026-08-20

Scope: `universal-agent-plugins`, the future `agentplugins-cli` repository, and
the Agent Plugins-specific parts currently implemented in `plugin-kit-ai`

## 1. Executive summary

Build a community Directory and lifecycle manager for Agent Plugins 1.0 that
lets a user install one standard package into one or more supported AI clients
with one command:

```bash
npx universal-agent-plugins add context7 --target codex,cursor,kiro
```

The implementation remains standard-first:

- Root `plugin.json` is the only canonical portable manifest.
- `skills/` and `mcp.json` remain the only Agent Plugins 1.0 core component
  locations.
- The Directory, source provenance, verification evidence, source priority,
  signatures, and lifecycle state are community infrastructure outside the
  Agent Plugins package format.
- The installer never modifies, supplements, or overrides a package's core
  `plugin.json` fields with an installer-specific manifest.

Short names use one reviewed default distribution. After eligibility and
promotion, that default is the verified package in the upstream owner's
repository. If upstream has not yet accepted an Agent Plugins package, the
Directory can immediately publish a verified community bridge. A bridge is
built from a full upstream commit SHA plus a small reviewed overlay, but users
receive one self-contained Agent Plugins package. The installer never combines
two mutable sources at install time.

The public product uses the name **Universal Agent Plugins Directory**. Internal
schemas remain versioned for safe parsing, but names such as "Registry v3" do
not appear in the website, README, or normal CLI output.

## 2. Desired outcomes

### 2.1 User outcomes

- Install one plugin into several supported clients with one command.
- Complete every automatable delivery step in that command and report
  `installed`, `prepared`, or `manual activation required` honestly per client.
- Use the same CLI to add, update, repair, inspect, switch, and remove it.
- See the selected source, package digest, affected clients, and required next
  steps before or immediately after mutation.
- Prefer a verified upstream-owned package without requiring the user to know
  an `owner/repo@SHA//path` reference.
- Use a working community package immediately while an upstream packaging PR is
  still open.
- Install supported components from any valid external Agent Plugins 1.0
  package without submitting it to the Directory when the selected adapters
  support its transports, extensions, and platform requirements.
- Never be silently moved from a community package to a different upstream
  package, or vice versa, during an update.
- Receive honest per-client statuses for materialization, activation,
  authentication, discovery, and runtime verification.

### 2.2 Maintainer outcomes

- Add or update a bridge through a normal, reviewable pull request.
- Reproduce every generated bridge from a pinned upstream revision.
- Promote an accepted upstream package without deleting historical community
  releases or breaking existing installations.
- Keep the public website static and generated from reviewable repository data.
- Avoid a database, moderation backend, or custom account system during the MVP.
- Release the security-critical installer independently from untrusted package
  contribution workflows.

### 2.3 Upstream maintainer outcomes

- Receive a standards-focused pull request titled along the lines of
  `Add Agent Plugins 1.0 package`.
- See validation and client installation evidence without depending on the
  community CLI in upstream CI.
- Accept, modify, or decline the package without blocking availability through
  the community bridge.

## 3. Non-goals

- Defining a new plugin manifest or adding community fields to core
  `plugin.json`.
- Claiming the Directory or CLI is an official Agent Plugins marketplace.
- Claiming a community bridge is authored, endorsed, or published by upstream.
- Installing arbitrary packages into ChatGPT without the app registration and
  user consent required by ChatGPT.
- Defining a portable permissions, secrets, dependencies, or sandbox model that
  Agent Plugins 1.0 does not define.
- Running untrusted MCP servers or plugin scripts in a privileged publishing
  workflow.
- Adding install telemetry, user accounts, ratings, payments, or a database in
  the initial implementation.
- Supporting plugin-to-plugin dependencies. Agent Plugins 1.0 does not define
  them.
- Building a Node/Python/Docker/system package manager. The CLI preflights
  declared stdio executables but does not install machine runtimes.
- Building a general-purpose package registry protocol before a second real
  consumer requires it.
- Building a transparency log or globally consistent freshness service. The
  signed snapshot protects integrity, expiry, the embedded floor, and each
  client's highest previously accepted sequence.
- Advertising cross-client project-scope installation before individual client
  adapters prove that scope through isolated E2E.

### 3.1 MVP simplicity guardrails

The first stable delivery includes only the smallest complete vertical slice:

- a Git-backed static Directory with no API service or database;
- one generated signed snapshot and one active signing key ID;
- three distribution kinds: upstream, community bridge, and community;
- a deterministic bridge builder limited to pinned copy plus reviewed overlay;
- pull-request-based package submission and promotion;
- one multi-target lifecycle built on the existing Go state, journal, and
  adapters;
- the current npm package and binary names.

The following are deliberately deferred until repeated real usage justifies
them:

- arbitrary bridge transformations or a general patch language;
- automatically tracking and repackaging every new upstream release;
- hosting package blobs in a custom artifact service;
- a generic registry federation protocol or private organization registries;
- install analytics, rankings, ratings, accounts, and moderation services;
- a generalized online key-management protocol;
- new client adapters based only on configuration-format similarity rather than
  proven install and runtime behavior.

Promotion handling for already-opened upstream packaging PRs remains in the
post-launch roadmap because it completes a real bridge-to-upstream lifecycle.
General upstream release watching is not planned.

### 3.2 Stable-launch boundary

The first stable public release requires Phases 0-6. It ships the
hardened standard loader, multi-target lifecycle, static Directory, first
bridges, signed snapshot consumption, and launch evidence.

Two useful but non-blocking follow-ups stay outside that launch gate:

- Phase 7 begins with an on-demand merge check that generates a promotion PR.
  Scheduled polling is enabled after the first real tracked upstream PR proves
  the workflow; exact-match auto-merge remains gated by three manual
  promotions.
- Phase 8 extracts the already-stable CLI from `plugin-kit-ai`. Until then the
  existing repository and release pipeline remain the implementation home;
  users still run the same npm package and command.

This boundary avoids delaying the product proof on repository movement or
unproven automation while preserving the decided long-term repository split.

## 4. Normative foundation

The current published standard is Agent Plugins 1.0.0. Agent Plugins 1.1.0 is a
working draft and is not treated as a stable compatibility target. The public
product may say "Agent Plugins 1.0", while loaders and evidence always use the
exact schema version.

The implementation follows these Agent Plugins 1.0.0 contracts:

1. A plugin is one directory rooted at one filesystem location.
2. Root `plugin.json` is required and is the only portable core manifest.
3. Portable component discovery uses fixed `skills/` and `mcp.json` locations.
4. Plugin-relative package paths must remain within the resolved plugin root.
5. Unsupported or invalid independent components are isolated according to the
   specification instead of silently corrupting otherwise valid components.
6. The declared `$schema` selects a locally supported loader. The installer does
   not fetch and execute a new schema dynamically.
7. Agent Plugins 1.0.0 package version strings are metadata. SemVer is recommended
   but not required, so Directory release ordering cannot depend only on SemVer.
8. Stdio MCP processes receive client-managed `PLUGIN_ROOT` and persistent
   `PLUGIN_DATA` directories. Package updates preserve `PLUGIN_DATA`.
9. Unknown client extension namespaces are ignored. A client consumes only the
   extension namespace it explicitly supports.
10. `plugin.json` and `mcp.json`, when both exist, must declare the same exact
    specification version. A mismatch disables MCP without invalidating
    independently valid skills.
11. Unknown top-level manifest fields are reported and ignored rather than
    assigned community semantics. Other manifest violations remain fatal under
    the published failure boundaries.
12. A bare stdio `command` uses platform executable lookup; a bundled command
    must be plugin-relative. The standard does not define runtime installation.

The CLI does not assume that a future schema is compatible merely because its
current JSON shape looks similar. A published successor receives an explicit
loader, fixtures, compatibility decision, and client evidence before support is
advertised.

References:

- [Agent Plugins 1.0.0 specification](https://github.com/agentplugins/agent-plugins-spec/blob/main/spec/1.0.0.md)
- [Agent Plugins specification status](https://github.com/agentplugins/agent-plugins-spec#status)
- [Agent Plugins future considerations](https://github.com/agentplugins/agent-plugins-spec/blob/main/FUTURE_CONSIDERATIONS.md)

## 5. Current implementation baseline

The plan extends working code rather than replacing it from scratch:

- `universal-agent-plugins` contains 26 standard packages, a static website,
  catalog schemas, generated OpenAI compatibility packages, and E2E evidence.
- The current catalog model is flat and assumes every short name resolves into
  `777genius/universal-agent-plugins/plugins/<name>`.
- The `agentplugins` Go engine currently lives in `plugin-kit-ai` and already
  includes package loading, client detection, adapters, state, locking,
  journaling, rollback primitives, and lifecycle operations.
- State schema 2 already models one installation with multiple client bindings,
  per-client package revisions, scopes, activation states, physical artifacts,
  ownership receipts, and `needs_rebind`; the public lifecycle still selects
  exactly one target at a time.
- The current `rebind` command is a narrow recovery flow for an inactive
  binding, while `migrate-format` explicitly moves a removed legacy
  `plugin.yaml` binding. Neither is the normal active-source migration UX.
- The CLI currently parses `--scope project`, but all Agent Plugins adapters
  advertise only user scope. The stable release must not present project scope
  as working until an adapter proves it.
- A hidden `--yes` and interactive confirmations still exist in the current
  implementation. Normal explicit lifecycle commands will remove that hidden
  requirement; destructive data purge remains explicit in the command itself.
- Local paths and immutable GitHub `owner/repo@SHA//path` sources already work
  without the catalog.
- Every current package has a README containing its short-name install command;
  the new Directory must preserve this through a generated consistency check.
- The npm package is `universal-agent-plugins`; its installed binary is
  `agentplugins`.

No migration should discard these proven lifecycle and adapter foundations.

## 6. Fixed architectural decisions

### 6.1 Package truth

`plugin.json` and files inside its plugin root are the canonical installed
package. Directory metadata can describe and verify a package, but cannot
override its portable contents.

The Agent Plugins loader never falls back to `plugin.yaml` when root
`plugin.json` exists but is invalid. `plugin.yaml` remains an explicit legacy
authoring input for `plugin-kit-ai`, outside the installed Agent Plugins
package. Client-native compatibility imports, including `.codex-plugin`, use
separate explicit loaders and cannot override a valid portable root manifest.

### 6.2 Public Directory, internal registry

- Public name: **Universal Agent Plugins Directory**.
- Internal implementation term: registry.
- Public paths and UI do not use names such as `v3`.
- Internal data includes a small schema version and signed snapshot sequence.
- Existing `catalog/v1` and `catalog/v2` outputs stay byte-frozen temporarily
  for older CLI builds, receive no new Directory entries, and disappear from
  public documentation.

### 6.3 Source priority

Every product declares one reviewed `default_distribution`. Directory CI
enforces that an eligible upstream distribution becomes that default after its
promotion PR merges. The CLI does not independently promote any source merely
because it is labelled upstream. A discovered or PR-merged upstream package is
not active for unqualified resolution until the Directory promotion change
passes and merges.

An unqualified short name first tries the declared default. If that distribution
cannot serve the complete selected target set, resolution falls back in this
order:

1. Eligible verified upstream-owned distribution.
2. Eligible verified community bridge derived from the upstream project.
3. Eligible reviewed community-authored distribution.
4. No eligible distribution: fail with a precise explanation and qualified
   alternatives, if any.

"Upstream-owned" means the complete installable plugin root, including
`plugin.json`, exists at a pinned revision in the upstream owner's repository.
An external PR branch, our fork, or a package assembled in our repository is not
labelled upstream-owned.

Origin alone is insufficient. An upstream package becomes the declared default
only after it passes schema, integrity, licensing, minimum capability, and
selected-client installation gates. A broken or materially incomplete upstream
package does not replace a working bridge automatically. Every fallback is
shown to the user with the reason the declared default was ineligible.

### 6.4 Build-time bridges

A bridge has two layers:

```text
bridges/<product-id>/
├── bridge.yaml
└── overlay/
    ├── plugin.json
    ├── mcp.json          # optional
    ├── README.md         # optional package documentation
    └── NOTICE            # when required

plugins/<product-id>/     # deterministic generated output
├── plugin.json
├── mcp.json              # optional
├── skills/               # optional copied upstream components
├── LICENSE               # when redistributed content requires it
├── NOTICE                # when required
└── README.md
```

`bridge.yaml` is build metadata and never enters the installed package.
`plugins/<product-id>` is a complete, inspectable Agent Plugins package and is
committed initially so current GitHub source resolution continues to work.

### 6.5 No install-time overlays

The CLI never downloads `plugin.json` from the community repository and then
combines it with mutable files from another repository on the user's machine.
All upstream inputs are fetched at build time from a full commit SHA. The final
package is validated, reviewed, hashed, and installed as one unit.

### 6.6 Distribution stickiness

- New installs use the current eligible default distribution.
- Existing installs remain attached to their recorded distribution.
- `update` searches for a newer release only within the same distribution.
- Switching source requires an explicit command:

  ```bash
  agentplugins switch context7 --to upstash/context7
  ```

  The first release accepts a qualified distribution ID or exact direct source;
  it does not add an ambiguous distribution-kind shortcut such as `--to
  upstream`.

- Adding a new target to an existing installation uses that installation's
  recorded desired release, not the latest short-name default. A separate
  `update` advances the desired release afterward.

### 6.7 Repository boundaries

Target ownership:

| Repository | Responsibility |
| --- | --- |
| `agentplugins-cli` | Security-critical Go loader, resolver, lifecycle, adapters, state, CLI, npm release binaries |
| `universal-agent-plugins` | Directory source data, static website, bridges, community packages, schemas, evidence, contribution workflow |
| `plugin-kit-ai` | Authoring toolkit, `init`, `generate`, `validate`, and legacy `plugin.yaml` workflows |

The CLI is extracted only after at least two stable releases prove its
standard-first command, state, and multi-target contracts. The user-facing npm
name and command do not change during the extraction.

## 7. Terminology and identity

### 7.1 Product

A stable Directory identity such as `context7`. A product owns its public card,
title, description, aliases, icon, categories, canonical manifest name, and
minimum capability contract. Every active distribution must match that
canonical name; a name change is an explicit identity migration, not a normal
release or source switch.

### 7.2 Distribution

One independently maintained package source for a product:

- `upstream`: complete package in the upstream owner's repository.
- `community_bridge`: generated from pinned upstream content plus our overlay.
- `community`: independently authored or packaged by a community publisher.

Distribution identity is namespaced and stable, for example
`upstash/context7` or `777genius/context7-bridge`.

### 7.3 Release

An immutable package revision containing:

- distribution ID;
- monotonically increasing sequence within that distribution;
- declared package version, if present;
- exact install-source repository, full revision, and plugin root subpath;
- optional pinned upstream build provenance for generated bridges;
- package tree digest;
- manifest digest;
- component inventory;
- publication timestamp.

The immutable fields above identify package bytes. A separate reviewable signed
release policy, keyed by that release identity, contains availability status,
minimum installer/capability requirements, statically compatible
clients/scopes, and current evidence pointers. Policy-only changes advance the
Directory snapshot sequence but never manufacture a package release. This lets
a newly proven adapter support existing unchanged package bytes without editing
their provenance.

The per-distribution release sequence orders updates even when
`plugin.json.version` is missing, non-SemVer, unchanged, or uses SemVer build
metadata. Sequences are never reused within a distribution. The signed snapshot
sequence is a separate monotonically increasing rollback-protection counter
within each supported internal snapshot-schema feed.

`(distribution_id, release_sequence)` is the complete stable release identity;
the MVP does not add a second opaque or human-assigned release ID.

For a bridge, `plugin.json.version` preserves a verified upstream version when
one exists. Otherwise it uses a valid informational value such as
`0.0.0+git.<short-sha>`. Bridge update ordering still uses the Directory release
sequence, never a guessed comparison of informational versions.

### 7.4 Verification

Verification records are separate append-only observations keyed by release,
test level, applicable environment dimensions, and evidence identity. They are
not part of release identity, do not advance the package release sequence, and
do not change package bytes. Schema validation is package-level and records the
package/schema-loader tuple without inventing a client or OS. Materialization,
discovery, runtime, and OAuth add only the dimensions that actually apply:
client, dependency identity, installer/adapter and client version, OS, and
architecture. For each complete applicable tuple, reviewable Directory source
points to at most one current trusted evidence record. Changing that pointer
requires normal review, while every older record remains immutable history.
Generated views and the signed
snapshot aggregate only those current pointers, never whichever file has the
newest contributor-controlled timestamp.

Contributor compatibility claims are never rendered as Directory verification.
A trusted `passed` result for materialization, discovery, runtime, or OAuth
requires an immutable artifact from an isolated workflow or an explicitly
reviewed external attestation. Other submissions remain a self-reported claim
with outcome `not_tested` and cannot select or replace the current trusted
evidence pointer.

Verification does not mix test depth with result. Each current summary has:

- test level: `schema`, `materialization`, `discovery`, `runtime`, or `oauth`;
- outcome: `passed`, `failed`, `inconclusive`, `not_tested`, or
  `not_applicable`.

Public labels such as `schema validated`, `runtime tested`, or `OAuth failed`
are derived from those two fields rather than stored as a second truth.

No single `tested` boolean may imply runtime or OAuth evidence that was not
actually collected.

Default Directory eligibility requires a conforming manifest, immutable source
and digests, license/provenance review, and static compatibility with the
selected client. Missing runtime or OAuth evidence remains visible but does not
silently make an otherwise valid package ineligible. A failed current trusted
materialization, discovery, or runtime record blocks that client until a newer
release or a reviewed replacement evidence record resolves it. A later trusted
pass can replace the current pointer without deleting the failure it supersedes.
Products may declare a stricter reviewed gate, but the resolver never invents
one from a marketing badge.

`failed` is reserved for a trusted, reproducible package/client incompatibility.
Cancelled consent, missing test credentials, harness failure, rate limiting, or
a transient vendor outage is `inconclusive`: visible and never presented as a
pass, but not an automatic install block.

Evidence is valid only for its recorded tuple: package tree digest, resolved
external command/package identity when applicable, installer/adapter version,
client and client version, OS/architecture, test level, and timestamp. A new
package digest, MCP endpoint, executable/dependency, adapter behavior, or client
version starts as untested until fresh evidence exists. Evidence may be
reused across Directory metadata-only changes when this tuple is unchanged.
An inapplicable old failure is still shown in history but cannot block a changed
tuple.

Evidence for a remote MCP service is a dated observation, not proof that the
service behind an unchanged URL cannot change later. The MVP displays that date
and does not add continuous remote-service monitoring.

### 7.5 Independent version axes

Four values remain intentionally independent:

- `plugin.json.version`: author metadata for one package;
- distribution release sequence: update order inside one source;
- Directory snapshot sequence: signed rollback protection;
- CLI/npm SemVer: installer code release.

They are never forced to match and do not trigger synchronized GitHub/npm
releases. Normal UI shows package version and CLI version; internal sequences
appear only in provenance, diagnostics, and structured output.

## 8. Data ownership and generated artifacts

| Data | Canonical owner | Derived outputs |
| --- | --- | --- |
| Portable package | Its complete plugin root | Client-native projections |
| Bridge definition | `bridges/<id>/bridge.yaml` plus `overlay/` | `plugins/<id>/` |
| Directory product/distribution metadata | Reviewable source files under `registry/` | deterministic review preview and trusted published snapshot |
| Production website catalog/search data | Exact published signed snapshot sequence | Product cards and copy-ready commands |
| Verification evidence | Immutable test result files pinned to revisions | Status summaries and badges |
| Signed publication ledger | Protected append-only publication branch | GitHub Pages deployment and CLI cache |
| Installed state | Local agentplugins state file | Human and JSON read views |
| Legacy catalog | Current byte-frozen compatibility files | None; CI verifies they do not drift |

Generated bridge packages are committed during the first implementation so a
pull request exposes exact content changes and current direct GitHub package
resolution remains usable. Moving generated packages to signed release
artifacts is deferred until repeated bridge volume proves that repository size
or release throughput is a real problem.

The MVP allows at most one in-repository `community` or `community_bridge`
package root per product at `plugins/<product-id>`. Alternative distributions
for the same product live in their publisher's external repository and are
indexed under qualified IDs. Replacing an existing in-repository community
package with a generated bridge is an explicit reviewed migration, never an
overwrite performed by the builder. A nested multi-publisher package layout is
deferred until a second real in-repository distribution requires it.

For a package stored in `universal-agent-plugins` itself, reviewable source data
stores its repository-relative path plus expected tree and manifest digests. A
pull request cannot predict its eventual merge SHA. After merge, the trusted
publisher resolves the actual default-branch commit, recomputes those digests,
and only then emits that exact commit SHA into the signed snapshot. That binding
and its publication timestamp are assigned once per release identity. Later
weekly, evidence-only, or metadata publications copy both values from the latest
cryptographically valid published snapshot; they never rebind unchanged release
bytes to a newer default-branch commit or make an old package look newly
published. Publisher recovery may read an expired snapshot only as an immutable
sequence/provenance ledger, never as client-eligible Directory data. External
upstream sources continue declaring their already-known immutable revision,
while the trusted publisher still assigns their Directory publication timestamp
once.

## 9. Proposed Directory source model

The exact serialization can evolve during implementation, but the domain model
is fixed. An illustrative product record:

```yaml
schema_version: 1
id: context7
display_name: Context7
manifest_name: context7
aliases:
  - context7
minimum_capabilities:
  skills: optional
  mcp: required
default_distribution: upstash/context7
distributions:
  - upstash/context7
  - 777genius/context7-bridge
```

An illustrative bridge distribution:

```yaml
schema_version: 1
id: 777genius/context7-bridge
product_id: context7
kind: community_bridge
status: active
packager: 777genius
releases:
  - sequence: 3
    package_source:
      repository: 777genius/universal-agent-plugins
      revision: ASSIGNED_BY_TRUSTED_PUBLISHER_AFTER_MERGE
      path: plugins/context7
    build_provenance:
      upstream_repository: upstash/context7
      upstream_revision: FULL_COMMIT_SHA
    tree_digest: sha256:...
    manifest_digest: sha256:...
release_policies:
  - release_sequence: 3
    status: active
    compatible_clients:
      - codex
      - cursor
```

Each release owns its immutable `package_source`; a distribution is only the
stable update channel and never overwrites source data from an older release.
Optional `build_provenance` records reproducible bridge input. For an upstream
or external community release, `package_source` points directly to that
publisher and build provenance is normally absent; for a bridge the source and
upstream provenance intentionally differ. Review source for an in-repository
package leaves `package_source.revision` unresolved, and only the trusted
post-merge publisher fills the real default-branch commit after digest
comparison.

CI generates a deterministic canonical review preview from source YAML/JSON;
the preview excludes publication sequence/expiry and leaves post-merge package
revisions unresolved. Generated previews and public snapshots are never edited
by hand.

Review source never contains a guessed future merge SHA. The signed snapshot is
the first artifact that binds an in-repository package path and reviewed digest
to the actual post-merge commit.

Directory validation requires `default_distribution` to reference an active
distribution of the same product with at least one publishable active release
that satisfies the product's minimum component contract. Selected-client
eligibility is evaluated later for the complete requested target set. Changing
the default is a reviewable source change produced by bridge publication or
upstream promotion, never an implicit client-side priority decision.

The MVP model stops here. It does not add dependency solving, version ranges,
release channels, publisher accounts, organization policy, or a generic query
API. A product has one explicit default distribution and a small immutable
release history. New installs select the highest eligible release sequence in
the chosen distribution. The MVP retains every published release record and
revocation in reviewable source and the signed snapshot so an old installation
remains explainable and authorizable; pruning or per-release index documents
wait until measured snapshot size makes them necessary.

## 10. Bridge recipe contract

### 10.1 Required fields

Each recipe declares:

- bridge schema version;
- product and distribution IDs;
- upstream GitHub `owner/repo`;
- full 40-character lowercase commit SHA;
- zero or more exact copy roots;
- overlay directory;
- expected upstream license and required attribution paths;
- package output path;
- optional expected upstream version;
- component expectations used to prevent accidental capability loss;
- for a zero-copy MCP bridge, exact pinned provenance paths that establish its
  endpoint/command identity.

### 10.2 Allowed build operations

The initial bridge builder supports only deterministic operations:

- sparse fetch exact paths from an exact Git commit;
- byte-preserving copy;
- deterministic path placement within the plugin root;
- overlay of reviewed repository files;
- validation and digest generation.

It does not support arbitrary shell build hooks, network scripts, package
installation, code generation from upstream executables, or dynamic templates
that can access secrets.

An overlay may replace an exact copied path, but that replacement is visible as
a complete reviewed file in the repository. The MVP does not implement a patch
engine. If repeated bridges later prove that deterministic patching is needed,
it is designed and reviewed as a separate extension.

### 10.3 MCP-only bridges

An MCP-only bridge often needs no copied upstream source. Its complete package
can contain only:

- `plugin.json`;
- `mcp.json`;
- package README and attribution.

It is classified as `community_bridge` only when its vendor/upstream provenance
is anchored to the declared pinned repository revision, even if the copy list
is empty. A zero-copy recipe must name at least one exact provenance path at
that revision whose reviewed contents establish the endpoint, command, or
package identity used by the overlay. The builder hashes that evidence but does
not copy or execute it. A package assembled only from a mutable documentation
page, an unrelated repository pin, or a community-chosen endpoint is a normal
`community` distribution, not an upstream-derived bridge.

Remote MCP URLs must come from vendor documentation. Stdio commands must use a
non-floating exact package version/revision when the ecosystem supports one, or
a bundled executable. Floating tags such as `latest` are prohibited. An exact
external package-manager version is still a runtime dependency outside the
plugin tree digest; plans and evidence show it explicitly and never call it
reproducible unless the executable itself is bundled and covered by the digest.

### 10.4 Skills and bundled executables

Skills and local executable files used through plugin-relative paths must exist
inside the final plugin root. They are copied from the pinned upstream revision
and covered by the final package digest.

Executable bits are preserved. Bridge generation runs with Git line-ending
conversion disabled so output bytes and digests remain platform-independent.

### 10.5 Licensing and attribution

- Redistribution is allowed only when the upstream license permits it.
- Required license and notice files are copied into the generated package.
- The package description says `Community package for ...`.
- The Directory records both upstream owner and community packager.
- If required source content has no redistributable license, no bridge is
  published. The project can still open an upstream packaging PR or publish an
  MCP-only configuration that contains no copied copyrighted content.

### 10.6 Reproducibility

`build-bridges --check` rebuilds every bridge into a temporary directory and
compares paths, bytes, executable bits, manifest digest, and tree digest with
the committed output. Timestamps and local checkout metadata are excluded.

## 11. Short-name resolution

### 11.1 Resolution algorithm

For `agentplugins add context7`:

1. Parse and normalize the selector without resolving a new default.
2. Load local state and first match recorded product/distribution IDs or
   declared manifest identity, including a `data_retained` record left after
   the last binding was removed.
3. If no exact local identity matches, load a valid signed Directory snapshot,
   resolve the alias to one stable product ID, and check local state again
   before choosing a distribution.
4. Detect clients and obtain the explicit or interactive target set.
5. If the product is already installed, retain its recorded distribution and
   desired immutable release. Show existing bindings as installed and plan only
   compatible unbound targets. Adding a target is a new exposure, so a
   Directory-managed install still requires an unexpired signed snapshot that
   confirms the distribution is not suspended and the recorded release is not
   revoked.
6. If no installation exists, use the valid signed snapshot to evaluate the
   product's declared default for the complete target set.
7. Within a distribution, evaluate active releases in descending release
   sequence and select the highest release eligible for the complete target set.
   Exclude releases incompatible with the current CLI or declared Agent Plugins
   schema, required component contract, integrity policy, or verification gate.
   Superseded releases remain available only to reproduce an existing recorded
   installation, and revoked releases are never selected.
8. Use the declared default when eligible; otherwise evaluate eligible
   upstream, then community bridge, then community, excluding the failed
   default from the fallback list.
9. Resolve one immutable release and print its provenance and any fallback
   reason.
10. Acquire the package once and verify its identity, inventory, tree digest,
   and manifest digest before planning any target mutation.

If acquisition or verification disagrees with the selected signed release, the
operation fails closed. It does not try another distribution in the same
operation: fallback is an explicit eligibility decision over already-signed
metadata, not recovery from changed or malicious downloaded bytes.

One multi-target operation always uses one distribution and one release. If an
upstream default supports only a subset of the selected clients but the bridge
supports all of them, the bridge is the explicit fallback for that operation.
If no single distribution supports the complete target set, the CLI fails
before mutation and suggests compatible target/source combinations. It never
silently mixes upstream and community packages between clients.

Qualified names bypass source priority but not validation:

```bash
agentplugins add upstash/context7 --target cursor
agentplugins add 777genius/context7-bridge --target cursor
```

Direct immutable GitHub and local paths bypass the Directory entirely:

```bash
agentplugins add owner/repo@FULL_COMMIT_SHA//path/to/plugin --target cursor
agentplugins add ./my-plugin --target cursor
```

Direct packages are labelled `direct source` rather than `verified Directory
package`.

Direct exact/local operation never depends on Directory availability or accepts
Directory policy as its authority. If an already-valid local/embedded snapshot
maps the acquired digest to a known revoked Directory release, the plan emits a
prominent best-effort warning and records the match, but it does not fetch the
Directory solely to decide an explicitly requested direct source or silently
rewrite that source.

A direct source whose manifest/native identity collides with an existing
Directory product is not silently adopted or rebound. The CLI requires an
explicit `switch` with an exact source and matching manifest name, while showing
that Directory publisher verification no longer applies. `rebind` remains only
for broken provenance recovery. Otherwise the source needs a distinct identity
that does not collide in the selected native backend.

For a direct local or full-SHA source, targets are selected first when explicit;
otherwise the package is loaded before the interactive compatibility choice.
The same complete-preflight rule still applies.

### 11.2 Alias safety

- One active product owns a short alias at a time.
- Previous aliases remain reserved after rename to prevent takeover.
- Publisher-qualified IDs remain available when aliases collide.
- Confusable Unicode is not accepted in IDs; IDs remain lowercase ASCII.
- A manifest name change does not automatically migrate product identity.
- Typosquatting and publisher impersonation require manual review.

## 12. Multi-target lifecycle semantics

### 12.1 CLI contract

The website and copy-ready README quick starts use the zero-install npm facade:

```bash
npx universal-agent-plugins add context7 --target codex,cursor
```

`agentplugins` is the installed binary and is used in CLI help, diagnostics,
and examples for users who installed the package globally. Both entry points
must execute the same versioned Go binary and expose identical arguments and
JSON output. The project does not publish or document a second npm package named
`agentplugins`.

The npm facade keeps its declared Node.js `>=22` requirement, and the website/
README state it immediately beside the first `npx` command. Native release
binaries remain the no-Node installation path; the CLI never confuses this
launcher requirement with a plugin's own runtime dependencies.

Explicit comma-separated targets:

```bash
agentplugins add context7 --target codex,cursor,kiro
```

Target parsing:

- trims whitespace;
- accepts documented aliases;
- canonicalizes client IDs;
- rejects empty values, unknown targets, and duplicates;
- preserves a deterministic adapter execution order;
- does not support a mutable `--target all` shortcut.

The first stable release installs at user scope because that is the only scope
currently declared by every supported adapter. Internal state remains
scope-aware. `--scope project` fails before mutation with a client-specific
diagnostic until at least one adapter implements and proves it; unsupported
scope never falls back silently to user scope. One command does not mix scopes.

When no target is supplied:

- one compatible detected client: select it;
- several compatible detected clients in an interactive terminal: show a
  multiselect with detected compatible clients selected by default;
- non-interactive execution: fail before mutation and require an explicit list;
- no compatible client: explain why and show supported clients.

The stable CLI removes `--yes` entirely. An explicit mutation command and exact
non-interactive target list are sufficient consent. Destructive data deletion
remains explicit in `--purge-data`, and `--dry-run` provides a complete read-only
plan.

Exit status `0` means every CLI-controlled action completed and each target
reached its adapter's documented terminal state, including an honest
`prepared/manual activation required` state where the client has no managed
activation API. Invalid input, failed preflight, failed/partial mutation, and
failed verification return non-zero; structured output carries the per-target
state and next action. A user-cancelled interactive plan returns `0` with an
explicit `cancelled/no changes` result. The MVP keeps broad stable
success/failure semantics rather than inventing a large public exit-code
taxonomy.

Every structured response includes an output-schema version, command, overall
result, and per-target results. Additive fields may be introduced compatibly;
renaming/removing a field or changing its meaning requires a new output-schema
version. Human wording is not an automation contract. This same fixture-backed
JSON contract survives the later repository extraction.

### 12.2 Plan once, apply after complete preflight

1. Parse explicit targets or detect clients once for interactive selection.
2. Resolve one distribution eligible for the complete target set, or accept one
   explicit direct source.
3. Fetch and validate one immutable package and record component diagnostics.
4. Build a plan for every selected target from that package.
5. Ask each adapter to observe the selected native identity before staging. A
   pre-existing object is accepted only when state plus ownership receipts
   prove it is ours, or when the client provides a namespace-qualified identity
   that the adapter proves cannot collide. An unmanaged or indeterminate
   collision fails closed; the MVP never auto-adopts it.
6. Fail without mutation if any explicitly selected target has no usable
   components, lacks a required executable for managed activation, collides
   with an unmanaged native object, or cannot be prepared safely.
7. Render one combined plan.
8. Stage all CLI-controlled filesystem mutations.
9. Revalidate ownership and observed native-config digests immediately before
   commit; abort if another tool or user changed a planned object.
10. Commit managed mutations with one operation group and per-client receipts.
11. Perform native client activation steps after managed materialization.
12. Persist per-client activation and authentication states.
13. Print a compact summary plus exact remaining actions.

### 12.3 Failure boundaries

- Failure during resolution, validation, detection, or planning: no mutation.
- A pre-existing native installation without matching ownership receipts is
  foreign. The CLI neither overwrites, adopts, updates, nor removes it; it
  reports the exact client identity and manual resolution choices.
- Failure during staging: delete staging and leave active state unchanged.
- Failure during managed commit: roll back every managed change in the operation
  group when ownership receipts prove rollback is safe.
- Failure during an external native-client activation: keep safely materialized
  package files, mark that client `activation_failed` or `manual_activation`,
  and print a precise `repair` action.
- Authentication and OAuth are never treated as part of a globally atomic
  filesystem transaction.
- The CLI never claims global atomicity across independent vendor clients.
- The MVP retains one process-wide mutation lock across journal recovery,
  managed client configuration, package commit, and state commit. Different
  plugins can touch the same native config file, so per-installation parallel
  mutation is unsafe. Read-only planning may remain concurrent; narrower locks
  wait for measured need and adapter-level conflict proofs.

### 12.4 Partial component behavior

Agent Plugins 1.0 isolates independent component failures. The loader therefore
returns a valid component inventory plus structured diagnostics:

- invalid manifest: reject package;
- invalid top-level `mcp.json`: disable MCP and continue with valid skills;
- invalid individual MCP server: skip that server;
- invalid individual skill: skip that skill;
- unsupported transport or component for one client: skip it for that client.

An install that has at least one usable component may complete as
`installed_with_warnings`. Skipped components are never hidden. A selected
target with zero usable components fails preflight.

For stdio MCP, preflight classifies each command as bundled, available on the
relevant executable path, missing, or client-managed/unknown. Missing is
blocking when the CLI controls activation. Client-managed/unknown remains a
visible manual verification step. The CLI never installs Node, Python, Docker,
or another system runtime implicitly.

### 12.5 Lifecycle defaults

- `add` creates one installation with multiple client bindings.
- Adding a new client to an existing installation retains its distribution.
- `update <name>` targets all installed bindings by default and stays within the
  recorded distribution.
- `repair <name>` targets bindings in degraded, failed, or manually incomplete
  states by default. It reapplies or reactivates the exact per-client package
  revision already recorded for each binding. It never chooses a newer release,
  changes source, or consults a newly preferred default; version convergence is
  the job of `update`.
- Interactive `remove <name>` shows installed targets and defaults to all;
  non-interactive removal requires an explicit target list. The only no-target
  exception is `remove <name> --purge-data` for a `data_retained` record,
  where the destructive intent and exact owned receipts are already explicit.
- Source migration uses `switch`, never `update`.
- `switch` always moves the complete installation and all of its client
  bindings. Per-client distribution mixing is not supported. For a
  `data_retained` record with no bindings, it validates the new package and
  identity, updates the recorded source deliberately, and preserves data with
  the same cross-distribution warning before a later `add`.
- `switch` is the normal active-installation source migration. Existing
  `rebind` remains a narrow recovery command for inactive or ambiguous bindings,
  and `migrate-format` remains the explicit legacy-format migration path.
- A switch stays within the same Directory product. A direct-source switch must
  preserve the manifest identity; otherwise the user creates a separate
  installation instead of leaking state between unrelated plugins.
- Every update candidate is preflighted against every installed binding. If the
  release is incompatible with one of them, no target updates. An explicit
  target subset may roll out an otherwise universally eligible release to only
  those bindings; `info` shows temporary mixed revisions and the remaining
  convergence work within the same distribution.
- An installation created from a direct full-SHA source has no moving update
  channel. Updating it requires an explicit new immutable source through
  `switch --to owner/repo@NEW_FULL_SHA//path` or reinstall; the CLI never
  follows mutable `main` automatically.
- A direct local-path installation may be updated only by an explicit `update`.
  The CLI snapshots the path again, shows old and new digests, and never watches
  or applies local changes automatically.
- For a Directory installation, a strictly newer release sequence authorizes an
  update even when `plugin.json.version` is absent, non-SemVer, unchanged, or
  lower according to SemVer. A direct local update instead requires stable
  manifest identity and explicit user invocation, shows the complete digest
  change, and does not pretend SemVer supplies missing provenance. Legacy
  format transitions keep their existing guarded migration path.
- `update` selects the highest eligible active release with a sequence greater
  than the recorded desired release. It never downgrades automatically. If all
  newer releases are incompatible with an installed binding, it makes no
  changes and explains which gates excluded them.
- `update`, `repair`, and source switching never delete or rewrite a
  client-managed `PLUGIN_DATA` directory. A source switch keeps the same product
  data path but warns that Agent Plugins 1.0 defines no cross-distribution data
  compatibility contract; package rollback cannot undo data changes made later
  by a newly launched external process.
- `repair` reacquires the recorded immutable source, or reuses a complete clean
  managed copy only after verifying its recorded digest. If neither is
  available, it makes no changes and reports the exact source requirement. A
  revoked release cannot be repaired or materialized into another target.
- `remove` removes the managed package and client bindings but keeps
  `PLUGIN_DATA` by default and prints its retained location. When the last
  binding is removed, the existing state record becomes `data_retained` and
  keeps only origin-appropriate provenance (Directory
  product/distribution/release or direct source/digests) plus ownership-safe
  data receipts; no second tombstone database is introduced. Re-adding that
  product finds this record before resolving a new default, deliberately reuses its
  distribution/data, and then offers normal `update` or explicit
  `switch`. Permanent deletion requires `--purge-data` and a complete ownership
  check, after which the retained record can be removed. If a native client owns
  an unknown data location, the CLI refuses to claim, reuse, or purge it and
  gives the client-specific cleanup step. When `--purge-data` is requested, any
  unknown or stale data receipt fails complete preflight before package,
  bindings, or known data are removed; the user can rerun a normal non-purging
  remove separately.
- One product has at most one active distribution per installation scope. If a
  qualified alternative is already represented by an installed product, the
  CLI proposes `switch` instead of creating a confusing side-by-side copy.
- The MVP has no general `adopt` command. A manually or externally installed
  native plugin must be removed or migrated through its owning client before
  `agentplugins add`, unless an adapter can prove a distinct
  namespace-qualified identity. This keeps ownership receipts truthful and
  makes later removal safe.

### 12.6 Shared physical backends

Logical clients can share one native installation backend. In the initial
matrix, GitHub Copilot CLI and VS Code can refer to the same Copilot plugin
installation. The MVP does not add reference-counted duplicate client bindings:

- detection assigns one physical backend identity;
- selecting both logical surfaces creates one mutation and reports that the
  plugin is available through both surfaces;
- update and remove operate on that one physical installation;
- a plan targeting either shared surface lists every surface affected by the
  physical mutation, so removal never appears VS Code-only or Copilot-only;
- data is purged only when the shared physical installation itself is removed
  with `--purge-data`;
- the website explains the shared backend instead of implying two installs.

If two selected targets cannot be safely collapsed to one physical backend,
preflight rejects the duplicate rather than mutating twice.

### 12.7 Standard subprocess data contract

For stdio MCP servers, each physical installed plugin instance receives a
dedicated writable `PLUGIN_DATA` directory and the resolved package root as
`PLUGIN_ROOT`. The adapter either proves that the native client supplies the
standard contract or projects an equivalent native configuration; it never
assumes unsupported placeholder behavior.

- create `PLUGIN_DATA` before first launch and keep it outside the replaceable
  package root;
- expand only `${PLUGIN_ROOT}` and `${PLUGIN_DATA}`, once and non-recursively,
  in `args`, `env`, and `cwd`;
- never expand placeholders in `command`;
- reject a stdio server entry that attempts to define either reserved variable
  itself;
- keep resolved `cwd` within its declared root;
- mark this contract `not_applicable` for packages without stdio MCP servers.

## 13. State evolution

State schema 2 already owns source/package bindings, per-client applied package
revisions, scopes, activation/authentication/verification states, physical
artifact IDs, ownership, and receipts. The next schema extends that model rather
than duplicating it.

The next internal state schema adds only:

- origin mode: `directory` or `direct`;
- for `directory`, required product ID, distribution ID/kind, immutable desired
  release sequence, whose distribution/sequence pair is the release identity;
- optional Directory snapshot schema, sequence, and digest used for resolution,
  allowed only for `directory` and required once a Directory-managed operation
  authorizes the installation;
- an operation group ID for one multi-target transaction;
- zero or more installation-level `data_receipts`, keyed by physical
  backend/scope and containing the owned `PLUGIN_DATA` locator; client bindings
  reference a receipt instead of owning its lifetime;
- a minimal `data_retained` installation/data receipt when the last binding is
  removed without `--purge-data`.

Existing source provenance remains in the source binding and is not copied into
a second state object. A `direct` local/full-SHA installation keeps its existing
manifest identity, source binding, and package digests; it does not receive a
fictional Directory product, distribution, release sequence, or snapshot.

The installation-level release is the desired release for newly added or
updated bindings. Each existing client binding continues recording the release
actually applied to it, so an explicit partial update can be explained and
repaired without pretending every client already converged. Distribution ID is
installation-wide and can never diverge per binding.

One physical plugin instance owns at most one data receipt. Shared logical
surfaces reference the same receipt. Removing a binding never deletes its
receipt implicitly; the receipt survives in `data_retained` state until reused
or ownership-checked purge removes it.

Directory status and full product metadata are not duplicated into local state.
The state stores only the immutable identities required to reproduce, update,
repair, remove, and explain an installation.

Migration from current state schema 2:

1. Read and validate old state without mutation.
2. Create a byte-exact backup.
3. Map current catalog installations to their current community distribution,
   never to a newly preferred upstream distribution. Preserve exact/local
   installations as `direct` without fabricating Directory identity.
4. Preserve client bindings, receipts, paths, digests, and timestamps.
5. Mark ambiguous installations `needs_rebind`.
6. Validate the complete new state.
7. Replace atomically and retain the backup until a later successful lifecycle
   operation.

`doctor` and a dry-run migration command expose the plan before mutation.

Migration from state schema 2 is explicit, not a side effect of `add` or
`update`. Read-only commands may inspect schema 2, while lifecycle mutations
return copy-ready `migrate-state --dry-run` and `migrate-state` commands. The
apply command first acquires the global mutation lock and completes or safely
refuses any outstanding journal recovery. It then rechecks the reviewed input
digest, takes the backup, and writes once; it does not require a hidden `--yes`.
An unresolved/degraded prior operation blocks migration instead of being hidden
inside a new schema. Ambiguous records remain installed but blocked from
update/switch until explicit `rebind`.

The existing `migrate-state` entry point remains the only state migration
command. It detects either the pre-v2 legacy file or authoritative schema 2 and
writes the new schema directly after one plan/backup, without committing an
intermediate state version. If authoritative new-schema state already exists,
it refuses rather than merging two stores. Package-format migration remains the
separate `migrate-format` command.

## 14. ChatGPT and client capability boundaries

ChatGPT and Codex are separate target identities even when they share generated
OpenAI package artifacts.

- Codex can receive a local/generated plugin package through its supported
  package workflow.
- ChatGPT may require a registered app binding, Plugins UI activation, OAuth,
  and user consent.
- An arbitrary third-party package is not advertised as installable into
  ChatGPT unless the required app binding exists.
- Website target selection disables ChatGPT for releases without an eligible
  binding and explains the reason.
- Every adapter declares capabilities rather than relying on a growing central
  client `switch`.

Adapter interfaces remain focused:

- detection;
- compatibility planning;
- materialization;
- activation;
- verification;
- removal/compensation.

Adding a client implements these interfaces without changing package parsing or
Directory resolution.

### 14.1 Initial support matrix

The first stable release advertises only behavior backed by the current
Agent Plugins manager and new E2E evidence:

| Target | Initial contract | Scope |
| --- | --- | --- |
| Codex | Managed package materialization with exact activation guidance | User |
| Cursor | Native package materialization and discovery verification | User |
| GitHub Copilot CLI | Managed marketplace/plugin installation through the detected CLI | User |
| VS Code | Copilot-backed installation when available; otherwise exact manual preparation guidance | User |
| Kiro | Native package preparation/import guidance and discovery verification | User |
| ChatGPT | Only releases with a registered app binding; visible UI/OAuth consent remains user-controlled | User |

`plugin-kit-ai` contains useful legacy infrastructure for Claude, Gemini, and
OpenCode, but that does not make them supported Agent Plugins targets. They are
added later only through the same adapter contracts and after isolated E2E.
Until then the website and README do not advertise them as supported by
`agentplugins`.

## 15. Directory integrity and availability

### 15.1 Signed snapshot

The trusted published snapshot contains:

- internal schema version;
- monotonically increasing snapshot sequence;
- generation and expiry timestamps;
- products, distributions, immutable release identities, signed release
  policies, and revocations;
- current trusted verification summaries plus immutable evidence IDs and
  digests needed to reproduce each summary.

Full evidence artifacts remain separate immutable files; the CLI trusts an
eligibility-affecting summary only when it is included in the signed snapshot.
Changing only the selected evidence pointer advances the Directory snapshot
sequence, not the package release sequence.

The snapshot never contains its own digest. The publishing workflow emits a
small detached signature envelope containing envelope schema version, active
key ID, SHA-256 of the exact snapshot bytes, and an Ed25519 signature over a
domain-separated form of those bytes. The CLI verifies digest, signature,
snapshot schema, expiry, and sequence before using short names.

A bounded rotation uses a stable CLI release that temporarily trusts both
current and next public keys, then publishes a snapshot signed by the next key;
a later CLI may remove the retired key. The MVP does not build a separate
key-management service or online trust protocol.

The signing key is available only to a protected release workflow, never to
untrusted pull-request jobs.

Snapshot sequence is publication state, not contributor-authored metadata. A
single-concurrency trusted workflow reads the highest cryptographically valid
sequence from the protected publication branch even when its client expiry has
elapsed, reuses every unchanged release's already-bound source revision,
assigns source
revision/publication time only to new in-repository releases, assigns the next
snapshot value, and builds from the current default branch. It publishes the
versioned JSON and detached envelope plus the small `latest` pointer in one
append-only publication branch commit, then deploys that exact tree. A failed
Pages deployment retries the same committed tree and does not allocate another
sequence. A second
publication rebuilds after the first completes instead of guessing a sequence
in a pull request. The pointer is untrusted; fetched data still passes
signature, digest, expiry, and monotonic-sequence verification. Publication
never requires a self-reference to the snapshot's own digest or Git commit.

The pointer contains only a validated sequence and relative versioned artifact
name, never an arbitrary URL. Production fetches stay on the configured HTTPS
Directory origin, enforce response size limits, and do not forward credentials
across redirects. Local/test origin overrides are explicit and never persisted
as trusted production state.

The MVP serves `/registry/` from the existing GitHub Pages deployment. The
trusted publisher appends versioned snapshot/envelope files and the updated
relative `latest.json` pointer to one protected publication branch that accepts
only that workflow and forbids force-push/deletion. That branch is the durable
publication ledger; `main` remains reviewable source, and the CDN is only a
delivery cache. Pages deploys the exact branch tree containing all previous
`registry/schemas/1/snapshots/<sequence>.json` artifacts plus the new snapshot;
no database or separate blob service is introduced. These internal paths are
not product branding and cannot be contributor-edited in pull requests. If the
CDN briefly exposes a pointer before its target, the CLI performs a bounded
idempotent retry and then keeps last-known-good or embedded data. A
missing/partial artifact is never accepted as a reason to skip signature or
sequence checks.

The binary embeds the production origin, trusted public key IDs, and one
known-good signed snapshot as its bootstrap/fallback. A fresh client accepts a
remote snapshot only for a locally supported internal schema and never below
the embedded sequence floor. Each internal snapshot schema has its own `latest`
pointer. The MVP publishes schema 1 only; a future incompatible schema must be
served side by side and cannot replace the schema 1 pointer while supported
CLI releases still depend on it.

A valid unexpired signed snapshot above the embedded floor but older than a
fresh client's unseen global latest cannot be distinguished without a
transparency/freshness service. The MVP makes this residual replay window
explicit; expiry bounds it, TLS protects the configured origin, and subsequent
local sequence history prevents rollback below the highest accepted value.

Initial snapshots are valid for 30 days. A trusted weekly workflow republishes
unchanged reviewed data with a higher snapshot sequence and fresh expiry, and a
Directory change or emergency action publishes immediately. Clock-skew errors
produce a specific diagnostic; the MVP does not add a time service.

### 15.2 Cache behavior

- Cache the last valid snapshot and its highest accepted sequence.
- The effective local sequence floor is the maximum of the binary's embedded
  floor, the cache's highest accepted sequence, and snapshot sequences recorded
  by existing Directory-managed installations for that schema. Deleting one
  cache file cannot lower provenance already recorded in installation state.
- A newer invalid snapshot never replaces last-known-good data.
- Reject a snapshot with a lower sequence to prevent rollback.
- Before expiry, cached data may be used when the network is unavailable.
- After expiry, new short-name installs fail closed.
- `list`, `info`, `remove`, and direct exact pinned/local source flows continue
  when they do not require new remote data.
- Directory-managed new-target add, repair, and rematerialization require an
  unexpired snapshot plus the exact verified package source so a known
  revocation or incomplete local copy cannot be bypassed.
- Discovering or authorizing a newer Directory release requires an unexpired
  snapshot. Before expiry, `update` may work offline only when both the eligible
  cached release metadata and immutable package source are available;
  otherwise it makes no changes.

### 15.3 Availability, suspension, and revocation

Distribution and release status are deliberately separate:

- Distribution `candidate`: reviewable source-only state, excluded from the
  public snapshot and all resolution until its activation change merges.
- Distribution `active`: eligible for default, fallback, or qualified
  resolution when it has an eligible active release.
- Distribution `suspended`: retained in the snapshot for history and warnings,
  but blocked for new installs, new-target exposure, and updates.
- Release `active`: considered for new installs and updates.
- Release `superseded`: immutable historical release used only to explain,
  repair, or extend an already recorded installation.
- Release `revoked`: known unsafe bytes; CLI blocks new installs and warns
  existing installations.

Publishing a higher release does not implicitly change older release statuses;
descending sequence already selects the newest eligible active release. A
reviewed change may move a release between `active` and `superseded`, while
`revoked` is terminal. A published release record is never deleted and its
source, sequence, package metadata, inventory, and digests never change.
Compatibility/evidence policy may change only through reviewed signed snapshot
publication. A distribution may move
`candidate -> active` and `active <-> suspended`; once published, it is retained
for provenance.

Changing the product default from a bridge to upstream does not suspend the
bridge. It remains an explicit qualified alternative and, when compatible, a
fallback; only the reviewed `default_distribution` changes.
- No status silently selects a different distribution for existing installs.
- Revocation blocks new installs, new-target exposure, repair, and
  rematerialization of those bytes. Removal is always allowed, and update to an
  eligible non-revoked release in the same distribution remains available.
- Suspension still allows ownership-safe removal and exact-revision repair of
  an existing non-revoked release. If the immutable source is unavailable,
  repair fails without mutation. A safety issue with package bytes uses release
  revocation rather than relying on suspension.
- If that distribution has no safe update, `doctor` suggests explicit qualified
  `switch` alternatives but never performs the source change automatically.
- A signed emergency snapshot is the primary control for future operations and
  warnings. It cannot remotely stop a process that is already running on a
  user's machine.

## 16. Upstream packaging and promotion workflow

### 16.1 Before the upstream PR

1. Identify upstream repository, owner, package components, license, and release
   policy.
2. Pin one full upstream commit SHA.
3. Build and review the community bridge.
4. Run schema and sandbox installation tests.
5. Publish the bridge through the Directory so the short name works now.
6. Prepare the upstream standards-focused PR from the same reviewed package
   content where upstream layout permits.

The bridge and upstream PR are reviewed together but do not need identical tree
digests: community attribution, description, and repository layout can differ.
The tracked promotion digest is computed from the exact proposed upstream
plugin root, never borrowed from the bridge release.

The PR should contain separable commits for package, validation, and optional
documentation, but it remains one coherent upstream PR.

Its title and body lead with the Agent Plugins 1.0 package, component layout,
and exact client test evidence. It does not present the PR as promotion for our
Directory or require upstream CI to install our CLI. A CLI command is added to
upstream documentation only when that maintainer explicitly wants it.

The first cohort is limited to three to five popular packages that either
already have an upstream Agent Plugins package or have a clear license and a
small bridge surface. Broader outreach begins only after this cohort proves the
builder, E2E, and promotion flow.

### 16.2 Merge monitoring

Upstream repositories normally do not send events to our workflows. The first
slice is a manual `workflow_dispatch` check with bounded idempotent retries. A
scheduled GitHub Actions poll is enabled only after one real tracked PR proves
the same path.

The MVP does not create a separate proposal database or workflow engine. Each
tracked record stores only the upstream PR URL, reviewed head SHA, reviewed
package digest, expected path, and optional release policy. Current PR state is
read from GitHub. Re-running the job is idempotent because promotion is keyed by
product, distribution, merge SHA, and package digest.

### 16.3 Promotion gate

After merge:

1. Resolve merge SHA and intended upstream branch.
2. Read the package root at that exact SHA.
3. Compute the package tree digest, not only the commit SHA.
4. Compare it with the reviewed PR package digest.
5. Validate manifest identity, component inventory, license, and capability
   contract.
6. Repeat schema and isolated installation tests.
7. Create a Directory promotion PR.
8. Publish a signed snapshot after the promotion PR merges.

Squash or rebase merges are safe when package digests remain equal. Maintainer
changes inside the package produce a digest mismatch and require manual review.
Changes outside the package root do not block promotion.

### 16.4 Rollout of promotion automation

1. Shadow mode: report what would be promoted.
2. Generated promotion PR with manual merge.
3. After at least three successful exact-match manual promotions, auto-merge
   only exact-digest, green, previously reviewed promotions.
4. Continue using manual review for changed packages, capability changes,
   license changes, auth changes, or publisher identity changes.

### 16.5 Existing installations

Promoting upstream changes only the default for new resolutions. Existing
bridge installations remain reproducible and update within the bridge channel.
The bridge remains an active qualified alternative and possible target-aware
fallback; promotion does not delete or relabel its package bytes. Only an
explicit bridge release update supersedes an older release in that bridge
distribution.

## 17. Bridge update workflow

The MVP updates a bridge through an explicit pull request that changes its
pinned upstream SHA and regenerates the package. It does not continuously watch
every upstream repository. After at least three bridges demonstrate the same
maintenance pattern, a small read-only watcher may open an issue or draft pull
request, but it must never publish changed instructions or executable content
automatically.

An update PR contains:

- old and new upstream SHAs;
- upstream changelog/release link when available;
- regenerated package diff;
- license diff;
- component inventory diff;
- MCP endpoint/command diff;
- validation and installation results;
- whether an overlay replaces content changed by upstream.

The PR fails when upstream paths disappear, LFS pointers replace required
content, submodules are introduced, a license changes, an overlay conflicts with
changed upstream content, component capability drops, or a new executable
appears without review.

## 18. Website and contributor UX

### 18.1 Product cards

The page opens with the concrete product promise: one CLI command installs one
Agent Plugins 1.0 package into one or several selected supported clients, and
the same CLI updates, repairs, inspects, switches, and removes it. It does not
imply that every package works in every client or that all detected clients are
mutated automatically.

The website shows one card per product, never one card per distribution. A card
contains:

- plugin name, logo, short description, and categories;
- current default source badge: `Upstream package` or `Community package`;
- component inventory;
- verified client levels;
- authentication requirement;
- selected-client install command;
- link to source, immutable revision, evidence, and alternatives.

The website is discovery UX, not installation authority. Its primary copy
action emits only a reviewed product/distribution selector and explicit target
IDs; the CLI independently resolves that selector from its valid signed
snapshot and verifies the downloaded package. No source revision, digest, or
eligibility claim copied from page state can override CLI resolution.

Pull requests may deploy a clearly labelled review preview from unresolved
source data. The production catalog, compatibility choices, and commands are
generated only from the exact signed snapshot sequence already committed to the
publication ledger. If signing or catalog deployment fails, production keeps
the previous complete catalog rather than showing a package the CLI cannot yet
resolve.

The source badge is explicitly labelled `Default source`. If the selected
target set would require a known fallback, the card previews that expected
source and reason beside the command instead of continuing to imply upstream;
the CLI still recomputes the decision authoritatively.

The term `bridge` may appear in provenance details but is not required for the
primary call to action.

Website evidence badges name the tested client version, OS/architecture, level,
and date or link to that detail; they never imply every environment was tested.
The CLI matches evidence against its detected applicability tuple. A pass from a
different tuple remains useful history but is shown as non-applicable rather
than silently upgraded to a local guarantee.

Every in-repository package README keeps one delimited, copy-ready short-name
install block validated from Directory data; human documentation outside that
block is not regenerated. External distributions receive their command on the
website without requiring changes to the publisher's README. Source labels,
client names, command syntax, and authentication summaries are checked from the
same data wherever they appear, so website cards, CLI help, package blocks, and
the root quick start cannot contradict one another.

The package block contains only stable command syntax and plugin identity.
Mutable default-source, verification, and availability labels stay in generated
website/root documentation so a Directory metadata change does not rewrite an
installed package tree or create a fake package release. An intentional package
README change still changes the package digest like any other package file.

### 18.2 Target selection

- Client selection is a custom accessible multiselect.
- The compact selector sits beside the quick-start command rather than in a
  separate explanatory section.
- Client choices use locally stored official logos where permitted, with an
  accessible text fallback and recorded asset source/trademark note.
- Only clients compatible with the selected release are selectable.
- Generated commands always contain an explicit comma-separated target list.
- The UI distinguishes prepared/manual activation from fully managed install.
- ChatGPT is disabled with an explanation when no app binding exists.
- Category, component, and source filters use accessible custom popovers;
  category selection is searchable and fully keyboard-operable.

### 18.3 External submissions

An external author can submit either:

- a direct upstream package reference;
- a complete community package;
- a proposed bridge recipe when redistribution is permitted.

An `Add a plugin` action is visible beside the catalog count and again after the
catalog. It opens the same concise pull-request contribution path; the MVP does
not add accounts, a submission API, or a database-backed form.

Untrusted pull-request CI validates data, reproduces bridges, scans secrets, and
materializes packages in isolated sandboxes without publishing, signing, or
using production secrets.

Directory titles and descriptions render as text, not contributor HTML. URLs
are restricted to reviewed HTTP(S) fields, and submitted SVG/icon assets are
sanitized or rejected before same-origin publication. The static site uses a
restrictive Content Security Policy and never renders package README HTML
directly into product cards.

The root README mirrors this UX with one multi-target quick start, add/update/
remove examples, the supported-client boundaries, and a direct immutable
GitHub/local-source example proving that Directory submission is optional.

If focused UI components are reused from the MIT-licensed `plugin-kit-ai`
landing, retain the required license/attribution and copy only the components
that fit this static site. Do not import its plugin-kit-specific backend,
content model, or dependency graph wholesale.

## 19. Repository extraction plan

Extraction occurs after the behavior is covered by stable tests.

### 19.1 Move to `agentplugins-cli`

Move the Agent Plugins-specific Go packages, CLI entry point, focused tests,
cross-platform release workflow, and npm binary downloader. Preserve history
where practical, but correctness and reviewable boundaries have priority over a
perfect history rewrite.

### 19.2 Compatibility

- npm remains `universal-agent-plugins`.
- installed binary remains `agentplugins`.
- existing commands and state paths remain compatible.
- the npm wrapper changes binary release origin without changing user syntax.
- `plugin-kit-ai` can retain one temporary compatibility notice or shim for a
  release, but does not retain a second lifecycle engine.
- no cyclic Go module dependency is introduced.

### 19.3 Release targets

Publish checksum-verified binaries for supported combinations of:

- macOS arm64 and amd64;
- Linux arm64 and amd64;
- Windows amd64, and arm64 when the adapter/toolchain matrix is proven.

Release CI verifies checksums, binary version output, npm smoke install, state
compatibility, and one isolated lifecycle smoke per OS.

## 20. Security model

### 20.1 Trust boundaries

- Directory contribution: untrusted input.
- Bridge build: untrusted upstream content processed without execution.
- Signed snapshot publication: privileged trusted workflow.
- CLI binary release: privileged trusted workflow independent from package PRs.
- Client runtime: outside Directory control and potentially destructive.

### 20.2 Package acquisition

- Registry GitHub sources use full commit SHAs.
- Directory distributions must be publicly fetchable without forwarding user
  credentials. Authenticated private GitHub resolution is not part of the first
  stable Directory contract; a private author can use a local direct package
  without publishing its identity or token.
- Sparse fetch limits transfer to declared package/bridge paths.
- Package size and file-count limits apply to the plugin root, not the entire
  upstream monorepo.
- Path traversal, absolute/non-UTF-8 paths, case- or Unicode-normalization
  collisions, device names, external symlinks, special files, and archive
  escapes are rejected.
- Tree digests include normalized relative path, filesystem kind, file bytes,
  executable mode, and an internal symlink's exact link target. A selected
  client that cannot safely materialize a valid internal symlink fails
  preflight rather than receiving altered package bytes.
- The tree-digest algorithm has an explicit internal version and domain tag.
  Entries are sorted by canonical path bytes and every path, kind, mode, target,
  and content field is length-prefixed before SHA-256; concatenated strings or
  host-dependent directory iteration are forbidden. A future incompatible
  digest algorithm gets a new digest version and cannot reinterpret an existing
  release.
- Immutable Git sources derive path type and executable mode from Git tree
  metadata rather than host checkout permissions, so the same revision hashes
  identically on macOS, Linux, and Windows. Direct local paths use their local
  filesystem metadata and are not claimed to be cross-machine portable.
- Git LFS pointers and submodules are unsupported initially unless the package
  contains no required content behind them.
- Executable bits enter the package digest.

### 20.3 Workflow security

- Pull-request workflows receive no signing or release secrets.
- Privileged workflows run only from trusted default-branch code.
- Post-merge publication has two jobs: a no-secret preparation job reacquires
  and validates package inputs and emits one size-bounded canonical candidate;
  the privileged signer validates only that candidate schema/digest, assigns
  publication fields, signs it, and updates the publication branch. The signing
  job never parses package trees, runs bridge builders, or executes contributed
  content.
- Signing workflows, canonicalization code, trust metadata, and release scripts
  require trusted code-owner review and branch protection.
- The publication-ledger branch accepts only the trusted publisher, forbids
  force-push/deletion, and rejects edits/removal of prior snapshot files.
- Never use `pull_request_target` to execute or build untrusted checked-out
  content with secrets.
- Actions remain pinned to immutable commit SHAs.
- Generated artifacts include checksums and release attestations where the
  hosting platform supports them.
- Static-site generation escapes contributor text, validates outbound URLs,
  sanitizes or rejects active image content, and ships a restrictive CSP.

### 20.4 Runtime transparency

Before installation, the plan exposes:

- local executable commands;
- remote MCP URLs;
- requested environment placeholders;
- client configuration paths;
- manual activation and authentication requirements;
- destructive/write-capable evidence when known.

No token, OAuth credential, cookie, or secret value enters Directory data,
bridge recipes, package manifests, command output, or test artifacts.

Human and local structured CLI output may include the exact operational paths
needed for activation, automation, repair, or retained-data cleanup. Those
fields are explicitly typed as local paths and never contain credentials or
environment values. Published evidence and CI artifacts use a separate redacted
export that exposes logical locators and digests, never absolute home-directory
paths or credential locations; raw local command JSON is not uploaded as
evidence.

## 21. Detailed implementation phases

### Phase 0 - Contract fixtures and decision lock

#### Summary

Turn the decisions in this document into executable fixtures before changing
behavior.

#### Steps

1. Add fixtures for product, upstream distribution, bridge distribution,
   immutable release identity, mutable signed release policy, verification,
   canonical snapshot, and detached signature envelope.
2. Add CLI golden outputs for single and comma-separated targets.
3. Add state migration fixtures for current community installations.
4. Add a bridge recipe fixture with a tiny pinned upstream test repository.
5. Document public terminology and prohibit `Registry v3` in UI copy.
6. Freeze the initial client support matrix and exact launch evidence targets.
7. Pin published 1.0.0 conformance fixtures and record 1.1.0 as an unsupported
   working draft rather than an implicit compatible alias.
8. Confirm the isolated Notion test workspace/identity and consent path needed
   for the mandatory 5-by-3 runtime gate before implementation relies on it.
   Confirm the named Cloudflare Docs ChatGPT development binding and manual UI
   consent path at the same time; neither credential is stored in CI.
9. Freeze practical package-root, file-count, individual-file, and manifest size
   limits in fixtures; diagnostics must print the violated limit.
10. Freeze the first three-to-five upstream cohort with exact repositories,
    package roots, licenses, bridge classification, and one named Directory
    owner per entry; no implementation slot is reserved for an unverified
    candidate.
11. Freeze distribution versus release status semantics, highest-eligible
    release selection, and the reviewed current-evidence pointer contract.

#### Tests

- Schema fixture validation.
- Stable canonical JSON output.
- Golden human and JSON CLI output.

#### Rollback

Fixtures and schemas are additive and do not affect current releases.

#### Acceptance criteria

- Every later phase can be tested against a fixed public behavior contract.

### Phase 1 - Package loader and source hardening

#### Summary

Make arbitrary standard packages safe and deterministic before broadening source
resolution or target count.

#### Steps

1. Split package resolution, acquisition, loading, validation, and diagnostics
   into focused interfaces.
2. Add schema-keyed loaders with published Agent Plugins 1.0.0 as the first
   implementation.
3. Preserve specification failure boundaries for manifest, skills, MCP config,
   and server entries.
4. Implement sparse GitHub acquisition for exact revisions.
5. Hash the complete plugin root deterministically.
6. Add path, case, special-file, size, LFS, submodule, and executable-mode checks.
7. Resolve one package once and retain one immutable local snapshot throughout
   planning and apply.
8. Enforce matching `plugin.json`/`mcp.json` schema versions and exact
   `PLUGIN_ROOT`/`PLUGIN_DATA` placeholder validation rules.
9. Record bundled and bare stdio executable requirements without running them.

#### Tests

- Official specification fixtures.
- Published 1.0.0 succeeds; unrecognized and working-draft schemas fail with an
  explicit unsupported-version diagnostic.
- Invalid manifest versus isolated invalid component fixtures.
- Unknown manifest fields remain diagnostic-only, while mismatched MCP schema
  disables MCP and preserves valid skills.
- Large monorepo with small plugin subpath.
- Changed source after resolution cannot affect the retained snapshot.
- Cross-platform digest parity.
- Digest framing fixtures with empty files, prefix-like paths, executable mode,
  and symlink targets.

#### Rollback

Keep the existing exact-source resolver behind an internal fallback during
development. Remove the fallback before release rather than silently using it
after a security validation failure.

#### Acceptance criteria

- A package is resolved once, validated once, and represented by one immutable
  digest before any client plan exists.

### Phase 2 - Multi-target lifecycle and state

#### Summary

Deliver the core product advantage: one command for several clients.

#### Steps

1. Add comma-separated target parsing and interactive multiselect.
2. Separate ChatGPT and Codex target identities.
3. Plan all selected clients from one package envelope.
4. Add operation groups and state fields for product/distribution/release.
5. Stage all managed changes before commit.
6. Implement group rollback and external activation compensation rules.
7. Add the public `repair` command, then extend update, remove, list, info, and
   doctor to multiple bindings using the exact-revision repair contract.
8. Add explicit digest-checked state schema migration with backup and
   `needs_rebind` handling; read commands remain available before migration.
9. Add active-installation `switch` while retaining `rebind` and
   `migrate-format` as narrow recovery/migration commands.
10. Replace Directory update ordering based on package-version comparison with
    signed release sequence; keep direct and legacy transition policy separate.
11. Remove the hidden `--yes` flag entirely and keep destructive data deletion
    explicit through `--purge-data`.
12. Expose only proven user scope, collapse shared physical backends, and add the
    standard stdio data-directory contract.
13. Add adapter-aware executable preflight and exact missing-runtime guidance;
    do not add runtime installation.
14. Add adapter-level observation of pre-existing native identities. Reject
    unmanaged or indeterminate collisions before staging; do not add automatic
    adoption in the MVP.

#### Tests

- Two- and three-client add/update/repair/remove.
- Source selection uses the complete selected target set and never combines
  releases or distributions across clients.
- Unsupported final target causes zero mutation.
- Second managed commit failure restores the first.
- Native config changed between plan and commit aborts without clobbering it.
- A same-name native plugin installed outside agentplugins is never overwritten,
  adopted, or removed; namespace-qualified coexistence passes only when the
  adapter proves the identities are distinct.
- Native activation failure retains valid materialization and gives repair
  instructions.
- Repair reuses the recorded client revision and cannot act as an implicit
  update or source migration.
- Selecting Copilot CLI and VS Code performs one physical backend mutation.
- Project scope fails complete preflight and never falls back to user scope.
- Missing managed stdio runtime causes zero mutation; client-managed runtime
  uncertainty remains an explicit verification action.
- Same-version and non-SemVer Directory releases update by increasing release
  sequence; an unrecognized source change cannot use that authority.
- A fixture marker in `PLUGIN_DATA` survives update, repair, source switch, and
  normal removal; only explicit `--purge-data` deletes it.
- Process crash and journal recovery at every phase boundary.
- Concurrent operations on the same installation serialize correctly.
- Concurrent mutations for different plugins that share a client config also
  serialize under the global lock.
- State schema 2 readout works before migration; lifecycle mutation refuses with
  copy-ready commands, and explicit migration needs no hidden `--yes`.
- Migration refuses while journal recovery is unresolved and never competes
  with an older CLI process for the state file.
- Root help and command parsing expose no `--yes` flag.

#### Rollback

Old state remains backed up. The previous binary must still read or clearly
refuse the new state without corrupting it. Because the current store strictly
rejects unknown schemas, once the new state has been committed and used, the
operational recovery path is a fixed-forward binary that understands that
schema, not a silent binary downgrade. A failed migration before commit restores
the old file byte-for-byte; the backup is not automatically restored after
later lifecycle mutations because that would discard newer state.

#### Acceptance criteria

```bash
agentplugins add ./fixture --target codex,cursor,kiro
```

uses one package digest, produces three client bindings, and never leaves hidden
partial managed mutations. Shared physical backends are collapsed, and package
updates do not delete persistent plugin data. A pre-existing unmanaged native
identity causes the complete operation to fail before staging.

### Phase 3 - Directory domain and website migration

#### Summary

Replace the public flat catalog mental model with product, distribution,
release, and verification while retaining compatibility outputs for old CLI
builds.

#### Steps

1. Add reviewable Directory source schemas.
2. Migrate 26 current packages into products and community distributions.
3. Generate the deterministic review preview and preview-only search data; do not
   create publication sequence/expiry or production catalog data in
   pull-request CI.
4. Implement declared-default selection, reviewed promotion, target-aware
   fallback priority, and qualified distribution resolution.
5. Update the website to one card per product and a target multiselect.
6. Hide legacy catalog version terminology from public copy.
7. Keep legacy catalog outputs byte-frozen during the transition and fail CI if
   Directory generation changes them.
8. Validate every in-repository package README install block from Directory
   data without rewriting surrounding human documentation.
9. Keep the root README and website quick start aligned for multi-target,
   lifecycle, and direct-source installation.

#### Tests

- Alias uniqueness and reservation.
- Default distribution eligibility.
- A valid but not-yet-promoted upstream candidate is inactive for unqualified
  resolution and does not bypass the declared default.
- An incompatible declared default falls back once for the complete target set
  and explains why.
- One website card despite multiple distributions.
- Deterministic review preview and preview-only search data.
- Production catalog is derived from one published signed snapshot sequence and
  never exposes an unresolved review candidate.
- In-repository package source binds to the real post-merge commit only after
  its reviewed tree/manifest digests match.
- Existing short names retain a working community default at migration time.
- All 26 package README commands match website and CLI syntax.
- Keyboard and screen-reader tests cover target multiselect, searchable filters,
  copy command, and `Add a plugin` actions.
- Evidence badge fixtures never collapse one OS/client-version pass into an
  all-environments claim.
- Contributor text, malicious links, and active SVG fixtures cannot produce
  executable site content.

#### Rollback

New CLI builds can be held on an embedded known-good Directory snapshot while
legacy catalog clients remain unaffected.

#### Acceptance criteria

- Every current short name resolves to exactly one eligible release.
- No current installation is silently rebound to a new source.

### Phase 4 - Build-time bridge pipeline

#### Summary

Make selected popular upstream projects available immediately without forks or
install-time source composition.

#### Steps

1. Add the minimal bridge recipe schema and deterministic builder.
2. Add only `build <id>` and `check`; add more commands only after repeated
   maintainer usage proves the need.
3. Enforce licensing, attribution, source pins, copy allowlists, and component
   expectations.
4. Create the first three to five bridge packages.
5. Run schema and isolated materialization tests for every client compatibility
   claim made by each bridge.
6. Open standards-focused upstream PRs in parallel with bridge publication.

#### Tests

- Reproduction from a clean checkout.
- Upstream SHA change produces a reviewable deterministic diff.
- License, overlay conflict, path, LFS, and submodule failures stop generation.
- A zero-copy MCP recipe without pinned endpoint/command provenance is rejected
  or classified as community rather than bridge.
- Builder never executes upstream code.
- Committed output equals temporary rebuild byte-for-byte.

#### Rollback

A bridge distribution can be suspended in Directory source data without
deleting its immutable package. Existing installs retain provenance and receive
a warning rather than a silent source switch.

#### Acceptance criteria

- A short name works before the upstream PR merges.
- The installed package is a complete standard package with one tree digest.
- Rebuilding from recipe and upstream SHA reproduces the committed package.

### Phase 5 - Signed Directory consumption

#### Summary

Allow autonomous Directory updates without tying every new package to a CLI
release.

#### Steps

1. Add canonical snapshot generation plus detached digest/signature envelope.
2. Add CLI signature, expiry, schema, and sequence verification.
3. Add atomic last-known-good cache persistence.
4. Add one active snapshot key ID and a documented two-key CLI overlap for
   rotation.
5. Move privileged signing to a protected default-branch workflow.
6. In a no-secret preparation job, bind in-repository package paths to the
   actual post-merge commit and reacquire every changed external package
   revision plus every release whose eligibility policy is being broadened;
   emit a canonical candidate only when all tree/manifest digests still equal
   the reviewed records. Reuse the prior signed binding for every unchanged
   release.
7. Implement distribution suspension, release revocation, current-evidence
   summary consumption, and their operation-specific warnings/blocks.
8. Add the weekly bounded-expiry refresh workflow and emergency on-demand
   publication path.
9. Persist signed artifacts in the protected append-only publication branch and
   deploy Pages from that exact tree.
10. Generate the production catalog/commands from that same committed snapshot;
    keep the prior catalog when signing or deployment is incomplete.

#### Tests

- Snapshot byte tampering, envelope digest mismatch, invalid signature, schema,
  expiry, and lower sequence.
- Absolute/cross-origin pointer, oversized response, and interrupted publication.
- Publication branch append-only enforcement, prior-file immutability, and exact
  Pages tree deployment.
- Actual merge SHA binding succeeds only when post-merge package digests equal
  the reviewed source record.
- Weekly, metadata-only, and evidence-only publication preserves every existing
  release's exact package-source revision and original publication timestamp.
- Changed external revision is unavailable/private or differs from reviewed
  digests: publish nothing.
- Malformed/oversized candidate, candidate-digest mismatch, or any package-tree
  access in the signer job prevents signing and branch mutation.
- Interrupted cache write.
- Unknown key ID, current/next overlap, replacement-key snapshot, and retired
  key rejection after the later CLI release.
- Remote sequence below the embedded floor and incompatible schema pointer
  replacement.
- Offline before and after expiry.
- Unchanged weekly refresh advances sequence and expiry without changing
  product or release identity.
- Evidence-only publication advances snapshot sequence without creating a
  package release, and a contributor timestamp cannot select current evidence.
- Suspended distribution and superseded/revoked release behavior match the
  operation matrix.
- Revoked release blocks install/new-target/repair, while remove and update to
  a non-revoked release remain available.
- Exact pinned/local source remains usable after Directory failure.

#### Rollback

Publish a higher-sequence signed emergency snapshot that suspends affected
distributions and/or revokes unsafe releases. If signing infrastructure itself
is compromised, release a CLI with rotated trust roots; do not bypass signature
verification.

#### Acceptance criteria

- A valid new Directory release becomes resolvable without a CLI release.
- A tampered snapshot or sequence below the embedded/cached floor cannot change
  source resolution.

### Phase 6 - Launch E2E and release

#### Summary

Prove the public promise in disposable environments and ship one stable release.

#### Required E2E scenarios

- All 26 Directory packages complete `add`, `info`, and `remove` in at least one
  isolated supported client. `info` must reconcile the owned receipt/native
  discovery state; a successful file copy alone is not enough. This gate proves
  lifecycle installation, not runtime behavior for all 26 services.
- The fixed launch hero set is Agent Code Navigator, Context7, Cloudflare Docs,
  Chrome DevTools, and Notion. These five complete add, update, remove,
  discovery, and runtime E2E in Codex, Cursor, and Kiro: 15 required
  client/plugin runtime results. Repair is fault-injected once per client
  adapter rather than redundantly for every plugin.
- Notion's OAuth evidence is a separate artifact but is required for its three
  runtime results. Phase 0 must secure an isolated test workspace/identity and
  human consent; without it the release is not allowed to claim 15/15. The four
  no-account plugins remain the public quick-start set.
- Cloudflare Docs completes one registered-binding ChatGPT E2E: the generated
  binding matches the registered app ID, the user activates it through the
  Plugins UI, and a read-only runtime call succeeds. The CLI must retain an
  honest manual-activation state until that user-controlled step is attested.
  If this named binding is unavailable, the stable launch does not advertise
  ChatGPT support.
- Upstream-owned short-name install.
- Community bridge short-name install.
- Direct external package without Directory submission.
- One real Context7 invocation with `--target codex,cursor,kiro` in disposable
  client homes, proving one acquisition/digest and three per-client outcomes.
- Multi-target update/repair/remove over that installation.
- Copilot CLI plus VS Code selection with one shared physical mutation.
- Persistent `PLUGIN_DATA` across update, repair, switch, and normal removal,
  followed by explicit purge.
- Stdio fixture proving the exact `PLUGIN_ROOT`/`PLUGIN_DATA` environment,
  placeholder expansion, writable persistence, and containment behavior.
- Missing-runtime fixture proving zero mutation and a copy-ready requirement
  message without automatic dependency installation.
- Explicit source switch.
- Managed rollback and external activation failure.
- Directory offline, expired, tampered, and rolled back.
- State migration and crash recovery.
- Fork-based external contributor submission.
- One complete manual promotion-gate simulation against immutable fixture
  repositories, including digest mismatch refusal.
- Cross-platform binary/npm install.
- No runtime or OAuth result is recorded without the required consent and test
  identity.

All client launch, provisioning, terminal runtime, and task-assignment tests run
only in new disposable sandbox/test projects, never in real user projects.

#### Acceptance criteria

- Every public support claim links to exact evidence.
- Runtime evidence records package digest, CLI/adapter version, client version,
  OS/architecture, level, and timestamp.
- The 26-package installation gate and 5-by-3 hero runtime gate are green.
- The named ChatGPT registered-binding/UI E2E is green and separately scoped;
  it is not presented as evidence for all 26 packages.
- CI is green on the release commit.
- The release contains immutable assets, checksums, and attestations.
- Root quick start, all in-repository package README blocks, website command
  generation, and CLI help agree.
- Ship a stable release, not a public beta-only command contract.

### Phase 7 - Upstream monitoring and promotion

#### Summary

Automate the safe transition from community bridge to upstream-owned package.

#### Steps

1. Track upstream PR URL, reviewed head, package path, and reviewed package
   digest.
2. Add an on-demand merge check and manual recovery dispatch; enable scheduled
   polling only after one real tracked PR proves the path.
3. Derive current PR state from GitHub and make every job idempotent without a
   separate proposal state machine.
4. Compare package closure digest after merge.
5. Run eligibility and installation gates.
6. Generate a promotion PR.
7. Roll out shadow, manual merge, and exact-match auto-merge stages.

Phase 7 is a post-launch operational improvement, not a dependency of the first
stable release. Its first usable slice is on-demand detection plus a generated
promotion PR that is manually reviewed and merged. Scheduled polling follows
one real tracked PR; exact-match auto-merge follows the three real manual
promotions required by section 16.4.

#### Tests

- Merge commit, squash, and rebase.
- Package changed by maintainer.
- Docs-only changes outside package.
- Merge into a non-release branch.
- PR closed, repository renamed/transferred, package reverted or removed.
- Duplicate schedule events and temporary GitHub 5xx responses.

#### Rollback

Promotion changes only the new-install default. A new higher-sequence signed
snapshot can restore a previous eligible default for future installs; an old
snapshot is never republished, and existing installations remain pinned
throughout.

#### Acceptance criteria

- Exact reviewed upstream content can become the new default without manual
  data editing.
- Any material difference enters manual review.

### Phase 8 - Extract the security-critical CLI

#### Summary

Move the stable Agent Plugins manager out of `plugin-kit-ai` without changing
the user's npm command or duplicating the engine.

This phase begins after at least two stable releases prove the public
command/state contracts. It is not part of the first stable-release gate.

#### Steps

1. Create `agentplugins-cli` with the focused Go module and release pipeline.
2. Move loader, resolver, lifecycle, adapters, state, CLI, and focused tests.
3. Remove dependencies on legacy authoring and unrelated integration targets.
4. Point the npm downloader at new release assets and checksums.
5. Retain a time-bounded compatibility notice/shim in `plugin-kit-ai`.
6. Update Directory README and security documentation.

#### Tests

- Same commands and JSON contracts before and after extraction.
- Existing state lifecycle on all supported OS targets.
- npm clean install and binary checksum verification.
- Previous release rollback and new release upgrade.

#### Rollback

The npm wrapper can be republished to the previous verified binary release.
State schema compatibility must be tested before switching release origin.

#### Acceptance criteria

- The community package repository cannot publish or alter CLI binaries.
- `plugin-kit-ai` and `agentplugins-cli` do not contain competing lifecycle
  implementations.

## 22. Edge-case matrix

| Edge case | Required behavior |
| --- | --- |
| Upstream already ships a valid package | Prefer it after eligibility verification; no bridge required |
| Upstream lacks `plugin.json` | Publish a verified bridge immediately and open one upstream packaging PR |
| Valid upstream has not passed Directory promotion | Keep it inactive for unqualified resolution and retain the reviewed declared default |
| Upstream package exists but loses promised components | Keep eligible bridge as default and explain capability difference |
| Upstream PR is modified before merge | Compare package digest; require manual review on any package change |
| Upstream PR is squashed/rebased | Promote when package digest and gates still match |
| Upstream PR is closed | Keep bridge active and perform no promotion |
| Upstream merge is later reverted | Suspend new upstream installs; do not silently move existing ones |
| Upstream repository is renamed | Preserve stable product/distribution IDs; review updated provenance |
| Upstream becomes private/deleted | Suspend the unavailable distribution; retain historical state and bridge alternative |
| Upstream license changes | Block bridge regeneration and promotion pending review |
| No redistributable license | Do not copy skills/binaries into a bridge |
| Second in-repository distribution is proposed for one product | Require an external publisher repository or an explicit migration; do not invent colliding `plugins/<id>` outputs |
| Bridge overlay conflicts with changed upstream content | Fail generation and require reviewed correction |
| Manifest name changes | Treat as explicit identity migration or new product, never automatic |
| Alias collides or is renamed | Keep publisher-qualified IDs and reserve historical aliases |
| Local selector matches multiple installed identities | Fail with installation IDs and require an exact selector; never guess from manifest name |
| Contributor metadata or icon contains active content | Escape text, validate URLs, sanitize/reject the asset, and publish nothing executable |
| Website page or copied UI metadata is stale/tampered | Treat it as discovery-only; CLI re-resolves the selector from a valid signed snapshot and verifies package bytes |
| Directory source merges but signing/publication fails | Keep the previous production catalog and commands; show the new data only in labelled review/admin diagnostics |
| Production site and registry would use different sequences | Refuse deployment; publish the site catalog and registry tree from one committed publication snapshot |
| Package version is missing/non-SemVer | Order with Directory release sequence |
| New Directory release keeps or lowers manifest version | Accept only a strictly higher signed release sequence within the same distribution |
| Runtime/OAuth evidence is missing | Keep the static-compatible release eligible with an honest untested label unless the product declares a stricter reviewed gate |
| Current trusted materialization/discovery/runtime evidence failed | Exclude that release for the failed client until a newer package or reviewed replacement evidence resolves it |
| Test is cancelled, rate-limited, or fails because of harness/vendor availability | Record an honest inconclusive observation; do not claim a pass or block installation as a deterministic incompatibility |
| A trusted retest passes after a failure for the same tuple | Reviewably move the current evidence pointer to the pass; retain the failure as immutable history |
| Package/dependency/adapter/client-version tuple changes after a failure | Treat the new tuple as untested; never let the inapplicable historical failure block it |
| Website has a pass for another OS/client version | Show the exact tested environment; CLI treats it as history, not proof for the detected tuple |
| Only verification evidence changes | Advance signed snapshot sequence and update its evidence reference; do not create a package release |
| Existing package gains support in a newly proven adapter | Update signed release policy and evidence only; keep package source/digests and release sequence unchanged |
| Several releases exist in one distribution | New install/update chooses the highest eligible active sequence; never downgrade when newer candidates fail gates |
| A newer release is published | Keep older statuses unchanged unless the same reviewed change explicitly supersedes them |
| Published release source/digest is edited or removed | Reject publication; add a new release or terminal revocation instead |
| Upstream supports only some selected targets | Choose one eligible bridge for the complete set or fail before mutation; never mix sources per client |
| Directory is offline | Use unexpired cache; exact/local sources remain available |
| Directory snapshot is expired | Fail new short-name resolution closed |
| Weekly refresh publishes unchanged data | Increase snapshot sequence and expiry only; do not create new package releases |
| Weekly/evidence-only publish sees a newer repository HEAD | Reuse each unchanged release's previously signed source revision; never rebind it to HEAD |
| Publisher resumes after the previous snapshot expired | Verify and reuse the expired artifact only as the immutable sequence/provenance ledger, then issue a fresh higher-sequence snapshot |
| CDN/Pages deployment is lost or stale | Rebuild it from the protected publication branch; never infer sequence or release bindings from CDN state |
| Publication branch history/file is rewritten or removed | Block publishing and require trusted recovery; never silently start sequence or provenance again |
| Local clock is outside snapshot validity | Fail new short-name resolution with a clock-specific diagnostic; keep remove and exact/local operations available |
| Directory sequence decreases | Reject as rollback attack |
| Snapshot cache is deleted but installed state records a higher sequence | Use the installed provenance as the local floor and reject the lower snapshot |
| Snapshot and detached envelope bytes/digest differ | Reject before parsing the snapshot into resolution state |
| Latest pointer contains an absolute/cross-origin path | Reject; construct fetches only from validated relative artifacts on the configured origin |
| Signing key rotates | Release a CLI trusting the replacement key before retiring the old key |
| Published snapshot uses the next signing key | Accept it only in the bounded CLI trust-overlap window; later remove the retired key |
| Directory needs an incompatible snapshot schema | Publish a separate schema-scoped pointer; do not replace the pointer used by supported old CLIs |
| Downloaded package differs from selected signed release | Fail closed; do not attempt another distribution as an implicit fallback |
| Installed release is later revoked | Warn in `info`/`doctor`; block new target and repair/rematerialization, allow remove and update to an eligible non-revoked release |
| Existing distribution is suspended | Block new install, new target, and update; allow safe removal and exact repair of non-revoked bytes when their immutable source is available |
| Selected client is not installed | Fail that explicit target during preflight with no mutation |
| Selected client supports no valid component | Fail complete multi-target preflight |
| Project scope is requested before adapter support | Fail complete preflight; never fall back to user scope |
| Existing distribution cannot support a newly selected target | Keep source stickiness, fail preflight, and suggest switching the whole installation |
| Existing short name is added again after Directory default changes | Resolve local product identity first and add only unbound targets from the recorded distribution/release |
| Partial update release breaks an unselected binding | Reject the release before any mutation; partial rollout may delay convergence, not bypass compatibility |
| One component is invalid | Follow Agent Plugins isolation rules and report degraded status |
| One managed target commit fails | Roll back safely owned mutations in the operation group |
| External activation fails | Keep valid materialization, mark activation state, provide repair step |
| Repair runs while a newer Directory release exists | Reapply the recorded client revision only; require `update` for convergence |
| Repair source and clean managed copy are unavailable | Make no changes, preserve state, and report the exact immutable source; removal stays available |
| OAuth is cancelled | Keep package state, mark auth pending/cancelled, do not claim runtime success |
| ChatGPT app binding is absent or its ID differs | Disable that target before mutation and never infer ChatGPT installability from a portable package alone |
| Copilot CLI and VS Code share one backend | Collapse to one physical install and report both logical surfaces |
| Removal names only one shared Copilot surface | Show both affected surfaces and perform one ownership-safe physical removal |
| Update or normal removal touches plugin data | Preserve `PLUGIN_DATA`; delete only after explicit `--purge-data` and ownership validation |
| Product is re-added after normal removal retained data | Resolve the local `data_retained` record before a new Directory default, reattach the same distribution/data, and require explicit switch for another source |
| User purges a `data_retained` record | Validate every ownership receipt, delete only owned data, then remove the retained record; refuse unknown paths |
| One of several requested purge receipts is unknown/stale | Fail the complete purge preflight with zero mutation; never report partial data deletion as success |
| Switched distribution interprets existing plugin data differently | Warn before switch; never claim cross-distribution data compatibility or roll back external runtime writes |
| Stdio config overrides `PLUGIN_ROOT` or `PLUGIN_DATA` | Reject that server entry; the adapter owns both reserved values |
| Native client lacks placeholder behavior | Project an equivalent proven configuration or mark the target unsupported |
| Bare stdio executable is missing | Block managed activation before mutation and print the exact runtime requirement; never auto-install it |
| Native client resolves executables in an unknown environment | Mark runtime verification pending instead of claiming success |
| Process crashes mid-operation | Recover from journal and receipts idempotently |
| Two lifecycle mutations race, even for different plugins | Serialize through the existing global mutation lock and reject stale operation state; shared client configs must not be edited concurrently |
| Two release PRs claim the same distribution sequence | Later PR rebases and takes the next unused sequence; sequences are never reused |
| Two snapshot publications overlap | Serialize the trusted workflow; the second rebuilds from merged source and the latest valid sequence |
| In-repository package PR cannot know its merge SHA | Review path and digests; bind the actual post-merge SHA only in trusted publication |
| Post-merge package digest differs from reviewed digest | Publish no snapshot and require investigation/review |
| External package becomes unavailable between review and publication | Publish no snapshot; never sign an unverified stale source record |
| User edits managed client files | Detect digest drift; repair or remove only with ownership-safe plan |
| User/client edits a planned native object after preflight | Revalidate its observed digest before commit and abort without clobbering the newer value |
| Existing community install adds target after upstream promotion | Use existing community distribution for the new target |
| User wants upstream source | Use explicit `switch`, with full dry-run and rollback plan |
| User installs a second distribution of the same product | Propose a whole-installation `switch`; do not create an ambiguous side-by-side copy in the same scope |
| Two products declare the same native plugin name | Detect the target/backend collision before mutation and require a reviewed identity change |
| A same-name native plugin already exists outside agentplugins state | Treat it as foreign and fail before staging; never overwrite, adopt, update, or remove it automatically |
| A client supports namespace-qualified plugin identities | Permit coexistence only when the adapter proves the qualified identities cannot collide and records only its own object |
| Direct full-SHA source is updated upstream | Require a new full SHA through explicit `switch`; never follow a branch implicitly |
| Direct GitHub source uses a branch or tag | Reject it and require a full 40-character commit SHA |
| Direct local package changes | Re-snapshot only during explicit update and show the digest change |
| Direct installation is migrated to new state | Preserve `origin_mode: direct`, source binding, and digests; never invent a Directory distribution or sequence |
| Direct source collides with an installed Directory/native identity | Require explicit switch/rebind or a distinct identity; never silently adopt it |
| Explicit direct source digest matches a known revoked Directory release | Warn from already-valid local/embedded data, but preserve direct-source independence and never fetch policy solely to rewrite the user's exact request |
| Large upstream monorepo | Sparse-fetch declared paths; limit plugin root, not repository size |
| Required Git LFS content | Reject initially with exact diagnostic |
| Required submodule content | Reject initially with exact diagnostic |
| Symlink escapes package root | Reject |
| Internal symlink is valid but target platform cannot materialize it safely | Fail that target during preflight without weakening containment |
| Case-colliding paths | Reject for cross-platform determinism |
| Paths collide after Unicode normalization | Reject before hashing/materialization |
| Windows reserved filename/special file | Reject before materialization |
| Upstream introduces a new executable | Require explicit review and refreshed evidence |
| Exact external stdio dependency changes | Treat it as a runtime dependency change and invalidate runtime evidence even when plugin tree bytes are unchanged |
| Remote MCP URL changes | Require review and invalidate prior runtime/auth evidence |
| Agent Plugins publishes a new schema | Add an explicit versioned loader; never reinterpret the old schema |
| Agent Plugins schema is only a working draft | Report unsupported version; do not advertise or infer compatibility |
| `plugin.json` and `mcp.json` schema versions differ | Disable MCP, preserve valid skills, and report the mismatch |
| Old CLI cannot read the signed Directory | It remains on the byte-frozen legacy catalog during the documented transition; new entries require the new CLI |
| Old CLI reads new state | Refuse safely or use supported migration boundary; never corrupt state |
| Binary rollback is requested after new-schema mutations | Do not run the old writer; ship a fixed-forward binary that preserves the committed schema and state |
| State schema 2 exists when a mutation is requested | Keep read commands available and require explicit digest-checked `migrate-state`; never migrate as a hidden lifecycle side effect |
| State migration starts with an unresolved journal | Recover under the global lock or refuse migration without writing the new schema |

## 23. Test strategy

### 23.1 Unit tests

- Alias and qualified-name resolution.
- Upstream/bridge/community priority and eligibility.
- Declared-default selection and target-aware fallback without implicit
  promotion.
- Default eligibility with missing evidence versus explicit failed evidence.
- Inconclusive infrastructure/auth results versus reproducible blocking failure.
- Deterministic current-evidence pointer replacement, historical failure
  retention, and changed-tuple invalidation.
- Release ordering independent of SemVer.
- Highest-eligible release selection and no automatic downgrade.
- Published-release immutability and allowed status transitions.
- Package identity versus mutable signed compatibility-policy boundaries.
- Bridge recipe validation and deterministic path mapping.
- Package diagnostics and failure boundaries.
- Multi-target parsing and ordering.
- Complete-target-set source selection and source-mixing rejection.
- Partial-update rollout still preflights every installed binding.
- Shared physical backend collapsing.
- Product, manifest-name, and native-backend collision rejection.
- Managed-versus-unmanaged native identity observation and fail-closed handling.
- Ambiguous installed selector refusal.
- State migration and source stickiness.
- Conditional Directory versus direct-origin state invariants.
- Existing-install resolution before Directory default selection.
- Exact-revision repair versus explicit update convergence.
- Directory release-sequence transitions versus direct-source version policy.
- Persistent plugin-data retention and explicit purge.
- `data_retained` re-add before Directory default and ownership-safe
  later purge.
- Exact stdio placeholder, reserved-variable, and data-root rules.
- Bundled, available, missing, and client-managed/unknown stdio executable
  preflight.
- Signature, sequence, expiry, and cache rules.
- Embedded sequence floor and schema-scoped latest-pointer rules.
- Installed-provenance sequence floor after cache loss.
- Revocation behavior for install, new-target add, repair, update, and remove.
- Distribution suspension versus release revocation behavior.
- Signed-metadata/package-digest disagreement fails without fallback.
- Best-effort known-revocation warning for direct sources without making
  Directory availability authoritative.
- Idempotent merge polling and promotion identity.

### 23.2 Contract tests

- Agent Plugins 1.0 manifest and MCP schema fixtures.
- Published 1.0.0, unsupported working draft, mismatched component schema, and
  unknown-manifest-field fixtures.
- Agent Skills fixtures.
- Directory source and generated snapshot schemas.
- Policy-only compatibility changes that preserve immutable release identity.
- Signed current-evidence summaries and immutable evidence-reference fixtures.
- Bridge source-to-output reproducibility.
- Adapter capability contracts.
- Stable JSON CLI output.
- Output-schema version compatibility and breaking-change rejection fixtures.
- Local operational JSON path fields and separate redacted evidence-export
  schema; raw local JSON never becomes a publishable artifact.
- `npx universal-agent-plugins` and installed `agentplugins` argument/JSON
  contract parity.

### 23.3 Integration tests

- GitHub sparse source acquisition at exact SHA.
- Large repository with small package path.
- Managed target staging, commit, rollback, and repair.
- Pre-existing unmanaged native object causes zero mutation; a proven
  namespace-qualified distinct object remains untouched through add/remove.
- Add-target and repair against an existing installation after its Directory
  default changes.
- Shared Copilot CLI/VS Code backend planning and mutation.
- `PLUGIN_DATA` preservation across update, switch, removal, and migration.
- Last-binding removal, `data_retained` re-add, and later explicit data purge.
- Shared physical backends retain exactly one data receipt across binding
  removal/re-add.
- User-scope success and unsupported project-scope zero-mutation behavior.
- State locks and crash recovery.
- Explicit schema 2-to-next migration, backup, stale-input digest refusal, and
  pre-migration read-only commands.
- Migration preserves direct local/full-SHA identity without fake Directory
  metadata.
- Migration serialization with journal recovery and fixed-forward behavior after
  the first new-schema write.
- Website generation from Directory source.
- Weekly signed-snapshot refresh with unchanged products/releases.
- Evidence-only snapshot publication without a package release.
- Legacy catalog byte-freeze guard while Directory entries evolve.

### 23.4 E2E tests

- All 26 Directory packages complete add/info/remove in at least one isolated
  supported client.
- Five hero plugins complete add/update/remove, discovery, and runtime across
  Codex, Cursor, and Kiro with 15 required runtime results.
- Context7 completes the actual one-command three-target lifecycle, not three
  separately composed single-target runs.
- Notion supplies three separately recorded OAuth/runtime results using the
  approved isolated test identity and human consent.
- Cloudflare Docs completes one registered app binding, Plugins UI activation,
  and read-only ChatGPT runtime flow with human consent.
- Upstream promotion with immutable test repositories.
- External contributor journey through a fork PR.

### 23.5 CI separation

- Fast untrusted PR validation.
- Bridge reproduction without execution.
- Isolated materialization tests.
- Scheduled upstream observation.
- Privileged signed snapshot publication.
- Independent privileged CLI binary release.

## 24. Observability

The CLI exposes human and structured JSON results containing:

- operation ID;
- product, distribution, release, source, revision, and digests;
- Directory snapshot sequence and digest;
- selected clients;
- planned and completed actions;
- component diagnostics;
- materialization, activation, authentication, and verification state;
- rollback/repair instructions.

Structured output uses stable logical IDs and may include typed local paths when
required for the next action. The evidence exporter strips those paths and
redacts diagnostics before anything becomes publishable; local operational JSON
and public evidence are distinct schemas rather than one compromised format.

Directory workflows publish concise machine-readable reports for bridge drift,
promotion decisions, suspended sources, and failed verification. The MVP adds
no usage telemetry.

## 25. Rollout and kill switches

1. Ship loader hardening behind tests without changing short-name defaults.
2. Ship multi-target behavior using existing community sources.
3. Generate the new Directory in shadow mode and compare every current alias.
4. Enable Directory resolution with an embedded bootstrap snapshot.
5. Publish the first bridges and upstream PRs.
6. Enable signed remote snapshot updates.
7. Complete Phase 6 evidence and ship the first stable release.
8. Post-launch, run promotion checks on demand, then enable scheduled shadow and
   manual-merge modes after one real tracked upstream PR.
9. Extract the CLI only after at least two stable releases prove public
   behavior and state compatibility.

Kill switches:

- signed distribution suspension and release revocation;
- stop publishing a generated bridge while retaining historical release data;
- disable promotion automation without disabling manual Directory updates;
- conditional npm wrapper rollback before any incompatible state write;
- state backup and safe migration refusal;
- exact/local source operation independent of Directory availability.

A Directory rollback is always a new reviewed snapshot with a higher sequence
that restores the previous eligible default or status. The publisher never
re-serves an older signed snapshot as `latest`.

The npm binary rollback kill switch is safe only before an incompatible state
schema has been committed. After migration, release recovery is fixed-forward
with the same state schema; an older binary may diagnose/refuse the file but may
not overwrite or downgrade it.

## 26. Acceptance criteria

### 26.1 First stable release

The first stable public release is ready when all of the following are true:

1. A user can run one documented command with an explicit comma-separated target
   list and install one package into at least three supported clients.
2. During `add`, the package is fetched and validated once and every client
   binding records the same immutable package release.
3. Short names use the reviewed declared default; a compatible verified bridge
   works without waiting for upstream merge, and an upstream candidate cannot
   become default without a reviewed promotion.
4. A bridge rebuild from pinned upstream SHA and overlay is byte-for-byte
   reproducible.
5. Website and CLI show one product, its default origin, alternatives, and exact
   per-client evidence without duplicate cards.
6. Existing installations never switch distribution during `update`.
7. `switch` previews, applies, verifies, and can safely recover controlled
   changes.
8. Tampered or expired Directory snapshots cannot influence short-name installs,
   and sequence rollback below the binary's embedded floor or the client's
   highest accepted sequence from cache or installed provenance is rejected.
9. Invalid selected targets or unmanaged/indeterminate native identity
   collisions cause zero mutation; independent invalid components are reported
   according to specification failure boundaries.
10. Explicit digest-checked state migration preserves existing client artifacts
    and provenance, takes a backup, and never runs as a hidden lifecycle side
    effect.
11. CLI binary publishing is isolated from untrusted package submissions.
12. Direct local and immutable GitHub packages continue to work without
    Directory submission.
13. Every public runtime or OAuth claim links to exact, immutable evidence.
14. All 26 Directory packages complete add/info/remove in at least one isolated
    supported client.
15. Agent Code Navigator, Context7, Cloudflare Docs, Chrome DevTools, and Notion
    complete add/update/remove, discovery, and runtime E2E in Codex, Cursor, and
    Kiro, producing 15 required client/plugin results; the three Notion results
    use the approved isolated identity and separate honest OAuth evidence.
16. Cloudflare Docs completes the separately scoped registered-binding,
    Plugins UI activation, and read-only ChatGPT runtime flow; no broader
    ChatGPT package claim is inferred from it.
17. Every in-repository package README contains a copy-ready install block
    checked against Directory and CLI syntax; external publishers are not
    required to modify their README.
18. Release CI is green and all E2E work occurs only in disposable test projects.
19. Copilot CLI and VS Code share one physical mutation when they resolve to the
    same backend.
20. Update, repair, switch, and normal removal preserve `PLUGIN_DATA`; only an
    explicit ownership-checked purge deletes it. Re-add after normal removal
    finds the local `data_retained` record before a changed Directory default.
21. Published Agent Plugins 1.0.0 is supported exactly; a working draft or
    unknown schema is rejected until an explicit loader and evidence exist.
22. The first stable release advertises user scope only, and any unsupported
    project-scope request fails before mutation.
23. Directory updates use signed release sequence rather than assuming SemVer,
    while direct full-SHA sources never follow a mutable branch.
24. The public CLI has no hidden `--yes`; explicit commands/targets are consent,
    manual activation remains visible, and destructive data purge remains
    explicit.
25. Missing managed stdio runtimes fail before mutation with exact guidance;
    the CLI never installs Node, Python, Docker, or system packages implicitly.
26. Re-adding an installed short name and `repair` both retain the recorded
    distribution/revision; revoked bytes cannot be newly exposed or repaired,
    while remove and a safe update remain available.

### 26.2 Post-launch completion

The decided operating model is complete when:

1. An on-demand exact-match upstream check creates a reviewable promotion PR;
   changed package content always requires human review.
2. Scheduled observation is proven on one real tracked PR, and exact-match
   auto-merge remains disabled until three successful manual promotions.
3. After at least two stable releases, the CLI engine is extracted to
   `agentplugins-cli` without changing npm commands, JSON contracts, state, or
   binary checksum verification.

## 27. Estimate

Chosen architecture:

```text
Confidence: 10/10
Reliability: 10/10
Complexity: 8/10
```

Expected effort:

- approximately 8,000-12,000 new or materially changed lines, excluding
  generated bridge payloads;
- approximately 10,000-12,000 moved lines during CLI extraction;
- first useful vertical slice (loader hardening, multi-target, Directory model,
  and first bridges): roughly 7-12 focused engineering days;
- first stable release through Phase 6: roughly 2-3 focused engineering weeks;
- post-launch promotion automation and repository extraction: roughly one
  additional focused week after real usage proves the contracts.

The stable estimate assumes the isolated Notion identity and named ChatGPT
development binding are available at Phase 0. Human consent or vendor access
can extend calendar time but does not justify weakening or silently skipping
those evidence gates. Upstream PR merge time is not on the launch critical path
because reviewed bridges provide the pre-merge distribution.

Phase estimates overlap because tests, schemas, and migrations serve several
phases. They should not be summed as independent rewrites.

## 28. Implementation order summary

```text
Contract fixtures
  -> loader/source hardening
  -> multi-target lifecycle and state
  -> Directory domain and website migration
  -> build-time bridge pipeline and first packages
  -> signed Directory consumption
  -> launch E2E and stable release
  -> on-demand then scheduled upstream promotion
  -> CLI repository extraction
```

The bridge and upstream PR work can begin in parallel once the hardened package
loader and reproducibility fixtures exist. Upstream automation and repository
extraction do not block the first stable release.
