# Cohesive project/source CLI implementation independent review R1

Reviewer: sources_lead, 2026-10-23. Reviewed checkpoint
`c11a071b17419025bf4c6214033c762c28d9036e` read-only. Independent execution used
clean frozen `91475b5120b4a21b92d87cacd15cb91d9ccd1248`, whose only changes above
the reviewed checkpoint are four Gemini compatibility golden scope arrays;
production code and reviewed test files are identical.

## Decision

**R1 GO.** No confirmed implementation or fixture-boundary blocker remains in
the selected source/project composition seam. This proves the implemented UAP
CLI, source acquisition, immutable storage, selected project authority and native
file transactions. It does not certify a native Gemini session, trust, MCP
initialization, model invocation or executable release.

## Independent assessment

- The integration correction is narrow and correct: `selectLockedLifecycle`
  passes the selected App's `environmentID()` to `savedAliasInstallation`.
  Exact installation-ID repair now resolves the project receipt rather than
  entering the compatibility user-default lane. Adjacent ownership helper calls
  retain their explicit selected environment. No portable source or agreement
  identity is changed by scope.
- The fixture constructs the actual `newAgentpluginsCLIApp` runtime and wraps
  its actual ForScope composer only to reapply bounded host detection/security
  and loopback source boundaries. Public Cobra selection, loader, solver/lock,
  planner, installer facade, environment/content stores, native kernel, data
  ownership and transactions remain production implementations. There is no
  alternate runtime or lifecycle adapter hidden in this journey.
- Real TLS Git and ZIP variants cover user, A and B installation of the same
  package, unchanged locked synchronization, exact installed-pin repair after
  upstream advance, moving update, selected removal and foreign preservation.
  Source/binding identity stays stable, package revision advances only on
  update, data remains in the selected namespace, and user/B state, native
  entries, managed package and marked data remain unchanged.
- The same portable manifest/lock pair is byte-identical across scopes and
  excludes local user/project/cache/repository paths. Content uses the shared
  immutable user store; mutable project state/data remain under each .uap.
- Export/import use an explicit exchange pair and a genuinely fresh cache and
  project namespace. Same-pair export refuses without writes. Changed original
  ZIP bytes fail their exact SHA256 before native/state/intent/content
  publication, without changing the exported agreement.
- Recovery reuses the existing dirswap fault seam and durable pending-journal
  fixture. The public selected-project recover command observes and consumes
  that journal, preserves committed native/state bytes and unpublished caller
  staging, and repeats harmlessly. The evidence accurately describes this as
  a transaction/journal fixture rather than a live native process crash.
- Negative scope/root/state-copy and native ownership contracts remain covered
  in the separately reviewed core and adapter packages. This seam adds the
  production generic Git/archive path without inventing a client-by-transport
  cross product or broadening existing user behavior.

## Independent bounded execution

```sh
go test -p=2 -count=1 -timeout=2m -v ./cmd/agentplugins -run '^TestProjectComposed'
```

**PASS 8.872s**: both real TLS Git/ZIP lifecycle variants and both cold portable
agreement transfer variants. Lifecycle subtests passed 3.94s/2.36s; transfer
subtests passed 1.60s/0.95s. No package payload, configured MCP server or model
was executed. No broad suite was repeated because no additional concrete risk
remained after the decisive production composition checks.

No corrective change was requested in this review rotation. The final remote
CI and the independent second composition review remain separate gates.
