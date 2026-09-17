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
Dependencies point inward.

| Layer | Package | May import | Enforced today |
|-------|---------|------------|----------------|
| Domain | `agentplugins/domain` | stdlib only | yes, `domain-stdlib-only` |
| Ports | `agentplugins/ports` | stdlib, `domain`, `install/integrationctl/ports` (see below) | yes, `ports-only-domain` |
| Use cases | `agentplugins/usecase` | stdlib, `domain`, `ports`, `transaction`, `pathcontract` | partly, `usecase-through-ports` |
| Adapters | `agentplugins/{adapters,providers,planner}` | the layers above | no rule yet |
| CLI | `agentpluginscli` | the public facades of the layers above | no rule yet |
| Composition root | `cmd/agentplugins` | everything, and nothing imports it | no rule yet |

The "Enforced today" column is deliberate: the middle column is the target, and
only the first three rows are currently checked by `depguard` in `.golangci.yml`.
`usecase-through-ports` is partial - it forbids `adapters`, `providers` and
`clients`, but `usecase` still legitimately imports `planner`, `pathpolicy` and
`install/integrationctl/ports`. Those imports are removed, and the deny list
extended, when the ports and DIP work lands. The adapter, CLI and composition
root rows have no rule at all yet.

### Accepted exceptions

**`ports` may import `install/integrationctl/ports`.** That package holds
`Command` and `CommandResult`: plain data types with no behavior and no I/O, and
`providers.CommandRunner` and `treeCommandRunner` are already defined on top of
them. Their runtime implementation, `adapters/process.OS`, stays an adapter and
is not covered by the exception. Duplicating the two types into `domain` would
create a second source of truth and force a conversion on every call, which costs
more than the formal purity is worth.

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
grow. Its limitation is stated in the package: it reads selector expressions, so
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
