---
title: "sdk"
description: "生成的 Go SDK package 参考 for github.com/777genius/plugin-kit-ai/sdk"
canonicalId: "go-package:github.com/777genius/plugin-kit-ai/sdk"
surface: "go-sdk"
section: "api"
locale: "zh"
generated: true
editLink: false
stability: "public-stable"
maturity: "stable"
sourceRef: "sdk"
translationRequired: false
---
<DocMetaCard surface="go-sdk" stability="public-stable" maturity="stable" source-ref="sdk" source-href="https://github.com/777genius/plugin-kit-ai/tree/main/sdk" />

# sdk

通过 gomarkdoc 从公开 Go package 生成。

**导入路径:** `github.com/777genius/plugin-kit-ai/sdk`

```go
import "github.com/777genius/plugin-kit-ai/sdk"
```

Package pluginkitai exposes the public root SDK for building plugin\-kit\-ai runtime binaries with typed Claude, Codex, and Gemini registrars.

## Index

- Constants
- type App
  - func New\(cfg Config\) \*App
  - func \(a \*App\) Claude\(\) \*claude.Registrar
  - func \(a \*App\) Codex\(\) \*codex.Registrar
  - func \(a \*App\) Gemini\(\) \*gemini.Registrar
  - func \(a \*App\) Run\(\) int
  - func \(a \*App\) RunContext\(ctx context.Context\) int
  - func \(a \*App\) Use\(mw Middleware\)
- type CapabilityID
- type Config
- type Env
- type Handled
- type IO
- type InvocationContext
- type Logger
- type MaturityLevel
- type Middleware
- type Next
- type NopLogger
- type Result
- type SupportEntry
  - func Supported\(\) \[\]SupportEntry
- type SupportStatus
- type TransportMode


## Constants

MaxPayloadBytes is the single wire limit for stdin and argv JSON payloads accepted by runtime decoders. Consumers should reference this constant instead of duplicating the number.

```go
const MaxPayloadBytes = runtime.MaxPayloadBytes
```

## type App

App owns middleware, handler registration, and invocation dispatch.

```go
type App struct {
    // contains filtered or unexported fields
}
```

### func New

```go
func New(cfg Config) *App
```

New builds an App with sane defaults for argv, process I/O, env, and logging.

### func \(\*App\) Claude

```go
func (a *App) Claude() *claude.Registrar
```

Claude returns a registrar for Claude\-specific hook handlers.


**Example**

```go
package main

import (
	pluginkitai "github.com/777genius/plugin-kit-ai/sdk"
	"github.com/777genius/plugin-kit-ai/sdk/claude"
)

func main() {
	app := pluginkitai.New(pluginkitai.Config{Name: "demo"})
	app.Claude().OnStop(func(*claude.StopEvent) *claude.Response {
		return claude.Allow()
	})
	_ = app
}
```


### func \(\*App\) Codex

```go
func (a *App) Codex() *codex.Registrar
```

Codex returns a registrar for Codex\-specific event handlers.


**Example**

```go
package main

import (
	pluginkitai "github.com/777genius/plugin-kit-ai/sdk"
	"github.com/777genius/plugin-kit-ai/sdk/codex"
)

func main() {
	app := pluginkitai.New(pluginkitai.Config{Name: "demo"})
	app.Codex().OnNotify(func(*codex.NotifyEvent) *codex.Response {
		return codex.Continue()
	})
	_ = app
}
```


### func \(\*App\) Gemini

```go
func (a *App) Gemini() *gemini.Registrar
```

Gemini returns a registrar for Gemini\-specific hook handlers.


**Example**

```go
package main

import (
	pluginkitai "github.com/777genius/plugin-kit-ai/sdk"
	"github.com/777genius/plugin-kit-ai/sdk/gemini"
)

func main() {
	app := pluginkitai.New(pluginkitai.Config{Name: "demo"})
	app.Gemini().OnBeforeTool(func(*gemini.BeforeToolEvent) *gemini.BeforeToolResponse {
		return gemini.BeforeToolContinue()
	})
	_ = app
}
```


### func \(\*App\) Run

```go
func (a *App) Run() int
```

Run dispatches the current process invocation with context.Background\(\).

### func \(\*App\) RunContext

```go
func (a *App) RunContext(ctx context.Context) int
```

RunContext dispatches the current process invocation using the supplied context.

### func \(\*App\) Use

```go
func (a *App) Use(mw Middleware)
```

Use appends middleware that wraps all subsequent handler dispatch.

## type CapabilityID

CapabilityID aliases the normalized cross\-platform capability identifier.

```go
type CapabilityID = runtime.CapabilityID
```

## type Config

Config configures a root SDK app instance before handlers are registered.

```go
type Config struct {
    // Name is the human-readable app label used in diagnostics and examples.
    Name string
    // Args overrides the process argv used to resolve the current invocation.
    Args []string
    // IO overrides the stdin/stdout/stderr implementation used by Run.
    IO  IO
    // Env overrides environment lookups used during invocation resolution.
    Env Env
    // Logger overrides structured logging emitted by the runtime engine.
    Logger Logger
}
```

## type Env

Env aliases the runtime environment reader used by invocation resolution.

```go
type Env = runtime.Env
```

## type Handled

Handled aliases the typed handler result container.

```go
type Handled = runtime.Handled
```

## type IO

IO aliases the runtime I/O contract used by the SDK app host.

```go
type IO = runtime.IO
```

## type InvocationContext

InvocationContext aliases the metadata that accompanies a decoded invocation.

```go
type InvocationContext = runtime.InvocationContext
```

## type Logger

Logger aliases the structured logger interface accepted by the SDK app host.

```go
type Logger = runtime.Logger
```

## type MaturityLevel

MaturityLevel aliases the API maturity enum exposed by support metadata.

```go
type MaturityLevel = runtime.MaturityLevel
```

## type Middleware

Middleware aliases the SDK middleware function signature.

```go
type Middleware = runtime.Middleware
```

## type Next

Next aliases the middleware continuation function.

```go
type Next = runtime.Next
```

## type NopLogger

NopLogger aliases the logger implementation that drops all log records.

```go
type NopLogger = runtime.NopLogger
```

## type Result

Result aliases the low\-level runtime result written back to the host process.

```go
type Result = runtime.Result
```

## type SupportEntry

SupportEntry aliases a generated public support\-matrix row.

```go
type SupportEntry = runtime.SupportEntry
```

### func Supported

```go
func Supported() []SupportEntry
```

Supported returns a copy of the generated public support matrix entries.

## type SupportStatus

SupportStatus aliases the support\-level enum used by generated support entries.

```go
type SupportStatus = runtime.SupportStatus
```

## type TransportMode

TransportMode aliases the runtime transport mode enum for supported hooks.

```go
type TransportMode = runtime.TransportMode
```
