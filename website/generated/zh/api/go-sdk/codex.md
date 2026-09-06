---
title: "codex"
description: "生成的 Go SDK package 参考 for github.com/777genius/plugin-kit-ai/sdk/codex"
canonicalId: "go-package:github.com/777genius/plugin-kit-ai/sdk/codex"
surface: "go-sdk"
section: "api"
locale: "zh"
generated: true
editLink: false
stability: "public-stable"
maturity: "stable"
sourceRef: "sdk/codex"
translationRequired: false
---
<DocMetaCard surface="go-sdk" stability="public-stable" maturity="stable" source-ref="sdk/codex" source-href="https://github.com/777genius/universal-agent-plugins/tree/main/sdk/codex" />

# codex

通过 gomarkdoc 从公开 Go package 生成。

**导入路径:** `github.com/777genius/plugin-kit-ai/sdk/codex`

```go
import "github.com/777genius/plugin-kit-ai/sdk/codex"
```

Package codex exposes typed public event inputs, responses, and registrars for Codex runtime integrations.

## Index

- func RegisterCustomJSON\[T any\]\(r \*Registrar, eventName string, fn func\(\*T\) \*Response\) error
- type NotifyEvent
  - func \(e \*NotifyEvent\) RawJSON\(\) json.RawMessage
- type Registrar
  - func NewRegistrar\(backend runtime.RegistrarBackend\) \*Registrar
  - func \(r \*Registrar\) OnNotify\(fn func\(\*NotifyEvent\) \*Response\)
- type Response
  - func Continue\(\) \*Response


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

## type Response

Response represents a successful Codex notify acknowledgement.

```go
type Response struct{}
```

### func Continue

```go
func Continue() *Response
```

Continue acknowledges the notify event and exits successfully.
