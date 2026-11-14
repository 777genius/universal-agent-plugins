# Archive implementation independent review R1

Reviewed 2026-10-23 by project_core, read-only, archive tree `6b8baf2f4849a19c33a1be24ac6a56b2c08aec33` (cumulative `fc6c4aee`, `2b267cc4`, `6b8baf2f`). Scope: new `adapters/sourceacquisition/archive*.go` and their reviewed source/redirect/dot-prefix contracts. No archive worktree edits.

## Decision

**Changes required: one P2 contract gap.** No containment, source-identity, redirect credential, or framing blocker found. Re-review the small fix below before R1 GO. Independent source lead R2 is still required at integration.

## R1-1 — P2: enforce a recognized initial filename suffix against stream format

`archive_download.go:archiveDownload` recognizes byte magic and sets explicit archive semantics but never compares the detected format with a recognized source suffix. `archiveCoordinatesFor`/`archiveExplicit` only use suffixes as a boolean. Consequently a ZIP payload fetched from `package.tar`, or an uncompressed TAR fetched from `package.tgz`, is accepted. The reviewed source plan explicitly requires detecting incompatible format/suffix (sources-plan.md archive policy).

Add a small pure check after positive magic recognition against the **original normalized source endpoint path** (or explicit local filename): `.zip` -> ZIP; `.tar` -> TAR; `.tar.gz`/`.tgz` -> gzip TAR. Extensionless input remains magic-classified. A redirected CDN filename or Content-Type must remain nonauthoritative; keep all errors after positive recognition as archive errors, never `ErrNotArchive`. Test the mismatches, matching formats, and extensionless inputs without a second download. Existing byte-pin and framing gates remain in place.

## Reviewed invariants

- The one retained original stream is SHA256 hashed and compared with requested/locked byte pins before extraction. Tree pins are checked after selected-package resealing; mismatches clean the caller-owned sealed snapshot.
- Private extraction uses rooted exclusive writes; links/special files are never created. NFC portable lexical names, implicit ancestor collision tracking, case/Unicode folds, entry/depth/file/expanded limits and exactly one conventional `./` prefix preserve containment across ZIP and effective TAR/PAX names.
- Remote initial URLs remain query-free public HTTPS. Redirects remain bounded HTTPS, strip headers, do not inherit a cookie jar or redirect exception, keep signed query encoding transient, and expose sanitized errors. Original source coordinates alone populate source identity.
- ZIP audits ordinary single-disk central/local records, directory bounds, contiguous declared regions and exact end/comment boundaries. TAR preflight requires two zero end blocks and bounded aligned zero padding, rejects special/global/sparse entries and audits metadata counts/sizes. Gzip accepts exactly one CRC/ISIZE-validated member with no outer tail.
- Wrapper removal is transient and logical package paths are stable; malformed root manifests do not silently select nested packages. Candidate discovery is bounded and explicit selection remains available.
- Context/timeout applies through HTTP, retained reads, unpacking and resealing; failure cleanup owns the private workspace. Package code is not executed and the existing standard loader remains schema authority.

## Verification

Independent full `go test -p=1 ./install/integrationctl/agentplugins/adapters/sourceacquisition -count=1 -timeout=2m` passed in **2.005s**, Go 1.26.10, GOMAXPROCS=3, using the shared module/build caches. The first sandboxed attempt could not bind its loopback TLS listener; the same test invocation with authorized loopback execution passed. This includes real TLS redirects, ZIP/TAR/TGZ acquisition, byte/tree pins, hostile paths/types/collisions, strict archive endings, cancellation, real >256 MiB download/gzip bounds and cleanup. These are transport fixtures; no real-client certification claim is made.

## Fix verification — R1 GO

Reviewed fix `b99a15a497b1d0fb916a62965c0a33568ab9988f`: the recognized initial local/HTTPS suffix now matches detected magic before the retained stream is written. All ZIP/TAR/gzip mismatch combinations are refused without `ErrNotArchive`; `.tgz` and `.tar.gz` remain equivalent, extensionless classification is unchanged, and a signed redirect's `.tar` filename remains nonauthoritative for an initial `.zip` source. Independent focused real TLS suffix/redirect tests passed **0.139s**. R1-1 is closed; **R1 GO**, pending independent R2 at integration.
