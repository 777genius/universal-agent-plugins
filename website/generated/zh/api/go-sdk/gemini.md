---
title: "gemini"
description: "生成的 Go SDK package 参考 for github.com/777genius/plugin-kit-ai/sdk/gemini"
canonicalId: "go-package:github.com/777genius/plugin-kit-ai/sdk/gemini"
surface: "go-sdk"
section: "api"
locale: "zh"
generated: true
editLink: false
stability: "public-stable"
maturity: "stable"
sourceRef: "sdk/gemini"
translationRequired: false
---
<DocMetaCard surface="go-sdk" stability="public-stable" maturity="stable" source-ref="sdk/gemini" source-href="https://github.com/777genius/universal-agent-plugins/tree/main/sdk/gemini" />

# gemini

通过 gomarkdoc 从公开 Go package 生成。

**导入路径:** `github.com/777genius/plugin-kit-ai/sdk/gemini`

```go
import "github.com/777genius/plugin-kit-ai/sdk/gemini"
```

Package gemini exposes typed public Gemini hook inputs, responses, and registrars for the production\-ready Gemini Go runtime lane, including the current 9\-hook runtime surface.

## Index

- type AfterAgentEvent
- type AfterAgentResponse
  - func AfterAgentAllow\(\) \*AfterAgentResponse
  - func AfterAgentClearContext\(\) \*AfterAgentResponse
  - func AfterAgentContinue\(\) \*AfterAgentResponse
  - func AfterAgentDeny\(reason string\) \*AfterAgentResponse
  - func AfterAgentStop\(reason string\) \*AfterAgentResponse
- type AfterModelEvent
- type AfterModelResponse
  - func AfterModelContinue\(\) \*AfterModelResponse
  - func AfterModelDeny\(reason string\) \*AfterModelResponse
  - func AfterModelReplaceResponse\(response json.RawMessage\) \*AfterModelResponse
  - func AfterModelReplaceResponseValue\(v any\) \(\*AfterModelResponse, error\)
  - func AfterModelStop\(reason string\) \*AfterModelResponse
- type AfterToolEvent
- type AfterToolResponse
  - func AfterToolAddContext\(context string\) \*AfterToolResponse
  - func AfterToolAllow\(\) \*AfterToolResponse
  - func AfterToolContinue\(\) \*AfterToolResponse
  - func AfterToolDeny\(reason string\) \*AfterToolResponse
  - func AfterToolStop\(reason string\) \*AfterToolResponse
  - func AfterToolTailCall\(name string, args json.RawMessage\) \*AfterToolResponse
  - func AfterToolTailCallValue\(name string, args any\) \(\*AfterToolResponse, error\)
- type BeforeAgentEvent
- type BeforeAgentResponse
  - func BeforeAgentAddContext\(context string\) \*BeforeAgentResponse
  - func BeforeAgentAllow\(\) \*BeforeAgentResponse
  - func BeforeAgentContinue\(\) \*BeforeAgentResponse
  - func BeforeAgentDeny\(reason string\) \*BeforeAgentResponse
  - func BeforeAgentStop\(reason string\) \*BeforeAgentResponse
- type BeforeModelEvent
- type BeforeModelResponse
  - func BeforeModelContinue\(\) \*BeforeModelResponse
  - func BeforeModelDeny\(reason string\) \*BeforeModelResponse
  - func BeforeModelOverrideRequest\(request json.RawMessage\) \*BeforeModelResponse
  - func BeforeModelOverrideRequestValue\(v any\) \(\*BeforeModelResponse, error\)
  - func BeforeModelSyntheticResponse\(response json.RawMessage\) \*BeforeModelResponse
  - func BeforeModelSyntheticResponseValue\(v any\) \(\*BeforeModelResponse, error\)
- type BeforeToolEvent
- type BeforeToolResponse
  - func BeforeToolAllow\(\) \*BeforeToolResponse
  - func BeforeToolContinue\(\) \*BeforeToolResponse
  - func BeforeToolDeny\(reason string\) \*BeforeToolResponse
  - func BeforeToolRewriteInput\(input json.RawMessage\) \*BeforeToolResponse
  - func BeforeToolRewriteInputValue\(v any\) \(\*BeforeToolResponse, error\)
  - func BeforeToolStop\(reason string\) \*BeforeToolResponse
