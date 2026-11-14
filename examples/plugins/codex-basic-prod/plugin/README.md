# codex-basic-prod

Reference Codex runtime repo for the current `plugin-kit-ai` production workflow.

Stable runtime lane:

- `Notify`
- repo-local MCP passthrough via `plugin/targets/codex-runtime/config.extra.toml`

## Workflow

```bash
plugin-kit-ai normalize .
plugin-kit-ai generate .
plugin-kit-ai generate --check .
plugin-kit-ai validate . --platform codex-runtime --strict
go test ./...
go build -o bin/codex-basic-prod ./cmd/codex-basic-prod
./bin/codex-basic-prod notify '{"client":"codex-tui"}'
codex -C . mcp get release-checks --json
```

This example covers the repo-local Codex notify/config lane, including repo-local MCP passthrough generated into `.codex/config.toml`. It does not claim to be the official Codex plugin bundle lane.
