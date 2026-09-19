# Architecture Notes

This document describes the current shipped monorepo architecture.

Current public contract docs live in:

- [SUPPORT.md](./SUPPORT.md)
- [STATUS.md](./STATUS.md)
- [generated/support_matrix.md](./generated/support_matrix.md)

Historical maintainer references live in:

- [FOUNDATION_REWRITE_VNEXT.md](./FOUNDATION_REWRITE_VNEXT.md)
- [adr/README.md](./adr/README.md)

## Composition Roots

| Layer | Location | Role |
|-------|----------|------|
| SDK runtime | `sdk/plugin_kit_ai.go` | Platform-neutral composition root that wires the generic engine, generated descriptor lookup, middleware, and platform registrars |
| SDK generator | `cmd/plugin-kit-ai-gen/main.go` | Generates descriptor-derived runtime, scaffold, validate, and docs artifacts |
| Plugin install library | `install/plugininstall/install.go` | Public install facade that wires use case and concrete adapters |
| CLI | `cli/plugin-kit-ai/cmd/plugin-kit-ai/main.go` | Process entrypoint; commands parse flags and call `internal/app`, `internal/scaffold`, and `internal/validate` |

Rule: the CLI must not construct `plugininstall` adapters directly. It uses the `plugininstall` facade.

## Agent Plugins Core: Layering, Import Rules, Size Limits

The `agentplugins` install core spans three packages: `install/integrationctl/agentplugins/...`,
`cli/plugin-kit-ai/internal/agentpluginscli/...` and `cli/plugin-kit-ai/cmd/agentplugins`.
`install/integrationctl/agentplugins` is its own Go module
(`github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins`), listed in
`go.work`. Import paths are unchanged. The parent `install/integrationctl` module
keeps shared adapters (`pathpolicy`, `atomicfile`, `filetree`, `process`) and
legacy `ports`; the nested module depends on those, never the reverse. Dependencies
point inward.

| Layer | Package | May import | Enforced today |
|-------|---------|------------|----------------|
| Domain | `agentplugins/domain` | stdlib only | yes, `domain-stdlib-only` |
| Ports | `agentplugins/ports` | stdlib, `domain`, `install/integrationctl/ports` (see below) | yes, `ports-only-domain` |
| Use cases | `agentplugins/usecase` | stdlib, `domain`, `ports`, `transaction`, `pathcontract`, `install/integrationctl/ports` for the legacy lock (see below) | yes, `usecase-through-ports` |
| Client contract | `agentplugins/clients` (+ `clients/shared`) | stdlib, `domain`, `ports`, `adapters/nativeconfig` (see below) | yes, `clients-no-upward`, `clients-no-concrete-clients` |
| Client adapters | `agentplugins/clients/<id>` | the client contract, `clients/shared`, `domain`, `ports` | yes, `clients-no-upward`, `clients-no-concrete-clients` |
| Adapters | `agentplugins/{adapters,providers,planner}` | the layers above, never `clients/all` | partly, `libraries-take-an-injected-registry` |
| Installer facade | `agentplugins/installer` | public DTOs plus the layers above through one composition boundary; injected registry only | yes, `libraries-take-an-injected-registry` and the client-id ratchet |
| CLI | `agentpluginscli` | the public facades of the layers above, never providers or pathpolicy | yes, `cli-no-core-internals` |
| Composition root | `cmd/agentplugins`, `cmd/plugin-kit-ai` | everything, and nothing imports them | yes: they are the production importers of `clients/all` |

The "Enforced today" column is deliberate: the middle column is the target, and
the left-hand rules are checked by `depguard` in `.golangci.yml`. Domain, ports,
the use cases, the client contract, the client adapters and the CLI are covered
in full. The adapter row is covered in part: `libraries-take-an-injected-registry`
forbids `clients/all` there, but nothing yet stops an adapter from importing
another one. The composition root is allowed to import everything because it is
the wiring, not a layer.
`usecase-through-ports` forbids `adapters` (both the `agentplugins` ones and
`install/integrationctl/adapters`), `providers`, `planner` and `clients`; the use
case reaches path containment through `ports.PathPolicy` and planning through
`ports.DeliveryPlanner`. It stays a deny list on purpose: adding an `allow` key
would turn the rule into a whitelist and reject every import not named in it,
including the standard library.