- type BeforeToolSelectionEvent
- type BeforeToolSelectionResponse
  - func BeforeToolSelectionAllowOnly\(allowedFunctionNames ...string\) \*BeforeToolSelectionResponse
  - func BeforeToolSelectionConfig\(mode ToolMode, allowedFunctionNames ...string\) \*BeforeToolSelectionResponse
  - func BeforeToolSelectionContinue\(\) \*BeforeToolSelectionResponse
  - func BeforeToolSelectionDisableAll\(\) \*BeforeToolSelectionResponse
  - func BeforeToolSelectionForceAny\(allowedFunctionNames ...string\) \*BeforeToolSelectionResponse
  - func BeforeToolSelectionForceAuto\(allowedFunctionNames ...string\) \*BeforeToolSelectionResponse
  - func BeforeToolSelectionQuiet\(\) \*BeforeToolSelectionResponse
- type CommonResponse
- type Registrar
  - func NewRegistrar\(backend runtime.RegistrarBackend\) \*Registrar
  - func \(r \*Registrar\) OnAfterAgent\(fn func\(\*AfterAgentEvent\) \*AfterAgentResponse\)
  - func \(r \*Registrar\) OnAfterModel\(fn func\(\*AfterModelEvent\) \*AfterModelResponse\)
  - func \(r \*Registrar\) OnAfterTool\(fn func\(\*AfterToolEvent\) \*AfterToolResponse\)
  - func \(r \*Registrar\) OnBeforeAgent\(fn func\(\*BeforeAgentEvent\) \*BeforeAgentResponse\)
  - func \(r \*Registrar\) OnBeforeModel\(fn func\(\*BeforeModelEvent\) \*BeforeModelResponse\)
  - func \(r \*Registrar\) OnBeforeTool\(fn func\(\*BeforeToolEvent\) \*BeforeToolResponse\)
  - func \(r \*Registrar\) OnBeforeToolSelection\(fn func\(\*BeforeToolSelectionEvent\) \*BeforeToolSelectionResponse\)
  - func \(r \*Registrar\) OnSessionEnd\(fn func\(\*SessionEndEvent\) \*SessionEndResponse\)
  - func \(r \*Registrar\) OnSessionStart\(fn func\(\*SessionStartEvent\) \*SessionStartResponse\)
- type SessionEndEvent
- type SessionEndResponse
  - func SessionEndContinue\(\) \*SessionEndResponse
  - func SessionEndMessage\(message string\) \*SessionEndResponse
- type SessionStartEvent
- type SessionStartResponse
  - func SessionStartAddContext\(context string\) \*SessionStartResponse
  - func SessionStartContinue\(\) \*SessionStartResponse
  - func SessionStartMessage\(message string\) \*SessionStartResponse
- type TailToolCallRequest
- type ToolMode


## type AfterAgentEvent

AfterAgentEvent is the Gemini AfterAgent hook input.

```go
type AfterAgentEvent = internalgemini.AfterAgentInput
```

## type AfterAgentResponse

AfterAgentResponse is the AfterAgent response type.

```go
type AfterAgentResponse struct {
    CommonResponse
    ClearContext bool
}
```

### func AfterAgentAllow

```go
func AfterAgentAllow() *AfterAgentResponse
```

AfterAgentAllow returns an explicit allow decision for AfterAgent.

### func AfterAgentClearContext

```go
func AfterAgentClearContext() *AfterAgentResponse
```

AfterAgentClearContext clears LLM conversation memory while preserving the UI display.

### func AfterAgentContinue

```go
func AfterAgentContinue() *AfterAgentResponse
```

AfterAgentContinue returns an explicit no\-op AfterAgent response.

### func AfterAgentDeny

```go
func AfterAgentDeny(reason string) *AfterAgentResponse
```

AfterAgentDeny rejects the response and requests a retry.

### func AfterAgentStop

```go
func AfterAgentStop(reason string) *AfterAgentResponse
```

AfterAgentStop stops the session without triggering a retry.

## type AfterModelEvent

AfterModelEvent is the Gemini AfterModel hook input.

```go
type AfterModelEvent = internalgemini.AfterModelInput
```

## type AfterModelResponse

AfterModelResponse is the AfterModel response type.

```go
type AfterModelResponse struct {
    CommonResponse
    LLMResponse json.RawMessage
}
```

### func AfterModelContinue

```go
func AfterModelContinue() *AfterModelResponse
```

AfterModelContinue returns an explicit no\-op AfterModel response.

### func AfterModelDeny

```go
func AfterModelDeny(reason string) *AfterModelResponse
```

AfterModelDeny blocks the model result with a deny decision.

### func AfterModelReplaceResponse

```go
func AfterModelReplaceResponse(response json.RawMessage) *AfterModelResponse
```

AfterModelReplaceResponse continues with a rewritten llm\_response payload.

### func AfterModelReplaceResponseValue

```go
func AfterModelReplaceResponseValue(v any) (*AfterModelResponse, error)
```

