package pluginkitairepo_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestClaudeHooks_E2E_Subprocess runs plugin-kit-ai-e2e as a real subprocess with JSON on stdin,
// matching how Claude Code invokes command hooks.
func TestClaudeHooks_E2E_Subprocess(t *testing.T) {
	t.Parallel()
	root := RepoRoot(t)
	bin := buildPluginKitAIE2E(t)

	cases := []struct {
		hook     string
		fixture  string
		wantCode int
		wantOut  string
		wantErr  string
	}{
		{"Stop", "stop.json", 0, "{}", ""},
		{"PreToolUse", "pretool_allow.json", 0, "{}", ""},
		{
			"PreToolUse", "pretool_deny.json", 0,
			`{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"dangerous"}}`,
			"",
		},
		{"UserPromptSubmit", "userprompt_allow.json", 0, "{}", ""},
		{
			"UserPromptSubmit", "userprompt_block.json", 0,
			`{"decision":"block","reason":"no secrets"}`,
			"",
		},
	}
	fixtureDir := filepath.Join(root, "repotests", "testdata", "e2e_claude")
	for _, tc := range cases {
		tc := tc
		t.Run(tc.hook+"_"+tc.fixture, func(t *testing.T) {
			t.Parallel()
			payload, err := os.ReadFile(filepath.Join(fixtureDir, tc.fixture))
			if err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			cmd := exec.Command(bin, tc.hook)
			cmd.Stdin = bytes.NewReader(payload)
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err = cmd.Run()
			var code int
			if err != nil {
				if ee, ok := err.(*exec.ExitError); ok {
					code = ee.ExitCode()
				} else {
					t.Fatalf("run: %v stderr=%q", err, stderr.String())
				}
			}
			if code != tc.wantCode {
				t.Fatalf("exit %d want %d stderr=%q stdout=%q", code, tc.wantCode, stderr.String(), stdout.String())
			}
			if got := stdout.String(); got != tc.wantOut {
				t.Fatalf("stdout %q want %q stderr=%q", got, tc.wantOut, stderr.String())
			}
			if got := stderr.String(); got != tc.wantErr {
				t.Fatalf("stderr %q want %q", got, tc.wantErr)
			}
		})
	}
}
