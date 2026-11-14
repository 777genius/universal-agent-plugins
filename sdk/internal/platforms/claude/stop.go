package claude

import (
	"encoding/json"
	"fmt"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

type stopInputDTO struct {
	SessionID            string `json:"session_id"`
	TranscriptPath       string `json:"transcript_path"`
	CWD                  string `json:"cwd"`
	PermissionMode       string `json:"permission_mode"`
	HookEventName        string `json:"hook_event_name"`
	StopHookActive       bool   `json:"stop_hook_active"`
	LastAssistantMessage string `json:"last_assistant_message"`
}

type stopOutputDTO struct {
	Decision string `json:"decision,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

func DecodeStop(env runtime.Envelope) (any, string, error) {
	dto, err := runtime.DecodeJSONPayload[stopInputDTO](env.Stdin, "stop input")
	if err != nil {
		return nil, "", err
	}
	return &StopInput{
		SessionID:            dto.SessionID,
		TranscriptPath:       dto.TranscriptPath,
		CWD:                  dto.CWD,
		PermissionMode:       dto.PermissionMode,
		HookEventName:        dto.HookEventName,
		StopHookActive:       dto.StopHookActive,
		LastAssistantMessage: dto.LastAssistantMessage,
	}, dto.HookEventName, nil
}

func EncodeStopOutcome(out StopOutcome) runtime.Result {
	if out.AllowStop {
		return runtime.Result{ExitCode: 0, Stdout: []byte("{}")}
	}
	if out.BlockExitCode != 0 {
		return runtime.Result{
			ExitCode: out.BlockExitCode,
			Stderr:   blockStderr(out.BlockReason),
		}
	}
	reason := out.BlockReason
	if reason == "" {
		reason = "blocked by hook"
	}
	b, err := json.Marshal(stopOutputDTO{Decision: "block", Reason: reason})
	if err != nil {
		return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("encode stop response: %v\n", err)}
	}
	return runtime.Result{ExitCode: 0, Stdout: b}
}

func EncodeStop(v any) runtime.Result {
	out, ok := v.(StopOutcome)
	if !ok {
		return runtime.Result{ExitCode: 1, Stderr: "encode stop response: internal outcome type mismatch\n"}
	}
	return EncodeStopOutcome(out)
}
