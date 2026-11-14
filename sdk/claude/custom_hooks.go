package claude

import (
	"encoding/json"
	"fmt"
	"strings"

	internalclaude "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/claude"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// RegisterCustomCommonJSON registers an experimental Claude hook backed by stdin JSON
// and a common synchronous response family.
func RegisterCustomCommonJSON[T any](r *Registrar, eventName string, fn func(*T) *CommonResponse) error {
	return r.registerCustomJSON(eventName, func(env runtime.Envelope) (any, string, error) {
		return decodeCustomJSON[T](env, eventName)
	}, func(v any) runtime.Result {
		return internalclaude.EncodeCommon(v, runtime.NormalizeHookName(eventName))
	}, func(_ runtime.InvocationContext, v any) runtime.Handled {
		ev, ok := v.(*T)
		if !ok {
			return runtime.Handled{Err: internalclaudeTypeMismatch("claude " + eventName)}
		}
		return runtime.Handled{Value: commonOutcomeFromResponse(fn(ev))}
	})
}

// RegisterCustomContextJSON registers an experimental Claude hook backed by stdin JSON
// and a context-producing response family.
func RegisterCustomContextJSON[T any](r *Registrar, eventName string, fn func(*T) *ContextResponse) error {
	return r.registerCustomJSON(eventName, func(env runtime.Envelope) (any, string, error) {
		return decodeCustomJSON[T](env, eventName)
	}, func(v any) runtime.Result {
		return internalclaude.EncodeContext(v, runtime.NormalizeHookName(eventName), eventName)
	}, func(_ runtime.InvocationContext, v any) runtime.Handled {
		ev, ok := v.(*T)
		if !ok {
			return runtime.Handled{Err: internalclaudeTypeMismatch("claude " + eventName)}
		}
		return runtime.Handled{Value: contextOutcomeFromResponse(fn(ev))}
	})
}

// RegisterCustomPostToolUseJSON registers an experimental Claude hook backed by stdin JSON
// and the PostToolUse response family.
func RegisterCustomPostToolUseJSON[T any](r *Registrar, eventName string, fn func(*T) *PostToolUseResponse) error {
	return r.registerCustomJSON(eventName, func(env runtime.Envelope) (any, string, error) {
		return decodeCustomJSON[T](env, eventName)
	}, func(v any) runtime.Result {
		out, ok := v.(internalclaude.PostToolUseOutcome)
		if !ok {
			return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("encode %s response: internal outcome type mismatch\n", runtime.NormalizeHookName(eventName))}
		}
		var specific any
		if strings.TrimSpace(out.AdditionalContext) != "" || len(out.UpdatedMCPToolOutput) > 0 {
			specific = map[string]any{
				"hookEventName":        eventName,
				"additionalContext":    out.AdditionalContext,
				"updatedMcpToolOutput": out.UpdatedMCPToolOutput,
			}
		}
		return encodeCustomCommon(runtime.NormalizeHookName(eventName), out.CommonOutcome, specific)
	}, func(_ runtime.InvocationContext, v any) runtime.Handled {
		ev, ok := v.(*T)
		if !ok {
			return runtime.Handled{Err: internalclaudeTypeMismatch("claude " + eventName)}
		}
		return runtime.Handled{Value: postToolUseOutcomeFromResponse(fn(ev))}
	})
}

// RegisterCustomPermissionRequestJSON registers an experimental Claude hook backed by stdin JSON
// and the permission decision response family.
func RegisterCustomPermissionRequestJSON[T any](r *Registrar, eventName string, fn func(*T) *PermissionRequestResponse) error {
	return r.registerCustomJSON(eventName, func(env runtime.Envelope) (any, string, error) {
		return decodeCustomJSON[T](env, eventName)
	}, func(v any) runtime.Result {
		out, ok := v.(internalclaude.PermissionRequestOutcome)
		if !ok {
			return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("encode %s response: internal outcome type mismatch\n", runtime.NormalizeHookName(eventName))}
		}
		var specific any
		if out.Permission != nil {
			behavior := strings.ToLower(strings.TrimSpace(string(out.Permission.Behavior)))
			switch behavior {
			case string(internalclaude.PermissionAllow), string(internalclaude.PermissionDeny):
			default:
				return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("encode %s response: unknown behavior %q\n", runtime.NormalizeHookName(eventName), out.Permission.Behavior)}
			}
			specific = map[string]any{
				"hookEventName": eventName,
				"decision": map[string]any{
					"behavior":           behavior,
					"updatedInput":       out.Permission.UpdatedInput,
					"updatedPermissions": out.Permission.UpdatedPermissions,
					"message":            out.Permission.Message,
					"interrupt":          out.Permission.Interrupt,
				},
			}
		}
		return encodeCustomCommon(runtime.NormalizeHookName(eventName), out.CommonOutcome, specific)
	}, func(_ runtime.InvocationContext, v any) runtime.Handled {
		ev, ok := v.(*T)
		if !ok {
			return runtime.Handled{Err: internalclaudeTypeMismatch("claude " + eventName)}
		}
		return runtime.Handled{Value: permissionOutcomeFromResponse(fn(ev))}
	})
}

func (r *Registrar) registerCustomJSON(eventName string, decode func(runtime.Envelope) (any, string, error), encode func(any) runtime.Result, handler runtime.TypedHandler) error {
	name := strings.TrimSpace(eventName)
	if name == "" {
		return fmt.Errorf("custom hook name required")
	}
	return r.backend.RegisterCustom(name, runtime.Descriptor{
		Platform: "claude",
		Event:    runtime.EventID(name),
		Carrier:  runtime.CarrierStdinJSON,
		Decode:   decode,
		Encode:   encode,
	}, handler)
}

func decodeCustomJSON[T any](env runtime.Envelope, eventName string) (any, string, error) {
	dto, err := runtime.DecodeJSONPayload[T](env.Stdin, runtime.NormalizeHookName(eventName)+" input")
	if err != nil {
		return nil, "", err
	}
	return dto, eventName, nil
}

func encodeCustomCommon(label string, out internalclaude.CommonOutcome, hookSpecific any) runtime.Result {
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
	b, err := json.Marshal(map[string]any{
		"continue":           out.Continue,
		"suppressOutput":     out.SuppressOutput,
		"stopReason":         out.StopReason,
		"decision":           out.Decision,
		"reason":             out.Reason,
		"systemMessage":      out.SystemMessage,
		"hookSpecificOutput": hookSpecific,
	})
	if err != nil {
		return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("encode %s response: %v\n", label, err)}
	}
	return runtime.Result{ExitCode: 0, Stdout: b}
}
