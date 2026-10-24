# Website Skill sources

Owner extension, 2026-10-24: finish website Skill installation end to end and
push into existing PR #473. This supersedes the original plan's exclusion of
website raw Skills only. Preserve the transport implementation and review
history. No merge, installer publication or release is authorized.

## Supported source contract

`agentplugins skills add https://publisher.example --skill exact-name` selects
one Skill from a machine-readable discovery index. It does not scrape HTML,
infer resource files from Markdown, or require GitHub. Website sources have a
distinct typed saved-source and identity namespace; existing GitHub keys and
lock semantics remain unchanged.

Support the deployed `files[]` discovery dialect: an index has a `skills` array
whose entries identify a name and complete list of relative files. Prefer
`/.well-known/agent-skills/index.json`, falling back to
`/.well-known/skills/index.json` when the preferred path returns HTTP 404 or an
HTML response with HTTP 200. A malformed JSON index is an error. Scoped addresses must retain
their scope and never silently select from the publisher's root catalog.

This is explicit compatibility with an emerging discovery convention, not a
claim that website discovery is a finalized Agent Skills standard. Discovery
proposal [#254](https://github.com/agentskills/agentskills/pull/254) and
[discussion #255](https://github.com/agentskills/agentskills/issues/255) were open
at research time. Proposed 0.2 artifact entries (`type`, `url`, `digest`) are
outside this slice and must produce an understandable unsupported-format error.

Research identified real deployed catalogs at
[Feishu](https://open.feishu.cn/.well-known/skills/index.json) and
[Uizze](https://uizze.sh/.well-known/agent-skills/index.json). Feishu's preferred
path served HTML, making content-aware fallback a required interoperability
case. Discovery metadata does not certify the Skill content or its compatibility.

## Acquisition and version invariants

Download every declared file into an owned disposable tree. Bound index bytes,
entry/file counts, individual/total file bytes and request duration;
honor cancellation and clean partial materialization. Reject unsafe or colliding
paths, malformed source URLs, credentials and unsupported formats before client
effects. Require HTTPS and normal certificate verification for every request;
refuse redirects and do not forward credentials. An index never authorizes arbitrary
filesystem paths or executable package content.

Feed the sealed raw tree through the existing standalone Skill normalizer,
validator, scanner and installer. Keep raw content identity separate from the
normalized plugin tree and its normalizer version. Preserve complete nested
resources; installing just SKILL.md is not successful acquisition.

Saved intent records the website URL and selected Skill name. The lock records
the discovered index URL/hash, exact file paths/URLs/hashes, raw tree digest and
normalizer version. A locked replay retrieves recorded files and verifies their
bytes without rediscovering or substituting the publisher's current selection.
Normal shared-store reuse still verifies source authority and content identity.

Only explicit update rediscovers current publisher content and produces a new
pin. Sync, repair, export/import and cold restore preserve the recorded pin;
changed or missing old files fail without replacing the lock or active content.
The lock does not promise historical hosting when a publisher deletes old bytes.

## Required evidence

Use a disposable TLS publisher and disposable client roots through the public
CLI composition. Install a selected Skill with nested resources, save intent
and lock, verify projection and repeated sync, change publisher content, prove
locked replay refuses replacement, explicitly update, and restore/export/import
the exact pinned version. Exercise missing historical bytes and no-effect
failure. Keep native agent launch and runtime provisioning outside these tests.

Adapter contract tests cover malformed/HTML preferred-index fallback, selection
ambiguity, scoped discovery, traversal/collisions, TLS/redirect policy, resource
limits, cancellation and digest mismatch. Existing GitHub Skill and plugin
transport lifecycle suites must remain green. Record actual commands and
results after implementation; this document alone is not completion evidence.
