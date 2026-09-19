# ADR 0007: Client Adapter Contract and Registry

## Status

Accepted

## Context

The agentplugins installer supports eleven clients whose detection, planning,
staging, activation, and identity inspection used to live as `switch` statements
in generic packages (`providers`, `planner`, `usecase`, CLI). That mixed two
reasons to change one file: a new product, and a change to a shared invariant.

`domain` already owns identity (`ClientDefinitions`, `ClientTraits`). The
refactoring plan required a second, injectable surface so generic dispatchers
can ask a client for a capability without naming the client, and so a binary can
be composed with a subset of adapters. Capability lookup is a runtime type
assertion (`clients.As[T]`): a renamed method makes an adapter silently stop
implementing an interface. Compile-time assertions and `contracttest` are the
compensation, not a substitute for a module boundary.

## Decision

Client-specific behavior lives in `install/integrationctl/agentplugins/clients/<id>`.
The composition root (`cli/plugin-kit-ai/cmd/agentplugins`) is the only
production package that imports `clients/all`. Generic packages receive a
`clients.Registry` and fail closed when it is nil.

Invariants:

- `domain` remains the identity authority. `NewRegistry` rejects an id
  `domain.IsSupportedClient` does not know, so an adapter cannot invent a
  twelfth product.
- A nil registry is an error, never a silent fallback to every shipped adapter.
- Detection is request-scoped: `domain.PlanRequest.Detected` is the only map
  `Planner.Plan` reads. The planner does not keep a `Detected` field.
- Optional capabilities are segregated interfaces (`Lifecycle`, `HostDetector`,
  `Projector`, and the rest). Dispatchers look them up with `clients.As[T]`.
- Every shipped adapter asserts the interfaces its traits declare, and
  `clients/contracttest` runs against `clients/all`.
- Adding a production client is three edits plus the harness: a row in
  `domain.ClientDefinitions`, a package `clients/<id>`, a `New()` line in
  `clients/all`, then `contracttest`. Generic packages (`planner`, `providers`,
  `usecase`, `adapters/clientdetect`) are not in that list. The test-only
  `clients/internal/exampleclient` registers through `NewRegistry` without those
  packages importing it.

## Consequences

- A new client does not require edits in generic dispatchers. Archtest enforces
  that: the ClientID-selector budget is empty outside `domain/clients.go`,
  `clients/<id>`, and `clients/all`.
- `As[T]` remains a runtime seam. A typo in a method name is a missed
  capability, not a compile error; the assertions and the harness are what
  notice.
- The public planner facade (`Capabilities`, `Compatibility`,
  `ApplyInstallIntent`, `ChatGPTAppBindingAction`, `DetectedPhysicalClient`,
  `KiroPrepareAction`) stays for consumers outside the core. That is not the
  same as constructing `planner.Planner{}` from the CLI.
- Extracting `domain`+`ports`+`clients` into a still-smaller Go module remains a
  later step: those packages still share a module with nativeconfig and the
  generic dispatchers because client adapters import `pathpolicy`/`atomicfile`/`filetree`
  from the parent. The install core as a whole is already a nested module
  (`install/integrationctl/agentplugins`).

## Non-Goals

- Separate Go modules for `domain`, `ports`, or `clients`.
- A twelfth supported product id for the example adapter.
- Replacing `As[T]` with codegen or a sealed interface sum.
- Moving `transaction`, `directoryv1`, `statev2`, or the large CLI files
  (`source.go`, `add_multi.go`, `lifecycle.go`, `read.go`, `search.go`) out of
  the size baseline. Those remain a later task.

## Rejected Alternatives

- Keep client switches in `providers`/`planner` and extract helpers per client.
  The generic packages would still change for every new product.

- Resolve a nil registry to `clients/all.Default()`. That would compile every
  adapter into any binary that imports a dispatcher and would hide a missing
  composition-root argument.

- Duplicate `Command`/`CommandResult` into `domain` so `ports` imports only
  `domain`. Those types are data, not adapters; two sources of truth would cost
  more than the formal purity.

- Introduce `ports.NativeConfigKernel` now so `clients` does not import
  `adapters/nativeconfig`. There is no second implementation and no module split
  yet; the port would be an unused abstraction.

The two accepted exceptions from the layering rules remain:

1. `agentplugins/ports` may import `install/integrationctl/ports` for
   `Command`/`CommandResult`.
2. `agentplugins/clients` may import `adapters/nativeconfig`, which pulls
   `hujson` and `atomicfile`. Lifting the contract into its own module would
   have to take that kernel along or hide it behind a port.
