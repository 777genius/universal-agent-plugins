---
title: "claude"
description: "Generated Go SDK package reference for github.com/777genius/plugin-kit-ai/sdk/claude"
canonicalId: "go-package:github.com/777genius/plugin-kit-ai/sdk/claude"
surface: "go-sdk"
section: "api"
locale: "ru"
generated: true
editLink: false
stability: "public-stable"
maturity: "stable"
sourceRef: "sdk/claude"
translationRequired: false
---
<DocMetaCard surface="go-sdk" stability="public-stable" maturity="stable" source-ref="sdk/claude" source-href="https://github.com/777genius/universal-agent-plugins/tree/main/sdk/claude" />

# claude

Сгенерировано из публичного Go-пакета через gomarkdoc.

**Путь импорта:** `github.com/777genius/plugin-kit-ai/sdk/claude`

```go
import "github.com/777genius/plugin-kit-ai/sdk/claude"
```

Пакет `claude` публикует типизированные входные hooks, ответы и регистраторы для runtime-интеграций Claude.

## Оглавление

- func OutcomeFromResponse\(r \*Response\) internalclaude.StopOutcome
- func PreToolOutcomeFromResponse\(r \*PreToolResponse\) internalclaude.PreToolUseOutcome
- func RegisterCustomCommonJSON\[T any\]\(r \*Registrar, eventName string, fn func\(\*T\) \*CommonResponse\) error
- func RegisterCustomContextJSON\[T any\]\(r \*Registrar, eventName string, fn func\(\*T\) \*ContextResponse\) error
- func RegisterCustomPermissionRequestJSON\[T any\]\(r \*Registrar, eventName string, fn func\(\*T\) \*PermissionRequestResponse\) error
- func RegisterCustomPostToolUseJSON\[T any\]\(r \*Registrar, eventName string, fn func\(\*T\) \*PostToolUseResponse\) error
- func UserPromptOutcomeFromResponse\(r \*UserPromptResponse\) internalclaude.UserPromptSubmitOutcome
- type CommonResponse
- type ConfigChangeEvent
- type ConfigChangeResponse
- type ContextResponse
- type NotificationEvent
- type NotificationResponse
- type PermissionBehavior
- type PermissionDecision
- type PermissionRequestEvent
- type PermissionRequestResponse
  - func PermissionApprove\(\) \*PermissionRequestResponse
  - func PermissionApproveWithUpdates\(input json.RawMessage, updates \[\]PermissionUpdate\) \*PermissionRequestResponse
  - func PermissionBlock\(message string, interrupt bool\) \*PermissionRequestResponse
- type PermissionRuleValue
- type PermissionUpdate
- type PostToolUseEvent
- type PostToolUseFailureEvent
- type PostToolUseFailureResponse
- type PostToolUseResponse
- type PreCompactEvent
- type PreCompactResponse
- type PreToolResponse
  - func PreToolAllow\(\) \*PreToolResponse
  - func PreToolAllowWithReason\(reason string\) \*PreToolResponse
  - func PreToolAsk\(reason string\) \*PreToolResponse
  - func PreToolBlockExit2\(reason string\) \*PreToolResponse
  - func PreToolDeny\(reason string\) \*PreToolResponse
