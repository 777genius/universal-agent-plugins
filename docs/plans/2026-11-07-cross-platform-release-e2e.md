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

### Implementation limits
- Never mutate an ambient user profile. Test HOME, XDG, APPDATA and CLI data roots remain disposable; inherited credentials filtered.
- No weakening OpenCode native executable identity; use pinned SHA-512 native executable, not npm .cmd/sh wrappers.
- No duplicate work on the source-built package manager/update/repair or local MCP; activate existing paths rather than inventing parallel engines.
- GitHub hosted ARM runners can have vendor-specific unsupported binaries; distinguish generic installer/package-manager proof from client-qualified native activation.
- Keep PR smoke fast (2 real source cases); reserve broad cases for manual/weekly workflows. Preserve raw fixture checksum and exact build/release SHA.
- Existing published Windows/macOS workflow references `0.1.101`; upgrade the weekly release acquisition to the current stable tag with strict release/npm agreement rather than silently calling historical 0.1.101 a new release.
- Avoid merges on queued, skipped, partial or unqualified platform evidence. Review including source-level test assertions and independent reviewer observations; close only applicable comments.
- No new npm release, no undocumented platform/client support, no external OAuth/model execution claim.

### Validation / exit gates

1. PR changes configure exactly six platform bootstrap targets and six scheduled deep package-manager targets.
2. Native Windows/macOS PR smoke remains green (known baseline 40/40 on 0.1.101); a scheduled broad run records its actual exact version and all attempted source cases.
3. The combined build/test CI and review are successful on the PR's last commit, with any platform-specific failures diagnosed and retested.
4. Merge with exact head SHA and verify `main` and post-merge jobs. A scheduled run is not retroactively a PR PASS; report future results accurately.

The scope deliberately avoids a new E2E framework, production CLI policy changes, authenticated MCP calls, and a mandatory all-clients × all-OS combinatorial PR matrix.
