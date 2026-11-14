package gemini

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func DecodeSessionStart(env runtime.Envelope) (any, string, error) {
	return decodeJSONInput[SessionStartInput](env, "session start", "SessionStart")
}

func DecodeSessionEnd(env runtime.Envelope) (any, string, error) {
	return decodeJSONInput[SessionEndInput](env, "session end", "SessionEnd")
}

func DecodeBeforeModel(env runtime.Envelope) (any, string, error) {
	return decodeJSONInput[BeforeModelInput](env, "before model", "BeforeModel")
}

func DecodeAfterModel(env runtime.Envelope) (any, string, error) {
	return decodeJSONInput[AfterModelInput](env, "after model", "AfterModel")
}

func DecodeBeforeToolSelection(env runtime.Envelope) (any, string, error) {
	return decodeJSONInput[BeforeToolSelectionInput](env, "before tool selection", "BeforeToolSelection")
}

func DecodeBeforeAgent(env runtime.Envelope) (any, string, error) {
	return decodeJSONInput[BeforeAgentInput](env, "before agent", "BeforeAgent")
}

func DecodeAfterAgent(env runtime.Envelope) (any, string, error) {
	return decodeJSONInput[AfterAgentInput](env, "after agent", "AfterAgent")
}

func DecodeBeforeTool(env runtime.Envelope) (any, string, error) {
	return decodeJSONInput[BeforeToolInput](env, "before tool", "BeforeTool")
}

func DecodeAfterTool(env runtime.Envelope) (any, string, error) {
	return decodeJSONInput[AfterToolInput](env, "after tool", "AfterTool")
}

func decodeJSONInput[T any](env runtime.Envelope, label, eventName string) (any, string, error) {
	dto, err := runtime.DecodeJSONPayload[T](env.Stdin, "Gemini "+label+" input")
	if err != nil {
		return nil, "", err
	}
	return dto, eventName, nil
}