`agentplugins/clients` is the extension point: one adapter per supported client,
resolved through a `Registry` the composition root injects. `clients-no-upward`
stops an adapter from importing `providers`, `planner`, `usecase` or
`adapters/clientdetect`, and `clients-no-concrete-clients` stops the contract,
its shared helpers and any client package from importing another client package
or the assembled `clients/all` registry. A nil `Registry` is an error, never a
silent fallback to "every client": resolving it to a default would compile every
adapter into any binary that imports a generic package and would put the registry
outside the composition root's control. `libraries-take-an-injected-registry`
holds the other side of that line: `providers`, `planner` and
`adapters/clientdetect` may not import `clients/all` outside their tests, so the
assembled registry reaches them only as an argument. `cmd/agentplugins` and
`cmd/plugin-kit-ai` are the production places that build it. The installer CLI
receives `Planner`, `Targets` and `Registry` from that root; it does not
construct `planner.Planner{}`. Detection is request-scoped:
`domain.PlanRequest.Detected` is the map `Planner.Plan` reads.

`cli-no-core-internals` keeps `agentpluginscli` off `providers` and
`pathpolicy`. The CLI may still import the thin public planner facade
(`Capabilities`, `ApplyInstallIntent`, and the rest) because those names are a
stable API for authoring, not a second composition root.

`agentplugins/installer` is the public embedding boundary. It owns concrete
composition but receives the supported client registry from the executable;
it does not import `clients/all`, branch on client constants, or export its raw
Store/Kernel. The production CLI gives this facade the qualified Claude/Codex
subset while its existing discovery and catalog paths keep the full registry.

Detection is the first capability to live behind the contract: each
`clients/<id>` implements `HostDetector` and reports the surfaces it observed
through a `clients.Host`, while `adapters/clientdetect` keeps the generic half -
detection status, display name from `domain.ClientDefinitions`, the version probe
and the stable ordering. The surface constructors belong to the contract rather
than to each client because the evidence strings they produce are a cross-client
output contract.

`clients/contracttest` runs every adapter against a host that reports nothing
installed and answers the same way every time. It checks that surface ids are
unique, that repeated observations agree down to the order and the number of
probe calls, and that nothing comes back detected - an adapter that stats a real
path behind the host's back reports evidence the host never gave it. That is a
necessary condition rather than a sandbox: an ambient read whose result never
reaches the `Detection` stays invisible to it.

### Accepted exceptions

**`ports` may import `install/integrationctl/ports`.** That package holds
`Command` and `CommandResult`: plain data types with no behavior and no I/O, and
`ports.CommandRunner`, `ports.TreeCommandRunner`, `ports.DuplexCommandRunner` and
`ports.DuplexCapabilityRunner` are defined on top of them. Their runtime
implementation, `adapters/process.OS`, stays an adapter and is not covered by the
exception. Duplicating the two types into `domain` would create a second source of
truth and force a conversion on every call, which costs more than the formal
purity is worth.

**`usecase.Service.LegacyLock` is typed `install/integrationctl/ports.LockManager`.** This
one is not the data-only exception above: `LockManager` is a behavioural
interface, so the use case does name a package outside its layer. It is the
remaining edge of the pre-refactor installer that still owns the lock, and it is
listed here rather than quietly excluded, because the deny list permits it only
by not mentioning it.

**`agentplugins/clients` is close to self-contained, but not a leaf.**
`clients.Env` carries a `nativeconfig.Kernel`, so the contract imports
`agentplugins/adapters/nativeconfig`, which in turn pulls
`github.com/tailscale/hujson` and `install/integrationctl/adapters/atomicfile`.
That package is a generic content-addressed kernel for native client configs
with no dependency on `providers`, which is why it moved under `adapters` rather
than being wrapped. The honest statement is therefore "close to extractable, with
one known exception", not "leaf package ready to move out": lifting
`clients`+`domain`+`ports` into a separate module would first have to take
`nativeconfig` along or hide it behind a narrow `ports.NativeConfigKernel`.
Introducing that port now would be an abstraction with no consumer, so the
constraint is recorded here instead of discovered later. See the Part 2 section
of `docs/plans/installer-core-clean-architecture-plan.md`.

