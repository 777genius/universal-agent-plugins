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
completion fact. `plugin-kit-ai-opencode-events` is a standalone npm package.
The source probe alone does not qualify the published package artifact,
Notifications delivery, lifecycle, or other operating systems.
