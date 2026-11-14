# claude-python-starter

Copy-first starter for Python teams that want the stable Claude default hook subset with the smallest possible repo-local setup.

## Who It Is For

- Teams wiring a local Claude plugin into an existing repo
- Python users who want `requirements.txt` plus a repo-local `.venv`
- Users who want the stable default hook subset instead of the extended hook surface

## Prerequisites

- `plugin-kit-ai` installed
- Python `3.10+` installed on the machine that will run the plugin
- Claude local plugin runtime lane

## Runtime

- Platform: `claude`
- Runtime: `python`
- Entrypoint: `./bin/claude-python-starter`
- Execution mode: `launcher`
- Status: `public-stable`, repo-local interpreted subset

## First Run

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai validate . --platform claude --strict
```

This starter keeps one canonical Python env story:

- `requirements.txt`
- repo-local `.venv`
- `plugin/plugin_runtime.py`

The local helper file mirrors the shared `plugin-kit-ai-runtime` package when you later want to move this API into a reusable dependency.

`plugin-kit-ai bootstrap .` creates `.venv` when needed and installs `requirements.txt`.
If you prefer `uv`, `poetry`, or `pipenv`, keep using the stable runtime lane, but this starter stays opinionated on `requirements.txt` plus `.venv`.
If you want downstream users to avoid installing Python at all, prefer the Go starter instead.

## Local Smoke

```bash
printf '%s' '{"session_id":"starter-session","transcript_path":"/tmp/t.jsonl","cwd":".","permission_mode":"default","hook_event_name":"Stop","stop_hook_active":false,"last_assistant_message":"ok"}' | ./bin/claude-python-starter Stop
```

## Stable Default

- `Stop`
- `PreToolUse`
- `UserPromptSubmit`

The scaffold wires only the public-stable Claude hook subset by default.
Treat `plugin-kit-ai validate --strict` as the CI-grade readiness gate for this plugin.
Use `plugin-kit-ai init <name> --platform claude --runtime python --claude-extended-hooks` only when you intentionally want the full runtime-supported hook set scaffolded.

## Target Files

- `plugin/targets/claude/hooks/hooks.json`: authored Claude hook routing
- `hooks/hooks.json`: generated managed Claude hook file
- `plugin/plugin_runtime.py`: official helper API for stable Claude hook handlers
- Optional first-class Claude breadth via `--extras`:
  - `plugin/targets/claude/settings.json` -> generated `settings.json`
  - `plugin/targets/claude/lsp.json` -> generated `.lsp.json`
  - `plugin/targets/claude/user-config.json` -> generated `plugin.json.userConfig`
  - `plugin/targets/claude/manifest.extra.json` -> manifest passthrough for non-managed keys only

Keep stdout reserved for Claude responses and write diagnostics to stderr only.

## Ship It

This starter already includes `.github/workflows/bundle-release.yml`.

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai validate . --platform claude --strict
plugin-kit-ai bundle publish . --platform claude --repo owner/repo --tag v1.0.0
plugin-kit-ai bundle fetch owner/repo --tag v1.0.0 --platform claude --runtime python --dest ./handoff-plugin
```
