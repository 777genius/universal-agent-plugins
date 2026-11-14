package qoder

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

func decodeJSONInput[T any](env runtime.Envelope, label string) (*T, error) {
	return runtime.DecodeJSONPayload[T](env.Stdin, label+" input")
}

func DecodeSessionStart(env runtime.Envelope) (any, string, error) {
	dto, err := decodeJSONInput[SessionStartInput](env, "sessionstart")
	if err != nil {
		return nil, "", err
	}
	return dto, dto.HookEventName, nil
}

func DecodeUserPromptSubmit(env runtime.Envelope) (any, string, error) {
	dto, err := decodeJSONInput[UserPromptSubmitInput](env, "userpromptsubmit")
	if err != nil {
		return nil, "", err
	}
	return dto, dto.HookEventName, nil
}

func DecodePostToolUse(env runtime.Envelope) (any, string, error) {
	dto, err := decodeJSONInput[PostToolUseInput](env, "posttooluse")
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(dto.ToolName) == "" {
		return nil, "", fmt.Errorf("decode posttooluse input: tool_name required")
	}
	return dto, dto.HookEventName, nil
}

func DecodeStop(env runtime.Envelope) (any, string, error) {
	dto, err := decodeJSONInput[StopInput](env, "stop")
	if err != nil {
		return nil, "", err
	}
	return dto, dto.HookEventName, nil
}
