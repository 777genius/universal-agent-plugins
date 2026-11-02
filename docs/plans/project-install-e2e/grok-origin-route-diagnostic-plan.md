# DRAFT: Grok first rejected GET — finite origin-route observation

Status: research/pre-code only. No implementation is authorized by this document until the owner/root obtains two independent plan-review/fix rounds. This research worker made no repository changes and ran no Go, lint, or Grok executable.

## Scope and inspected source

Read AGENTS.md, docs/PROJECT_INSTALL_OWNER_SCOPE.md, docs/AUTHORING_CAPABILITY_PRESERVATION.md, and docs/plans/project-install-e2e/completion-plan.md. Observed /tmp/uap-grok-publish at c6c58579674e0bdb230a556f0bef1de38ce40612 with a clean short status. Root owns all checkout, checkpoint, manifest and remote mutations. Preserve parallel work in both existing checkouts. VS Code is deferred, no release is authorized, and legacy capabilities remain preserved.

Relevant current files: repotests/agentplugins_grok_project_native_route_test.go, repotests/agentplugins_grok_project_native_provider_test.go, scripts/project_native_runtime.py and its focused runtime-admission contracts. The current finite Go URI observer and the Python grok_provider_route_observation validator recognize exactly /v1/chat/completions, /chat/completions, /v1/models, /models, /v1/responses and /responses; all other RequestURI strings become other in the observer. The separate unchanged provider admission guard allows only the literal POST /v1/chat/completions plus its existing loopback/Host/URL/MIME predicates and exact fixture bearer check.

## Genuine new blocker and limits

Exact source c6c58579674e0bdb230a556f0bef1de38ce40612, tree a9c7f08283d631d8e0af2e448601b89f201dbc43; run 40569312299, job 122421982285. Native artifact 12766872743 outer ZIP SHA256 9696d09f9d1f94a868d1830da142a373e4a38ea671c5baca89935a0fec6b075d (4673 bytes); frontend artifact 12766048484 outer ZIP SHA256 b856a564db2322a913318d5955a54d086871851fa2ff0faaf74e5222a0c3205e (3391 bytes). The root independently audited both outer/closed ZIP archives at /tmp/uap-additional-grok-c6-audit-u8yod99g/archive-audit.json.

The migration sentinel prevents the previous full-config fatal. Genuine native inspect succeeds with projectTrusted=false, the owned project plugin visible as enabled=false with one skill, prepared only. The first native --single attempt aborts at 325 ms after provider failure=route: observed method GET, URI other, loopback peer and matching Host, relative URL, empty URL host and raw path, missing Content-Type. Admitted requests/responses remain 0/0 and ordinary provider shutdown completes without forced close. Native streams are empty; all runtime qualifications remain false/incomplete.

The provider checks route before bearer and cancels the attempt context on first failure. Therefore credential_matched=false is not evidence of credential absence, and the deadline=true receipt is not a measured 150-second timeout. other excludes only the six exact observer literals; it does not identify any other path, query, endpoint purpose, or caller.

## Primary-source research, exact identities

Official xai-org/grok-build public snapshot 2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8 has a complete 5102-entry tree. All seven cited source blobs were fetched read-only via the GitHub API, verified by recomputing the Git blob SHA1 from the exact UTF-8 bytes, and independently SHA256 hashed. Exact hashes, source URLs and line excerpts are in /tmp/uap-additional-grok-first-route-research.json.

- crates/codegen/xai-grok-shell/src/agent/mvp_agent/session_setup.rs, blob 061460db05fa95afe7d0fa8aba726c34a7bbec5d, SHA256 266eaf035f690ec57bf62b7d9f06bbf72922b17e07f6cc6229820a015cf05ea7: line 610 spawns sampler transport prewarm using session_sampling.base_url; line 1071 does so on restored sampling base URL.
- crates/codegen/xai-grok-shell/src/agent/mvp_agent/sampler_prewarm.rs, blob ae011abbbfb595ccf79bdac2acf68c7af08a5773, SHA256 2b825cbe4165140211510ae0e77b438acc6349f5ab355683b18e1222b599a254: lines 30–31 spawn prewarm_and_record, whose line 64 calls xai_grok_sampler::prewarm_transport.
- crates/codegen/xai-grok-sampler/src/prewarm.rs, blob 4ee8ad76886f1d28c8a7e1269623fa27e9f5fb3d, SHA256 626aa25d3515423372a812b785f7ccdc090161a3a83dd7a0f4355401602e34e5: lines 70–83 resolve endpoint origin and obtain pooled client; line 200 issues client.get(origin).send().await; lines 218–224 derive url.origin().ascii_serialization(), stripping the inference path/query.
- crates/codegen/xai-grok-sampler/tests/prewarm.rs, blob a806fd1b01327e86e31a1d46177a4eb0c135539e, SHA256 c8bbcd17803f5607650907ce97c499ddb0ccdc29633cb5b259004aaaec50c822: lines 40–44 assert the wire request begins GET / HTTP/1.1. Lines 47–63 assert no content-length, authorization or x-api-key for this public-source test.
- crates/codegen/xai-grok-sampler/src/prewarm_tests.rs, blob e0a65970db6506d125e89f0dffe3c3ad5520ebe0, SHA256 a6e64926e03fe22d2cdcfa8bde780692bdf55a5d7d125e7f4dc25beced8eff5c: lines 12–23 expressly test path/query stripping and loopback port preservation.
- crates/codegen/xai-grok-sampler/src/shared_http.rs, blob e659c7fcc4eb8bba52a3c882fa57f670269c4bf4, SHA256 75e919946fb52b887f9bb0a905d8244596b0228235b1eed8313b76d50f8b5943: source documents GROK_SAMPLER_SHARED_CLIENT=0 (or false) and pooled_client returns SharingDisabled before obtaining a client when this source-defined switch is set. Ordinary sampling still constructs a fresh client.
- crates/codegen/xai-grok-sampler/tests/shared_http_kill_switch.rs, blob 4db871673b5c07023bc3955840397cc478bf90d2, SHA256 43a0dc29fe72e561f125c382a0b58cf9e4a1a2f5d78248ba1b137c9eedcd93d5: source test sets that exact switch and checks two independent sampling clients/connections.

