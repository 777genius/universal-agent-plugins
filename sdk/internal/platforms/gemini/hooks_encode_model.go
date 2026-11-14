package gemini

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

func EncodeBeforeModel(v any) runtime.Result {
	out, ok := v.(BeforeModelOutcome)
	if !ok {
		return outcomeTypeMismatch("Gemini BeforeModel")
	}
	if err := validateModelResponse(out.LLMResponse); err != nil {
		return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("Gemini BeforeModel: %v\n", err)}
	}
	if err := validateModelRequest(out.LLMRequest); err != nil {
		return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("Gemini BeforeModel: %v\n", err)}
	}
	return encodeSync("Gemini BeforeModel", out.CommonOutcome, encodeModelHook("BeforeModel", out.LLMRequest, out.LLMResponse))
}

func EncodeAfterModel(v any) runtime.Result {
	out, ok := v.(AfterModelOutcome)
	if !ok {
		return outcomeTypeMismatch("Gemini AfterModel")
	}
	if err := validateModelResponse(out.LLMResponse); err != nil {
		return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("Gemini AfterModel: %v\n", err)}
	}
	return encodeSync("Gemini AfterModel", out.CommonOutcome, encodeModelHook("AfterModel", nil, out.LLMResponse))
}

func encodeModelHook(name string, request, response []byte) any {
	if len(request) == 0 && len(response) == 0 {
		return nil
	}
	return modelHookSpecificDTO{
		HookEventName: name,
		LLMRequest:    request,
		LLMResponse:   response,
	}
}
