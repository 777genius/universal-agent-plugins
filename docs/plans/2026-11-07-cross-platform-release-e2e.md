# Cross-platform release E2E coverage expansion (2026-11-07)

## Goal and evidence boundaries

Increase confidence in the **published** `universal-agent-plugins` installer and in the **current source** without confusing their evidence or requiring paid model/API credentials. Reuse existing harnesses and keep native security/ownership checks fail-closed.

### Scope and tiering

| Gate | Platforms | Real verification | Cadence |
| --- | --- | --- | --- |
| Native installer | Linux x64/ARM64, macOS Intel/ARM64, Windows x64/ARM64 | Latest public stable release binary, version, checksummed native launcher | Relevant PR, scheduled, manual |
| Published npm client smoke | Windows x64, macOS ARM64 | Existing frozen Codex, Claude, OpenCode, Gemini and Copilot CLI × plugin/Skill, 2 sources, install/list/repeat/remove and immutable native proofs | Relevant PR |
| Published npm deeper catalog | Windows x64, macOS ARM64 | Larger frozen source set including Marketing Kit, native registration, repeat-add/idempotence, scope-specific lock/source bytes, safe removal and foreign sentinels; extend to safely qualified Cline, Kimi, Qoder, Pi (Skills only) | Weekly or manual, not each PR |
| Published npm Linux catalog | Linux x64 | Existing 17 real native lanes / 348 source cases; real native clients, explicit expected negatives (not PASS) | Existing PR and manual; weekly if feasible |
| Source-built package manager | Linux x64/ARM64, macOS Intel/ARM64, Windows x64/ARM64 | Existing real ZIP/Registry graph, Project/User isolation, update/repair, missing-cache cold restore, rollback, foreign-state and lock ownership | Relevant PR on existing four, weekly six-platform |
| Real native runtime/MCP | Supported native macOS, Windows, Linux clients | Existing Code OSS/Codex/Claude/OpenCode native-client test harness, loopback MCP initialize/list/call where attested; no paid provider API | Existing weekly native-client runtime workflow |

**Negative evidence:** no CLI/bootstrap, missing supported native client, external uninstall requirement, Prepared-only, unsupported package/scope or host containment refusal must never count as a native PASS. A green workflow requires all advertised cases and enough source-bound artifacts. Test failures remain visible and are fixed by root cause, not by skipping them.

### Actual event-based coverage tiers

- **On relevant PRs:** draft Windows/macOS ARM64 matrices use the five established agents × 2 frozen sources; Ready-for-Review expands to nine pinned agents (Pi Skills only) × 2 sources. Source-built Project/User update/repair catalog runs on four OSs when ready; native installer verifies all six.
- **On merge to `main` of these exact workflows/fixtures:** Windows/macOS ARM64 expand to nine real agents (Pi Skills only), **all 30 frozen sources** in each supported lane. The package-manager graph/update/repair catalog covers all six OSs; Linux npm qualification retains its own 17-source-backed lanes.
- **Weekly:** Windows/macOS ARM64 test 20 signed Directory sources per lane (including Marketing Kit and negative target-policy cases), with 9 clients; full source-built graph tests run on all six OSs. Linux's 17 published npm lanes retain their original 30/12/15 pinned source counts, and existing native client local-MCP runtime remains weekly.
- **Manual:** `workflow_dispatch` for Windows/macOS accepts 2, 20 or 30 real sources and a validated exact stable release version; six-platform native installer can also validate any exact prior public stable tag. The default uses the latest public stable.
- **After npm publication:** the five released-platform workflows are triggered by the existing trusted `Agentplugins NPM Publish` completion event. An unsuccessful publisher prevents the resolver from starting; GitHub latest release and exact npm package version must agree.

**Platform differences are explicit:** UAP native-install and source-built package-manager tests run on six architectures; real-agent published installation is on Linux x64, Windows x64 and macOS ARM64. macOS Intel, Linux ARM64 and Windows ARM64 are not described as real-agent MCP-call qualified. Unsupported generic Pi plugin mode is excluded, not counted PASS.

### Implementation limits
- Never mutate an ambient user profile. Test HOME, XDG, APPDATA and CLI data roots remain disposable; inherited credentials filtered.
- No weakening OpenCode native executable identity; use pinned SHA-512 native executable, not npm .cmd/sh wrappers.
- No duplicate work on the source-built package manager/update/repair or local MCP; activate existing paths rather than inventing parallel engines.
- GitHub hosted ARM runners can have vendor-specific unsupported binaries; distinguish generic installer/package-manager proof from client-qualified native activation.
- Keep PR smoke fast (2 real source cases); reserve broad cases for manual/weekly workflows. Preserve raw fixture checksum and exact build/release SHA.
- Use one reusable release resolver to bind the public stable GitHub tag to exactly the same published npm version; pin real native client versions separately and attest their versions in each JSON.
- Avoid merges on queued, skipped, partial or unqualified platform evidence. Review including source-level test assertions and independent reviewer observations; close only applicable comments.
- No new npm release, no undocumented platform/client support, no external OAuth/model execution claim.

### Validation / exit gates

1. PR changes configure exactly six platform bootstrap targets and six scheduled deep package-manager targets.
2. Native Windows/macOS PR smoke remains green (known baseline 40/40 on 0.1.101); a scheduled broad run records its actual exact version and all attempted source cases.
3. The combined build/test CI and review are successful on the PR's last commit, with any platform-specific failures diagnosed and retested.
4. Merge with exact head SHA and verify `main` and post-merge jobs. A scheduled run is not retroactively a PR PASS; report future results accurately.

The scope deliberately avoids a new E2E framework, production CLI policy changes, authenticated MCP calls, and a mandatory all-clients × all-OS combinatorial PR matrix.
