package gemini

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

func EncodeBeforeToolSelection(v any) runtime.Result {
	out, ok := v.(BeforeToolSelectionOutcome)
	if !ok {
		return outcomeTypeMismatch("Gemini BeforeToolSelection")
	}
	if err := validateToolConfig(out.ToolConfig); err != nil {
		return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("Gemini BeforeToolSelection: %v\n", err)}
	}
	if out.ToolConfig == nil && !out.SuppressOutput {
		return runtime.Result{ExitCode: 0, Stdout: []byte("{}")}
	}
	body, err := json.Marshal(syncOutputDTO{
		SuppressOutput:     out.SuppressOutput,
		HookSpecificOutput: encodeToolSelectionHook(out.ToolConfig),
	})
	if err != nil {
		return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("Gemini BeforeToolSelection: %v\n", err)}
	}
	return runtime.Result{ExitCode: 0, Stdout: body}
}

func EncodeBeforeTool(v any) runtime.Result {
	out, ok := v.(BeforeToolOutcome)
	if !ok {
		return outcomeTypeMismatch("Gemini BeforeTool")
	}
	if err := validateToolInputObject(out.ToolInput); err != nil {
		return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("Gemini BeforeTool: %v\n", err)}
	}
	return encodeSync("Gemini BeforeTool", out.CommonOutcome, encodeBeforeToolHook(out.ToolInput))
}

func EncodeAfterTool(v any) runtime.Result {
	out, ok := v.(AfterToolOutcome)
	if !ok {
		return outcomeTypeMismatch("Gemini AfterTool")
	}
	if err := validateTailToolCallRequest(out.TailToolCallRequest); err != nil {
		return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("Gemini AfterTool: %v\n", err)}
	}
	return encodeSync("Gemini AfterTool", out.CommonOutcome, encodeAfterToolHook(out))
}

func encodeToolSelectionHook(toolConfig *ToolConfig) any {
	if toolConfig == nil {
		return nil
	}
	return beforeToolSelectionHookSpecificDTO{
		HookEventName: "BeforeToolSelection",
		ToolConfig: &toolConfigDTO{
			Mode:                 normalizeToolConfigMode(toolConfig.Mode),
			AllowedFunctionNames: normalizeFunctionNames(toolConfig.AllowedFunctionNames),
		},
	}
}

func encodeBeforeToolHook(toolInput json.RawMessage) any {
	if len(toolInput) == 0 {
		return nil
	}
	return toolHookSpecificDTO{
		HookEventName: "BeforeTool",
		ToolInput:     toolInput,
	}
}

func encodeAfterToolHook(out AfterToolOutcome) any {
	if strings.TrimSpace(out.AdditionalContext) == "" && out.TailToolCallRequest == nil {
		return nil
	}
	dto := afterToolHookSpecificDTO{
		HookEventName:     "AfterTool",
		AdditionalContext: out.AdditionalContext,
	}
	if out.TailToolCallRequest != nil {
		dto.TailToolCallRequest = &tailToolCallRequestDTO{
			Name: out.TailToolCallRequest.Name,
			Args: out.TailToolCallRequest.Args,
		}
	}
	return dto
}
