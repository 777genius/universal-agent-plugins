---
title: "Agent host detection SDK"
description: "Distinguish Claude and Codex hook payloads without guessing from shared environment variables."
canonicalId: "page:sdk:host-detection"
section: "sdk"
locale: "en"
generated: false
translationRequired: true
---

# Detect the invoking agent (Go)

`github.com/777genius/plugin-kit-ai/sdk/hostdetect` is a small **hook-host detector**. It identifies the product that invoked a hook process. It is not a scan of which agents are installed, a profile locator, or a proof that native activation succeeded.

## What the built-in detector knows

`DefaultRegistry()` recognizes **Codex** and **Claude Code** only. It looks at top-level keys in bounded hook JSON:

| Signal | Result |
| --- | --- |
| Top-level `turn_id` | `PlatformCodex` |
| Top-level `hook_event_name` without `turn_id` | `PlatformClaude` |
| No matching shape, malformed JSON or payload over `MaxPayloadBytes` | `PlatformUnknown` |

Registry order is Codex then Claude. The default registry deliberately has **no environment markers**: Codex may export both `PLUGIN_ROOT` and `CLAUDE_PLUGIN_ROOT`, so those names do not distinguish the two products. The legacy Codex notify argv JSON (with keys like `turn-id`) is also **not** detected by this stdin JSON detector.

## Minimal usage

```go
package main

import (
    "fmt"

    "github.com/777genius/plugin-kit-ai/sdk/hostdetect"
)

func main() {
    payload := []byte(`{"hook_event_name":"Stop","session_id":"example"}`)
    host, err := hostdetect.Detect(hostdetect.DefaultRegistry(), "", nil, payload)
    if err != nil {
        panic(err)
    }
    switch host {
    case hostdetect.PlatformClaude:
        fmt.Println("Claude hook")
    case hostdetect.PlatformCodex:
        fmt.Println("Codex hook")
    default:
        fmt.Println("Unknown; do not assume a default agent")
    }
}
```

`Detect(registry, override, env, payload)` evaluates (1) an explicit valid override, (2) any ordered custom registry environment markers, then (3) top-level JSON signals. An **unknown override is an error**. Without a match, it returns `PlatformUnknown` and a nil error. `DefaultRegistry()` returns an isolated copy, so callers can extend it without mutating global state.

This package is present in the current source tree. Verify that the **exact published SDK tag** you consume includes it before promising binary distribution; source availability is not itself a release claim.

## Extending the registry

`Registry` is an ordered `[]Signal` with `Platform`, `EnvMarkers` and optional `PayloadSniff`. Add another detector only when a host has a **verified, discriminating signal**, not an environment variable shared across agents. The first matching registry entry wins within a tier. Custom sniffers receive top-level keys, not decoded typed events. Keep unrecognized inputs as unknown.

## Hook detection vs installation discovery

For a desktop/plugin manager, the [TypeScript SDK](./typescript-management) provides `discoverTargets({ executable, ... })`, which asks the pinned management engine to inspect **native client presence and private profiles**. This is distinct from a hook payload detector and does not install files. The [Go installer SDK](./go-installer) also exposes `Engine.Discover()` for explicitly registered adapters, but registration or file presence is **not** a guarantee of complete native support.

[Source and tests](https://github.com/777genius/universal-agent-plugins/tree/main/sdk/hostdetect) · [Go Hooks SDK](./go-hooks)
