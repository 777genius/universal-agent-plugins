# plugin-kit-ai SDK

Module: `github.com/777genius/plugin-kit-ai/sdk`

Normal consumption path:

```bash
go get github.com/777genius/plugin-kit-ai/sdk@v1.1.0
```

The canonical release contract for this subdirectory module is:

- root release tag: `vX.Y.Z`
- SDK module tag from the same commit: `sdk/vX.Y.Z`

The first truthful normal-module release for this path is `v1.0.4`. `v1.0.3` remains published as a root release, but it should not be used for Go SDK module consumption.

The SDK exposes a platform-neutral runtime core with platform-specific public registrars.

Current contract status in this source tree: the root SDK plus the approved Claude/Codex stable event set shipped as `public-stable` in `v1.0.0`, and Gemini's promoted 9-hook runtime surface is now also `public-stable`. Event-level support claims come from [../../docs/generated/support_matrix.md](../../docs/generated/support_matrix.md). Compatibility policy lives in [STABILITY.md](./STABILITY.md).

`plugin-kit-ai.Supported()` returns runtime-event metadata only. Stable Claude, Codex, and Gemini runtime paths are production-ready within the declared contract. The root `(*plugin-kit-ai.App).Gemini()` registrar remains stable, and the promoted Gemini hook-level event, response, and helper surfaces now sit inside the stable runtime promise for the current 9-hook lane.

## Public API

Root package:

- `plugin-kit-ai.New(plugin-kit-ai.Config)`
- `(*plugin-kit-ai.App).Use(...)`
- `(*plugin-kit-ai.App).Claude()`
- `(*plugin-kit-ai.App).Codex()`
- `(*plugin-kit-ai.App).Gemini()`
- `(*plugin-kit-ai.App).Cursor()` (`public-beta`, stop only)
- `(*plugin-kit-ai.App).RunCursorObserver(ctx)` (`public-beta`)
- `(*plugin-kit-ai.App).Run()`
- `(*plugin-kit-ai.App).RunContext(ctx)`
- `plugin-kit-ai.Supported()`

Platform packages:

- `github.com/777genius/plugin-kit-ai/sdk/claude`
- `github.com/777genius/plugin-kit-ai/sdk/codex`
- `github.com/777genius/plugin-kit-ai/sdk/gemini`
- `github.com/777genius/plugin-kit-ai/sdk/cursor` (`public-beta`)

## Runtime Contract Boundary

- Production-ready stable runtime paths:
  - `claude/Stop`
  - `claude/PreToolUse`
  - `claude/UserPromptSubmit`
  - `codex/Notify`
  - `gemini/SessionStart`
  - `gemini/SessionEnd`
  - `gemini/BeforeModel`
  - `gemini/AfterModel`
  - `gemini/BeforeToolSelection`
  - `gemini/BeforeAgent`
  - `gemini/AfterAgent`
  - `gemini/BeforeTool`
  - `gemini/AfterTool`
- Runtime-supported but not stable:
  - `claude/SessionStart` (`public-beta`)
  - `claude/SessionEnd` (`public-beta`)
  - `claude/Notification` (`public-beta`)
  - `claude/PostToolUse` (`public-beta`)
  - `claude/PostToolUseFailure` (`public-beta`)
  - `claude/PermissionRequest` (`public-beta`)
  - `claude/SubagentStart` (`public-beta`)
  - `claude/SubagentStop` (`public-beta`)
  - `claude/PreCompact` (`public-beta`)
  - `claude/Setup` (`public-beta`)
  - `claude/TeammateIdle` (`public-beta`)
  - `claude/TaskCompleted` (`public-beta`)
  - `claude/ConfigChange` (`public-beta`)
  - `claude/WorktreeCreate` (`public-beta`)
  - `claude/WorktreeRemove` (`public-beta`)
  - `cursor/stop` (`public-beta`, invocation `CursorStop`; native qualification pending)
  - `codex/Stop` (`public-beta`, invocation name `CodexStop`)
  - `codex/SubagentStop` (`public-beta`, invocation name `CodexSubagentStop`)
  - `codex/PreToolUse` (`public-beta`, invocation name `CodexPreToolUse`)
  - `codex/PermissionRequest` (`public-beta`, invocation name `CodexPermissionRequest`)

Codex lifecycle hooks use prefixed invocation names (`CodexStop`, not `Stop`) because the flat
resolver already assigns the bare event names to Claude; the descriptor `Event` stays clean
(`Stop`). The payloads arrive as snake_case stdin JSON and the handlers are observation-style:
success means empty stdout and exit 0. Note on the generated support matrix: the
`codex_notify` live-test profile is a platform-level attribute — the live lane exercises the
legacy notify path only, and these four lifecycle events are covered by unit/app-level tests,
not by live execution.

