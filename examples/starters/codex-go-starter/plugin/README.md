# codex-go-starter

Copy-first starter for Go teams that want the stable `codex-runtime` Notify path with the SDK-first production lane.

## Who It Is For

- Teams starting a new Codex plugin in Go
- Users who want the strongest typed-handler path from day one
- Users who want a copy-first entrance without giving up the production-default Go lane

## Prerequisites

- `plugin-kit-ai` installed
- Go `1.22+`
- Codex local runtime lane
- access to `github.com/777genius/plugin-kit-ai/sdk@v1.1.0`

## Runtime

- Platform: `codex-runtime`
- Runtime: `go`
- Entrypoint: `./bin/codex-go-starter`
- Execution mode: `launcher`
- Status: strongest production-ready path, starter-layer copy-first entrance

## First Run

```bash
go test ./...
go build -o bin/codex-go-starter ./cmd/codex-go-starter
plugin-kit-ai validate . --platform codex-runtime --strict
```

This starter keeps one canonical Go story:

- SDK-first handlers
- `go test ./...`
- `go build -o bin/codex-go-starter ./cmd/codex-go-starter`

Unlike the interpreted Python/Node starters, this lane does not depend on the launcher bootstrap command or the Python/Node bundle handoff workflow.
The public Go SDK now resolves as a normal module release through `github.com/777genius/plugin-kit-ai/sdk@v1.1.0`.

## Local Smoke

```bash
./bin/codex-go-starter notify '{"client":"codex-tui"}'
```

## Stable Default

- `Notify`

Treat `plugin-kit-ai validate --strict` as the CI-grade readiness gate for the authored launcher and target metadata.

## Target Files

- `plugin/launcher.yaml`: runtime and entrypoint for local Notify integration
- `plugin/targets/codex-runtime/package.yaml`: authored Codex runtime metadata
- `.codex/config.toml`: generated managed Codex config

Keep stdout reserved for Codex responses and write diagnostics to stderr only.

## Ship It

```bash
go test ./...
go build -o bin/codex-go-starter ./cmd/codex-go-starter
plugin-kit-ai validate . --platform codex-runtime --strict
```

For the broader long-term release story, use the Go production guidance in the main repo:

- [Production plugin examples](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins)
- [Production guide](https://github.com/777genius/plugin-kit-ai/blob/main/docs/PRODUCTION.md)
