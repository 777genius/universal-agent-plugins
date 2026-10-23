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
