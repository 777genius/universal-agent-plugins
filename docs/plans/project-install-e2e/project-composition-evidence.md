# Cohesive project/source CLI lifecycle evidence

Date: 2026-10-23. Production source/project integration base: true merge
`936523458613cf3ae88a50179192cb2b913b10d6`, containing reviewed source transports
through `8b133b63`, reviewed Gemini project qualification and the shared core
review corrections. Independent review of the cohesive checkpoint follows this
author verification; this document does not substitute for that review.

## Actual composition exercised

The new `cli/cmd/agentplugins/project_lifecycle_*_test.go` contracts construct
the production `newAgentpluginsCLIApp` / `newScopedAgentpluginsCLIApp` runtime.
Every command passes through the public Cobra CLI and its normal scope/root
selection. The fixture wraps ForScope to reapply only host boundaries after
the production composer reconstructs the selected runtime:

- Explicit disposable Gemini detection; the detected ConfigRoot remains the
  same original user profile for both project operations.
- An inert security evaluator with absent signed-feed fixtures explicitly
  disabled. Existing security qualification is separate; these tests do not
  claim a production scanner or signed advisory service was exercised.
- Real source acquisition with a private disposable TLS CA and native Git HTTP
  backend or ZIP download. The Go HTTP client trusts only the fixture's TLS
  server; native Git receives the fixture CA through its existing supported
  authentication/CA boundary. No insecure TLS option is enabled.

The standard package loader, immutable digest store, dependency solver/lock,
scope capability policy, planner, installer facade, filesystem/native kernels,
ownership receipts, data lifecycle and transactions are production components.
The configured inert MCP server is never launched. No Gemini session, model,
native discovery, workspace trust or hosted certification is claimed.

## Journeys and assertions

Both real TLS Git and ZIP variants execute these journeys:

1. Add the same saved package to user scope, project A and project B. All receive
   the native skill and MCP configuration while preserving foreign settings and
   skills. Scope, actual native profile, environment ID and artifact containment
   are checked in persisted state. User remains active under its existing
   behavior; project file-only success remains prepared/installation verified.
2. Compare canonical source identity and package tree across all environments.
   The exact portable manifest/lock pair is identical and contains none of the
   fixture's user, project, cache, repository or native profile paths. All use
   the verified user content store; neither project creates its own store.
3. Allocate and mark persistent data independently in each namespace. Repeat
   strict locked sync in A and require byte-identical receipts afterward.
4. Drift A's managed package, advance the remote Git commit/ZIP bytes, and repair
   using A's exact installation ID. Repair restores the installed pin rather
   than following the moving source. User and B retain their old pin.
5. Reuse the existing dirswap fault seam and durable pending-journal fixture
   under A's actual `.uap` namespace. Inspect observes the journal; public
   `deps recover --yes` consumes it and leaves committed package/native state
   unchanged. Existing rollback semantics preserve unpublished caller staging;
   recovery does not adopt it. A second recovery is harmless.
6. Update A to the advanced source. Its source/binding authority and data
   locators remain stable, its tree/revision and native skill advance, and its
   project readiness remains prepared/installation verified.
7. Remove A's saved alias. Exact managed/native objects disappear, environment
   ownership is released, foreign configuration/skills and persistent data
   remain, and user/B state, native objects, managed bytes and data markers are
   unchanged.
8. Export A's exact portable pair to an explicit exchange directory. Export to
   the same project directory refuses with no writes. Import from the exchange
   directory with `--install --yes` into a fresh project and empty content cache
   restores the same source/tree/pair using locked acquisition.
9. For ZIP, change the endpoint bytes after export and attempt a second cold
   import. Original-byte SHA256 refusal occurs before native mutation or
   manifest/lock/state/content publication; the exchange pair is unchanged.

## Integration correction discovered by the journey

The first exact-ID project repair reproduced a real missed ownership context:
`selectLockedLifecycle` called `savedAliasInstallation` without the selected
environment ID, allowing its compatibility default to select user-default.
The fix threads `app.environmentID()` through that call. The end-to-end test
retains the exact installation-ID selector which previously failed; it does
not replace it with a less discriminating selector. Other production calls to
the variadic ownership helpers were audited for explicit context.

## Author verification

Go1.26.10, `GOMAXPROCS=3`, `-p=2`, disposable roots, independent local TLS:

```sh
go test -p=2 ./cli/cmd/agentplugins -run '^TestProject' -count=1 -timeout=3m -v
```

**PASS 9.815s**, including existing read-only scope composition/init/transfer/
unsupported boundaries and both new composed Git/ZIP lifecycle/transfer tests.
Exact golangci-lint2.13.2 changed-code and whole-file size/architecture checks on
`cmd/agentplugins/...` and `internal/agentpluginscli/...`: **0 issues** each.
No lint baseline, ownership assertion or native hosted-only guard was relaxed.

The existing core-fast Linux, macOS and Windows project-contract job discovers
these TestProject tests. Applicable remote CI and independent cohesive review
remain required before integration approval or any release claim.
