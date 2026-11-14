package gemini

import "encoding/json"

// BeforeToolContinue returns an explicit no-op BeforeTool response.
func BeforeToolContinue() *BeforeToolResponse {
	return &BeforeToolResponse{}
}

// BeforeToolAllow returns an explicit allow decision for BeforeTool.
func BeforeToolAllow() *BeforeToolResponse {
	return &BeforeToolResponse{CommonResponse: CommonResponse{Decision: "allow"}}
}

// BeforeToolDeny blocks the tool invocation with a deny decision.
func BeforeToolDeny(reason string) *BeforeToolResponse {
	return &BeforeToolResponse{CommonResponse: CommonResponse{Decision: "deny", Reason: reason}}
}

// BeforeToolStop stops the entire Gemini agent loop before the tool executes.
func BeforeToolStop(reason string) *BeforeToolResponse {
	return &BeforeToolResponse{CommonResponse: stopCommonResponse(reason)}
}

// BeforeToolRewriteInput continues with a rewritten tool_input payload.
func BeforeToolRewriteInput(input json.RawMessage) *BeforeToolResponse {
	return &BeforeToolResponse{ToolInput: input}
}

// AfterToolContinue returns an explicit no-op AfterTool response.
func AfterToolContinue() *AfterToolResponse {
	return &AfterToolResponse{}
}

// AfterToolAddContext appends additional text to the tool result sent back to
// the agent.
func AfterToolAddContext(context string) *AfterToolResponse {
	return &AfterToolResponse{AdditionalContext: context}
}

// AfterToolAllow returns an explicit allow decision for AfterTool.
func AfterToolAllow() *AfterToolResponse {
	return &AfterToolResponse{CommonResponse: CommonResponse{Decision: "allow"}}
}

// AfterToolDeny blocks the follow-up path with a deny decision.
func AfterToolDeny(reason string) *AfterToolResponse {
	return &AfterToolResponse{CommonResponse: CommonResponse{Decision: "deny", Reason: reason}}
}

// AfterToolStop stops the entire Gemini agent loop after tool execution.
func AfterToolStop(reason string) *AfterToolResponse {
	return &AfterToolResponse{CommonResponse: stopCommonResponse(reason)}
}

// AfterToolTailCall requests an immediate follow-up tool invocation.
func AfterToolTailCall(name string, args json.RawMessage) *AfterToolResponse {
	return &AfterToolResponse{
		TailToolCallRequest: &TailToolCallRequest{
			Name: name,
			Args: args,
		},
	}
}
