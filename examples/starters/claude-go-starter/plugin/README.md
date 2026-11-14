# claude-go-starter

Copy-first starter for Go teams that want the stable Claude default hook subset with the SDK-first production lane.

## Who It Is For

- Teams starting a new Claude plugin in Go
- Users who want the strongest typed-handler path from day one
- Users who want a copy-first entrance without giving up the production-default Go lane

## Prerequisites

- `plugin-kit-ai` installed
- Go `1.22+`
- Claude local plugin runtime lane
- access to `github.com/777genius/plugin-kit-ai/sdk@v1.1.0`

## Runtime

- Platform: `claude`
- Runtime: `go`
- Entrypoint: `./bin/claude-go-starter`
- Execution mode: `launcher`
- Status: strongest production-ready path, starter-layer copy-first entrance

## First Run

```bash
go test ./...
go build -o bin/claude-go-starter ./cmd/claude-go-starter
plugin-kit-ai validate . --platform claude --strict
```

This starter keeps one canonical Go story:

- SDK-first handlers
- `go test ./...`
- `go build -o bin/claude-go-starter ./cmd/claude-go-starter`

Unlike the interpreted Python/Node starters, this lane does not depend on the launcher bootstrap command or the Python/Node bundle handoff workflow.
The public Go SDK now resolves as a normal module release through `github.com/777genius/plugin-kit-ai/sdk@v1.1.0`.

## Local Smoke

```bash
printf '%s' '{"session_id":"starter-session","transcript_path":"/tmp/t.jsonl","cwd":".","permission_mode":"default","hook_event_name":"Stop","stop_hook_active":false,"last_assistant_message":"ok"}' | ./bin/claude-go-starter Stop
```

## Stable Default

- `Stop`
- `PreToolUse`
- `UserPromptSubmit`

The scaffold wires only the public-stable Claude hook subset by default.
Use `plugin-kit-ai init <name> --platform claude --runtime go --claude-extended-hooks` only when you intentionally want the full runtime-supported hook set scaffolded.

## Target Files

- `plugin/targets/claude/hooks/hooks.json`: authored Claude hook routing
- `hooks/hooks.json`: generated managed Claude hook file
- `.claude-plugin/plugin.json`: generated Claude plugin manifest

If you move the project, update the authored hook command paths so they still point at `./bin/claude-go-starter`.
Keep stdout reserved for Claude responses and write diagnostics to stderr only.

## Ship It

```bash
go test ./...
go build -o bin/claude-go-starter ./cmd/claude-go-starter
plugin-kit-ai validate . --platform claude --strict
```

For the broader long-term release story, use the Go production guidance in the main repo:

- [Production plugin examples](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins)
- [Production guide](https://github.com/777genius/plugin-kit-ai/blob/main/docs/PRODUCTION.md)
