package runtime

import "testing"

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
}
