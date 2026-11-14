package gemini

import (
	"strings"

	internalgemini "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/gemini"
)

func commonOutcomeFromResponse(r *CommonResponse) internalgemini.CommonOutcome {
	if r == nil {
		return internalgemini.CommonOutcome{}
	}
	return internalgemini.CommonOutcome{
		Continue:       r.Continue,
		SuppressOutput: r.SuppressOutput,
		StopReason:     r.StopReason,
		Decision:       r.Decision,
		Reason:         r.Reason,
		SystemMessage:  r.SystemMessage,
	}
}

func lifecycleOutcomeFromResponse(r *CommonResponse) internalgemini.CommonOutcome {
	out := commonOutcomeFromResponse(r)
	out.Continue = nil
	out.StopReason = ""
	out.Decision = ""
	out.Reason = ""
	return out
}

func sessionStartOutcomeFromResponse(r *SessionStartResponse) internalgemini.SessionStartOutcome {
	if r == nil {
		return internalgemini.SessionStartOutcome{}
	}
	return internalgemini.SessionStartOutcome{
		CommonOutcome:     lifecycleOutcomeFromResponse(&r.CommonResponse),
		AdditionalContext: r.AdditionalContext,
	}
}

func beforeToolOutcomeFromResponse(r *BeforeToolResponse) internalgemini.BeforeToolOutcome {
	if r == nil {
		return internalgemini.BeforeToolOutcome{}
	}
	return internalgemini.BeforeToolOutcome{
		CommonOutcome: commonOutcomeFromResponse(&r.CommonResponse),
		ToolInput:     r.ToolInput,
	}
}

func sessionEndOutcomeFromResponse(r *SessionEndResponse) internalgemini.SessionEndOutcome {
	return internalgemini.SessionEndOutcome{CommonOutcome: lifecycleOutcomeFromResponse(r)}
}

func beforeModelOutcomeFromResponse(r *BeforeModelResponse) internalgemini.BeforeModelOutcome {
	if r == nil {
		return internalgemini.BeforeModelOutcome{}
	}
	return internalgemini.BeforeModelOutcome{
		CommonOutcome: commonOutcomeFromResponse(&r.CommonResponse),
		LLMRequest:    r.LLMRequest,
		LLMResponse:   r.LLMResponse,
	}
}

func afterModelOutcomeFromResponse(r *AfterModelResponse) internalgemini.AfterModelOutcome {
	if r == nil {
		return internalgemini.AfterModelOutcome{}
	}
	return internalgemini.AfterModelOutcome{
		CommonOutcome: commonOutcomeFromResponse(&r.CommonResponse),
		LLMResponse:   r.LLMResponse,
	}
}

func beforeToolSelectionOutcomeFromResponse(r *BeforeToolSelectionResponse) internalgemini.BeforeToolSelectionOutcome {
	if r == nil {
		return internalgemini.BeforeToolSelectionOutcome{}
	}
	if strings.TrimSpace(string(r.Mode)) == "" && len(r.AllowedFunctionNames) == 0 && !r.SuppressOutput {
		return internalgemini.BeforeToolSelectionOutcome{}
	}
	out := internalgemini.BeforeToolSelectionOutcome{
		SuppressOutput: r.SuppressOutput,
	}
	if strings.TrimSpace(string(r.Mode)) == "" && len(r.AllowedFunctionNames) == 0 {
		return out
	}
	out.ToolConfig = &internalgemini.ToolConfig{
		Mode:                 string(r.Mode),
		AllowedFunctionNames: append([]string(nil), r.AllowedFunctionNames...),
	}
	return out
}

func beforeAgentOutcomeFromResponse(r *BeforeAgentResponse) internalgemini.BeforeAgentOutcome {
	if r == nil {
		return internalgemini.BeforeAgentOutcome{}
	}
	return internalgemini.BeforeAgentOutcome{
		CommonOutcome:     commonOutcomeFromResponse(&r.CommonResponse),
		AdditionalContext: r.AdditionalContext,
	}
}

func afterAgentOutcomeFromResponse(r *AfterAgentResponse) internalgemini.AfterAgentOutcome {
	if r == nil {
		return internalgemini.AfterAgentOutcome{}
	}
	return internalgemini.AfterAgentOutcome{
		CommonOutcome: commonOutcomeFromResponse(&r.CommonResponse),
		ClearContext:  r.ClearContext,
	}
}

func afterToolOutcomeFromResponse(r *AfterToolResponse) internalgemini.AfterToolOutcome {
	if r == nil {
		return internalgemini.AfterToolOutcome{}
	}
	out := internalgemini.AfterToolOutcome{
		CommonOutcome:     commonOutcomeFromResponse(&r.CommonResponse),
		AdditionalContext: r.AdditionalContext,
	}
	if r.TailToolCallRequest != nil {
		out.TailToolCallRequest = &internalgemini.TailToolCallRequest{
			Name: r.TailToolCallRequest.Name,
			Args: r.TailToolCallRequest.Args,
		}
	}
	return out
}
