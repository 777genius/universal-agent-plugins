# claude-basic-prod

Reference Claude plugin repo for the current `plugin-kit-ai` production workflow.

Stable runtime promise:

- `Stop`
- `PreToolUse`
- `UserPromptSubmit`

This example matches the stable-default Claude scaffold. It does not opt into the extended hook set.
It also exercises the first-class Claude `settings.json` surface through `plugin/targets/claude/settings.json`.

## Workflow

```bash
plugin-kit-ai normalize .
plugin-kit-ai generate .
plugin-kit-ai generate --check .
plugin-kit-ai validate . --platform claude --strict
go test ./...
go build -o bin/claude-basic-prod ./cmd/claude-basic-prod
printf '%s' '{"session_id":"s","cwd":"/tmp","hook_event_name":"Stop"}' | ./bin/claude-basic-prod Stop
printf '%s' '{"session_id":"e2e-session","transcript_path":"/tmp/t.jsonl","cwd":"/tmp","permission_mode":"default","hook_event_name":"PreToolUse","tool_name":"Bash","tool_use_id":"toolu_e2e","tool_input":{"command":"echo ok"}}' | ./bin/claude-basic-prod PreToolUse
printf '%s' '{"session_id":"e2e-session","transcript_path":"/tmp/t.jsonl","cwd":"/tmp","permission_mode":"default","hook_event_name":"UserPromptSubmit","prompt":"hello e2e"}' | ./bin/claude-basic-prod UserPromptSubmit
```

Included MCP servers:

- `linear` (remote, `https://mcp.linear.app/mcp`)
- `supabase` (remote, `https://mcp.supabase.com/mcp`)
- `playwright` (stdio, `npx @playwright/mcp@0.0.70`)

Linear and Supabase headers are configured via `LINEAR_API_KEY` and `SUPABASE_ACCESS_TOKEN`/`SUPABASE_PROJECT_REF` placeholders. Without credentials, MCPs should fail open or skip tool availability according to server behavior.
