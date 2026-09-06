# Changelog

All notable changes to this module are documented here.

The format is inspired by Keep a Changelog; versions follow SemVer. This unreleased entry is the pending `v1.0.0` release candidate.

## [Unreleased]

### Added

- Codex stdin-JSON lifecycle hooks (`public-beta`): `codex/Stop`, `codex/SubagentStop`, and `codex/PermissionRequest` with prefixed invocation names `CodexStop`, `CodexSubagentStop`, `CodexPermissionRequest` (bare event names stay owned by Claude in the flat resolver). These invocation names are now reserved: `codex.RegisterCustomJSON` with any of them fails registration with a "conflicts with built-in invocation" error instead of being silently shadowed.
- `hostdetect` package (`public-beta`): fail-closed host product detection with explicit override, env markers, and bounded top-level payload sniffing for Claude and Codex.
- Root `plugin-kit-ai.MaxPayloadBytes` export so consumers reference the single wire limit instead of duplicating it.

- Generated descriptor system for runtime registry, invocation resolution, scaffold definitions, validate rules, and support docs.
- Platform-neutral runtime core under `internal/runtime`.
- Public peer platform packages:
  - `claude`
  - `codex`
- Codex runtime support for `Notify`.
- CLI-facing generated support artifacts and support matrix.
- Repository-level executable plugin ABI documentation for Go-first, polyglot runtime scaffolds.

### Changed

- Root package `plugin-kit-ai` now acts as composition/runtime only.
- Public registration moved from root-Claude methods to platform registrars:
  - `app.Claude().OnStop(...)`
  - `app.Claude().OnPreToolUse(...)`
  - `app.Claude().OnUserPromptSubmit(...)`
  - `app.Codex().OnNotify(...)`
- App construction now uses `plugin-kit-ai.New(plugin-kit-ai.Config{...})`.
- Registration after `Run` panics.
- Public SDK consumption now uses the canonical module path `github.com/777genius/plugin-kit-ai/sdk` with the submodule tag contract `sdk/vX.Y.Z`.

### Removed

- Root-Claude registration methods:
  - `OnStop`
  - `OnPreToolUse`
  - `OnUserPromptSubmit`
- Claude-shaped dispatcher and `ClaudeWireCodec`-centered runtime.
- Legacy `domain`, `ports`, `usecase`, and old Claude adapter layout from the SDK runtime.

### Migration

- Replace:

```go
app := plugin-kit-ai.New()
app.OnStop(...)
```

with:

```go
app := plugin-kit-ai.New(plugin-kit-ai.Config{Name: "my-plugin"})
app.Claude().OnStop(...)
```

- For Codex plugins, register `Notify` through `app.Codex().OnNotify(...)` and use the generated `.codex/config.toml` scaffold contract.
