---
title: "codex"
description: "Generated Go SDK package reference for github.com/777genius/plugin-kit-ai/sdk/codex"
canonicalId: "go-package:github.com/777genius/plugin-kit-ai/sdk/codex"
surface: "go-sdk"
section: "api"
locale: "en"
generated: true
editLink: false
stability: "public-stable"
maturity: "stable"
sourceRef: "sdk/codex"
translationRequired: false
---
<DocMetaCard surface="go-sdk" stability="public-stable" maturity="stable" source-ref="sdk/codex" source-href="https://github.com/777genius/plugin-kit-ai/tree/main/sdk/codex" />

# codex

Generated from the public Go package via gomarkdoc.

**Import path:** `github.com/777genius/plugin-kit-ai/sdk/codex`

```go
import "github.com/777genius/plugin-kit-ai/sdk/codex"
```

Package codex exposes typed public event inputs, responses, and registrars for Codex runtime integrations.

## Index

- func RegisterCustomJSON\[T any\]\(r \*Registrar, eventName string, fn func\(\*T\) \*Response\) error
- type NotifyEvent
  - func \(e \*NotifyEvent\) RawJSON\(\) json.RawMessage
- type PermissionRequestEvent
- type PreToolUseEvent
- type Registrar
  - func NewRegistrar\(backend runtime.RegistrarBackend\) \*Registrar
  - func \(r \*Registrar\) OnNotify\(fn func\(\*NotifyEvent\) \*Response\)
  - func \(r \*Registrar\) OnPermissionRequest\(fn func\(\*PermissionRequestEvent\) \*Response\)
  - func \(r \*Registrar\) OnPreToolUse\(fn func\(\*PreToolUseEvent\) \*Response\)
  - func \(r \*Registrar\) OnStop\(fn func\(\*StopEvent\) \*Response\)
  - func \(r \*Registrar\) OnSubagentStop\(fn func\(\*SubagentStopEvent\) \*Response\)
- type Response
  - func Continue\(\) \*Response
- type StopEvent
- type SubagentStopEvent


## func RegisterCustomJSON

```go
func RegisterCustomJSONT any *Response) error
```

RegisterCustomJSON registers an experimental future Codex hook whose payload is delivered as a JSON argv argument. The handler remains fully typed.

## type NotifyEvent

NotifyEvent is the decoded Codex notify payload and its raw JSON form.

```go
type NotifyEvent struct {
    // Raw keeps the original notify payload as it was received from argv JSON.
    Raw json.RawMessage
    // Client identifies the Codex client variant that emitted the event.
    Client string
}
```

### func \(\*NotifyEvent\) RawJSON

```go
func (e *NotifyEvent) RawJSON() json.RawMessage
```

RawJSON returns the original JSON payload for pass\-through or custom decoding.

## type PermissionRequestEvent

PermissionRequestEvent is the Codex PermissionRequest hook input \(decoded from stdin JSON\). ToolInput stays raw JSON for typed consumers.

```go
type PermissionRequestEvent = internalcodex.PermissionRequestInput
```

## type PreToolUseEvent

PreToolUseEvent is the Codex PreToolUse hook input \(decoded from stdin JSON\). ToolInput stays raw JSON for typed consumers.

```go
type PreToolUseEvent = internalcodex.PreToolUseInput
```

## type Registrar

Registrar registers public Codex event handlers on a root SDK app.

```go
type Registrar struct {
    // contains filtered or unexported fields
}
```

### func NewRegistrar

```go
func NewRegistrar(backend runtime.RegistrarBackend) *Registrar
```

NewRegistrar builds a Codex registrar on top of the shared runtime backend.

### func \(\*Registrar\) OnNotify

```go
func (r *Registrar) OnNotify(fn func(*NotifyEvent) *Response)
```

OnNotify registers a handler for the Codex Notify.

### func \(\*Registrar\) OnPermissionRequest

```go
func (r *Registrar) OnPermissionRequest(fn func(*PermissionRequestEvent) *Response)
```

OnPermissionRequest registers a handler for the Codex PermissionRequest.

### func \(\*Registrar\) OnPreToolUse

```go
func (r *Registrar) OnPreToolUse(fn func(*PreToolUseEvent) *Response)
```

OnPreToolUse registers a handler for the Codex PreToolUse.

### func \(\*Registrar\) OnStop

```go
func (r *Registrar) OnStop(fn func(*StopEvent) *Response)
```

OnStop registers a handler for the Codex Stop.

### func \(\*Registrar\) OnSubagentStop

```go
func (r *Registrar) OnSubagentStop(fn func(*SubagentStopEvent) *Response)
```

OnSubagentStop registers a handler for the Codex SubagentStop.

## type Response

Response represents a successful acknowledgement of a Codex event \(Notify and the observation\-style lifecycle hooks Stop, SubagentStop, PreToolUse, and PermissionRequest\).

```go
type Response struct{}
```

### func Continue

```go
func Continue() *Response
```

Continue acknowledges the event and exits successfully with empty output.

## type StopEvent

StopEvent is the Codex Stop lifecycle hook input \(decoded from stdin JSON\). Field names follow the Codex hooks schema; wire uses snake\_case via the platform adapter.

```go
type StopEvent = internalcodex.StopInput
```

## type SubagentStopEvent

SubagentStopEvent is the Codex SubagentStop hook input \(decoded from stdin JSON\). It carries the Stop fields plus subagent identity.

```go
type SubagentStopEvent = internalcodex.SubagentStopInput
```
