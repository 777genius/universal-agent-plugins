package gemini

import "encoding/json"

// BeforeModelContinue returns an explicit no-op BeforeModel response.
func BeforeModelContinue() *BeforeModelResponse {
	return &BeforeModelResponse{}
}

// BeforeModelDeny blocks the LLM request with a deny decision.
func BeforeModelDeny(reason string) *BeforeModelResponse {
	return &BeforeModelResponse{CommonResponse: CommonResponse{Decision: "deny", Reason: reason}}
}

// BeforeModelOverrideRequest continues with a rewritten llm_request payload.
func BeforeModelOverrideRequest(request json.RawMessage) *BeforeModelResponse {
	return &BeforeModelResponse{LLMRequest: request}
}

// BeforeModelSyntheticResponse short-circuits the LLM request with a synthetic
// llm_response payload.
func BeforeModelSyntheticResponse(response json.RawMessage) *BeforeModelResponse {
	return &BeforeModelResponse{LLMResponse: response}
}

// AfterModelContinue returns an explicit no-op AfterModel response.
func AfterModelContinue() *AfterModelResponse {
	return &AfterModelResponse{}
}

// AfterModelDeny blocks the model result with a deny decision.
func AfterModelDeny(reason string) *AfterModelResponse {
	return &AfterModelResponse{CommonResponse: CommonResponse{Decision: "deny", Reason: reason}}
}

// AfterModelStop stops the entire Gemini agent loop immediately.
func AfterModelStop(reason string) *AfterModelResponse {
	return &AfterModelResponse{CommonResponse: stopCommonResponse(reason)}
}

// AfterModelReplaceResponse continues with a rewritten llm_response payload.
func AfterModelReplaceResponse(response json.RawMessage) *AfterModelResponse {
	return &AfterModelResponse{LLMResponse: response}
}