AfterModelReplaceResponseValue marshals a replacement llm\_response object for Gemini AfterModel hooks.

### func AfterModelStop

```go
func AfterModelStop(reason string) *AfterModelResponse
```

AfterModelStop stops the entire Gemini agent loop immediately.

## type AfterToolEvent

AfterToolEvent is the Gemini AfterTool hook input.

```go
type AfterToolEvent = internalgemini.AfterToolInput
```

## type AfterToolResponse

AfterToolResponse is the AfterTool response type.

```go
type AfterToolResponse struct {
    CommonResponse
    AdditionalContext   string
    TailToolCallRequest *TailToolCallRequest
}
```

### func AfterToolAddContext

```go
func AfterToolAddContext(context string) *AfterToolResponse
```

AfterToolAddContext appends additional text to the tool result sent back to the agent.

### func AfterToolAllow

```go
func AfterToolAllow() *AfterToolResponse
```

AfterToolAllow returns an explicit allow decision for AfterTool.

### func AfterToolContinue

```go
func AfterToolContinue() *AfterToolResponse
```

AfterToolContinue returns an explicit no\-op AfterTool response.

### func AfterToolDeny

```go
func AfterToolDeny(reason string) *AfterToolResponse
```

AfterToolDeny blocks the follow\-up path with a deny decision.

### func AfterToolStop

```go
func AfterToolStop(reason string) *AfterToolResponse
```

AfterToolStop stops the entire Gemini agent loop after tool execution.

### func AfterToolTailCall

```go
func AfterToolTailCall(name string, args json.RawMessage) *AfterToolResponse
```

AfterToolTailCall requests an immediate follow\-up tool invocation.

### func AfterToolTailCallValue

```go
func AfterToolTailCallValue(name string, args any) (*AfterToolResponse, error)
```

AfterToolTailCallValue marshals a typed follow\-up tool request. Gemini expects tailToolCallRequest.args to be a JSON object, so non\-object values return an error.

## type BeforeAgentEvent

BeforeAgentEvent is the Gemini BeforeAgent hook input.

```go
type BeforeAgentEvent = internalgemini.BeforeAgentInput
```

## type BeforeAgentResponse

BeforeAgentResponse is the BeforeAgent response type.

```go
type BeforeAgentResponse struct {
    CommonResponse
    AdditionalContext string
}
```

### func BeforeAgentAddContext

```go
func BeforeAgentAddContext(context string) *BeforeAgentResponse
```

BeforeAgentAddContext appends additional context to the current turn prompt.

### func BeforeAgentAllow

```go
func BeforeAgentAllow() *BeforeAgentResponse
```

BeforeAgentAllow returns an explicit allow decision for BeforeAgent.

### func BeforeAgentContinue

```go
func BeforeAgentContinue() *BeforeAgentResponse
```

BeforeAgentContinue returns an explicit no\-op BeforeAgent response.

### func BeforeAgentDeny

```go
func BeforeAgentDeny(reason string) *BeforeAgentResponse
```

BeforeAgentDeny blocks the turn and discards the user's prompt from history.

### func BeforeAgentStop

```go
func BeforeAgentStop(reason string) *BeforeAgentResponse
```

BeforeAgentStop aborts the current turn but keeps the user's prompt in history, matching Gemini's continue=false semantics.

## type BeforeModelEvent

BeforeModelEvent is the Gemini BeforeModel hook input.

```go
type BeforeModelEvent = internalgemini.BeforeModelInput
```

## type BeforeModelResponse

BeforeModelResponse is the BeforeModel response type.

```go
type BeforeModelResponse struct {
    CommonResponse
    LLMRequest  json.RawMessage
    LLMResponse json.RawMessage
}
```

### func BeforeModelContinue

```go
func BeforeModelContinue() *BeforeModelResponse
```

BeforeModelContinue returns an explicit no\-op BeforeModel response.

### func BeforeModelDeny

```go
func BeforeModelDeny(reason string) *BeforeModelResponse
```

BeforeModelDeny blocks the LLM request with a deny decision.

### func BeforeModelOverrideRequest

```go
func BeforeModelOverrideRequest(request json.RawMessage) *BeforeModelResponse
```

BeforeModelOverrideRequest continues with a rewritten llm\_request payload.

### func BeforeModelOverrideRequestValue

```go
func BeforeModelOverrideRequestValue(v any) (*BeforeModelResponse, error)
```

BeforeModelOverrideRequestValue marshals a replacement llm\_request object for Gemini BeforeModel hooks.

### func BeforeModelSyntheticResponse

```go
func BeforeModelSyntheticResponse(response json.RawMessage) *BeforeModelResponse
```

