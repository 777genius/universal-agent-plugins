# P1 — ChatGPT Desktop/Web release and consumer gates

## Current implementation / dependency
Continue existing draft PR #773, do not fork its code. CLI target semantics: `codex` means Codex CLI, `chatgpt` means local ChatGPT Desktop using shared Codex backend, `chatgptweb` means manual remote app binding. Current public npm 0.1.101 does **not** advertise those names. A future release is expected but must be verified when it actually exists, not assumed.

## Steps
1. Finish exact-head CLI aliases, safe no-op repeat/add/remove and project-scope refusal; `chatgptweb` must require verified signed `app_binding` and explicit user consent, never silent remote account mutation. Shared Codex backend must not install twice.
2. Keep site output compatible with **latest actually published npm**. If CLI release not yet published, prepare separate CLI producer and site consumer commits/PRs; do not deploy site commands for unavailable names. Source changes and public release qualification are distinct.
3. Verify the release version contract from GitHub tag, published npm version and executable `agentplugins version` — no time-based guess or unpinned fallback.
4. E2E desktop and mobile command copy, Desktop/Web selector, user/project scope, malformed and missing binding refusal, light/dark, keyboard focus, no private account interaction; source-level real Codex-native tests plus signed/unsupported negatives.
5. Review, exact-head CI, gated merge and post-merge site browser checks. Actual signed-in ChatGPT Desktop plugin execution or Web app registration remains a separately attested user-account gate, never a fake green CI.

## Non-goals
No second adapter/store, no CLI feature flag, no new release initiated here, no duplicate changes to #773.
