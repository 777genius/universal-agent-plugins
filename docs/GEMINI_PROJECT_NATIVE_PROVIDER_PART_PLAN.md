# Gemini native provider: observed call-part and wire identity grammar

## Exact failed capture and source proof

Current PR526 head eef6c202abf02489338db54c0ffbb9cb2c283ca6, tree
3f17100dcc7de10e24e635d2ee91752e8c5b7de5, is unchanged by a fresh API read.
Local a6eeb072116d390151dbefc90151ce8c8a11713b has exactly that tree.
Runtime run40405537328/job121905922854 fails after native tool_use/tool_result
and a correlated installed probe call. Provider request2 is rejected by
"Gemini part union"; native exit144 and failed terminal output remain unqualified.

Closed native12698460531 outer SHA256
0aba508ca5cbaebc1054a04654d6afc6b534ecdc298b91497a14857cecb03cc0
and frontend12698965124 outer SHA256
b107a9e30cb57e6460b5725fadf390e7ee525c4cdd4a2aea59dd754413c10151
were checked for bounded canonical unique ordinary members, CRC and exact
committed source/tree. Raw provider-request-2.json SHA256 is
350f5d85ebcd940ea1082699ef340c1eb388bc0a5bc4157b5022829235d97b4f.
It has one model functionCall part with exactly the functionCall and
thoughtSignature fields; the signature is the literal
skip_thought_signature_validator. Both HTTP functionCall and functionResponse
IDs are uap_<fixture nonce>_attempt. Native stream tool_use/tool_result IDs are
mcp_uapgeminia_inspect_runtime__uap_<fixture nonce>_attempt.

Pinned publisher google-gemini/gemini-cli@6006c14a4decfe4b891c6263ad6ad5992e404b5b
packages/core/src/core/geminiChat.ts blob095a5292c4820c8158bbb73c5af97d729e69bd87
(SHA2567aeca2fc0eff93f003c8d628889cf6941f9359597673c907fb64d049fadd3945)
was fetched at that immutable commit and its full git-blob hash independently
recomputed. Lines111/1387-1433 define and inject that synthetic signature on the
first function call of a model turn; lines1531-1544 retain/add name__ prefixes
for native identities; lines1178/1948-1976 strip exactly name__ from calls and
responses before the outgoing provider request. Full source and provenance are
retained in /tmp/uap-shell-fresh-oct27/pinned-gemini.

## Finite correction after two independent plan GOs

Change only the existing provider and contract Go test files:

1. Keep text and functionResponse part unions exactly single-key. For a
   functionCall require exactly functionCall plus thoughtSignature, the exact
   observed signature string, and model role. Reject missing, changed, empty,
   null, wrong-type, extra-field, text/signature, response/signature or multiple
   payload forms. Keep exact function ID/name/arguments and all generic strict
   JSON, request/schema, request/body count and bounds.
2. Represent the two source-proven identities explicitly: retain p.id as the
   existing native tool-prefixed identity and retain its issuance unchanged.
   Add a separately constructed expected wire ID uap_<nonce>_attempt for only
   provider HTTP history validation. Do not normalize received IDs or accept
   either form interchangeably. Both HTTP call and response must equal the
   exact wire ID; native stream checks continue to require only p.id.
3. Update inert provider exchange positives to the independently stated
   observed two-key functionCall part and normalized HTTP IDs. Retain existing
   foreign route/key/tool/call/result/wrapper/facts/extra-turn controls. Add
   missing/wrong/malformed signature and wrong-role/extra-payload controls,
   unstripped or foreign HTTP call and response IDs, and a native stream case
   rejecting wire IDs even when both tool events agree. Assert the original
   issued/native identity remains distinct from the expected wire identity.

Scope is exactly repotests/agentplugins_gemini_project_native_provider_test.go,
repotests/agentplugins_gemini_project_native_contract_test.go and this plan.
No production/client adapter, source/pins, provider response, native argv/env,
authentication, permission/trust, probe/facts, stream/session/terminal grammar,
budgets, cleanup, artifact closure or qualification flags change. In particular,
the separate currently strict native stderr diagnostic guard remains strict;
this provider grammar correction alone does not claim the full attempt passes.

## Validation and review

After the exclusive shared Go/lint slot is granted, run the affected existing
and new inert Gemini request/provider/stream contracts and actual retained
provider-request2 replay through the exact candidate parser. An ephemeral
verification test may supply that retained JSON to the unexported parser, must
be removed before commit, and is offline parser replay, not native execution.
Run affected all-enabled lint plus the whole-file size/architecture gate and
gofmt/whitespace checks. Do not run a vendor binary locally or add a skip,
suppression, broader unknown-field allowance or qualification promotion.
Stop and report unrelated lint failures rather than expanding this plan.

Independent implementation review precedes root-owned publication to existing
PR526. Fresh genuine source-bound native output and full applicable lifecycle
gates remain required. The failed native trace cannot be converted into a
passing terminal session by this amendment. Research diagnostics have a
separate plan and commit; no new PR, release or VS Code work is included.

