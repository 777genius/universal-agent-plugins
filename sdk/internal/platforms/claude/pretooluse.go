package claude

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

type preToolInputDTO struct {
	SessionID      string          `json:"session_id"`
	TranscriptPath string          `json:"transcript_path"`
	CWD            string          `json:"cwd"`
	PermissionMode string          `json:"permission_mode"`
	HookEventName  string          `json:"hook_event_name"`
	ToolName       string          `json:"tool_name"`
	ToolUseID      string          `json:"tool_use_id"`
	ToolInput      json.RawMessage `json:"tool_input"`
}

type preToolHookSpecificDTO struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
}

type preToolStdoutDTO struct {
	HookSpecificOutput preToolHookSpecificDTO `json:"hookSpecificOutput"`
}

func DecodePreToolUse(env runtime.Envelope) (any, string, error) {
	dto, err := runtime.DecodeJSONPayload[preToolInputDTO](env.Stdin, "pretooluse input")
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(dto.ToolName) == "" {
		return nil, "", fmt.Errorf("decode pretooluse input: tool_name required")
	}
	ti := dto.ToolInput
	if ti == nil {
		ti = json.RawMessage([]byte("null"))
	}
	return &PreToolUseInput{
		SessionID:      dto.SessionID,
		TranscriptPath: dto.TranscriptPath,
		CWD:            dto.CWD,
		PermissionMode: dto.PermissionMode,
		HookEventName:  dto.HookEventName,
		ToolName:       dto.ToolName,
		ToolUseID:      dto.ToolUseID,
		ToolInput:      ti,
	}, dto.HookEventName, nil
}

func EncodePreToolUseOutcome(out PreToolUseOutcome) runtime.Result {
	if out.BlockExitCode != 0 {
		return runtime.Result{
			ExitCode: out.BlockExitCode,
			Stderr:   blockStderr(out.BlockReason),
		}
	}
	p := strings.ToLower(strings.TrimSpace(out.Permission))
	switch p {
	case "", "allow":
		if strings.TrimSpace(out.Reason) == "" {
			return runtime.Result{ExitCode: 0, Stdout: []byte("{}")}
		}
		return marshalPreTool(preToolHookSpecificDTO{
			HookEventName:            "PreToolUse",
			PermissionDecision:       "allow",
			PermissionDecisionReason: out.Reason,
		})
	case "deny", "ask":
		reason := out.Reason
		if reason == "" && p == "deny" {
			reason = "blocked by hook"
		}
		return marshalPreTool(preToolHookSpecificDTO{
			HookEventName:            "PreToolUse",
			PermissionDecision:       p,
			PermissionDecisionReason: reason,
		})
	default:
		return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("encode pretooluse response: unknown permission %q\n", out.Permission)}
	}
}

func marshalPreTool(dto preToolHookSpecificDTO) runtime.Result {
	b, err := json.Marshal(preToolStdoutDTO{HookSpecificOutput: dto})
	if err != nil {
		return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("encode pretooluse response: %v\n", err)}
	}
	return runtime.Result{ExitCode: 0, Stdout: b}
}

func EncodePreToolUse(v any) runtime.Result {
	out, ok := v.(PreToolUseOutcome)
	if !ok {
		return runtime.Result{ExitCode: 1, Stderr: "encode pretooluse response: internal outcome type mismatch\n"}
	}
	return EncodePreToolUseOutcome(out)
}
