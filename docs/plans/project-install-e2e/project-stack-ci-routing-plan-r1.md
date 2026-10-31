# Project stack CI routing amendment — independent plan R1

Reviewed frozen plan `94be629c7d3a3675828214f6f84659f9ddc7a0b4`, the existing `.github/workflows/core-fast.yml` and `.github/workflows/agentplugins-native-clients.yml`, and `scripts/run-native-client-matrix.py` on 2026-10-23. **GO for the additive routing implementation.** No workflow code was changed for this review.

The observed gap is present: the native workflow's pull-request base filter is `[main]`; core additionally admits the exact project/source feature bases and `refactor/**`, but omits other project stack bases. GitHub pull-request branch filters select the base branch, so the planned `feat/project-*` addition admits the task's flat client/checkpoint bases and subsumes the existing exact project base. Adding `feat/source-transports-e2e` to native admits the second authorized feature stack. `main` and core's `refactor/**` remain admitted.

The amendment is limited to existing workflow base routing. It explicitly preserves path filters, dispatch events, permissions, pinned actions, jobs/platforms/timeouts, exact source checkout and credential handling, native guards, source/draft/release selection, and required assertions. The existing real guard still requires explicit disposable GitHub-hosted admission and the actual native target; routing does not grant local execution authority. The Codex/Claude source-selector union is described as preservation of both already reviewed selections, with exact selector tests retained, rather than a new runtime capability.

The proposed validation is sufficient for this bounded change: inspect the final additive filter diff, run existing selector/guard contracts, and observe actual Actions runs on exact published heads. Missing or queued jobs remain pending evidence; configuration, discovery, runtime and release qualification remain separate. Carrying the change by true merges preserves stack ancestry and does not authorize merges into main/release.

No requested plan correction remains. Implementation still requires the second independent plan GO before editing workflows and the stated bounded verification afterward.
