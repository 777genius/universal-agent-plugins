package gemini

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

func encodeSync(label string, out CommonOutcome, hookSpecific any) runtime.Result {
	if err := validateDecision(out.Decision); err != nil {
		return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("%s: %v\n", label, err)}
	}
	if hookSpecific == nil &&
		out.Continue == nil &&
		!out.SuppressOutput &&
		strings.TrimSpace(out.StopReason) == "" &&
		strings.TrimSpace(out.Decision) == "" &&
		strings.TrimSpace(out.Reason) == "" &&
		strings.TrimSpace(out.SystemMessage) == "" {
		return runtime.Result{ExitCode: 0, Stdout: []byte("{}")}
	}
	body, err := json.Marshal(syncOutputDTO{
		SystemMessage:      out.SystemMessage,
		SuppressOutput:     out.SuppressOutput,
		Continue:           out.Continue,
		StopReason:         out.StopReason,
		Decision:           out.Decision,
		Reason:             out.Reason,
		HookSpecificOutput: hookSpecific,
	})
	if err != nil {
		return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("%s: %v\n", label, err)}
	}
	return runtime.Result{ExitCode: 0, Stdout: body}
}

func sanitizeLifecycleOutcome(out CommonOutcome) CommonOutcome {
	return sanitizeAdvisoryOutcome(out)
}

func sanitizeAdvisoryOutcome(out CommonOutcome) CommonOutcome {
	out.Continue = nil
	out.StopReason = ""
	out.Decision = ""
	out.Reason = ""
	return out
}

func outcomeTypeMismatch(label string) runtime.Result {
	return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("encode %s response: internal outcome type mismatch\n", label)}
}
