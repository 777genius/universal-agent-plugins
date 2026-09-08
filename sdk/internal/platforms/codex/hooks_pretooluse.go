package codex

import (
	"encoding/json"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

type preToolUseInputDTO struct {
	SessionID      string          `json:"session_id"`
	TurnID         string          `json:"turn_id"`
	TranscriptPath string          `json:"transcript_path"`
	CWD            string          `json:"cwd"`
	HookEventName  string          `json:"hook_event_name"`
	Model          string          `json:"model"`
	PermissionMode string          `json:"permission_mode"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
	ToolUseID      string          `json:"tool_use_id"`
	AgentID        string          `json:"agent_id"`
	AgentType      string          `json:"agent_type"`
}

func DecodePreToolUse(env runtime.Envelope) (any, string, error) {
	dto, err := runtime.DecodeJSONPayload[preToolUseInputDTO](env.Stdin, "codex pre tool use input")
	if err != nil {
		return nil, "", err
	}
	return &PreToolUseInput{
		SessionID:      dto.SessionID,
		TurnID:         dto.TurnID,
		TranscriptPath: dto.TranscriptPath,
		CWD:            dto.CWD,
		HookEventName:  dto.HookEventName,
		Model:          dto.Model,
		PermissionMode: dto.PermissionMode,
		ToolName:       dto.ToolName,
		ToolInput:      dto.ToolInput,
		ToolUseID:      dto.ToolUseID,
		AgentID:        dto.AgentID,
		AgentType:      dto.AgentType,
	}, dto.HookEventName, nil
}

// EncodePreToolUseOutcome emits the observation acknowledgement: empty stdout, exit 0.
func EncodePreToolUseOutcome(PreToolUseOutcome) runtime.Result {
	return runtime.Result{ExitCode: 0}
}

func EncodePreToolUse(v any) runtime.Result {
	out, ok := v.(PreToolUseOutcome)
	if !ok {
		return runtime.Result{ExitCode: 1, Stderr: "encode codex pre tool use response: internal outcome type mismatch\n"}
	}
	return EncodePreToolUseOutcome(out)
}
