# Changelog

This file preserves accumulated Go runtime SDK changes. It is not a per-release
ledger: entries below may already be included in published versions. The SDK is
no longer a pending `v1.0.0` release candidate.

Use the [SDK integration guide](./README.md) for the recommended published module
version and [tagged releases](https://github.com/777genius/universal-agent-plugins/releases)
for version-specific notes. Go module tags use `sdk/vX.Y.Z`; they are separate
from the installer's `agentplugins-v*` releases and the TypeScript Management SDK.
The recorded beta/native-qualification limits below are not promoted by a release
number. Historical migration examples are not a replacement for the current guide.

<a id="unreleased"></a>

## Accumulated source history

### Added

- VS Code Local Stop/SubagentStop observers (`public-beta`) with
  `App.VSCodeLocal()`, typed presence-preserving input and neutral `{}` output
  through the existing `RunContext` engine. Runtime `vscode-local` stays separate
  from installer `vscode` and Notifications `copilot-vscode`. This library slice
  does not qualify an installed integration or executable release.

- Cursor native `stop` observer (`public-beta`): descriptor-generated `CursorStop`
  registrar/metadata, documented DTO with pointer presence, empty response,
  `App.Cursor()` and opt-in `App.RunCursorObserver(ctx)` with owned Linux/macOS
  pipes or connected anonymous AF_UNIX/SOCK_STREAM sockets (empty local AND peer
  names), deadlines and fixed `{}\n`. Named/abstract/network sockets and other
  files are refused. Transferred IO requires exclusive ownership; cancellation
  interrupts IO and joins its callback within one four-second budget with a
  100 ms output reserve. Injected IO and consumer callbacks retain their
  cooperation contract. Manual package activation and pending native
  qualification retain their availability limits. Ordinary
  SDK output and permission exits remain unchanged.

- Codex stdin-JSON lifecycle hooks (`public-beta`): `codex/Stop`, `codex/SubagentStop`, `codex/PreToolUse`, and `codex/PermissionRequest` with prefixed invocation names `CodexStop`, `CodexSubagentStop`, `CodexPreToolUse`, `CodexPermissionRequest` (bare event names stay owned by Claude in the flat resolver). These invocation names are now reserved: `codex.RegisterCustomJSON` with any of them fails registration with a "conflicts with built-in invocation" error instead of being silently shadowed.
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
