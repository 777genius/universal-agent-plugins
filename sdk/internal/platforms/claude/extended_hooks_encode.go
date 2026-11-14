package claude

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

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

func EncodeCommon(v any, label string) runtime.Result {
	out, ok := v.(CommonOutcome)
	if !ok {
		return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("encode %s response: internal outcome type mismatch\n", label)}
	}
	return encodeSync(label, out, nil)
}

func EncodeContext(v any, label, hookEventName string) runtime.Result {
	out, ok := v.(ContextOutcome)
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

func EncodePermissionRequestOutcome(out PermissionRequestOutcome) runtime.Result {
	var specific any
	if out.Permission != nil {
		behavior := strings.ToLower(strings.TrimSpace(string(out.Permission.Behavior)))
		switch behavior {
		case string(PermissionAllow), string(PermissionDeny):
		default:
			return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("encode permissionrequest response: unknown behavior %q\n", out.Permission.Behavior)}
		}
		specific = permissionHookSpecificDTO{
			HookEventName: "PermissionRequest",
			Decision: permissionDecisionDTO{
				Behavior:           behavior,
				UpdatedInput:       out.Permission.UpdatedInput,
				UpdatedPermissions: out.Permission.UpdatedPermissions,
				Message:            out.Permission.Message,
				Interrupt:          out.Permission.Interrupt,
			},
		}
	}
	return encodeSync("permissionrequest", out.CommonOutcome, specific)
}

func EncodePermissionRequest(v any) runtime.Result {
	out, ok := v.(PermissionRequestOutcome)
	if !ok {
		return runtime.Result{ExitCode: 1, Stderr: "encode permissionrequest response: internal outcome type mismatch\n"}
	}
	return EncodePermissionRequestOutcome(out)
}

func EncodeSessionStart(v any) runtime.Result {
	return EncodeContext(v, "sessionstart", "SessionStart")
}

func EncodeNotification(v any) runtime.Result {
	return EncodeContext(v, "notification", "Notification")
}

func EncodePostToolUseFailure(v any) runtime.Result {
	return EncodeContext(v, "posttoolusefailure", "PostToolUseFailure")
}

func EncodeSessionEnd(v any) runtime.Result { return EncodeCommon(v, "sessionend") }

func EncodeSubagentStart(v any) runtime.Result {
	return EncodeContext(v, "subagentstart", "SubagentStart")
}

func EncodeSubagentStop(v any) runtime.Result   { return EncodeCommon(v, "subagentstop") }
func EncodePreCompact(v any) runtime.Result     { return EncodeCommon(v, "precompact") }
func EncodeSetup(v any) runtime.Result          { return EncodeContext(v, "setup", "Setup") }
func EncodeTeammateIdle(v any) runtime.Result   { return EncodeCommon(v, "teammateidle") }
func EncodeTaskCompleted(v any) runtime.Result  { return EncodeCommon(v, "taskcompleted") }
func EncodeConfigChange(v any) runtime.Result   { return EncodeCommon(v, "configchange") }
func EncodeWorktreeCreate(v any) runtime.Result { return EncodeCommon(v, "worktreecreate") }
func EncodeWorktreeRemove(v any) runtime.Result { return EncodeCommon(v, "worktreeremove") }
