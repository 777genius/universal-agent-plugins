# Independent sources plan review — round 1

Reviewer: project-core lane. Reviewed `sources-plan.md` against implementation
base `3ea24e3014ddf6679ca86729c56f1069112b0d3a`, including current source
acquisition/ref resolution, content store, dependency staging/candidate loading,
source-binding identity, and graph pin/edge validation. No implementation.

Verdict: **changes required before implementation**. The one engine/store,
typed additive source variants, native credential isolation and checkout
separation, signed Directory boundary, archive limits, and native transport E2E
direction are accepted. The remaining issues are narrow contracts, not grounds
for another architecture.

## P1 — shared tree cache must not lend another source its provenance

The existing immutable store is keyed only by tree digest. `Store.Put` returns
an existing entry without changing/comparing source metadata; `Metadata.Source`
describes whichever source populated that digest first. `deps_candidates.go`
currently loads that metadata as the package Source. With new Git/archive
variants, the exact same tree may come from two endpoints or two byte-distinct
archives. Trusting cached source provenance can manufacture the wrong binding
ID, requested source, archive checksum or installed repair coordinates even
though package bytes correctly match.

The plan mentions one identity-from-lock helper; make its use obligatory on
**every** cache hit/load/replay path, including existing store candidate loading,
not just missing-pin downloads. Treat stored source metadata as historical
provenance only. Reconstruct current immutable source identity from the validated
selected locked package; keep tree/executable verification against the content
store. Do not add per-source copies of identical trees or rewrite metadata to
the latest caller.

Required regression: install the same tree from two distinct generic Git or
archive sources, in both cache population orders; the store remains shared,
and each lock/installation/binding/repair operation retains its own endpoint,
subpath, commit or byte checksum. Also use two different archive byte streams
with the same extracted tree and verify their separate locked byte pins.

## P1 — archive wrapper names must not become moving source identity

The archive selection policy says to strip one outer wrapper and return the
selected package path in snapshot metadata, while ownership uses URL + package
path and locks persist Path. A normal provider archive wrapper often changes
with the commit/version. If a discovered physical wrapper name becomes Path,
updating the same URL/package appears to switch source, or locked restore looks
for a path in the wrong logical archive root.

Define logical package path independently of extraction wrappers and temporary
directories. Specify precedence and meaning for an explicit Path versus a
detected wrapper: normalize one deterministic wrapper root, then select a
logical package path relative to it, or preserve an explicit documented raw-root
path mode. The chosen model must keep identical package identity across
`repo-old/` -> `repo-new/` archive updates. Do not persist extracted absolute paths
or a version-derived wrapper as canonical package identity.

Required regression: moving archive URL changes wrapper name and package bytes;
normal update retains source/binding ownership, exact locked replay still checks
the original archive checksum/tree, and an explicit logical nested package path
selects the same package under either wrapper. Malformed logical root manifests
must still block nested fallback.

## P2 — pure provider parsing needs an explicit deferred tree-selector shape

`sourcespec.Parse` is pure, but browser tree URLs with slash-containing refs need
one native remote-ref enumeration to identify selector/path boundaries. The
proposed `Git{URL,Path,Ref,Commit}` has only resolved coordinates and the direct
interface `ResolveGit(...Git, selector, exactRef)` returns a commit/ref without a
resolved package Path. There is no agreed carrier for an ambiguous raw suffix or
the resulting package path.

Keep the existing pure package. Add the smallest transient representation for
unresolved browser suffix and provider grammar, plus one network-layer helper
that returns a fully resolved Git spec and immutable revision. Alternatively
resolve explicitly in the CLI before constructing final `sourcespec.Git`.
Declare that boundary and its output shape in the plan/API so neither the pure
parser nor archive worker guesses the first URL segment as the ref. Only final
resolved coordinates enter manifest/lock/source identity. Tests must cover one
unambiguous slash branch and two valid ref/path decompositions rejected with the
clone-URL hint.

## P2 — finalize source-key encoding and cross-lane integration seam

The plan leaves source-key encoding as structured escaping **or** JSON hash.
Freeze one before S1/S4 and archive worker implementation. A compact canonical
tuple encoded with an unambiguous delimiter/escaping rule or a versioned hash
is sufficient; preserve old GitHub keys exactly. Use identical canonicalization
in Requirement.SourceKey, locked validation, SourceIdentity/SourceBinding and
snapshot-cache keys. Test delimiter-containing valid URL/path inputs and generic
path case, including old GitHub canonical keys.

Coordinate `App` and environment model/coordinator edits with project core:
source helpers must consume the selected App/environment context, preserve
explicit project scope in Directory policy requests, and never rebuild a user
App during replay. Sources remain stacked above project core. New source facts
must also survive copies of a project binding through repair/remove/recovery;
one source journey in a qualified project fixture is enough beyond the full
transport matrix's representative user fixture.

## Accepted boundaries and verification

* Preserve existing anonymous signed GitHub methods and old fake interface;
  additive direct capability is appropriately bounded.
* Fetch with native credentials, then materialize with isolated config and
  complete blobs. No ambient clone/checkouts or legacy engine dependency.
* Query token rejection and no stderr/environment logging keep source intent,
  locks and diagnostics free of authentication material.
* Complete-package archive extraction, no-follow exclusive writes, streamed
  expansion/entry limits, executable inventory and byte/tree digest checks fit
  existing standard snapshots. No plugin execution is needed for validation.
* Real HTTPS smart Git/private helper/SSH and HTTPS archive fixtures exercise
  transports beyond fake-runner command construction. Reuse one full client
  lifecycle, then one project-context seam scenario instead of an unnecessary
  transport-by-client cross product.
* Old locks, selectors, manifest digest and canonical source IDs stay exact;
  additive formats fail closed in old readers. Preserve signed Directory
  eligibility rather than using new transport support as endorsement.

No style-only requests. Parent still owns second independent review and the
implementation authorization gate after fixes.

## Fix verification — 2026-10-23

Verified the revised sources plan contains mandatory identity reconstruction
from the validated selected lock on all store/candidate/replay paths;
wrapper-independent logical archive roots and package paths; the transient
BrowserTree plus final resolved Git coordinates; one versioned tuple-hash key
encoding; and selected-App/project lifecycle integration. All R1 findings are
closed. **R1 GO**, with parent authorization and independent R2 still required
before implementation. Preserve the requested changing-wrapper update and
explicit nested logical-path regression during implementation verification.
