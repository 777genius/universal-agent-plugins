# Project client stack integration evidence

Recorded 2026-10-23. These records distinguish the production CLI/filesystem
journey from genuine native-client observations and release qualification.

## Reviewed composition checkpoint

`daa705adbfb1681d52f8d6259db7cdf716b75195` integrates the separately reviewed
Gemini, Codex, Cursor, Claude and OpenCode project lanes plus ChatGPT's explicit
unsupported-project admission boundary. It includes the reviewed source
transports, shared component transaction helper, project namespace guard and
Windows identity correction. Both independent integration reviews are GO.

Merge resolutions preserve both sides of the two functional seams:

- Dependency locking keeps project-target preflight before the process lock,
  followed by recovery inspection and authoritative validation under the lock.
  Lock publication retains the manifest recheck and immutable-store population
  before writing the portable lock. Neither client admission nor source cache
  restoration is dropped by the merge.
- Native source selection is the union of the reviewed Codex project component
  and Claude project plugin tests. Both distinct exact-selection regressions,
  all existing required user tests, disposable-host guards and release/draft
  restrictions remain intact.

The independently compared client adapter directories and client-named tests
match the reviewed leaf commits. No tracked implementation or test was deleted
by this integration. Shared-helper review records retain their later whole-file
lint follow-up rather than replacing it with an earlier report.

## Bounded combined execution

Go 1.26.10, GOMAXPROCS=3, `-p=2 -count=1 -timeout=3m`, exact integrated
production code at the checkpoint above:

```sh
go test -p=2 ./cli/cmd/agentplugins ./cli/internal/agentpluginscli \
  ./install/integrationctl/agentplugins/domain \
  ./install/integrationctl/agentplugins/planner \
  -run 'Project|CursorProject|Compatibility' -count=1 -timeout=3m
```

| Package | Observed result |
| --- | --- |
| Public CLI composition and project lifecycle selection | PASS, 19.434s |
| Internal CLI scope and admission selection | PASS, 0.124s |
| Domain target and compatibility selection | PASS, 0.005s |
| Planner compatibility selection | PASS, 0.021s |

The retained native matrix selector/guard suite passes all 18 tests. The
independent integration reviewer also ran all 42 released-evidence verifier
tests successfully. These inert tests do not execute a native client or qualify
MCP, trust, authentication, model behavior or an installer release.

## Actual stack CI routing

The native workflow previously accepted only PRs targeting main. Core CI also
missed temporary client/checkpoint bases. The amendment and its two independent
plan reviews precede the two-line routing implementation `7a351367`; both final
implementation reviews are GO. Existing jobs, paths, platforms, action pins,
permissions, guard code and failure semantics are unchanged.

After publication, GitHub created Native Client Runtime runs
[40011489662](https://github.com/777genius/universal-agent-plugins/actions/runs/40011489662)
for the Codex client head `0d1319f947c7f4403691b3a60b1193baa0074ee9` and
[40011624952](https://github.com/777genius/universal-agent-plugins/actions/runs/40011624952)
for the Claude client head `2d8102782861ca3005ca4b8c5572ebde26b91d08`.
Run creation confirms routing, not a passing outcome. Native project evidence
remains pending until the exact required tests complete on genuine disposable
GitHub-hosted machines. The final PR status and artifacts are authoritative for
those outcomes.

## Remaining work at this checkpoint

Other client leaves and their public lifecycle coverage are in progress. This
document is an integration checkpoint, not an all-client completion or release
claim. Client PRs merge only into the project feature; the two deliverable
feature PRs remain unmerged into main until separately authorized.

## Reviewed eight-client integration

Production checkpoint `e18b94a68ea280414e199a286142dd335fdfdf7d` adds the
independently reviewed Kiro and Copilot project adapters to the earlier
composition. Both final leaf reviews and both root merge reviews are GO.
The merge preserves the union of Codex/Copilot native codecs, every earlier
project capability, and all non-scope compatibility golden fields. Client
implementation and E2E files match their reviewed leaves byte for byte.

A bounded combined run on that exact production checkpoint used Go 1.26.10,
GOMAXPROCS=3 and `-p=2 -count=1 -timeout=3m` with
`-run 'Test(Project|Compatibility|Copilot|Codex)'`. Observed results:
nativeconfig **0.039s**, domain **0.003s**, planner **0.018s**, production public
CLI project journeys **29.299s**, all PASS. This is configuration/installer
execution, not native client discovery, trust or MCP runtime qualification.

The source-only project guide and updated dependency/README instructions at
`bbc0473a` plus `d194b7f0` also received two independent GO reviews. The latter
clarifies that only local directories/local archives use imperative source
intent; eligible remote HTTPS archives retain saved dependency support.

Gemini leaf commit `938bcb8fa00da3fa402430de058d27df472a3bab` now has actual
project-contracts PASS on Ubuntu 24.04, macOS 15 and Windows 2025 in Core run
[40008798665](https://github.com/777genius/universal-agent-plugins/actions/runs/40008798665).
Windows job 120603490211 passed the corrected namespace identity and complete
selected scoped lifecycle checks. This does not relabel the still-pending
other-client/native jobs or claim all-client completion.
