# SkillSpector directory reference display contract

Companion to Registry [PR #404](https://github.com/777genius/universal-agent-plugins-registry/pull/404) and issue #402.

An explicit directory link with at least one acquired descendant may be classified as `uninspected` with a canonical non-null target and `partial` disposition. This is **not** a complete or verified scan; the site must preserve the original evidence and continue to reject falsely complete reports.

Changes: accept the new producer status in the strict v3 decoder and TypeScript type, add negative decoder tests for forged complete/analyzed state, and explain the limitation in all nine maintained locales. No changes to report hashes, counters, rankings, or scanner output. Merge the consumer before publishing the producer.
