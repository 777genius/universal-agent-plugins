package gemini

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func wrapSessionStart(fn func(*SessionStartEvent) *SessionStartResponse) runtime.TypedHandler {
	return wrapGeminiHandler("SessionStart", fn, func(r *SessionStartResponse) any {
		return sessionStartOutcomeFromResponse(r)
	})
}

func wrapSessionEnd(fn func(*SessionEndEvent) *SessionEndResponse) runtime.TypedHandler {
	return wrapGeminiHandler("SessionEnd", fn, func(r *SessionEndResponse) any {
		return sessionEndOutcomeFromResponse(r)
	})
}

func wrapBeforeModel(fn func(*BeforeModelEvent) *BeforeModelResponse) runtime.TypedHandler {
	return wrapGeminiHandler("BeforeModel", fn, func(r *BeforeModelResponse) any {
		return beforeModelOutcomeFromResponse(r)
	})
}

func wrapAfterModel(fn func(*AfterModelEvent) *AfterModelResponse) runtime.TypedHandler {
	return wrapGeminiHandler("AfterModel", fn, func(r *AfterModelResponse) any {
		return afterModelOutcomeFromResponse(r)
	})
}

func wrapBeforeToolSelection(fn func(*BeforeToolSelectionEvent) *BeforeToolSelectionResponse) runtime.TypedHandler {
	return wrapGeminiHandler("BeforeToolSelection", fn, func(r *BeforeToolSelectionResponse) any {
		return beforeToolSelectionOutcomeFromResponse(r)
	})
}

func wrapBeforeAgent(fn func(*BeforeAgentEvent) *BeforeAgentResponse) runtime.TypedHandler {
	return wrapGeminiHandler("BeforeAgent", fn, func(r *BeforeAgentResponse) any {
		return beforeAgentOutcomeFromResponse(r)
	})
}

func wrapAfterAgent(fn func(*AfterAgentEvent) *AfterAgentResponse) runtime.TypedHandler {
	return wrapGeminiHandler("AfterAgent", fn, func(r *AfterAgentResponse) any {
		return afterAgentOutcomeFromResponse(r)
	})
}

func wrapBeforeTool(fn func(*BeforeToolEvent) *BeforeToolResponse) runtime.TypedHandler {
	return wrapGeminiHandler("BeforeTool", fn, func(r *BeforeToolResponse) any {
		return beforeToolOutcomeFromResponse(r)
	})
}

func wrapAfterTool(fn func(*AfterToolEvent) *AfterToolResponse) runtime.TypedHandler {
	return wrapGeminiHandler("AfterTool", fn, func(r *AfterToolResponse) any {
		return afterToolOutcomeFromResponse(r)
	})
}

func internalgeminiTypeMismatch(name string) error {
	return runtime.InternalHookTypeMismatch(name)
}