BeforeModelSyntheticResponse short\-circuits the LLM request with a synthetic llm\_response payload.

### func BeforeModelSyntheticResponseValue

```go
func BeforeModelSyntheticResponseValue(v any) (*BeforeModelResponse, error)
```

BeforeModelSyntheticResponseValue marshals a synthetic llm\_response object for Gemini BeforeModel hooks.

## type BeforeToolEvent

BeforeToolEvent is the Gemini BeforeTool hook input.

```go
type BeforeToolEvent = internalgemini.BeforeToolInput
```

## type BeforeToolResponse

BeforeToolResponse is the BeforeTool response type.

```go
type BeforeToolResponse struct {
    CommonResponse
    ToolInput json.RawMessage
}
```

### func BeforeToolAllow

```go
func BeforeToolAllow() *BeforeToolResponse
```

BeforeToolAllow returns an explicit allow decision for BeforeTool.

### func BeforeToolContinue

```go
func BeforeToolContinue() *BeforeToolResponse
```

BeforeToolContinue returns an explicit no\-op BeforeTool response.

### func BeforeToolDeny

```go
func BeforeToolDeny(reason string) *BeforeToolResponse
```

BeforeToolDeny blocks the tool invocation with a deny decision.

### func BeforeToolRewriteInput

```go
func BeforeToolRewriteInput(input json.RawMessage) *BeforeToolResponse
```

BeforeToolRewriteInput continues with a rewritten tool\_input payload.

### func BeforeToolRewriteInputValue

```go
func BeforeToolRewriteInputValue(v any) (*BeforeToolResponse, error)
```

BeforeToolRewriteInputValue marshals a replacement tool\_input object for Gemini BeforeTool hooks. Gemini expects hookSpecificOutput.tool\_input to be a JSON object, so non\-object values return an error.

### func BeforeToolStop

```go
func BeforeToolStop(reason string) *BeforeToolResponse
```

BeforeToolStop stops the entire Gemini agent loop before the tool executes.

## type BeforeToolSelectionEvent

BeforeToolSelectionEvent is the Gemini BeforeToolSelection hook input.

```go
type BeforeToolSelectionEvent = internalgemini.BeforeToolSelectionInput
```

## type BeforeToolSelectionResponse

BeforeToolSelectionResponse is the Gemini BeforeToolSelection response type.

```go
type BeforeToolSelectionResponse struct {
    SuppressOutput       bool
    Mode                 ToolMode
    AllowedFunctionNames []string
}
```

### func BeforeToolSelectionAllowOnly

```go
func BeforeToolSelectionAllowOnly(allowedFunctionNames ...string) *BeforeToolSelectionResponse
```

BeforeToolSelectionAllowOnly restricts Gemini tool selection to the provided allowlist by using ANY mode, which is the vendor\-accepted shape for allowedFunctionNames.

### func BeforeToolSelectionConfig

```go
func BeforeToolSelectionConfig(mode ToolMode, allowedFunctionNames ...string) *BeforeToolSelectionResponse
```

BeforeToolSelectionConfig applies a tool selection mode. Gemini currently accepts allowedFunctionNames only together with ANY mode.

### func BeforeToolSelectionContinue

```go
func BeforeToolSelectionContinue() *BeforeToolSelectionResponse
```

BeforeToolSelectionContinue returns an explicit no\-op BeforeToolSelection response.

### func BeforeToolSelectionDisableAll

```go
func BeforeToolSelectionDisableAll() *BeforeToolSelectionResponse
```

BeforeToolSelectionDisableAll disables all tools for the current decision step.

### func BeforeToolSelectionForceAny

```go
func BeforeToolSelectionForceAny(allowedFunctionNames ...string) *BeforeToolSelectionResponse
```

BeforeToolSelectionForceAny requires Gemini to pick at least one tool and optionally narrows the candidate set with an allowlist.

### func BeforeToolSelectionForceAuto

```go
func BeforeToolSelectionForceAuto(allowedFunctionNames ...string) *BeforeToolSelectionResponse
```

BeforeToolSelectionForceAuto explicitly restores AUTO tool mode. Gemini does not currently accept allowedFunctionNames outside ANY mode, so any optional allowlist arguments are ignored.

### func BeforeToolSelectionQuiet

```go
func BeforeToolSelectionQuiet() *BeforeToolSelectionResponse
```

BeforeToolSelectionQuiet suppresses Gemini's internal hook metadata for the current tool\-selection step without changing toolConfig.

## type CommonResponse

CommonResponse contains fields shared by Gemini's synchronous hook envelope.

