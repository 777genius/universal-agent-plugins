package claude

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

func DecodeSessionEnd(env runtime.Envelope) (any, string, error) {
	dto, err := decodeJSONInput[SessionEndInput](env, "sessionend")
	if err != nil {
		return nil, "", err
	}
	return dto, dto.HookEventName, nil
}

func DecodeNotification(env runtime.Envelope) (any, string, error) {
	dto, err := decodeJSONInput[NotificationInput](env, "notification")
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

func DecodePostToolUseFailure(env runtime.Envelope) (any, string, error) {
	dto, err := decodeJSONInput[PostToolUseFailureInput](env, "posttoolusefailure")
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(dto.ToolName) == "" {
		return nil, "", fmt.Errorf("decode posttoolusefailure input: tool_name required")
	}
	return dto, dto.HookEventName, nil
}

func DecodePermissionRequest(env runtime.Envelope) (any, string, error) {
	dto, err := decodeJSONInput[PermissionRequestInput](env, "permissionrequest")
	if err != nil {
		return nil, "", err
	}
	return dto, dto.HookEventName, nil
}

func DecodeSubagentStart(env runtime.Envelope) (any, string, error) {
	dto, err := decodeJSONInput[SubagentStartInput](env, "subagentstart")
	if err != nil {
		return nil, "", err
	}
	return dto, dto.HookEventName, nil
}

func DecodeSubagentStop(env runtime.Envelope) (any, string, error) {
	dto, err := decodeJSONInput[SubagentStopInput](env, "subagentstop")
	if err != nil {
		return nil, "", err
	}
	return dto, dto.HookEventName, nil
}

func DecodePreCompact(env runtime.Envelope) (any, string, error) {
	dto, err := decodeJSONInput[PreCompactInput](env, "precompact")
	if err != nil {
		return nil, "", err
	}
	return dto, dto.HookEventName, nil
}

func DecodeSetup(env runtime.Envelope) (any, string, error) {
	dto, err := decodeJSONInput[SetupInput](env, "setup")
	if err != nil {
		return nil, "", err
	}
	return dto, dto.HookEventName, nil
}

func DecodeTeammateIdle(env runtime.Envelope) (any, string, error) {
	dto, err := decodeJSONInput[TeammateIdleInput](env, "teammateidle")
	if err != nil {
		return nil, "", err
	}
	return dto, dto.HookEventName, nil
}

func DecodeTaskCompleted(env runtime.Envelope) (any, string, error) {
	dto, err := decodeJSONInput[TaskCompletedInput](env, "taskcompleted")
	if err != nil {
		return nil, "", err
	}
	return dto, dto.HookEventName, nil
}

func DecodeConfigChange(env runtime.Envelope) (any, string, error) {
	dto, err := decodeJSONInput[ConfigChangeInput](env, "configchange")
	if err != nil {
		return nil, "", err
	}
	return dto, dto.HookEventName, nil
}

func DecodeWorktreeCreate(env runtime.Envelope) (any, string, error) {
	dto, err := decodeJSONInput[WorktreeCreateInput](env, "worktreecreate")
	if err != nil {
		return nil, "", err
	}
	return dto, dto.HookEventName, nil
}

func DecodeWorktreeRemove(env runtime.Envelope) (any, string, error) {
	dto, err := decodeJSONInput[WorktreeRemoveInput](env, "worktreeremove")
	if err != nil {
		return nil, "", err
	}
	return dto, dto.HookEventName, nil
}
