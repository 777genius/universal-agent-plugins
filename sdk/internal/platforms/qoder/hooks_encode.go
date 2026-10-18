package qoder

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

type stopOutputDTO struct {
	Decision string `json:"decision,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type userPromptOutDTO struct {
	Decision          string `json:"decision,omitempty"`
	Reason            string `json:"reason,omitempty"`
	AdditionalContext string `json:"additionalContext,omitempty"`
}

type syncOutputDTO struct {
	Continue           *bool  `json:"continue,omitempty"`
	SuppressOutput     bool   `json:"suppressOutput,omitempty"`
	StopReason         string `json:"stopReason,omitempty"`
	Decision           string `json:"decision,omitempty"`
	Reason             string `json:"reason,omitempty"`
	SystemMessage      string `json:"systemMessage,omitempty"`
	HookSpecificOutput any    `json:"hookSpecificOutput,omitempty"`
}

type contextHookSpecificDTO struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext,omitempty"`
}

type postToolUseHookSpecificDTO struct {
	HookEventName        string          `json:"hookEventName"`
	AdditionalContext    string          `json:"additionalContext,omitempty"`
	UpdatedMCPToolOutput json.RawMessage `json:"updatedMcpToolOutput,omitempty"`
}

func blockStderr(reason string) string {
	if strings.TrimSpace(reason) == "" {
		return ""
	}
	return reason + "\n"
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

func EncodeUserPromptSubmit(v any) runtime.Result {
	out, ok := v.(UserPromptSubmitOutcome)
	if !ok {
		return runtime.Result{ExitCode: 1, Stderr: "encode userpromptsubmit response: internal outcome type mismatch\n"}
	}
	return EncodeUserPromptSubmitOutcome(out)
}

func encodeSync(label string, out CommonOutcome, hookSpecific any) runtime.Result {
	if hookSpecific == nil &&
		out.Continue == nil &&
		!out.SuppressOutput &&
		strings.TrimSpace(out.StopReason) == "" &&
		strings.TrimSpace(out.Decision) == "" &&
		strings.TrimSpace(out.Reason) == "" &&
		strings.TrimSpace(out.SystemMessage) == "" {
		return runtime.Result{ExitCode: 0, Stdout: []byte("{}")}
	}
	switch out.Decision {
	case "", "approve", "block":
	default:
		return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("encode %s response: unknown decision %q\n", label, out.Decision)}
	}
	b, err := json.Marshal(syncOutputDTO{
		Continue:           out.Continue,
		SuppressOutput:     out.SuppressOutput,
		StopReason:         out.StopReason,
		Decision:           out.Decision,
		Reason:             out.Reason,
		SystemMessage:      out.SystemMessage,
		HookSpecificOutput: hookSpecific,
	})
	if err != nil {
		return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("encode %s response: %v\n", label, err)}
	}
	return runtime.Result{ExitCode: 0, Stdout: b}
}

// EncodeContext writes the Claude-style common synchronous envelope with an
// additionalContext hookSpecificOutput block; it backs both SessionStart and
// experimental custom context hooks.
func EncodeContext(v any, label, hookEventName string) runtime.Result {
	out, ok := v.(SessionStartOutcome)
	if !ok {
		return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("encode %s response: internal outcome type mismatch\n", label)}
	}
	var specific any
	if strings.TrimSpace(out.AdditionalContext) != "" {
		specific = contextHookSpecificDTO{
			HookEventName:     hookEventName,
			AdditionalContext: out.AdditionalContext,
		}
	}
	return encodeSync(label, out.CommonOutcome, specific)
}

func EncodeSessionStart(v any) runtime.Result {
	return EncodeContext(v, "sessionstart", "SessionStart")
}

func EncodePostToolUseOutcome(out PostToolUseOutcome) runtime.Result {
	var specific any
	if strings.TrimSpace(out.AdditionalContext) != "" || len(out.UpdatedMCPToolOutput) > 0 {
		specific = postToolUseHookSpecificDTO{
			HookEventName:        "PostToolUse",
			AdditionalContext:    out.AdditionalContext,
			UpdatedMCPToolOutput: out.UpdatedMCPToolOutput,
		}
	}
	return encodeSync("posttooluse", out.CommonOutcome, specific)
}

func EncodePostToolUse(v any) runtime.Result {
	out, ok := v.(PostToolUseOutcome)
	if !ok {
		return runtime.Result{ExitCode: 1, Stderr: "encode posttooluse response: internal outcome type mismatch\n"}
	}
	return EncodePostToolUseOutcome(out)
}
