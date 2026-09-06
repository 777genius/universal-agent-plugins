package codex

import (
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

type stopInputDTO struct {
	SessionID            string `json:"session_id"`
	TurnID               string `json:"turn_id"`
	TranscriptPath       string `json:"transcript_path"`
	CWD                  string `json:"cwd"`
	HookEventName        string `json:"hook_event_name"`
	Model                string `json:"model"`
	PermissionMode       string `json:"permission_mode"`
	StopHookActive       bool   `json:"stop_hook_active"`
	LastAssistantMessage string `json:"last_assistant_message"`
}

func DecodeStop(env runtime.Envelope) (any, string, error) {
	dto, err := runtime.DecodeJSONPayload[stopInputDTO](env.Stdin, "codex stop input")
	if err != nil {
		return nil, "", err
	}
	return &StopInput{
		SessionID:            dto.SessionID,
		TurnID:               dto.TurnID,
		TranscriptPath:       dto.TranscriptPath,
		CWD:                  dto.CWD,
		HookEventName:        dto.HookEventName,
		Model:                dto.Model,
		PermissionMode:       dto.PermissionMode,
		StopHookActive:       dto.StopHookActive,
		LastAssistantMessage: dto.LastAssistantMessage,
	}, dto.HookEventName, nil
}

// EncodeStopOutcome emits the observation acknowledgement: empty stdout, exit 0.
func EncodeStopOutcome(StopOutcome) runtime.Result {
	return runtime.Result{ExitCode: 0}
}

func EncodeStop(v any) runtime.Result {
	out, ok := v.(StopOutcome)
	if !ok {
		return runtime.Result{ExitCode: 1, Stderr: "encode codex stop response: internal outcome type mismatch\n"}
	}
	return EncodeStopOutcome(out)
}
