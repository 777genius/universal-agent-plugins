package gen

import (
	"fmt"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
	"strings"
)

func ResolveInvocation(args []string, _ runtime.Env) (runtime.Invocation, error) {
	if len(args) < 2 {
		return runtime.Invocation{}, fmt.Errorf("usage: <binary> <hookName>")
	}
	raw := args[1]
	if strings.EqualFold(raw, "Stop") {
		return runtime.Invocation{Platform: "claude", Event: "Stop", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "PreToolUse") {
		return runtime.Invocation{Platform: "claude", Event: "PreToolUse", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "UserPromptSubmit") {
		return runtime.Invocation{Platform: "claude", Event: "UserPromptSubmit", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "SessionStart") {
		return runtime.Invocation{Platform: "claude", Event: "SessionStart", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "SessionEnd") {
		return runtime.Invocation{Platform: "claude", Event: "SessionEnd", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "Notification") {
		return runtime.Invocation{Platform: "claude", Event: "Notification", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "PostToolUse") {
		return runtime.Invocation{Platform: "claude", Event: "PostToolUse", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "PostToolUseFailure") {
		return runtime.Invocation{Platform: "claude", Event: "PostToolUseFailure", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "PermissionRequest") {
		return runtime.Invocation{Platform: "claude", Event: "PermissionRequest", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "SubagentStart") {
		return runtime.Invocation{Platform: "claude", Event: "SubagentStart", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "SubagentStop") {
		return runtime.Invocation{Platform: "claude", Event: "SubagentStop", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "PreCompact") {
		return runtime.Invocation{Platform: "claude", Event: "PreCompact", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "Setup") {
		return runtime.Invocation{Platform: "claude", Event: "Setup", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "TeammateIdle") {
		return runtime.Invocation{Platform: "claude", Event: "TeammateIdle", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "TaskCompleted") {
		return runtime.Invocation{Platform: "claude", Event: "TaskCompleted", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "ConfigChange") {
		return runtime.Invocation{Platform: "claude", Event: "ConfigChange", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "WorktreeCreate") {
		return runtime.Invocation{Platform: "claude", Event: "WorktreeCreate", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "WorktreeRemove") {
		return runtime.Invocation{Platform: "claude", Event: "WorktreeRemove", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "GeminiSessionStart") {
		return runtime.Invocation{Platform: "gemini", Event: "SessionStart", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "GeminiSessionEnd") {
		return runtime.Invocation{Platform: "gemini", Event: "SessionEnd", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "GeminiBeforeModel") {
		return runtime.Invocation{Platform: "gemini", Event: "BeforeModel", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "GeminiAfterModel") {
		return runtime.Invocation{Platform: "gemini", Event: "AfterModel", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "GeminiBeforeToolSelection") {
		return runtime.Invocation{Platform: "gemini", Event: "BeforeToolSelection", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "GeminiBeforeAgent") {
		return runtime.Invocation{Platform: "gemini", Event: "BeforeAgent", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "GeminiAfterAgent") {
		return runtime.Invocation{Platform: "gemini", Event: "AfterAgent", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "GeminiBeforeTool") {
		return runtime.Invocation{Platform: "gemini", Event: "BeforeTool", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "GeminiAfterTool") {
		return runtime.Invocation{Platform: "gemini", Event: "AfterTool", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "notify") {
		return runtime.Invocation{Platform: "codex", Event: "Notify", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "CodexStop") {
		return runtime.Invocation{Platform: "codex", Event: "Stop", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "CodexSubagentStop") {
		return runtime.Invocation{Platform: "codex", Event: "SubagentStop", RawName: raw}, nil
	}
	if strings.EqualFold(raw, "CodexPermissionRequest") {
		return runtime.Invocation{Platform: "codex", Event: "PermissionRequest", RawName: raw}, nil
	}
	return runtime.Invocation{}, fmt.Errorf("unknown invocation %q", raw)
}
