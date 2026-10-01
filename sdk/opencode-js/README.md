# OpenCode event observer

`createObserver({client, emit})` accepts native `event` hook values as untrusted
input and emits content-free wire v1 facts. A consumer can pass an OpenCode
plugin hook's `client` and call `await observer.observe(input.event)`; the
callback returns no OpenCode control response. OpenCode's event hook does not
await background delivery, so emission is best effort and process shutdown may
drop work. Consumers decide their own policy and delivery.

The observer recognizes `message.updated`, `session.status`, `session.idle`,
`question.asked`, `question.replied`, `question.rejected`, `permission.asked`,
`permission.replied`, and `session.error`. It uses a timed, concurrency-limited
`client.session.messages` lookup before reporting a request, terminal error, or verified turn
idle. Both `session.status` idle and `session.idle` use the same semantic
deduplication key. A verified idle requires user and final assistant
`message.updated` events observed during this process, the current user
and latest assistant in the lookup, and a final assistant message parented to the user
with `finish: "stop"` and a completion time. A retry or resolved request
requires a later final assistant update before completion is eligible.
Cancellation, unresolved requests, stale messages, and initial idle do not
produce a completion fact.
An error is emitted only when its session still has the latest observed user
turn; an error after a verified completion is ignored. Optional native
`messageID` must identify an assistant message parented to that turn.
Late metadata updates for an older user do not replace the current turn.
Context overflow is held until idle because OpenCode also emits it before
automatic compaction; a final assistant error at idle confirms a terminal
failure. An assistant error without `session.error`, such as a structured
output failure, is verified at idle as well.
Deduplication lasts only while the session remains in this observer instance's
bounded memory. Unknown native events use a separate evicting cache and cannot
evict the completion/error admission of a retained session.
Lookups that finish after their session state was evicted produce no fact.

Every actionable fact carries `rootSession`, verified through `client.session.get`.
`false` means an OpenCode child session; the consumer must filter it when it
only supports main-session notifications. If ancestry cannot be verified, the
observer emits no actionable fact and reports a content-free diagnostic.

The shared fixture `fixtures/wire-v1.json` is the v1 contract for this producer
and `sdk/opencode`'s Go decoder. Future native events become `unknown` facts
without their body. The wire carries no prompt, response, question text,
error body, tool arguments, project identity, or delivery metadata.
IDs are limited to 256 UTF-8 bytes and exclude JSON control characters.

The exact OpenCode 1.18.33 plugin and SDK versions are pinned for type checks.
A disposable OpenCode 1.18.33 profile loaded this adapter source as a local JS
plugin and produced all four wire facts: turn completion, question, permission,
and terminal error. The error run also emitted a native idle, without a false
completion fact. `universal-agent-plugins-opencode-events` is a standalone npm package.
The source probe alone does not qualify the published package artifact,
Notifications delivery, lifecycle, or other operating systems.

## Native OpenCode V2

`createV2Observer({client, location, emit})` is additive. Its structural client
receives native `ctx.session` with bare-value `get({sessionID})` and
`context({sessionID})`; no V1 methods or V2 plugin runtime dependency
are required. The exact V1 SDK peer is optional for V2 JavaScript consumers;
V1 TypeScript consumers still install the pinned SDK for the public native alias.

```js
import { createV2Observer } from 'universal-agent-plugins-opencode-events';
const observer = createV2Observer({ client: ctx.session,
  location: ctx.location, emit: forward });
// Pass each native envelope directly, without awaiting external delivery.
observer.observe(event);
// The subscription owner also closes its native iterator on cleanup.
observer.dispose();
```

V2 recognizes native inbox/execution/step/retry/compaction, question forms,
permission requests and their resolutions. It emits only the four actionable
wire v1 facts for authoritatively verified root sessions. Global plugin bus
subscriptions receive other locations, so both directory and optional workspaceID
must match the observer owner and native session lookup. Moves/deletion fence
old ownership before filtering foreign events. No history is replayed at startup.

Completion means the latest delivered user input and final primary assistant
settled successfully for an observed native busy period. Coalesced queued inputs
can produce one completion. A successful execution alone or a finished tool
step cannot produce completion: the live final stop must match current context,
including completed time and no error, no later continuation, unresolved request,
retry or interruption. V2 idle control rows can follow the answer. Compaction
can remove the visible user only when the observer retained live delivery and
compaction continuity in the same work epoch. Terminal execution failure is
separate from retry, step failure and cancellation. Requests resolved during a
lookup never emit afterward.

Metadata reduction is synchronous; ownership verification and delivery proceed
in the background. Defaults are 512 session records, 64 admissions/pending
requests per session, 16 **actual outstanding host calls per observer**, and a
2-second soft timeout (clamped to 100-10000 ms). Context rejects more than 4096
rows and copies metadata only. Native context has no server-side row limit,
so these limits do not bound network bytes. OpenCode 2.0.0 ignores the optional
signal: timing out or disposing does not release an actual slot until the host
Promise settles. Saturation fails closed without a retry queue.
An inconclusive ownership lookup stays unverified and may be tried once again
when a new execution starts; facts from the earlier epoch are never replayed.
Confirmed child/foreign ownership remains rejected. Session records use recency
and evict child/foreign or inactive records before active work; unavoidable
active eviction emits the fixed `session_capacity` diagnostic.

Dispose immediately fences late results, clears timers, and does not wait for
hung calls. It cannot cancel an external callback already admitted. Semantic
admission happens before emit; callback failure, timeout or capacity never
retries that fact. Deduplication is process-local and bounded, not durable
exactly-once delivery. Diagnostics use fixed codes and carry no native bodies,
paths or identifiers. Native qualification and platform support belong to the
consumer's exact installed artifact evidence, not this SDK's contract tests.
