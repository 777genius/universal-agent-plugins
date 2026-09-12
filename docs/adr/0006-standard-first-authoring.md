# ADR 0006: Shared Standard-First Authoring

## Status

Accepted for bounded implementation, 2026-09-06, under the approved
[implementation plan](../STANDARD_FIRST_AUTHORING_ENGINE_IMPLEMENTATION_PLAN.md).
This decision extends [ADR 0005](./0005-standard-first-agent-plugins-installer.md);
it does not replace or relax installer invariants. Acceptance is architectural,
not a claim that the standard-first CLI is released.

## Context

The current `plugin-kit-ai` v1 authors `plugin/plugin.yaml`. Agent Plugins 1.0
uses root `plugin.json`, optional root `mcp.json`, and immediate
`skills/<name>/SKILL.md`. Reusing the legacy application behind a renamed
command would preserve the wrong authority and conceal format conversion.
Cobra commands also retain parent pointers, flag values, and streams, making
package globals unsuitable for two independent entrypoints.

## Owner clarification: capability preservation

Accepted 2026-09-06: preserve useful YAML-specific implementation and its tests
while excluding it from the standard dependency graph. Command retirement does
not authorize code deletion. Unresolved capabilities remain preserved; each
removal requires an inventoried decision explicitly accepted by the owner.
See the [preservation contract](../AUTHORING_CAPABILITY_PRESERVATION.md).
This does not authorize a second supported authoring engine.

## Decision

1. Root `plugin.json` is the sole portable source of identity and metadata.
   `mcp.json` and `skills/` are the portable component sources. No required
   sidecar, generated client projection, build file, or publication metadata
   may supplement, override, or repair portable fields.
2. Normal authoring accepts exactly `FormatIDAgentPluginsV1`. The OpenAI
   compatibility layout `.codex-plugin/plugin.json` is a separate explicit
   input format. It never supplies missing standard files. Legacy input is
   accepted only by a later explicit read-only importer at the canonical
   `<project>/plugin/plugin.yaml` path, writing to a new destination.
3. `plugin-kit-ai` and `agentplugins author` construct fresh commands from
   the same Go factories and call the same services in process. Neither invokes
   the other binary. Each command injects its own typed request/result runner;
   there is no application-wide backend interface or legacy manifest adapter.
4. Reuse bounded decoding and the lossless `PackageEnvelope` beside the standard
   domain. Keep mutable authoring roots, snapshot ownership, CAS plans, and
   archive selection outside the installer domain. Preserve unknown values and
   supported opaque extensions; never serialize the envelope into legacy types.
5. Separate normative conformance, host safety, installer policy, authoring
   hygiene, release/publication policy, target compatibility, and runtime
   evidence. A skipped invalid MCP document/server or Skill does not erase
   valid components and does not constitute full authoring conformance.
6. Embed exact schema and Agent Skills validation profile identities. Validation
   never fetches rules, launches an agent/model, reads client credentials, or
   executes package content. Runtime/network testing belongs to explicit later
   phases with disposable roots and bounded process cleanup.
7. Existing installer flags stay at the root. The adapter copies only supported
   `--format`, `--no-color`, `--dry-run`, and `--target` into a fresh value.
   Explicit `--scope`, `--accept-security-risk`, and `--security-details`, even
   explicit false/default values, fail before authoring decoding or effects.
   Unsupported explicit shared flags also fail. Help explains the positional
   package path, authoring doctor, and separate installer policy commands.
8. Equivalent commands preserve exit errors, JSON data/envelopes, diagnostics,
   and filesystem effects. Only product branding, invocation prefix, and product
   version may differ; successful future authoring payloads carry the same
   engine revision. Renderers own existing output contracts, and JSON mode
   emits one document without prompts. No new exit-code mapping is introduced.
9. Keep Go module identities `github.com/777genius/plugin-kit-ai/...`. A future
   rename requires a separate compatibility ADR and consumer inventory.

Dependency directions are enforced by the foundation import-boundary test:

```text
authoringcli -> command-specific ports and renderers
authoring services -> standard facts + author policy + low-level infrastructure
installer loader/policy -> standard facts (never authoring)
standard domain/conformance -> no Cobra, mutation, clients, or publication
legacy importer -> standard mutation plan (migration only)
standard authoring -X-> pluginmanifest/pluginmodel/app/publicationmodel/legacyimport
```