- type PreToolUseEvent
- type Registrar
  - func NewRegistrar\(backend runtime.RegistrarBackend\) \*Registrar
  - func \(r \*Registrar\) OnConfigChange\(fn func\(\*ConfigChangeEvent\) \*ConfigChangeResponse\)
  - func \(r \*Registrar\) OnNotification\(fn func\(\*NotificationEvent\) \*NotificationResponse\)
  - func \(r \*Registrar\) OnPermissionRequest\(fn func\(\*PermissionRequestEvent\) \*PermissionRequestResponse\)
  - func \(r \*Registrar\) OnPostToolUse\(fn func\(\*PostToolUseEvent\) \*PostToolUseResponse\)
  - func \(r \*Registrar\) OnPostToolUseFailure\(fn func\(\*PostToolUseFailureEvent\) \*PostToolUseFailureResponse\)
  - func \(r \*Registrar\) OnPreCompact\(fn func\(\*PreCompactEvent\) \*PreCompactResponse\)
  - func \(r \*Registrar\) OnPreToolUse\(fn func\(\*PreToolUseEvent\) \*PreToolResponse\)
  - func \(r \*Registrar\) OnSessionEnd\(fn func\(\*SessionEndEvent\) \*SessionEndResponse\)
  - func \(r \*Registrar\) OnSessionStart\(fn func\(\*SessionStartEvent\) \*SessionStartResponse\)
  - func \(r \*Registrar\) OnSetup\(fn func\(\*SetupEvent\) \*SetupResponse\)
  - func \(r \*Registrar\) OnStop\(fn func\(\*StopEvent\) \*Response\)
  - func \(r \*Registrar\) OnSubagentStart\(fn func\(\*SubagentStartEvent\) \*SubagentStartResponse\)
  - func \(r \*Registrar\) OnSubagentStop\(fn func\(\*SubagentStopEvent\) \*SubagentStopResponse\)
  - func \(r \*Registrar\) OnTaskCompleted\(fn func\(\*TaskCompletedEvent\) \*TaskCompletedResponse\)
  - func \(r \*Registrar\) OnTeammateIdle\(fn func\(\*TeammateIdleEvent\) \*TeammateIdleResponse\)
  - func \(r \*Registrar\) OnUserPromptSubmit\(fn func\(\*UserPromptEvent\) \*UserPromptResponse\)
  - func \(r \*Registrar\) OnWorktreeCreate\(fn func\(\*WorktreeCreateEvent\) \*WorktreeCreateResponse\)
  - func \(r \*Registrar\) OnWorktreeRemove\(fn func\(\*WorktreeRemoveEvent\) \*WorktreeRemoveResponse\)
- type Response
  - func Allow\(\) \*Response
  - func Block\(reason string\) \*Response
  - func BlockExit2\(reason string\) \*Response
  - func Continue\(reason string\) \*Response
  - func \(r \*Response\) AllowStop\(\) bool
  - func \(r \*Response\) BlockReason\(\) string
- type SessionEndEvent
- type SessionEndResponse
- type SessionStartEvent
- type SessionStartResponse
- type SetupEvent
- type SetupResponse
- type StopEvent
- type SubagentStartEvent
- type SubagentStartResponse
- type SubagentStopEvent
- type SubagentStopResponse
- type TaskCompletedEvent
- type TaskCompletedResponse
- type TeammateIdleEvent
- type TeammateIdleResponse
- type UserPromptEvent
- type UserPromptResponse
  - func UserPromptAllow\(\) \*UserPromptResponse
  - func UserPromptAllowWithContext\(context string\) \*UserPromptResponse
  - func UserPromptBlock\(reason string\) \*UserPromptResponse
  - func UserPromptBlockExit2\(reason string\) \*UserPromptResponse
- type WorktreeCreateEvent
- type WorktreeCreateResponse
- type WorktreeRemoveEvent
- type WorktreeRemoveResponse


## func OutcomeFromResponse

```go
func OutcomeFromResponse(r *Response) internalclaude.StopOutcome
```

OutcomeFromResponse maps a handler return value to a platform outcome \(nil → allow\).

## func PreToolOutcomeFromResponse

```go
func PreToolOutcomeFromResponse(r *PreToolResponse) internalclaude.PreToolUseOutcome
```

PreToolOutcomeFromResponse maps handler return to platform outcome \(nil → allow\).

## func RegisterCustomCommonJSON

```go
func RegisterCustomCommonJSONT any *CommonResponse) error
```

RegisterCustomCommonJSON registers an experimental Claude hook backed by stdin JSON and a common synchronous response family.

## func RegisterCustomContextJSON

```go
func RegisterCustomContextJSONT any *ContextResponse) error
```

RegisterCustomContextJSON registers an experimental Claude hook backed by stdin JSON and a context\-producing response family.

## func RegisterCustomPermissionRequestJSON

```go
func RegisterCustomPermissionRequestJSONT any *PermissionRequestResponse) error
```

RegisterCustomPermissionRequestJSON registers an experimental Claude hook backed by stdin JSON and the permission decision response family.

## func RegisterCustomPostToolUseJSON

```go
func RegisterCustomPostToolUseJSONT any *PostToolUseResponse) error
```

