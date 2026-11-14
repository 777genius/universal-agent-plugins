# Client adapter contract tests

`contracttest` is the executable contract every client adapter has to satisfy,
in-tree and out-of-tree. Capability lookup is `clients.As[T]`, a runtime type
assertion: a renamed method makes an adapter silently stop implementing an
interface. These tests are what notice.

## What to run

From `install/integrationctl/agentplugins`:

```text
go test ./clients/contracttest ./clients/all
```

`clients/all` runs the harness against every shipped adapter. `contracttest`
itself also has negative tests that a bad adapter fails.

## Adding a client

Three production edits, then this harness:

1. A row in `domain.ClientDefinitions` (identity and `ClientTraits`).
2. A package `clients/<id>` with compile-time assertions for every capability
   the traits declare (`var _ clients.Lifecycle = (*Adapter)(nil)`, and so on).
3. A `New()` line in `clients/all`.

Generic packages (`planner`, `providers`, `usecase`, `adapters/clientdetect`)
are not in that list. `clients/internal/exampleclient` is a test-only adapter
that registers through `clients.NewRegistry` without those packages importing
it.

## What the harness checks

- `RunAdapter`: stable id that `domain` actually defines.
- `RunHostDetector`: surface ids are unique, repeated observations agree, and
  the adapter does not invent evidence the host never gave it.
- Trait parity: a declared `ClientTraits` capability has a matching interface
  implementation on the adapter in `clients/all`.
- Lifecycle, identity, projector, and plan-refiner suites cover the remaining
  segregated interfaces as those capabilities moved into adapters.