The canonical domain/conformance guard scans production sources for every build
constraint and follows transitive local imports. Standard-library data packages
and schema registry ports remain valid dependencies; Cobra, process/HTTP access,
CLI, client/provider, planning/state mutation, and publication dependencies do
not. This is a dependency check, not a function-level purity proof.

The [command and policy inventory](./0006-authoring-inventory.md) records
conversion decisions and the failure classification handoff. Host restrictions
must never be reported as normative schema failures. In particular, Windows
leaf IDs, archive symlink restrictions, and publication SemVer requirements are
stricter policy; a non-SemVer string is not itself an Agent Plugins 1.0 violation.
Unknown fields and non-object extension containers may load with diagnostics
while still failing schema conformance. Unknown namespaces stay opaque.

## Consequences

The bounded foundation adds factories, an explicit flag adapter, tests, this
ADR, and historical markers. It deliberately leaves v1 command wiring and
installer behavior intact, without moving hundreds of legacy files. The
existing main is already a small execution composition point; individual legacy
constructors remain until each replacement has a complete standard service.
New factory roots are internal seams, not public success-shaped placeholders.
No released main imports them yet. The author subtree is hidden when explicitly
mounted by internal integration code.

The first public PR must integrate a working `init -> validate -> inspect ->
test` E2E slice. Foundation-only changes may be committed for orchestration but
must not become that first public PR. The complete MVP/release gates still
require generated Skill and MCP packages, both entrypoints, isolated installer
planner checks, unchanged installer tests, and matching engine provenance.
Runtime, migration, preview, publication, and retirement retain their later
phase gates. Historical artifacts and v1 binaries remain immutable.

Factories allocate fresh Cobra trees and flag sets for **every invocation**.
Returned mutable roots/subtrees are single-invocation, including successful
execution and help. Composition executes the enclosing root through
`authoringcli.Factory.Execute`, which constructs a tree per call and rejects
previously consumed commands. There is no flag reset or reusable-tree contract:
parse, required/group validation, argument, pre-run, runner/render failures,
help, and cancellation all consume the tree. Installer-owned options are never
reset by authoring. Factories must allocate flag bindings and mutable captures
inside each construction; callers must not cache commands or share these values.
Runner, decoder, and renderer implementations must honor cancellation and
stream/JSON contracts. Independent invocations may execute concurrently with
concurrency-safe factories and injected runners.

## Non-Goals

This foundation neither implements later services nor changes installer flag
placement, lifecycle, state, trust, cache, rollback, or providers. It neither
publishes a v2 binary nor deletes legacy code. It does not register deferred
commands, implement a second manifest, or establish an authoring runtime SDK.

## Rejected Alternatives

- Mechanical movement of hundreds of legacy files: obscures domain coupling
  and expands the review without a standard-first capability.
- A single reusable `*cobra.Command`: leaks parent pointers and mutable flags.
- Shelling out to v1 or generating temporary YAML: preserves the legacy engine
  and creates version, quoting, cancellation, and error drift.
- Relaxing the installer loader to get green authoring results: confuses
  conformance with security and weakens an established safety boundary.
- Replacing the current public root before the vertical slice: breaks working
  v1 behavior and advertises unfinished commands.

## Darwin acquisition clarification (2026-09-07, PR 1)

Accepted delegated technical decision: bounded quiescent writable APFS, an
explicit reduction of the hostile concurrent writer guarantee, **not equivalent
security**. This supersedes read-only-only authoring admission, not installer
acquisition. Implementation stays unmerged until actual native review/proof;
Linux cross-compilation cannot qualify Darwin. PR 2 owns public composition,
release producers, workflows, help and current user-guide activation.

Writable macOS local authoring uses `packageview-local-darwin-v2` on supported
local APFS. During each Reader.Open-through-Lease.Close acquisition interval,
the source tree and ancestor bindings establishing its selected location and
containment must remain quiescent, including the gap between core decoding and
component capture. Static package content remains untrusted. The reader retains
metadata-first type checks, legacy identity/alias exclusion, contained resolution,
bounded private capture, offline operation and observed-change failure. It does
not protect acquisition from an active concurrent source or ancestor writer.
Violation can cause a forbidden open or an out-of-scope/excluded read before an
error; repeated checks do not equal Linux's inode-bound acquisition or prove an
atomic source revision. Mutation-plan rechecks, public stage validation, installer
invariants and full native macOS release gates remain required.

