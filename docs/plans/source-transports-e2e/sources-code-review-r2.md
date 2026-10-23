# Source transports cumulative implementation review R2

Independent reviewer: project_core, 2026-10-23. Frozen starting tree `501d7f5610cd584d81a801059a817f7cb5f61657`, cumulative source changes after `1a9b25de`. Read-only source worktree review. Archive implementation was separately reviewed twice, including redirect/dot-prefix amendments and subsequent cancellation/suffix corrections; this review concentrates on native transport corrections and S5 lifecycle/cache integration.

## Decision

**Changes required: R2-1 P1 and R2-2 P2 below.** The source lead confirmed both and is implementing narrow fixes and regressions. The architecture, typed source/lock model, native credential boundary and cache-provenance direction are approved. Final GO requires independent fix verification on a frozen follow-up tree and the focused lifecycle tests.

## R2-1 — P1: imperative generic Git/archive repair uses a source hash as a URL

`cli/internal/agentpluginscli/lifecycle.go:repairSource` retains its legacy empty-Repository fallback of `CanonicalSource + "#" + revision`. New typed sources deliberately use `git-url-v1:<hash>` / `archive-url-v1:<hash>` as canonical stable identity; these are not acquisition endpoints. Consequently installations created with new generic Git or archive `--no-save` cannot repair through either the grouped or legacy single-target CLI path. Saved dependency repair takes another typed route, so the initial saved-only S5 journey test does not expose the failure. Explicit local archives have the same reconstruction issue and cannot safely invent a new portable path dependency or drop the original-byte pin.

Fix both imperative repair routes with recorded typed coordinates: stored endpoint and logical package path plus the selected **binding's exact installed revision**, archive original-byte SHA256, tree digest and manifest digest. Preserve the existing standard loader, security/confirmation gates and source identity checks. Do not resolve today's default ref/download or infer a source from a canonical identity hash. Local archive coordinates remain explicit local source semantics.

Required regressions: new `--no-save` generic Git, remote archive and explicit local archive installs; advance/change their source, then repair the installed pin (or refuse changed unavailable archive bytes before writes). Also strengthen the saved fixture: its initial repair runs immediately after update while the remote still equals the installed pin; advance the remote again before repair, including cold changed-archive byte-pin refusal with unchanged state/intent/lock.

## R2-2 — P2: new command-local snapshot caching detaches cancellation

`deps_resolver.go:dependencyResolver.finishLoaded` newly copies potentially large acquired packages into the command-local snapshot cache using `context.Background()`. Its existing content-store Put also uses Background. This drops the operation's cancellation/deadline during the new resealing/caching boundary. It may finish another bounded but expensive copy after cancellation, contrary to the reviewed command cancellation contract.

Thread the existing operation context through `finishLoaded` and its production callers, including Put; retain cleanup of both the original loaded package and any failed/cancelled retained snapshot. Add a cancellation regression proving no retained command snapshot or durable store/intent publication after cancellation. No new cancellation mechanism is needed.

## Provisional observation closed

The reviewer initially questioned encoded repository separator validation based on an earlier archive worktree's `normalizedURL`. The current source tree preserves RawPath. An independent public-parser probe confirmed both `git+https://github.com/acme%2Fdemo.git` and plain HTTPS forms reject the input. No grammar change is requested for this closed observation.

## Component assessment

| Boundary | Assessment |
| --- | --- |
| Pure source grammar and portable model | GO. Explicit Git/archive variants, one-of source counting, archive selector/pin identity, provider browser/ref handling and stable tuple keys are explicit; historical GitHub/Directory key forms remain separate. |
| Native Git policy corrections | GO by code review. Direct native commands retain trusted user authentication and CA/agent settings; command-local policy restores TLS verification including URL-specific overrides and only permits HTTPS/SSH. File transport is confined to the explicit test mapping. Read/checkout phases use isolated Git configuration; fetched repository content cannot activate user filters/hooks. |
| Signed Directory/Discovery | GO. Signed immutable GitHub acquisition remains on the anonymous runner and new direct inputs do not become signed Directory selectors. |
| Shared store and command cache identity | GO direction. Candidate description, materialization and pinned replay rebuild SourceIdentity from the selected pin, never first-writer metadata. Command cache keys include stable source, immutable revision (archive byte checksum), tree and algorithm; snapshot consumers reseal bytes. Cancellation correction required above. |
| Backward GitHub ownership | GO. Legacy source reconstruction preserves historical Repository/source-binding hint semantics; direct transport provenance is replaced by the selected legacy lock before admission. |
| Saved generic source lifecycle | GO direction. Source kind/URL/archive SHA propagate through snapshots, acquired requests, state source binding and inspection; installed repair reconstructs binding revision. Additional old-pin drift fixture required above. |
| Imperative generic source lifecycle | Changes required under R2-1. |
| Confirmation and private preview | GO direction. New saved resolution retains acquired bytes in command-local snapshots, keeps declined/dry-run store/intent unpublished and preserves existing security evaluation. Operation context must remain attached. |
| Project integration seam | GO direction. New helpers operate on their received App; merge must preserve project-selected App, scope/root in every new request and selected environment IDs. Actual generic Git/archive project journey follows qualified adapter integration. |

## Verification ledger

- Read native transport policy, pure parser/provider handling, additive source/lock/coordinator variants, lock-derived provenance, all new S5 dispatch/cache/acquired-request helpers and their lifecycle projections.
- Independently inspected the archive suffix fix and ran focused real TLS suffix/redirect tests during archive R1 (0.139s); full acquisition suite passed earlier (2.005s). No redundant large archive expansion run in this cumulative review.
- Independent pure parser probe on the frozen source tree rejects the two encoded-separator inputs above.
- Await frozen fix commit and focused native/direct lifecycle/provenance/cancellation verification before final R2 GO. Agent process behavior in these source transport journeys is fixture-backed and does not certify native agent products.

## Corrective implementation evidence (author; independent verification pending)

The follow-up implementation adds a recorded typed-source repair loader shared by
grouped and legacy single-target imperative repair. It uses each selected binding's
immutable revision, tree and manifest, and archive original-byte SHA256, together
with the persisted endpoint/logical path. Local archive transport stays local.
A changed unavailable archive refuses before target/state effects; restoring its
exact original stream permits repair. All finishLoaded production callers now
pass the operation context through retained resealing and store Put.

The focused completion suite passed in13.847s: actual HTTPS/private Git and
native SSH/SCP CLI saved lifecycle; imperative Git/remote archive/local archive
repair after source advance; saved old-pin drift repair and cold changed-archive
refusal; selected-lock shared-store provenance in both population orders; declined
and dry-run no publication; mid-retention cancellation cleanup; single-download
archive selection, wrapper-independent logical paths, malformed-root precedence,
corrupt archive refusal and no raw SKILL import. Parser/model checks passed too.
Agent process responses remain inert fixtures, and no package code runs.

Independent reviewer verification and GO follow on this frozen corrective commit.
