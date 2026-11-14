package gemini

import (
	"encoding/json"

	internalgemini "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/gemini"
)

// SessionStartEvent is the Gemini SessionStart hook input.
type SessionStartEvent = internalgemini.SessionStartInput

// SessionEndEvent is the Gemini SessionEnd hook input.
type SessionEndEvent = internalgemini.SessionEndInput

// BeforeModelEvent is the Gemini BeforeModel hook input.
type BeforeModelEvent = internalgemini.BeforeModelInput

// AfterModelEvent is the Gemini AfterModel hook input.
type AfterModelEvent = internalgemini.AfterModelInput

// BeforeToolSelectionEvent is the Gemini BeforeToolSelection hook input.
type BeforeToolSelectionEvent = internalgemini.BeforeToolSelectionInput

// BeforeAgentEvent is the Gemini BeforeAgent hook input.
type BeforeAgentEvent = internalgemini.BeforeAgentInput

// AfterAgentEvent is the Gemini AfterAgent hook input.
type AfterAgentEvent = internalgemini.AfterAgentInput

// BeforeToolEvent is the Gemini BeforeTool hook input.
type BeforeToolEvent = internalgemini.BeforeToolInput

// AfterToolEvent is the Gemini AfterTool hook input.
type AfterToolEvent = internalgemini.AfterToolInput

// CommonResponse contains fields shared by Gemini's synchronous hook envelope.
type CommonResponse struct {
	Continue       *bool
	SuppressOutput bool
	StopReason     string
	Decision       string
	Reason         string
	SystemMessage  string
}

// SessionStartResponse is the SessionStart response type.
type SessionStartResponse struct {
	CommonResponse
	AdditionalContext string
}

// SessionEndResponse is the SessionEnd response type.
type SessionEndResponse = CommonResponse

// BeforeModelResponse is the BeforeModel response type.
type BeforeModelResponse struct {
	CommonResponse
	LLMRequest  json.RawMessage
	LLMResponse json.RawMessage
}

// AfterModelResponse is the AfterModel response type.
type AfterModelResponse struct {
	CommonResponse
	LLMResponse json.RawMessage
}

// ToolMode configures Gemini BeforeToolSelection tool routing.
type ToolMode string

const (
	ToolModeAuto ToolMode = "AUTO"
	ToolModeAny  ToolMode = "ANY"
	ToolModeNone ToolMode = "NONE"
)

// BeforeToolSelectionResponse is the Gemini BeforeToolSelection response type.
type BeforeToolSelectionResponse struct {
	SuppressOutput       bool
	Mode                 ToolMode
	AllowedFunctionNames []string
}

// BeforeAgentResponse is the BeforeAgent response type.
type BeforeAgentResponse struct {
	CommonResponse
	AdditionalContext string
}

// AfterAgentResponse is the AfterAgent response type.
type AfterAgentResponse struct {
	CommonResponse
	ClearContext bool
}

// BeforeToolResponse is the BeforeTool response type.
type BeforeToolResponse struct {
	CommonResponse
	ToolInput json.RawMessage
}

// TailToolCallRequest requests an immediate follow-up tool execution from an
// AfterTool hook.
type TailToolCallRequest struct {
	Name string
	Args json.RawMessage
}

// AfterToolResponse is the AfterTool response type.
type AfterToolResponse struct {
	CommonResponse
	AdditionalContext   string
	TailToolCallRequest *TailToolCallRequest
}