Host detection for multi-host binaries lives in
`github.com/777genius/plugin-kit-ai/sdk/hostdetect` (`public-beta`): an explicit override always
wins, unknown overrides are errors, and detection fails closed with `PlatformUnknown` instead of
silently assuming a host. The root package also exports `plugin-kit-ai.MaxPayloadBytes`, the single
wire limit used by runtime decoders.
Gemini's current production-ready 9-hook runtime boundary is audited in [../../docs/GEMINI_RUNTIME_AUDIT.md](../../docs/GEMINI_RUNTIME_AUDIT.md).

The additional `gemini/Notification` event is **public-beta**. Its native input
shape follows official Gemini CLI v0.62.0 source (`b460678f3db508407554afd604cc9d6635becb2a`);
SDK stdin/process checks do not qualify native CLI execution. Native qualification
is pending. Runtime metadata now contains ten Gemini events: nine stable and one beta.

Generated support matrix: [../../docs/generated/support_matrix.md](../../docs/generated/support_matrix.md)

## Experimental Custom Claude Hooks

When upstream `plugin-kit-ai` support lags behind a new Claude hook, plugin projects can register a local typed hook without falling back to raw `map[string]any` handlers:

```go
type TeamHeartbeat struct {
	HookEventName string `json:"hook_event_name"`
	Message       string `json:"message"`
}

err := claude.RegisterCustomContextJSON(app.Claude(), "TeamHeartbeat", func(e *TeamHeartbeat) *claude.ContextResponse {
	return &claude.ContextResponse{AdditionalContext: "seen"}
})
```

This extension path is `public-experimental`: typed and usable, but outside the stable compatibility promise.

Codex has a matching experimental escape hatch for future argv-JSON hooks:

```go
type TaskEvent struct {
	Client string `json:"client"`
	Task   string `json:"task"`
}

err := codex.RegisterCustomJSON(app.Codex(), "task_event", func(e *TaskEvent) *codex.Response {
	return codex.Continue()
})
```

## Generation

Runtime/scaffold/validate registries are generated from descriptor definitions.

```bash
go run ./cmd/plugin-kit-ai-gen
```

## Claude Example

```go
package main

import (
	"os"

	pluginkitai "github.com/777genius/plugin-kit-ai/sdk"
	"github.com/777genius/plugin-kit-ai/sdk/claude"
)

func main() {
	app := pluginkitai.New(pluginkitai.Config{Name: "claude-demo"})
	app.Claude().OnStop(func(*claude.StopEvent) *claude.Response {
		return claude.Allow()
	})
	os.Exit(app.Run())
}
```

## Codex Example

```go
package main

import (
	"os"

	pluginkitai "github.com/777genius/plugin-kit-ai/sdk"
	"github.com/777genius/plugin-kit-ai/sdk/codex"
)

func main() {
	app := pluginkitai.New(pluginkitai.Config{Name: "codex-demo"})
	app.Codex().OnNotify(func(*codex.NotifyEvent) *codex.Response {
		return codex.Continue()
	})
	os.Exit(app.Run())
}
```

## Gemini Example

```go
package main

import (
	"os"

	pluginkitai "github.com/777genius/plugin-kit-ai/sdk"
	"github.com/777genius/plugin-kit-ai/sdk/gemini"
)

func main() {
	app := pluginkitai.New(pluginkitai.Config{Name: "gemini-demo"})
	app.Gemini().OnBeforeTool(func(*gemini.BeforeToolEvent) *gemini.BeforeToolResponse {
		return gemini.BeforeToolContinue()
	})
	os.Exit(app.Run())
}
```

Gemini helper rule of thumb:

- use `gemini.SessionStartContinue()`, `gemini.SessionEndContinue()`, `gemini.BeforeModelContinue()`, `gemini.AfterModelContinue()`, `gemini.BeforeToolContinue()`, and `gemini.AfterToolContinue()` for a true no-op response that renders as minimal `{}` output
- Gemini treats `SessionStart` and `SessionEnd` as advisory hooks: `continue`, `decision`, `reason`, and `stopReason` are ignored there, so only `systemMessage` and the documented hook-specific fields are emitted
- use `gemini.SessionStartMessage(...)` and `gemini.SessionEndMessage(...)` when you want a typed helper for the advisory `systemMessage` path instead of setting the field manually
- use `gemini.BeforeModelOverrideRequestValue(...)` when you want to rewrite `llm_request` from a normal Go map/struct, `gemini.BeforeModelSyntheticResponseValue(...)` when you want to short-circuit the model call with a synthetic response, and `gemini.AfterModelReplaceResponseValue(...)` when you want to rewrite the returned `llm_response`
- use `gemini.BeforeToolSelectionConfig(...)` when you want to steer Gemini tool choice with official `toolConfig.mode`; Gemini currently accepts `allowedFunctionNames` only together with `mode:"ANY"`. Use `gemini.BeforeToolSelectionDisableAll()` when you intentionally want `mode:"NONE"`
- use `gemini.BeforeToolSelectionAllowOnly(...)` when you want an allowlist in the vendor-accepted `ANY` shape, `gemini.BeforeToolSelectionForceAny(...)` when you want Gemini to call at least one tool, `gemini.BeforeToolSelectionForceAuto()` when you want explicit `AUTO` mode, and `gemini.BeforeToolSelectionQuiet()` when you only want to suppress hook metadata for the tool-selection step
- use `gemini.BeforeAgentAddContext(...)` when you want turn-local prompt context, and `gemini.AfterAgentClearContext()` when you intentionally want Gemini to drop prior conversation memory before the next retry/turn
- use `gemini.BeforeAgentDeny(...)` to reject a turn and discard the prompt, or `gemini.AfterAgentDeny(...)` to reject a final answer and trigger a retry
- use `gemini.BeforeAgentStop(...)` when you want to stop the turn but keep the prompt in history, `gemini.AfterAgentStop(...)` when you want to stop the session without retrying, and `gemini.AfterModelStop(...)`, `gemini.BeforeToolStop(...)`, or `gemini.AfterToolStop(...)` when you intentionally want `continue:false` loop-stop behavior
- use `gemini.BeforeToolAllow()` or `gemini.AfterToolAllow()` only when you intentionally want an explicit `"decision":"allow"` in the Gemini hook response
- use `gemini.BeforeToolRewriteInputValue(...)` when you want to rewrite `tool_input` from a normal Go map/struct; it validates the result is a JSON object, which matches the Gemini hooks contract
- use `gemini.AfterToolAddContext(...)` to append extra text to the tool result, or `gemini.AfterToolTailCallValue(...)` to request an immediate follow-up tool call with typed Go args

### Gemini Notification observer (beta)

The existing Gemini scaffold and extension renderer retain their nine stable
hooks. Notification requires explicit handler registration and a native hook
entry. Generated Scaffold, Validate and Live Test fields describe platform
capabilities and do not promise Notification-specific coverage.

Register with `app.Gemini().OnNotification(func(e *gemini.NotificationEvent) *gemini.NotificationResponse { ... })`
and invoke the consumer with `GeminiNotification`. Bare `Notification` continues
to select Claude. [The public consumer fixture](./testdata/gemini-observer/main.go)
registers both `OnAfterAgent` and `OnNotification` using only public SDK imports;
its stderr records contain synthetic test data and are not a production logging example.

`NotificationEvent` exposes native `notification_type`, `message`, `details`, and
the existing base fields (`session_id`, `timestamp`, `hook_event_name`, `cwd`,
`transcript_path`). `Details` retains arbitrary JSON values as `json.RawMessage`.
`NotificationTypeToolPermission` is supported; unknown subtype strings decode
unchanged and consumers should ignore subtypes they do not use. Unknown top-level
fields follow the existing decoder policy and are ignored. The SDK trusts the
invocation selector; consumers that need native event admission must also check
`HookEventName` against that selector.

`NotificationResponse` is an empty struct. Returning nil or
`&gemini.NotificationResponse{}` produces `{}` with exit 0. This is an advisory
observer subset of native `NotificationOutput`: native `suppressOutput` and
`systemMessage` options are omitted. It cannot decide permission or add context.
Filtering sensitive message/details, choosing notifications, and delivering them
belong to consumers. The SDK supplies no notification configuration or delivery
policy. Existing `AfterAgent` behavior is unchanged; an observation does not prove
a successful final turn, and no native turn/request/root identifier is invented.

## Cursor stop beta observer

The descriptor-derived [Cursor stop guide](../docs/generated/cursor_stop.md)
explains typed native fields, the fixed `{}\n` observer response and exclusive
process-pipe IO ownership. `RunCursorObserver` honors `Config.IO`; arbitrary
caller IO must cooperate with cancellation and cannot be forcibly interrupted.
This source slice proves SDK protocol handling only. It does not qualify native
Cursor IDE/CLI, installation, delivery or availability.