**`ports.PathPolicy` has exactly one implementation.** Inverting path
containment into an interface makes a permissive stand-in possible for the first
time, and a stand-in that accepts a symlinked or escaping path removes the last
check before a destructive operation. `adapters/pathpolicy.Policy` is the only
implementation; `internal/archtest` fails the build on a second type that carries
the whole method set, in test files too, and any candidate has to pass
`ports/contracttest.RunPathPolicy`. `usecase.Service.Paths` and `planner.Planner.Paths`
are required with no default, so a caller that forgets to wire one fails fast
instead of running with weaker rules than it thinks.

### Size limits

New code is held to 500 lines per file, 60 lines and 40 statements per function,
cyclomatic complexity 20, cognitive complexity 25. Files that already exceeded
these limits are listed in the shrink-only legacy baseline in `.golangci.yml`.
See [CONTRIBUTING.md](../CONTRIBUTING.md#lint-gate-and-size-limits) for the
commands and the baseline policy.

### Executable guardrails

The linter is the first layer; `go test` carries two more, so the rules still
hold for anyone running a plain `go test ./...` without golangci-lint.

`install/integrationctl/agentplugins/internal/archtest` restates the `depguard`
import rules as a test and measures a ratchet: how many times each core package
names a client identity from `domain`. The committed baseline lives in
`internal/archtest/testdata/client_id_budget.json`; a package may shrink, never
grow. Production ClientID selectors are allowed only in `domain/clients.go`,
`clients/<id>`, and `clients/all` — the committed packages object is empty. Its limitation is stated in the package: it reads selector expressions, so
a string literal client id or a comparison on `BackendFamily` slips past it. It
detects regressions, it does not prove their absence.

Golden files under `planner/testdata/golden`, `adapters/clientdetect/testdata/golden`,
`providers/testdata/golden` and `agentpluginscli/testdata/golden` record what the
core produces today for all eleven clients, including the operational fields the
public JSON tags hide. Refactor parts must leave them byte-identical; a
deliberate behavior change rewrites them with `UPDATE_GOLDEN=1 go test ./...` in
the same commit that explains why.

## SDK Runtime

- `sdk` exposes only shared runtime composition.
- Public platform APIs are peer packages:
  - `sdk/claude`
  - `sdk/codex`
  - `sdk/gemini`
- Core runtime lives under `sdk/internal/runtime`.
- Descriptor definitions live under `sdk/internal/descriptors/defs`.
- Generated runtime registries and resolvers live under `sdk/internal/descriptors/gen`.
- Platform wire codecs live under:
  - `sdk/internal/platforms/claude`
  - `sdk/internal/platforms/codex`
  - `sdk/internal/platforms/gemini`

Current runtime carriers:

- Claude events use `stdin_json`
- Codex `Notify` uses `argv_json`
- Gemini runtime hooks use `stdin_json`

## CLI Application Layer

`cli/plugin-kit-ai/internal/app` keeps Cobra out of install/init application logic:

- `InstallRunner` delegates to `plugininstall.Install`
- `InitRunner` resolves generated scaffold definitions and delegates generating to `scaffold`

`cli/plugin-kit-ai/internal/validate` enforces generated platform rules for scaffolded projects.

## Generated Sources

`go run ./cmd/plugin-kit-ai-gen` is the canonical generation entrypoint.

Generated artifacts include:

- descriptor registry and invocation resolvers
- public platform registrars
- scaffold platform definitions
- validation rules
- support contract documentation

Generator drift is enforced by tests in `sdk/generator`.

## Exit Codes

- `plugin-kit-ai install`: domain errors map through `plugininstall.ExitCodeFromErr` and CLI `exitx`
- `plugin-kit-ai init`: failures surface as CLI errors and exit code `1`
- `plugin-kit-ai validate`: invalid scaffold or buildability failures exit non-zero

## Tests

- `sdk/...`: runtime, descriptors, generator drift, examples
- `cli/plugin-kit-ai/...`: app and scaffold coverage
- `repotests/...`: generated project integration and installer integration

Note: installer integration tests create a local `httptest` server and require loopback bind permissions.
