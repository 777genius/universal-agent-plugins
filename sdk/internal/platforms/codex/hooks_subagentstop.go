package codex

import (
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

type subagentStopInputDTO struct {
	SessionID            string `json:"session_id"`
	TurnID               string `json:"turn_id"`
	TranscriptPath       string `json:"transcript_path"`
	CWD                  string `json:"cwd"`
	HookEventName        string `json:"hook_event_name"`
	Model                string `json:"model"`
	PermissionMode       string `json:"permission_mode"`
	StopHookActive       bool   `json:"stop_hook_active"`
	LastAssistantMessage string `json:"last_assistant_message"`
	AgentID              string `json:"agent_id"`
	AgentType            string `json:"agent_type"`
	AgentTranscriptPath  string `json:"agent_transcript_path"`
}

func DecodeSubagentStop(env runtime.Envelope) (any, string, error) {
	dto, err := runtime.DecodeJSONPayload[subagentStopInputDTO](env.Stdin, "codex subagent stop input")
	if err != nil {
		return nil, "", err
	}
	return &SubagentStopInput{
		SessionID:            dto.SessionID,
		TurnID:               dto.TurnID,
		TranscriptPath:       dto.TranscriptPath,
		CWD:                  dto.CWD,
		HookEventName:        dto.HookEventName,
		Model:                dto.Model,
		PermissionMode:       dto.PermissionMode,
		StopHookActive:       dto.StopHookActive,
		LastAssistantMessage: dto.LastAssistantMessage,
		AgentID:              dto.AgentID,
		AgentType:            dto.AgentType,
		AgentTranscriptPath:  dto.AgentTranscriptPath,
	}, dto.HookEventName, nil
}

// EncodeSubagentStopOutcome emits the observation acknowledgement: empty stdout, exit 0.
func EncodeSubagentStopOutcome(SubagentStopOutcome) runtime.Result {
	return runtime.Result{ExitCode: 0}
}

func EncodeSubagentStop(v any) runtime.Result {
	out, ok := v.(SubagentStopOutcome)
	if !ok {
		return runtime.Result{ExitCode: 1, Stderr: "encode codex subagent stop response: internal outcome type mismatch\n"}
	}
	return EncodeSubagentStopOutcome(out)
}
