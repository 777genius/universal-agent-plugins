# Reviewed project context follow-up integration

2026-10-23. Integration commit `042452234027d3656dd00e807bafd692549153a1`
contains two clean, ordinary Git merges with no conflict resolution or novel
production edits:

- Retained-data guidance final `852e3e0b1339c03e0fb00a876dc21439b497075d`,
  reviewed source `33004e06cf5d65f112efad0455cc793bc0a56f39`. Both plan and
  implementation rounds are committed. Project follow-up commands retain the
  validated canonical project selection; user wording and actual purge
  admission remain unchanged.
- Shared host context final `8fec136f9dfa4e290e67f71b2f46b28362ccb335`, reviewed
  source `64c7ee6d48084da6247a200eb3e77e8e6eb0dc29`. Both plan and implementation
  rounds are committed. The selected detected version travels in an ephemeral
  request field, and an absent legacy profile is admitted only for an already
  validated project request. User and strict sibling-profile checks remain.

After the clean merges, the public CLI retained-data regression passed in
**1.993s**, including all three JSON/human/saved routes, captured shell argument
boundaries and replay from an unrelated cwd. Exact user/project-B/cache bytes
remain unchanged while only project A's retained data is purged. Existing
POSIX presentation scope is preserved; no additional Windows shell claim is
made by that test.

The two project-only absent-profile/user-admission tests passed in **0.012s**.
The separately selected `TestProjectPreparePreservesSelectedHostVersion`
passed in **0.010s**. Go1.26.10, bounded `-p=2` execution and fresh disposable
test roots were used. No native agent was launched by these checks.

This checkpoint supplies shared reviewed context for the remaining client PRs.
It does not enable the pending Kimi/Devin adapters, replace their independent
code reviews or qualify native discovery, authentication, MCP or a release.