```go
type CommonResponse struct {
    Continue       *bool
    SuppressOutput bool
    StopReason     string
    Decision       string
    Reason         string
    SystemMessage  string
}
```

## type Registrar

Registrar registers public Gemini hook handlers on a root SDK app.

```go
type Registrar struct {
    // contains filtered or unexported fields
}
```

### func NewRegistrar

```go
func NewRegistrar(backend runtime.RegistrarBackend) *Registrar
```

NewRegistrar builds a Gemini registrar on top of the shared runtime backend.

### func \(\*Registrar\) OnAfterAgent

```go
func (r *Registrar) OnAfterAgent(fn func(*AfterAgentEvent) *AfterAgentResponse)
```

OnAfterAgent registers a handler for the gemini AfterAgent.

### func \(\*Registrar\) OnAfterModel

```go
func (r *Registrar) OnAfterModel(fn func(*AfterModelEvent) *AfterModelResponse)
```

OnAfterModel registers a handler for the gemini AfterModel.

### func \(\*Registrar\) OnAfterTool

```go
func (r *Registrar) OnAfterTool(fn func(*AfterToolEvent) *AfterToolResponse)
```

OnAfterTool registers a handler for the gemini AfterTool.

### func \(\*Registrar\) OnBeforeAgent

```go
func (r *Registrar) OnBeforeAgent(fn func(*BeforeAgentEvent) *BeforeAgentResponse)
```

OnBeforeAgent registers a handler for the gemini BeforeAgent.

### func \(\*Registrar\) OnBeforeModel

```go
func (r *Registrar) OnBeforeModel(fn func(*BeforeModelEvent) *BeforeModelResponse)
```

OnBeforeModel registers a handler for the gemini BeforeModel.

### func \(\*Registrar\) OnBeforeTool

```go
func (r *Registrar) OnBeforeTool(fn func(*BeforeToolEvent) *BeforeToolResponse)
```

OnBeforeTool registers a handler for the gemini BeforeTool.

### func \(\*Registrar\) OnBeforeToolSelection

```go
func (r *Registrar) OnBeforeToolSelection(fn func(*BeforeToolSelectionEvent) *BeforeToolSelectionResponse)
```

OnBeforeToolSelection registers a handler for the gemini BeforeToolSelection.

### func \(\*Registrar\) OnSessionEnd

```go
func (r *Registrar) OnSessionEnd(fn func(*SessionEndEvent) *SessionEndResponse)
```

OnSessionEnd registers a handler for the gemini SessionEnd.

### func \(\*Registrar\) OnSessionStart

```go
func (r *Registrar) OnSessionStart(fn func(*SessionStartEvent) *SessionStartResponse)
```

OnSessionStart registers a handler for the gemini SessionStart.

## type SessionEndEvent

SessionEndEvent is the Gemini SessionEnd hook input.

```go
type SessionEndEvent = internalgemini.SessionEndInput
```

## type SessionEndResponse

SessionEndResponse is the SessionEnd response type.

```go
type SessionEndResponse = CommonResponse
```

### func SessionEndContinue

```go
func SessionEndContinue() *SessionEndResponse
```

SessionEndContinue returns an explicit no\-op SessionEnd response.

### func SessionEndMessage

```go
func SessionEndMessage(message string) *SessionEndResponse
```

SessionEndMessage emits a systemMessage during SessionEnd.

## type SessionStartEvent

SessionStartEvent is the Gemini SessionStart hook input.

```go
type SessionStartEvent = internalgemini.SessionStartInput
```

## type SessionStartResponse

SessionStartResponse is the SessionStart response type.

```go
type SessionStartResponse struct {
    CommonResponse
    AdditionalContext string
}
```

### func SessionStartAddContext

```go
func SessionStartAddContext(context string) *SessionStartResponse
```

SessionStartAddContext appends additional context during SessionStart.

### func SessionStartContinue

```go
func SessionStartContinue() *SessionStartResponse
```

SessionStartContinue returns an explicit no\-op SessionStart response.

### func SessionStartMessage

```go
func SessionStartMessage(message string) *SessionStartResponse
```

SessionStartMessage emits a systemMessage during SessionStart.

## type TailToolCallRequest

TailToolCallRequest requests an immediate follow\-up tool execution from an AfterTool hook.

```go
type TailToolCallRequest struct {
    Name string
    Args json.RawMessage
}
```

## type ToolMode

ToolMode configures Gemini BeforeToolSelection tool routing.

```go
type ToolMode string
```

```go
const (
    ToolModeAuto ToolMode = "AUTO"
    ToolModeAny  ToolMode = "ANY"
    ToolModeNone ToolMode = "NONE"
)
```
