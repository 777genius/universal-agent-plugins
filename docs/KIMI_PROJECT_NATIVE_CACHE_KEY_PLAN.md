# Kimi single-tool provider: observed session cache metadata

Status: PRE-CODE draft; no provider implementation changes authorized until two independent GOs on this exact plan. Base source is PR524 b2bb2b7f85b5461420f932478b197905e6fc5ad9; implementation will be composed on the reviewed disjoint polling-diagnostic branch (code8311c8e9f571e9fa272abc2daed1a2f367ad160e) and then the independently reviewed scoped-selection fix. Provider source is byte-identical on these bases.

## Concrete observed failure

Genuine disposable GitHub-hosted run40569825577, job122438517585, source b2bb2b7f85b5461420f932478b197905e6fc5ad9/tree b12354b2565a05d14037556f8493b149d7d36ea2 produced native artifact12768461663 (23 members). Its original provider-request-1.json has SHA256 ddd8a402037716fe582f854dcf5d5dae1ebb902dc6b3a962466e4d34874c4de7. It offers exactly the installed fixture tool/model and includes prompt_cache_key, a44-byte string of the form session_<lowercase UUID>. Raw request remains in /tmp/uap-additional-qoder-kimicall-fresh-ka537i9_/12768461663/members/provider-request-1.json. No session identifier needs to be copied into repository documentation.

The current kimiAttemptRequest top-level allowlist excludes prompt_cache_key. The provider appends the raw request, validates that envelope, and returns400 on error before setting issued or producing its owned tool call. This source path plus the observed body identifies the first provider rejection. HTTP response bytes and the native empty approval response are not independently exported. The observed failed result, ordinary quarantine, SIGTERM cleanup and allfalse qualifications remain truthful.

## Pinned primary-source basis

Repository MoonshotAI/kimi-code, commit b8dd3a134060c6e80ba4a1659335cd63b68483f2. Source archive and exact Git blobs were read only; no vendor code was executed locally. Every cited file was verified against the complete pinned Git tree and its Git blob hash.

- packages/agent-core-v2/src/workspace/sessionLifecycle/sessionLifecycleService.ts, blob b3eab53a221dca58aab5876a68d04da64da0e6f5: createSessionId returns session_ followed by randomUUID().
- packages/agent-core-v2/src/agent/profile/profileService.ts, blob c73828a3be887c221ad16155898ecd52f9218ffe: model request params use this.sessionContext.sessionId as cacheKey.
- packages/agent-core-v2/src/llm-adapter/model/model-requester-impl.ts, blob83cc1cbde9c44724864b4f19a7ce73c96366673a: forwards params.cacheKey to the requester.
- packages/agent-core-v2/src/human/llm/requester/bases/openai/requester.ts, blob3726b5b38dace425ea3da588e53971991a6b1307: encodes an optional input.cacheKey.
- packages/agent-core-v2/src/human/llm/requester/bases/openai/format.ts, blob d074e9c1fb310d78f3dd1f497f28bb4481e2ed75: maps the value to prompt_cache_key.

Source receipts, hashes and excerpts: /tmp/uap-additional-kimi-cache-key-source-thrtbnmr/cache-key-source-trace.json and session-id-source-trace.json. The field is request metadata, not a credential or tool ownership proof.

## Exact intended change

