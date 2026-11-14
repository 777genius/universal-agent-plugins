package gemini

// BeforeAgentContinue returns an explicit no-op BeforeAgent response.
func BeforeAgentContinue() *BeforeAgentResponse {
	return &BeforeAgentResponse{}
}

// BeforeAgentAddContext appends additional context to the current turn prompt.
func BeforeAgentAddContext(context string) *BeforeAgentResponse {
	return &BeforeAgentResponse{AdditionalContext: context}
}

// BeforeAgentAllow returns an explicit allow decision for BeforeAgent.
func BeforeAgentAllow() *BeforeAgentResponse {
	return &BeforeAgentResponse{CommonResponse: CommonResponse{Decision: "allow"}}
}

// BeforeAgentDeny blocks the turn and discards the user's prompt from history.
func BeforeAgentDeny(reason string) *BeforeAgentResponse {
	return &BeforeAgentResponse{CommonResponse: CommonResponse{Decision: "deny", Reason: reason}}
}

// BeforeAgentStop aborts the current turn but keeps the user's prompt in
// history, matching Gemini's continue=false semantics.
func BeforeAgentStop(reason string) *BeforeAgentResponse {
	response := stopCommonResponse(reason)
	response.Reason = reason
	return &BeforeAgentResponse{CommonResponse: response}
}

// AfterAgentContinue returns an explicit no-op AfterAgent response.
func AfterAgentContinue() *AfterAgentResponse {
	return &AfterAgentResponse{}
}

// AfterAgentAllow returns an explicit allow decision for AfterAgent.
func AfterAgentAllow() *AfterAgentResponse {
	return &AfterAgentResponse{CommonResponse: CommonResponse{Decision: "allow"}}
}

// AfterAgentDeny rejects the response and requests a retry.
func AfterAgentDeny(reason string) *AfterAgentResponse {
	return &AfterAgentResponse{CommonResponse: CommonResponse{Decision: "deny", Reason: reason}}
}

// AfterAgentStop stops the session without triggering a retry.
func AfterAgentStop(reason string) *AfterAgentResponse {
	return &AfterAgentResponse{CommonResponse: stopCommonResponse(reason)}
}

// AfterAgentClearContext clears LLM conversation memory while preserving the
// UI display.
func AfterAgentClearContext() *AfterAgentResponse {
	return &AfterAgentResponse{ClearContext: true}
}
