# Source transports cumulative implementation review R2

Independent reviewer: project_core, 2026-10-23. Frozen starting tree `501d7f5610cd584d81a801059a817f7cb5f61657`, cumulative source changes after `1a9b25de`. Read-only source worktree review. Archive implementation was separately reviewed twice, including redirect/dot-prefix amendments and subsequent cancellation/suffix corrections; this review concentrates on native transport corrections and S5 lifecycle/cache integration.

## Decision

**R2 GO at `236c331983d9356345a6654776e1b30a995aabc5`.** Both findings below are closed after independent code and focused execution verification. Their original findings are retained as review history. The architecture, typed source/lock model, native credential boundary and cache-provenance direction are approved. Cohesive source/project execution remains an integration gate after the qualified adapter stack; it is not a blocker in this source-only review.

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
- Agent process behavior in these source transport journeys is fixture-backed and does not certify native agent products.

## Independent fix verification

Reviewed corrective `d35d28633bd939eaee85e6dc3d4b42125cbe5f53` and final frozen `236c331983d9356345a6654776e1b30a995aabc5` read-only. The final follow-up preserves those runtime fixes while splitting helpers to satisfy the existing whole-file gates and isolating native Git test credential/CA configuration.

- **R2-1 closed.** Both grouped and single imperative repair dispatch recorded typed sources through `loadRecordedDirect`. Git acquisition uses the selected binding's exact revision; archive acquisition uses its tagged original-byte SHA256. Recorded logical path, tree and manifest digests are checked. The acquisition endpoint comes from SourceURL, never the canonical hash key. Explicit local archives retain local coordinates. Saved drift repair retains its installed lock pin after the remote advances; unavailable changed archive bytes fail before target/state/intent/lock writes.
- **R2-2 closed.** `finishLoaded` and all production callers, including content-store Put and executable snapshot retention, receive the operation context. Cancellation cleans the original and failed retained snapshot; the regression asserts no command cache or durable content publication.
- Independently ran the frozen tree's `TestDependencyRetentionHonorsOperationCancellation`, `TestDirectImperativeRepairUsesRecordedBytes`, `TestSavedRepairRetainsOldPinAfterSourceAdvance`, `TestDirectSourceSavedLifecycleRealSSH`, `TestDirectSourcesWithIdenticalBytesKeepSelectedLockProvenance`, `TestDirectSavedPreviewDoesNotPublishStoreOrIntent`, `TestLegacyGitHubLockIdentitySurvivesNativeTransport`, `TestDirectArchiveSelectionUsesOneDownloadAndLogicalPath`, and `TestDirectRecognizedInvalidPackagesNeverInstallOrFallback` with `-count=1 -p=2 -timeout=2m`: **PASS, 7.936s**.

The focused run covers real SSH/SCP Git and TLS archive transport with inert agent fixtures, exact old-pin repairs and changed-byte refusal, same-bytes distinct provenance, private preview, legacy identity, cancellation cleanup, one-download logical selection and no-fallback malformed package handling. No confirmed review finding remains open.
