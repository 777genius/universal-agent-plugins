package runtime

import (
	"fmt"
	"strings"
)

func CanonicalInvocationName(platform PlatformID, raw string) string {
	raw = strings.TrimSpace(raw)
	switch platform {
	case "claude":
		switch {
		case strings.EqualFold(raw, "Stop"):
			return "Stop"
		case strings.EqualFold(raw, "PreToolUse"):
			return "PreToolUse"
		case strings.EqualFold(raw, "UserPromptSubmit"):
			return "UserPromptSubmit"
		}
	case "codex":
		switch {
		case strings.EqualFold(raw, "CodexStop"):
			return "Stop"
		case strings.EqualFold(raw, "CodexSubagentStop"):
			return "SubagentStop"
		case strings.EqualFold(raw, "CodexPermissionRequest"):
			return "PermissionRequest"
		}
	case "gemini":
		switch {
		case strings.EqualFold(raw, "GeminiSessionStart"):
			return "SessionStart"
		case strings.EqualFold(raw, "GeminiSessionEnd"):
			return "SessionEnd"
		case strings.EqualFold(raw, "GeminiBeforeModel"):
			return "BeforeModel"
		case strings.EqualFold(raw, "GeminiAfterModel"):
			return "AfterModel"
		case strings.EqualFold(raw, "GeminiBeforeToolSelection"):
			return "BeforeToolSelection"
		case strings.EqualFold(raw, "GeminiBeforeAgent"):
			return "BeforeAgent"
		case strings.EqualFold(raw, "GeminiAfterAgent"):
			return "AfterAgent"
		case strings.EqualFold(raw, "GeminiBeforeTool"):
			return "BeforeTool"
		case strings.EqualFold(raw, "GeminiAfterTool"):
			return "AfterTool"
		}
	}
	return raw
}

func attachMismatchWarning(res Result, platform PlatformID, rawName, decodedName string) Result {
	decodedName = strings.TrimSpace(decodedName)
	if decodedName == "" {
		return res
	}
	if strings.EqualFold(decodedName, CanonicalInvocationName(platform, rawName)) {
		return res
	}
	res.Stderr = fmt.Sprintf("plugin-kit-ai: hook_event_name %q does not match argv hook %q; using argv\n", decodedName, rawName) + res.Stderr
	return res
}
