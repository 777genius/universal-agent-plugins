# Direct source lifecycle implementation review — S5 round 2

**Current status: S5 R2 GO at `816e577a` after corrective verification.**
The initial verification requests below are retained as history and all three
are closed by the final ledger.

Reviewer: `client_plans`, independent of the source implementation. Initial
frozen tree: `236c331983d9356345a6654776e1b30a995aabc5`, after `501d7f56` and
`d35d2863`. Read-only production review. The archive implementation's internals
are excluded from self-review; its two independent reviews and approved
redirect/member-prefix amendments remain authoritative.

## Decision

**Initial assessment: implementation direction GO; three narrow verification
gaps required closure before complete S5 qualification.** No new production correctness blocker was
confirmed in this round. Source lead and parent received the concrete fixture
and CI requests below; these are already explicit requirements of the approved
plan, not a new transport matrix or architecture.

## S5-R2-1 — distinguish archive bytes in the shared-tree provenance proof

`TestDirectSourcesWithIdenticalBytesKeepSelectedLockProvenance` exercises both
endpoint/cache population orders but serves the identical ZIP byte stream to
both endpoints. It proves endpoint identity reconstruction, without exercising
the separately required byte-distinct archives whose selected package tree is
identical. A first-writer archive checksum is precisely the metadata that must
not be adopted from the shared tree store.

Strengthen the existing fixture by varying its physical provider wrapper for
each endpoint (`provider-a/`, `provider-b/`) and assert that original-byte SHA256
differs while tree digest remains equal. The existing materialize/describe/state
and fresh shared-store restore assertions can then validate checksum provenance
in both orders without new duplicated journeys.

## S5-R2-2 — prove private extensionless HTTPS fallback end to end

The existing private Git lifecycle uses `/repo.git`, which the parser classifies
directly as Git. The extensionless lifecycle is a valid archive. Neither runs
the required credential-free HTTPS-auto probe -> nonarchive 401 -> native
credential-backed Git fallback boundary.

Add one real private extensionless clone endpoint to the existing TLS smart-Git
fixture. Its initial root probe returns 401 without receiving Authorization or
cookies; native Git subsequently authenticates using the disposable helper and
loads the standard package. This proves classification and credential isolation
jointly. Inspection of `acquireDirectPackage` found the expected `KindHTTPSAuto`
plus `ErrNotArchive` gate and no fallback after recognized archives; no new
production fallback engine is requested.

## S5-R2-3 — require the SSH transport proof in one Linux CI lane

Both real SSH fixtures skip when `sshd` is absent. No source-transport workflow
in the frozen source tree provisions/asserts it, so a green generic test job
can silently omit SSH/SCP qualification. The approved plan requires a bounded
Linux lane that installs or verifies openssh-server and explicitly runs these
fixtures. Parent assigned a separate source-transport workflow to avoid core
workflow edit contention. Platform-optional local skips remain appropriate;
the required lane must fail if its native prerequisite is missing.

## Component ledger

| Boundary | Result |
| --- | --- |
| Source dispatch and old capability fakes | GO: old narrow acquirer remains compatible; new direct transport requires an explicit optional capability. Directory and Discovery retain their separate routes. |
| Browser selectors and saved intent | GO: unresolved browser coordinates are resolved before persistence; final Git endpoint/ref/path and immutable commit enter the lock. Archive wrapper paths remain logical. |
| Generic Git/archive saved add/update | GO: typed requirement/lock variants are carried through candidate acquisition, retained-source admission, moving update, exact checksum/commit retention and graph validation. |
| Shared tree cache provenance | Implementation GO: every described/cache/replay/materialization path reconstructs identity from the selected pin; store metadata remains historical. Cache keys bind stable source + immutable revision + algorithm + tree. Proof gap S5-R2-1 remains. |
| Exact installed repair | GO: both single and grouped imperative repair use `loadRecordedDirect`, selected binding revision, source endpoint/path, archive byte pin and tree/manifest digests. Saved repair uses the old installed pin after the remote advances. Changed unavailable archive bytes fail before state/target mutation. |
| Cancellation and consent | GO: `finishLoaded` threads operation context through resealing and store Put; cleanup handles original/failed retained snapshots. Dry-run/decline retain command-local bytes and do not publish store/intent/lock/ownership. |
| Domain/state source facts | GO: common source binding helper carries kind/URL/archive SHA through new installation, update/switch, grouped commit, inspection and state receipt source-key validation. Legacy source-key/identity forms remain separate. |
| Complete-package boundary | GO: standard loader stays authoritative; malformed root manifests, raw skills and recognized corrupt archives do not install or fall through to Git. |
| Transport proof boundaries | Native Git/HTTPS/SSH and archive fixtures exercise actual transports. Agent process behavior in these journeys is inert fixture behavior, not native Gemini/other client runtime certification. S5-R2-2/3 remain. |
| Project/source integration | Pending combined-branch seam proof owned by parent/core. This source branch predates project core and retains existing user-scope Directory request sites; integration must preserve the selected App/context and run a qualified project source journey. This report does not claim that combined seam is already qualified. |

## Independent verification so far

Original S1/S2/S4 findings are independently closed in
`sources-code-review-r1.md`: parser full suite 0.003s, environment full suite
0.016s, four actual native Git scoped/rewrite TLS, protocol-file and downgrade
regressions 0.547s on the frozen tree. The source lead and core reviewer also
reported the bounded cumulative native transport/lifecycle suites passing;
final verification in this round will use the frozen follow-up containing the
three narrow proof corrections. No production files were changed by this
reviewer.

## Corrective verification — all requests closed

Frozen correction: `816e577a8f73dbc2fc83a0a5ccaeecc5c9d096e4`.

| Request | Verified correction |
| --- | --- |
| S5-R2-1 | Existing two-order shared-store fixture now uses different provider wrapper names, explicitly asserts distinct archive SHA256 and source key with equal selected tree, and checks pin-derived identity through materialize, describe, persisted state and fresh shared-store restore. |
| S5-R2-2 | Existing real TLS lifecycle adds a private extensionless `/clone` endpoint. The root probe returns 401, records exactly one probe and no Authorization/cookie, then actual native smart-Git requests authenticate with the disposable helper. Add/update/repair/cold restore complete through the production source/lifecycle composition. |
| S5-R2-3 | Dedicated Ubuntu24.04 source-transport workflow provisions and asserts SSH tools, sets `UAP_REQUIRE_NATIVE_SSH=1`, and requires exact JSON pass events for native host verification plus CLI ssh-url and SCP subcases. Missing tools and skips fail the required lane. Local platform-optional behavior remains unchanged. |

Independent exact Go1.26.10 execution passed the corrective/repair/cancellation
selection in6.889s. A subsequent explicit complete bounded lifecycle selection
with required SSH mode passed in14.500s, covering all real HTTPS Git/private
Git/private extensionless Git/archive forms, both SSH spellings, both shared
store orders with distinct archive bytes, imperative and saved exact old-pin
repair after source advancement, and retained snapshot cancellation. No real
agent/model/MCP runtime or external service was executed.

**S5 R2 GO.** No production correction or verification request remains open in
this source review. Parent owns remote CI and combined source/project seam
qualification; those remain separate from this source-only verdict. Parent also
requested avoiding duplicate feature push/PR SSH jobs; that workflow capacity
cleanup does not change the reviewed product or transport proof contract.
