package runtime

import (
	"strings"
	"testing"
)

func TestCanonicalInvocationNameGemini(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"GeminiSessionStart":        "SessionStart",
		"GeminiSessionEnd":          "SessionEnd",
		"GeminiBeforeModel":         "BeforeModel",
		"GeminiAfterModel":          "AfterModel",
		"GeminiBeforeToolSelection": "BeforeToolSelection",
		"GeminiBeforeAgent":         "BeforeAgent",
		"GeminiAfterAgent":          "AfterAgent",
		"GeminiBeforeTool":          "BeforeTool",
		"GeminiAfterTool":           "AfterTool",
	}
	for raw, want := range cases {
		if got := CanonicalInvocationName("gemini", raw); got != want {
			t.Fatalf("%s => %s, want %s", raw, got, want)
		}
	}
}

func TestCanonicalInvocationNameCodex(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"CodexStop":              "Stop",
		"CodexSubagentStop":      "SubagentStop",
		"CodexPreToolUse":        "PreToolUse",
		"CodexPermissionRequest": "PermissionRequest",
		"codexstop":              "Stop",
		"notify":                 "notify",
	}
	for raw, want := range cases {
		if got := CanonicalInvocationName("codex", raw); got != want {
			t.Fatalf("%s => %s, want %s", raw, got, want)
		}
	}
}

func TestCanonicalInvocationNameQoder(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"session-start":      "SessionStart",
		"user-prompt-submit": "UserPromptSubmit",
		"post-tool-use":      "PostToolUse",
		"stop":               "Stop",
		"Stop":               "Stop",
		"Session-Start":      "SessionStart",
		"session_start":      "session_start",
	}
	for raw, want := range cases {
		if got := CanonicalInvocationName("qoder", raw); got != want {
			t.Fatalf("%s => %s, want %s", raw, got, want)
		}
	}
}

func TestAttachMismatchWarningUsesCanonicalInvocationName(t *testing.T) {
	t.Parallel()
	res := attachMismatchWarning(Result{ExitCode: 0, Stdout: []byte("{}")}, "gemini", "GeminiSessionStart", "SessionStart")
	if res.Stderr != "" {
		t.Fatalf("stderr = %q", res.Stderr)
	}
	res = attachMismatchWarning(Result{ExitCode: 0}, "codex", "CodexStop", "Stop")
	if res.Stderr != "" {
		t.Fatalf("codex stderr = %q", res.Stderr)
	}
	res = attachMismatchWarning(Result{ExitCode: 0}, "qoder", "stop", "Stop")
	if res.Stderr != "" {
		t.Fatalf("qoder stderr = %q", res.Stderr)
	}
	res = attachMismatchWarning(Result{ExitCode: 0}, "qoder", "stop", "SessionStart")
	if !strings.Contains(res.Stderr, "does not match argv hook") {
		t.Fatalf("qoder mismatch stderr = %q", res.Stderr)
	}
}
