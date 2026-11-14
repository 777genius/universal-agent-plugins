package codex

import (
	"encoding/json"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

type permissionRequestInputDTO struct {
	SessionID      string          `json:"session_id"`
	TurnID         string          `json:"turn_id"`
	TranscriptPath string          `json:"transcript_path"`
	CWD            string          `json:"cwd"`
	HookEventName  string          `json:"hook_event_name"`
	Model          string          `json:"model"`
	PermissionMode string          `json:"permission_mode"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
	AgentID        string          `json:"agent_id"`
	AgentType      string          `json:"agent_type"`
}

func DecodePermissionRequest(env runtime.Envelope) (any, string, error) {
	dto, err := runtime.DecodeJSONPayload[permissionRequestInputDTO](env.Stdin, "codex permission request input")
	if err != nil {
		return nil, "", err
	}
	return &PermissionRequestInput{
		SessionID:      dto.SessionID,
		TurnID:         dto.TurnID,
		TranscriptPath: dto.TranscriptPath,
		CWD:            dto.CWD,
		HookEventName:  dto.HookEventName,
		Model:          dto.Model,
		PermissionMode: dto.PermissionMode,
		ToolName:       dto.ToolName,
		ToolInput:      dto.ToolInput,
		AgentID:        dto.AgentID,
		AgentType:      dto.AgentType,
	}, dto.HookEventName, nil
}

// EncodePermissionRequestOutcome emits the observation acknowledgement: empty stdout, exit 0.
func EncodePermissionRequestOutcome(PermissionRequestOutcome) runtime.Result {
	return runtime.Result{ExitCode: 0}
}

func EncodePermissionRequest(v any) runtime.Result {
	out, ok := v.(PermissionRequestOutcome)
	if !ok {
		return runtime.Result{ExitCode: 1, Stderr: "encode codex permission request response: internal outcome type mismatch\n"}
	}
	return EncodePermissionRequestOutcome(out)
}
