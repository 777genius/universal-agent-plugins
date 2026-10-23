# Cohesive project/source CLI implementation independent review R2

Reviewer: core_clients, 2026-10-23. Reviewed composed implementation
`c11a071b17419025bf4c6214033c762c28d9036e` at frozen integration
`c96fd09604adc7bfeaad32f2b140e27ffb5d8342`, work/uap-project. The reviewed
production correction and all five project_lifecycle fixture files are
byte-identical between those commits. Later integration adds independently
reviewed core/helper work; this review does not repeat their separate reviews.
No source, test or production file was edited by this reviewer.

## Decision: R2 GO

No remaining confirmed blocker was found in the composed generic source,
scope/ownership, recovery or portable transfer seam. This approves the bounded
implemented CLI/filesystem journey; it supplies no native Gemini session, MCP
runtime, trust, model execution, cross-platform runtime or release proof.

## Production correction

selectLockedLifecycle now supplies app.environmentID() to savedAliasInstallation
instead of taking its compatibility user-default argument. This retains the
selected project ownership receipt when an exact installation ID resolves the
saved alias. Both alias and exact-ID lifecycle paths still validate the locked
selection, source/package identity, binding and selected environment. Scope does
not enter the portable source or manifest/lock identities.

## Composition and source assessment

- Tests construct the production newAgentpluginsCLIApp and retain its actual
  ForScope composer. Every operation uses public Cobra scope/root selection.
  The wrappers reapply only disposable detection, explicit inert security
  evaluation and private loopback source transport. Installer, solver,
  manifest/lock, immutable store, planner, native kernels and ownership/data
  transactions remain production implementations.
- The Git variant uses real native Git HTTP-backend service through TLS;
  native Git receives the fixture's private CA through its supported boundary.
  ZIP uses the actual HTTP archive acquisition path and original-byte digest.
  Neither variant bypasses TLS validation or substitutes a custom package
  acquisition result. Security fixture boundaries are described honestly.
- User/A/B additions use the same source/tree identity and exact portable pair.
  Persisted scope, environment ID, native profile and artifact/data locators
  are checked against each selected namespace. Content uses the shared
  immutable user store; mutable project receipt/data state stays in `.uap`.
- Locked no-op sync requires byte-stable state. After selected managed-package
  drift and upstream Git/ZIP advance, exact-ID repair restores the installed
  pin. Moving update alone advances selected A while retaining source/binding
  and persistent-data authority. Removal consumes only owned native/package
  objects and environment ownership; foreign settings/skills and data survive.
  User and project B retain state, native/package bytes and data markers.
- Portability assertions inspect manifest/lock for every fixture machine/root
  spelling. Export preserves the exact pair; same-directory export refuses
  without effects. Cold import starts with absent cache and project namespace,
  then reacquires the locked source/tree. Altered original ZIP bytes fail SHA256
  before manifest/lock/state/content or native publication, preserving the
  exported agreement and foreign native configuration.
- Recovery reuses dirswap's existing fault seam and receipt format. The fixture
  intentionally persists a pending intent for the interrupted transaction;
  actual selected-project inspection/recover observes and consumes it, keeps
  committed native/state bytes intact and preserves unpublished caller staging.
  Repeat recovery is harmless. This is accurately bounded transaction/journal
  evidence; it is not claimed as an abrupt real native-client process crash.

## Independent bounded execution

Go 1.26.10, GOMAXPROCS=3, `-p=2 -count=1 -timeout=3m`, frozen c96fd096:

```sh
go test -p=2 ./cli/cmd/agentplugins \
  -run '^TestProjectComposed|^TestProjectCLI|^TestProjectRuntime' \
  -count=1 -timeout=3m
```

**PASS 9.098s.** This includes real TLS Git and ZIP full lifecycle, both cold
portable-agreement restores, ZIP original-byte refusal, selected scope/runtime
composition and CLI transfer/admission boundaries. No native Gemini process,
authored MCP command, package payload, model, trust or login operation ran.

The original R1, author exact lint gates, separately reviewed source transports
and project core remain supporting records. Actual hosted platform and native
client outcomes must continue to be reported separately from this passing
production CLI/filesystem contract.
