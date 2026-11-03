---
title: "Go Hooks SDK"
description: "Build typed Claude, Codex and Gemini hook handlers with a pinned Go module."
canonicalId: "page:sdk:go-hooks"
section: "sdk"
locale: "en"
generated: false
translationRequired: true
---

# Go Hooks SDK

The Go Hooks SDK lets a single executable register **typed, native agent hooks**, dispatch one invocation and return a host-specific result. It is not an installer and does not make Skills or MCP portable automatically.

## Install and choose the contract

The independently versioned Go module is `github.com/777genius/plugin-kit-ai/sdk`. The [SDK README](https://github.com/777genius/universal-agent-plugins/blob/main/sdk/README.md) documents the published **v1.3.0** module:

```bash
go get github.com/777genius/plugin-kit-ai/sdk@v1.3.0
```

The module declares Go 1.22 with a Go 1.26.10 toolchain directive. For reliable reproduction use the toolchain and module version recorded in your build. Source `main` may expose later beta events; those are **not implicitly available in v1.3.0**.

## Minimal typed handler

Register handlers before calling `Run`. The host passes the native hook name as an invocation argument. The SDK reads the event, dispatches its typed callback and writes the correct response.

```go
package main

import (
    "os"

    uap "github.com/777genius/plugin-kit-ai/sdk"
    "github.com/777genius/plugin-kit-ai/sdk/claude"
    "github.com/777genius/plugin-kit-ai/sdk/codex"
)

func main() {
    app := uap.New(uap.Config{Name: "review-hooks"})

    app.Claude().OnStop(func(event *claude.StopEvent) *claude.Response {
        return claude.Allow()
    })
    app.Codex().OnNotify(func(event *codex.NotifyEvent) *codex.Response {
        return codex.Continue()
    })

    os.Exit(app.Run())
}
```

This example demonstrates **registration and dispatch**, not automatic delivery. Build an executable and configure the chosen agent's **native hook** to invoke it with the expected event name and payload. Hook names, registration syntax and native installation differ by client. See [runtime examples in source](https://github.com/777genius/universal-agent-plugins/blob/main/sdk/example_test.go).

## Public surfaces

- Root: `New(Config)`, `App.Use`, `App.Run`, `App.RunContext`, `Supported()` and platform registrars `Claude()`, `Codex()`, `Gemini()`, `Qoder()`.
- Typed platforms: `sdk/claude`, `sdk/codex`, `sdk/gemini` and `sdk/qoder`. Other source-tree registrars have their own maturity and release boundaries.
- `Supported()` exposes **runtime event** status; it is not a matrix of supported installers, profile scopes or authenticated services.

For the documented stable event subset, see the [SDK's runtime boundary](https://github.com/777genius/universal-agent-plugins/blob/main/sdk/README.md#runtime-contract-boundary). It includes Claude `Stop` / `PreToolUse` / `UserPromptSubmit`, Codex `Notify`, Gemini's promoted nine-hook path and the documented Qoder hooks in current source. Other events can be `public-beta`; consult the **version-specific** support matrix and [stability policy](https://github.com/777genius/universal-agent-plugins/blob/main/sdk/STABILITY.md). In particular, Cursor's stop observer is **beta**, not equivalent to a qualified native Cursor integration.

## Event lifecycle

1. Agent launches the configured hook executable and supplies its native event (stdin or invocation-specific transport).
2. `RunContext(ctx)` resolves the invoked platform/event, decodes bounded data and applies middleware/registered handlers.
3. The SDK serializes the matching response and exit code to the hook process; use **stderr** for diagnostics, not random stdout logs.
4. Test the exact agent, version, hook registration and payload separately. An SDK unit test proves protocol handling, not native activation.

`App.Use` adds middleware for subsequent dispatch. Configure `Config{Args, IO, Env, Logger}` when embedding or testing. The shared `MaxPayloadBytes` constant is the runtime input limit; do not invent your own maximum.

## Shared executable and detection

If one binary may be called by multiple clients, use [Host detection](./host-detection) on the **same captured payload** before selecting a platform-specific path. Do not consume stdin once for detection and expect `Run` to read it again unless you provide a suitable replayable `Config.IO`.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| Hook never runs | Native hook registration, executable permissions, client reload and exact event name |
| Unknown invocation | Arguments/event spelling and whether that event exists in the pinned SDK version |
| Invalid payload | Host-specific wire format, size limit and whether data is stdin JSON or legacy argv JSON |
| Unexpected text in response | Keep stdout reserved for SDK wire output |
| Hook works in a unit test but not in a client | Reproduce with the exact native host version and profile; installation is a separate contract |

[Go SDK source](https://github.com/777genius/universal-agent-plugins/tree/main/sdk) · [Host detection](./host-detection) · [Go installer](./go-installer)