RegisterCustomPostToolUseJSON registers an experimental Claude hook backed by stdin JSON and the PostToolUse response family.

## func UserPromptOutcomeFromResponse

```go
func UserPromptOutcomeFromResponse(r *UserPromptResponse) internalclaude.UserPromptSubmitOutcome
```

UserPromptOutcomeFromResponse maps handler return to platform outcome \(nil → allow\).

## type CommonResponse

CommonResponse contains fields shared by Claude's common hook response envelope.

```go
type CommonResponse struct {
    // Continue overrides the continue flag for compatible hooks when non-nil.
    Continue *bool
    // SuppressOutput omits the usual hook output message when true.
    SuppressOutput bool
    // StopReason explains why processing should stop.
    StopReason string
    // Decision carries the Claude decision token such as approve or block.
    Decision string
    // Reason carries a human-readable reason for the decision.
    Reason string
    // SystemMessage injects a system message into Claude when supported.
    SystemMessage string
}
```

## type ConfigChangeEvent

ConfigChangeEvent is the Claude ConfigChange hook input.

```go
type ConfigChangeEvent = internalclaude.ConfigChangeInput
```

## type ConfigChangeResponse

ConfigChangeResponse is the response type for ConfigChange.

```go
type ConfigChangeResponse = CommonResponse
```

## type ContextResponse

ContextResponse extends CommonResponse with additional context text.

```go
type ContextResponse struct {
    CommonResponse
    // AdditionalContext appends hook-specific context visible to Claude.
    AdditionalContext string
}
```

## type NotificationEvent

NotificationEvent is the Claude Notification hook input.

```go
type NotificationEvent = internalclaude.NotificationInput
```

## type NotificationResponse

NotificationResponse is the response type for Notification.

```go
type NotificationResponse = ContextResponse
```

## type PermissionBehavior

PermissionBehavior enumerates allow or deny decisions for PermissionRequest.

```go
type PermissionBehavior = internalclaude.PermissionBehavior
```

```go
const (
    // PermissionAllow approves the pending action.
    PermissionAllow PermissionBehavior = internalclaude.PermissionAllow
    // PermissionDeny rejects the pending action.
    PermissionDeny PermissionBehavior = internalclaude.PermissionDeny
)
```

## type PermissionDecision

PermissionDecision describes the approval or denial returned from PermissionRequest.

```go
type PermissionDecision struct {
    // Behavior is either PermissionAllow or PermissionDeny.
    Behavior PermissionBehavior
    // UpdatedInput replaces the pending input before Claude continues.
    UpdatedInput json.RawMessage
    // UpdatedPermissions amends stored permission rules when approving.
    UpdatedPermissions []PermissionUpdate
    // Message explains the deny decision to the user.
    Message string
    // Interrupt asks Claude to interrupt the current flow after the message.
    Interrupt bool
}
```

## type PermissionRequestEvent

PermissionRequestEvent is the Claude PermissionRequest hook input.

```go
type PermissionRequestEvent = internalclaude.PermissionRequestInput
```

## type PermissionRequestResponse

PermissionRequestResponse extends CommonResponse with a permission decision.

```go
type PermissionRequestResponse struct {
    CommonResponse
    // Permission holds the allow or deny decision when one is returned.
    Permission *PermissionDecision
}
```

### func PermissionApprove

```go
func PermissionApprove() *PermissionRequestResponse
```

PermissionApprove returns a response that approves the requested action.

### func PermissionApproveWithUpdates

```go
func PermissionApproveWithUpdates(input json.RawMessage, updates []PermissionUpdate) *PermissionRequestResponse
```

PermissionApproveWithUpdates approves the action and sends updated input or rules.

### func PermissionBlock

```go
func PermissionBlock(message string, interrupt bool) *PermissionRequestResponse
```

PermissionBlock rejects the action with a message and interrupt choice.

## type PermissionRuleValue

PermissionRuleValue mirrors the supported rule value payload for permission updates.

```go
type PermissionRuleValue = internalclaude.PermissionRuleValue
```

## type PermissionUpdate

PermissionUpdate mirrors a single permission rule update returned to Claude.

```go
type PermissionUpdate = internalclaude.PermissionUpdate
```