1. Extend only the Kimi single-tool test provider's known optional envelope fields with prompt_cache_key. If present, require a built-in Go string of exactly44 ASCII bytes matching session_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}. Use anchored bounded validation; reject null, booleans, numeric/array/object values, malformed/oversized/noncanonical strings. Absence remains valid for preserved inert requests and the source-supported no-cache case. Do not add other request fields or widen message/tool grammars.
2. Hold the first admitted request's cache metadata presence/value in provider state (empty string represents absence, because empty values are invalid). Require exact consistency on the second admitted completion request; added, removed or changed cache metadata fails before validating tool result or marking returned. This is an additional same-session invariant supported by the source's sessionContext binding. It does not establish equality to the native API session, which is not available to the Go provider at this stage; no such join will be claimed. Do not include raw values in errors or new diagnostics.
3. Preserve original bearer key, host/route/MIME/body/count limits, installed model/exact-one-tool declaration and schema, first-call no preexisting result, owned call ID/nonce/operation/marker, two-turn result validation, SSE framing, first-error latch and counters. Cache metadata cannot change any ownership or qualification decision.
4. Preserve client pin/config, default manual permission mode, actual approval validation/action, native service supervision, deadlines/poll count, artifact format/privacy/admission, original HOME/profile and whole-profile guard. No auth/trust/credential/pin/route/timeout/baseline workaround, fake hosted marker or local vendor execution.

## Meaningful validation

Before parser code changes, add a permanent inert regression with a fresh synthetic canonical session key and capture its failure against the old parser. Separately replay the unchanged genuine raw request through the old exact parser and the real in-process provider using a temporary test file; retain the assertion failure and exact raw hash. Do not commit the private/session-bearing raw request. Environmental setup failure does not count as RED.

After implementation:
- Exact unchanged genuine raw request passes its envelope and reaches the existing owned first SSE response under a separately held inert provider credential. This is an inert request replay, not native execution or new end-to-end proof. Verify issued=true/returned=false, one captured request, unchanged raw bytes, one request count, no validation callback, expected owned call and no terminal completion.
- Permanent positive tests cover absent metadata compatibility and exact same canonical metadata through both real in-process provider turns, using the existing installed tool/nonce/marker facts. Verify original second-turn facts and terminal sentinel, request counts and no error.
- Permanent table-driven refusals cover wrong type, empty, prefix/length/hyphen/hex/case/control/oversized values and duplicate JSON key. Unknown fields/foreign tools/schema/duplicate JSON remain refused. Exercise the real provider path for representative invalid metadata, not only the helper.
- Second-turn changed/added/removed metadata refuses before result validation/returned. A later valid request cannot recover a refused provider. Verify source arguments, original model/tool/credential/route and counts remain required. Existing wrong-key/host/call/arguments/extra-turn tests stay intact; include metadata-bearing wrong-key, wrong route or foreign tool controls so metadata cannot bypass those gates.
- Run focused inert Go Kimi/provider contracts and existing relevant project-native contracts with no vendor opt-in. Serialized Go/lint slot; compile/test failures are recorded separately. gofmt and git diff --check. Affected all-enabled lint plus current six-linter gate. Read/replay the original failed artifact through the exact composed-source Python intake to confirm it stays failed/allfalse and keeps identical member bodies. No redundant broad test fan-out.

## Review, publication and evidence boundary

Record both exact pre-code GOs, RED/GREEN and final source manifest. Obtain two independent exact implementation reviews, fix actual findings, then compose with the separately reviewed scoped-selection and poll-diagnostic slices. Root alone performs normal expected-head fast-forward publication to existing PR524; no forced update, replacement PR, release or main merge. Actual hosted E2E after publication determines the next client state. Retain failures/unknowns without promoting artifact admission or source guidance to native qualification. VSCode remains deferred and unchanged.

## Accepted before implementation

Two independent GPT-6.1 Sol xhigh reviewers accepted corrected draft SHA256 `383c3df38324513560aca8c0d1cc878eb312c59668cfbb9429e9a11e9f633978`. The earlier draft mistakenly labeled the cache-value hash as the whole request hash; both reviewers verified the corrected original7839B body before granting GO.

- `/tmp/uap-additional-kimi-cache-key-plan-R1.md`: SHA256 `8728c97e602b2177dfe3a9a3b50dc1220311917ef4fcfcd6f6d2649ab1aa6c6d`.
- `/tmp/uap-kimi-cache-key-plan-r2-review.json`: SHA256 `37651c4f69c89fd8def9101dee346eee709fe5cb20f657ea5df1bf6eb2d28c03`.
