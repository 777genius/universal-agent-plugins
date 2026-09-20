package platformexec

import "strings"

func stableGeminiHookNames() []string {
	return []string{
		"SessionStart",
		"SessionEnd",
		"BeforeModel",
		"AfterModel",
		"BeforeToolSelection",
		"BeforeAgent",
		"AfterAgent",
		"BeforeTool",
		"AfterTool",
	}
}

func geminiInvocationAlias(hookName string) string {
	switch strings.TrimSpace(hookName) {
	case "SessionStart":
		return "GeminiSessionStart"
	case "SessionEnd":
		return "GeminiSessionEnd"
	case "BeforeModel":
		return "GeminiBeforeModel"
	case "AfterModel":
		return "GeminiAfterModel"
	case "BeforeToolSelection":
		return "GeminiBeforeToolSelection"
	case "BeforeAgent":
		return "GeminiBeforeAgent"
	case "AfterAgent":
		return "GeminiAfterAgent"
	case "BeforeTool":
		return "GeminiBeforeTool"
	case "AfterTool":
		return "GeminiAfterTool"
	default:
		return ""
	}
}

type geminiHooksFile struct {
	Hooks map[string][]geminiHookGroup `json:"hooks"`
}

type geminiHookGroup struct {
	Matcher string                `json:"matcher,omitempty"`
	Hooks   []importedHookCommand `json:"hooks"`
}

type importedHookCommand struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}