## type PostToolUseEvent

PostToolUseEvent is the Claude PostToolUse hook input.

```go
type PostToolUseEvent = internalclaude.PostToolUseInput
```

## type PostToolUseFailureEvent

PostToolUseFailureEvent is the Claude PostToolUseFailure hook input.

```go
type PostToolUseFailureEvent = internalclaude.PostToolUseFailureInput
```

## type PostToolUseFailureResponse

PostToolUseFailureResponse is the response type for PostToolUseFailure.

```go
type PostToolUseFailureResponse = ContextResponse
```

## type PostToolUseResponse

PostToolUseResponse extends CommonResponse with tool output overrides.

```go
type PostToolUseResponse struct {
    CommonResponse
    // AdditionalContext appends hook-specific context visible to Claude.
    AdditionalContext string
    // UpdatedMCPToolOutput replaces the MCP tool output payload on the wire.
    UpdatedMCPToolOutput json.RawMessage
}
```

## type PreCompactEvent

PreCompactEvent is the Claude PreCompact hook input.

```go
type PreCompactEvent = internalclaude.PreCompactInput
```

## type PreCompactResponse

PreCompactResponse is the response type for PreCompact.

```go
type PreCompactResponse = CommonResponse
```

## type PreToolResponse

PreToolResponse is the hook outcome for PreToolUse \(hookSpecificOutput on wire\).

```go
type PreToolResponse struct {
    // содержит скрытые или неэкспортируемые поля
}
```

### func PreToolAllow

```go
func PreToolAllow() *PreToolResponse
```

PreToolAllow lets the tool run \(exit 0 \+ "\{\}" or optional reason for allow path\).

### func PreToolAllowWithReason

```go
func PreToolAllowWithReason(reason string) *PreToolResponse
```

PreToolAllowWithReason returns allow with permissionDecisionReason \(shown to user\).

### func PreToolAsk

```go
func PreToolAsk(reason string) *PreToolResponse
```

PreToolAsk prompts the user to confirm.

### func PreToolBlockExit2

```go
func PreToolBlockExit2(reason string) *PreToolResponse
```

PreToolBlockExit2 blocks the tool via exit 2; stderr carries reason to Claude.

### func PreToolDeny

```go
func PreToolDeny(reason string) *PreToolResponse
```

PreToolDeny blocks the tool with hookSpecificOutput deny.

## type PreToolUseEvent

PreToolUseEvent is the Claude Code PreToolUse hook input \(stdin JSON\).

```go
type PreToolUseEvent = internalclaude.PreToolUseInput
```

## type Registrar

Registrar registers public Claude hook handlers on a root SDK app.

```go
type Registrar struct {
    // contains filtered or unexported fields
}
```

### func NewRegistrar

```go
func NewRegistrar(backend runtime.RegistrarBackend) *Registrar
```

NewRegistrar builds a Claude registrar on top of the shared runtime backend.

### func \(\*Registrar\) OnConfigChange

```go
func (r *Registrar) OnConfigChange(fn func(*ConfigChangeEvent) *ConfigChangeResponse)
```

OnConfigChange registers a handler for the Claude ConfigChange.

### func \(\*Registrar\) OnNotification

```go
func (r *Registrar) OnNotification(fn func(*NotificationEvent) *NotificationResponse)
```

OnNotification registers a handler for the Claude Notification.

### func \(\*Registrar\) OnPermissionRequest

```go
func (r *Registrar) OnPermissionRequest(fn func(*PermissionRequestEvent) *PermissionRequestResponse)
```

OnPermissionRequest registers a handler for the Claude PermissionRequest.

### func \(\*Registrar\) OnPostToolUse

```go
func (r *Registrar) OnPostToolUse(fn func(*PostToolUseEvent) *PostToolUseResponse)
```

OnPostToolUse registers a handler for the Claude PostToolUse.

### func \(\*Registrar\) OnPostToolUseFailure

```go
func (r *Registrar) OnPostToolUseFailure(fn func(*PostToolUseFailureEvent) *PostToolUseFailureResponse)
```

OnPostToolUseFailure registers a handler for the Claude PostToolUseFailure.

### func \(\*Registrar\) OnPreCompact

