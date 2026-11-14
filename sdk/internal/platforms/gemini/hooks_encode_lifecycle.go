package gemini

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

func EncodeSessionStart(v any) runtime.Result {
	out, ok := v.(SessionStartOutcome)
	if !ok {
		return outcomeTypeMismatch("Gemini SessionStart")
	}
	out.CommonOutcome = sanitizeLifecycleOutcome(out.CommonOutcome)
	return encodeSync("Gemini SessionStart", out.CommonOutcome, encodeContextHook("SessionStart", out.AdditionalContext))
}

func EncodeSessionEnd(v any) runtime.Result {
	out, ok := v.(SessionEndOutcome)
	if !ok {
		return outcomeTypeMismatch("Gemini SessionEnd")
	}
	out.CommonOutcome = sanitizeLifecycleOutcome(out.CommonOutcome)
	return encodeSync("Gemini SessionEnd", out.CommonOutcome, nil)
}

func EncodeBeforeAgent(v any) runtime.Result {
	out, ok := v.(BeforeAgentOutcome)
	if !ok {
		return outcomeTypeMismatch("Gemini BeforeAgent")
	}
	return encodeSync("Gemini BeforeAgent", out.CommonOutcome, encodeContextHook("BeforeAgent", out.AdditionalContext))
}

func EncodeAfterAgent(v any) runtime.Result {
	out, ok := v.(AfterAgentOutcome)
	if !ok {
		return outcomeTypeMismatch("Gemini AfterAgent")
	}
	var hookSpecific any
	if out.ClearContext {
		hookSpecific = afterAgentHookSpecificDTO{
			HookEventName: "AfterAgent",
			ClearContext:  true,
		}
	}
	return encodeSync("Gemini AfterAgent", out.CommonOutcome, hookSpecific)
}

func encodeContextHook(name, additionalContext string) any {
	if strings.TrimSpace(additionalContext) == "" {
		return nil
	}
	return contextHookSpecificDTO{
		HookEventName:     name,
		AdditionalContext: additionalContext,
	}
}