Public source supports a concrete hypothesis: the first GET may be the transport prewarm to /. Shipping build f50904eaee6c does not resolve in this public repository, so source guidance is not proof of binary equivalence. No current genuine observation identifies / or measures absence of credentials. Do not label the c6 route as prewarm.

## Proposed next slice: diagnostic only

1. In grokProjectObserveRoute, add only the exact literal / to the existing closed RequestURI switch. Store that same public fixed literal in the existing URI field. No new receipt field, raw path, URL, request URI, query, header, bearer, body, error text or arbitrary digest is exported.
2. Keep grokProjectProviderRoute, admit, serve, failWithRequest, first-failure locking/cancellation, authored configuration, environment, credential, response, request counters, all deadlines, shutdown and qualification logic unchanged.
3. Preserve other for /?private=..., //, /v1/, escaped/absolute forms and every unknown URI; compare RequestURI exactly, never infer the route from URL.Path. A root-path request with a query must remain other.
4. Update deterministic contract coverage to prove / receives the finite label, GET / still fails existing admission, and first actual refused GET / records only those facts with 0/0 admitted counters and unchanged cancellation. Existing first-error/nonroute-error retention controls remain required.
5. Extend private-value controls with /?credential=private-query (and an unknown root-like value) so no query/body/header text can escape. Keep serialized observation bounded under the existing 512-byte test.
6. Add only the same exact / literal to the Python grok_provider_route_observation diagnostic URI enum. Preserve every closed-field, exact-type, failure-state, contradiction, source, archive, completeness and false-qualification rule. This is failed-evidence admission, not provider request admission. Do not infer the label from URL.Path or admit arbitrary root-like strings.
7. Focused Python controls must admit an authored failed GET / observation under the existing closed provider envelope and reject query, absolute, escaped and other root-like arbitrary URI values. Replay the actual source-bound c6 other capture unchanged, and retain the legacy optional-no-observation envelope contract. No raw old evidence is edited.

A fixed / label is enough to discriminate the strongest source-derived call path. Hashing arbitrary request data is unnecessary for this slice. No accepted GET, synthetic models listing, guessed health reply, MIME relaxation, bearer bypass, deadline increase or second provider transaction is proposed.

## Review and verification gates

Obtain two independent plan-review/fix rounds against this document and the exact inspected c6 source. Reviewers must assess finite-data privacy, exact RequestURI semantics, matching Go/Python diagnostic enums, unchanged admission/cancellation and honest qualification. Root alone implements after those gates, followed by independent implementation review and ordinary exact-head repository verification, including the actual focused inert Go/Python controls and retained failed-evidence replay. No local vendor execution or fake hosted-guard environment is permitted.

A new genuine hosted native run at the reviewed implementation commit/tree must preserve the existing source/workflow/client/archive provenance and ordinary unique artifact-name/closed-ZIP checks. Record exact run/job/artifact IDs plus ZIP SHA256 and size; audit the original native-inspect and native-attempt streams and provider receipt. A successful diagnostic run may still end in the same intentional route refusal. Preserve all false/incomplete runtime/lifecycle claims.

Interpretation is bounded:
- GET plus exact URI / proves only that the actual rejected exchange was GET / under the existing envelope predicates. It is consistent with public-source prewarm; it does not prove caller identity or absence of credentials/body.
- URI other remains unresolved. Do not widen this label set speculatively or serve a response.
- Any different first failure is recorded honestly under existing first-error semantics.
- No diagnostic result promotes enabled=false/prepared discovery to native runtime, skill loading, session trust, MCP, model or lifecycle success.

## Conditional follow-up, excluded from this slice

If new genuine evidence records exact GET /, the source-defined GROK_SAMPLER_SHARED_CLIENT=0 becomes a concrete narrow fixture-environment candidate. Draft and independently review that separate plan using the new evidence before editing the environment. It would preserve the sole accepted authenticated POST route and suppress an optional source-defined pooling optimization rather than inventing a GET response; new genuine proof would still be mandatory. This document does not authorize the switch or claim it exists/works in shipping build f50904eaee6c.

If genuine evidence remains other, return to source-driven finite diagnostics without guessing. The prepared Grok leaf and its existing source/tests remain preserved.
