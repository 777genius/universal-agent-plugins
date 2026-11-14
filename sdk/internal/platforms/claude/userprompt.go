package claude

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

type userPromptInputDTO struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
	PermissionMode string `json:"permission_mode"`
	HookEventName  string `json:"hook_event_name"`
	Prompt         string `json:"prompt"`
}

type userPromptOutDTO struct {
	Decision          string `json:"decision,omitempty"`
	Reason            string `json:"reason,omitempty"`
	AdditionalContext string `json:"additionalContext,omitempty"`
}

func DecodeUserPromptSubmit(env runtime.Envelope) (any, string, error) {
	dto, err := runtime.DecodeJSONPayload[userPromptInputDTO](env.Stdin, "userpromptsubmit input")
	if err != nil {
		return nil, "", err
	}
	return &UserPromptSubmitInput{
		SessionID:      dto.SessionID,
		TranscriptPath: dto.TranscriptPath,
		CWD:            dto.CWD,
		PermissionMode: dto.PermissionMode,
		HookEventName:  dto.HookEventName,
		Prompt:         dto.Prompt,
	}, dto.HookEventName, nil
}

func EncodeUserPromptSubmitOutcome(out UserPromptSubmitOutcome) runtime.Result {
	if out.BlockExitCode != 0 {
		return runtime.Result{
			ExitCode: out.BlockExitCode,
			Stderr:   blockStderr(out.BlockReason),
		}
	}
	if out.Allow {
		ctx := strings.TrimSpace(out.AdditionalContext)
		if ctx == "" {
			return runtime.Result{ExitCode: 0, Stdout: []byte("{}")}
		}
		b, err := json.Marshal(userPromptOutDTO{AdditionalContext: ctx})
		if err != nil {
			return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("encode userpromptsubmit response: %v\n", err)}
		}
		return runtime.Result{ExitCode: 0, Stdout: b}
	}
	reason := out.BlockReason
	if reason == "" {
		reason = "blocked by hook"
	}
	b, err := json.Marshal(userPromptOutDTO{Decision: "block", Reason: reason})
	if err != nil {
		return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("encode userpromptsubmit response: %v\n", err)}
	}
	return runtime.Result{ExitCode: 0, Stdout: b}
}

func blockStderr(reason string) string {
	if strings.TrimSpace(reason) == "" {
		return ""
	}
	return reason + "\n"
}

func EncodeUserPromptSubmit(v any) runtime.Result {
	out, ok := v.(UserPromptSubmitOutcome)
	if !ok {
		return runtime.Result{ExitCode: 1, Stderr: "encode userpromptsubmit response: internal outcome type mismatch\n"}
	}
	return EncodeUserPromptSubmitOutcome(out)
}