```go
func (r *Registrar) OnPreCompact(fn func(*PreCompactEvent) *PreCompactResponse)
```

OnPreCompact registers a handler for the Claude PreCompact.

### func \(\*Registrar\) OnPreToolUse

```go
func (r *Registrar) OnPreToolUse(fn func(*PreToolUseEvent) *PreToolResponse)
```

OnPreToolUse registers a handler for the Claude PreToolUse.

### func \(\*Registrar\) OnSessionEnd

```go
func (r *Registrar) OnSessionEnd(fn func(*SessionEndEvent) *SessionEndResponse)
```

OnSessionEnd registers a handler for the Claude SessionEnd.

### func \(\*Registrar\) OnSessionStart

```go
func (r *Registrar) OnSessionStart(fn func(*SessionStartEvent) *SessionStartResponse)
```

OnSessionStart registers a handler for the Claude SessionStart.

### func \(\*Registrar\) OnSetup

```go
func (r *Registrar) OnSetup(fn func(*SetupEvent) *SetupResponse)
```

OnSetup registers a handler for the Claude Setup.

### func \(\*Registrar\) OnStop

```go
func (r *Registrar) OnStop(fn func(*StopEvent) *Response)
```

OnStop registers a handler for the Claude Stop.

### func \(\*Registrar\) OnSubagentStart

```go
func (r *Registrar) OnSubagentStart(fn func(*SubagentStartEvent) *SubagentStartResponse)
```

OnSubagentStart registers a handler for the Claude SubagentStart.

### func \(\*Registrar\) OnSubagentStop

```go
func (r *Registrar) OnSubagentStop(fn func(*SubagentStopEvent) *SubagentStopResponse)
```

OnSubagentStop registers a handler for the Claude SubagentStop.

### func \(\*Registrar\) OnTaskCompleted

```go
func (r *Registrar) OnTaskCompleted(fn func(*TaskCompletedEvent) *TaskCompletedResponse)
```

OnTaskCompleted registers a handler for the Claude TaskCompleted.

### func \(\*Registrar\) OnTeammateIdle

```go
func (r *Registrar) OnTeammateIdle(fn func(*TeammateIdleEvent) *TeammateIdleResponse)
```

OnTeammateIdle registers a handler for the Claude TeammateIdle.

### func \(\*Registrar\) OnUserPromptSubmit

```go
func (r *Registrar) OnUserPromptSubmit(fn func(*UserPromptEvent) *UserPromptResponse)
```

OnUserPromptSubmit registers a handler for the Claude UserPromptSubmit.

### func \(\*Registrar\) OnWorktreeCreate

```go
func (r *Registrar) OnWorktreeCreate(fn func(*WorktreeCreateEvent) *WorktreeCreateResponse)
```

OnWorktreeCreate registers a handler for the Claude WorktreeCreate.

### func \(\*Registrar\) OnWorktreeRemove

```go
func (r *Registrar) OnWorktreeRemove(fn func(*WorktreeRemoveEvent) *WorktreeRemoveResponse)
```

OnWorktreeRemove registers a handler for the Claude WorktreeRemove.

## type Response

Response is the hook outcome for Stop \(see Stop decision control in Claude Code hooks reference\).

```go
type Response struct {
    // contains filtered or unexported fields
}
```

### func Allow

```go
func Allow() *Response
```

Allow lets Claude stop \(omit decision / empty JSON object on wire\).

### func Block

```go
func Block(reason string) *Response
```

Block prevents Claude from stopping; maps to \{"decision":"block","reason":...\} with exit 0.

### func BlockExit2

```go
func BlockExit2(reason string) *Response
```

BlockExit2 requests a non\-zero exit without JSON stdout \(alternate path per Claude Stop docs\).

### func Continue

```go
func Continue(reason string) *Response
```

Continue is the same as Block for Stop: keep the session running \(decision block on wire\).

### func \(\*Response\) AllowStop

```go
func (r *Response) AllowStop() bool
```

AllowStop reports whether the handler allows the session to stop.

### func \(\*Response\) BlockReason

```go
func (r *Response) BlockReason() string
```

BlockReason is set when AllowStop is false \(JSON block path or exit\-2 path\).

