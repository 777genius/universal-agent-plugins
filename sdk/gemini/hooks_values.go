package gemini

import (
	"encoding/json"
	"fmt"
	"strings"
)

// BeforeModelOverrideRequestValue marshals a replacement llm_request object for
// Gemini BeforeModel hooks.
func BeforeModelOverrideRequestValue(v any) (*BeforeModelResponse, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal Gemini llm_request override: %w", err)
	}
	if !looksLikeJSONObject(body) {
		return nil, fmt.Errorf("marshal Gemini llm_request override: expected JSON object")
	}
	return BeforeModelOverrideRequest(body), nil
}

// BeforeModelSyntheticResponseValue marshals a synthetic llm_response object
// for Gemini BeforeModel hooks.
func BeforeModelSyntheticResponseValue(v any) (*BeforeModelResponse, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal Gemini llm_response override: %w", err)
	}
	if !looksLikeJSONObject(body) {
		return nil, fmt.Errorf("marshal Gemini llm_response override: expected JSON object")
	}
	return BeforeModelSyntheticResponse(body), nil
}

// AfterModelReplaceResponseValue marshals a replacement llm_response object for
// Gemini AfterModel hooks.
func AfterModelReplaceResponseValue(v any) (*AfterModelResponse, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal Gemini llm_response replacement: %w", err)
	}
	if !looksLikeJSONObject(body) {
		return nil, fmt.Errorf("marshal Gemini llm_response replacement: expected JSON object")
	}
	return AfterModelReplaceResponse(body), nil
}

// BeforeToolRewriteInputValue marshals a replacement tool_input object for
// Gemini BeforeTool hooks. Gemini expects hookSpecificOutput.tool_input to be a
// JSON object, so non-object values return an error.
func BeforeToolRewriteInputValue(v any) (*BeforeToolResponse, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal Gemini tool_input rewrite: %w", err)
	}
	if !looksLikeJSONObject(body) {
		return nil, fmt.Errorf("marshal Gemini tool_input rewrite: expected JSON object")
	}
	return BeforeToolRewriteInput(body), nil
}

// AfterToolTailCallValue marshals a typed follow-up tool request. Gemini
// expects tailToolCallRequest.args to be a JSON object, so non-object values
// return an error.
func AfterToolTailCallValue(name string, args any) (*AfterToolResponse, error) {
	body, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("marshal Gemini tail tool call args: %w", err)
	}
	if !looksLikeJSONObject(body) {
		return nil, fmt.Errorf("marshal Gemini tail tool call args: expected JSON object")
	}
	return AfterToolTailCall(name, body), nil
}

func looksLikeJSONObject(body []byte) bool {
	return strings.HasPrefix(strings.TrimSpace(string(body)), "{")
}