Quiescence begins before the first source-path resolution/metadata operation
and ends only after source access and cleanup through Close (or failed Open
cleanup). It includes writes, truncation, rename/replacement, link changes,
permissions/types and directory membership, including excluded legacy metadata.
Root-name, relative CWD, allowed ancestor symlink targets and descendant bindings
must remain stable and attached. Unrelated siblings outside the package may
change; ancestor comparisons therefore ignore size/mtime/ctime/link-count changes
caused by siblings, while checking identity, type, access mode, owner, flags and
generation. Trusted kernel/mount administration and private storage remain
assumptions. A private 0700 directory does not exclude another same-account process.

Use public fstatat(AT_SYMLINK_NOFOLLOW), bounded readlinkat, single-component
openat with O_DIRECTORY/O_NOFOLLOW for directories, and metadata-approved
regular opens with O_NOFOLLOW/O_NONBLOCK/O_NOCTTY. The held pin owns the parent,
not the final inode. Directory prefixes are replayed with finite metadata
records instead of retaining a handle per entry. Checks detect observed identity,
metadata, entry-set, link, byte and legacy changes as fatal `source_changed`,
discarding partial Input/digest. No retry-until-stable loop is added. ABA,
same-tick changes and substitutions after checks can evade observation; a FIFO
or device open, detached-tree read or legacy read cannot be undone by fstat or
zero subsequent Read calls. The writable profile relinquishes precisely this
hostile-concurrency protection. Read-only APFS/EROFS tests remain separate,
stronger evidence for their filesystem precondition.

Each synchronous Open/Capture/Close phase runs on a short-lived locked OS
thread. A small Darwin arm64 cgo binding to system libSystem saves the documented
getiopolicy_np/setiopolicy_np thread materialization policy, sets
IOPOL_MATERIALIZE_DATALESS_FILES_OFF, reads back, runs the phase, then restores
and verifies the saved policy. Restore failure invalidates success and retires
the still-locked thread; panic/error cleanup precedes delivery. Cancellation
waits for terminal cleanup. No process-wide policy change, helper, dependency,
source hardlink, source lockfile, snapshot, watch or second engine is added.
Darwin without cgo rejects before source acquisition as `platform_unavailable`.
Observed SF_DATALESS entries reject before payload open. Supported resolution
paths are already mounted ordinary local APFS, not autofs triggers, network or
third-party redirectors; trusted mounts must stay stable. APFS alone does not
prove residency and OFF does not suppress unrelated OS background traffic.

Existing limits remain 10,000 entries, depth 64, file 64 MiB, total 256 MiB,
plugin 1 MiB, MCP 4 MiB, Skill 1 MiB and documents 16 MiB; paths/links 4,096 bytes,
40 expansions and 8,192 resolver steps. Metadata records are separately bounded
by entries plus resolver work. Reads retain size+1, <=32 KiB chunks and finite
byte verification, private 0600 numeric files sealed to 0400, and exact-child
cleanup. Source writes are never performed. Legacy initial/current identities
and conservative multiple-hardlink rejection remain; independently copied bytes
are not identified by semantic resemblance to YAML.

Init must finish staging before real public validation. Skills must close its
initial and core-recheck leases before staging/publication and acquire fresh
post-commit evidence. Mutation root/parent/stage/payload checks, affected-input
digests, exclusive publication and committed-state reporting remain mandatory.
No CLI flag, environment opt-in or advisory ritual enforces quiescence.

Native proof requires disposable writable local APFS on macOS arm64, unprivileged
reader execution, exact revision/binary hashes, Go 1.25.13, Apple SDK/clang and
deployment target, OS floor/build, FSID/device/mount flags, native open observations,
real thread policy restoration and genuine disposable dataless rejection without
download. No real cloud/profile/device/provisioning experiment is authorized.
Missing device/dataless prerequisites remain required-unproven. Both entrypoints,
five template journeys, installer dry-run and same-revision native cgo release
assets/wrappers remain PR 2 and release gates. Keep `macos_writable=unproven`
until native evidence passes; no CGO=0 replacement asset qualifies.