## type SessionEndEvent

SessionEndEvent is the Claude SessionEnd hook input.

```go
type SessionEndEvent = internalclaude.SessionEndInput
```

## type SessionEndResponse

SessionEndResponse is the response type for SessionEnd.

```go
type SessionEndResponse = CommonResponse
```

## type SessionStartEvent

SessionStartEvent is the Claude SessionStart hook input.

```go
type SessionStartEvent = internalclaude.SessionStartInput
```

## type SessionStartResponse

SessionStartResponse is the response type for SessionStart.

```go
type SessionStartResponse = ContextResponse
```

## type SetupEvent

SetupEvent is the Claude Setup hook input.

```go
type SetupEvent = internalclaude.SetupInput
```

## type SetupResponse

SetupResponse is the response type for Setup.

```go
type SetupResponse = ContextResponse
```

## type StopEvent

StopEvent is the Claude Code Stop hook input \(decoded from stdin JSON\). Field names follow Claude docs; wire uses snake\_case via the platform adapter.

```go
type StopEvent = internalclaude.StopInput
```

## type SubagentStartEvent

SubagentStartEvent is the Claude SubagentStart hook input.

```go
type SubagentStartEvent = internalclaude.SubagentStartInput
```

## type SubagentStartResponse

SubagentStartResponse is the response type for SubagentStart.

```go
type SubagentStartResponse = ContextResponse
```

## type SubagentStopEvent

SubagentStopEvent is the Claude SubagentStop hook input.

```go
type SubagentStopEvent = internalclaude.SubagentStopInput
```

## type SubagentStopResponse

SubagentStopResponse is the response type for SubagentStop.

```go
type SubagentStopResponse = CommonResponse
```

## type TaskCompletedEvent

TaskCompletedEvent is the Claude TaskCompleted hook input.

```go
type TaskCompletedEvent = internalclaude.TaskCompletedInput
```

## type TaskCompletedResponse

TaskCompletedResponse is the response type for TaskCompleted.

```go
type TaskCompletedResponse = CommonResponse
```

## type TeammateIdleEvent

TeammateIdleEvent is the Claude TeammateIdle hook input.

```go
type TeammateIdleEvent = internalclaude.TeammateIdleInput
```

## type TeammateIdleResponse

TeammateIdleResponse is the response type for TeammateIdle.

```go
type TeammateIdleResponse = CommonResponse
```

## type UserPromptEvent

UserPromptEvent is the Claude Code UserPromptSubmit hook input.

```go
type UserPromptEvent = internalclaude.UserPromptSubmitInput
```

## type UserPromptResponse

UserPromptResponse is the hook outcome for UserPromptSubmit.

```go
type UserPromptResponse struct {
    // contains filtered or unexported fields
}
```

### func UserPromptAllow

```go
func UserPromptAllow() *UserPromptResponse
```

UserPromptAllow lets the prompt proceed.

### func UserPromptAllowWithContext

```go
func UserPromptAllowWithContext(context string) *UserPromptResponse
```

UserPromptAllowWithContext adds additionalContext on exit 0 \(JSON\).

### func UserPromptBlock

```go
func UserPromptBlock(reason string) *UserPromptResponse
```

UserPromptBlock blocks the prompt with decision/reason \(exit 0 \+ JSON\).

### func UserPromptBlockExit2

```go
func UserPromptBlockExit2(reason string) *UserPromptResponse
```

UserPromptBlockExit2 rejects the prompt via exit 2; stderr to Claude.

## type WorktreeCreateEvent

WorktreeCreateEvent is the Claude WorktreeCreate hook input.

```go
type WorktreeCreateEvent = internalclaude.WorktreeCreateInput
```

## type WorktreeCreateResponse

WorktreeCreateResponse is the response type for WorktreeCreate.

```go
type WorktreeCreateResponse = CommonResponse
```

## type WorktreeRemoveEvent

WorktreeRemoveEvent is the Claude WorktreeRemove hook input.

```go
type WorktreeRemoveEvent = internalclaude.WorktreeRemoveInput
```

## type WorktreeRemoveResponse

WorktreeRemoveResponse is the response type for WorktreeRemove.

```go
type WorktreeRemoveResponse = CommonResponse
```
