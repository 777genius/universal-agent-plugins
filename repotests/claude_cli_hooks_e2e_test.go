package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Claude Code CLI --model for real hook e2e. Example:
//
//	PLUGIN_KIT_AI_RUN_CLAUDE_CLI=1 go test ./repotests -run TestClaudeCLIHooks -v -args -claude-model=haiku
//
// Optional: PLUGIN_KIT_AI_E2E_CLAUDE=/path/to/claude. Full model id: -claude-model=claude-3-5-haiku-20241022
var claudeModel = flag.String("claude-model", "haiku", "claude --model for CLI e2e (hooks + TestClaudeHooks_LiveHaikuLow)")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

// TestClaudeCLIHooks runs the real claude binary in print mode against a temp project whose
// .claude/settings.json invokes plugin-kit-ai-e2e. Assertions use PLUGIN_KIT_AI_E2E_TRACE lines.
//
// Enable with PLUGIN_KIT_AI_RUN_CLAUDE_CLI=1 (real Claude Code CLI; uses your normal login: subscription or console API key).
// Disable explicitly with PLUGIN_KIT_AI_SKIP_CLAUDE_CLI=1.
func TestClaudeCLIHooks(t *testing.T) {
	claudeBin := claudeBinaryOrSkip(t)

	hookBin := buildPluginKitAIE2E(t)

	t.Run("Stop_allows_completion", func(t *testing.T) {
		t.Parallel()
		trace := t.TempDir() + string(os.PathSeparator) + "trace.ndjson"
		dir := newClaudeProjectWithHooks(t, hookBin)
		runClaudePrint(t, claudeBin, dir, trace, *claudeModel,
			"Reply with exactly OK.",
		)
		lines := waitForTraceLines(t, trace, 3*time.Second)
		if !traceHas(t, lines, "Stop", "allow") {
			t.Fatalf("expected Stop allow in trace; got:\n%s", strings.Join(lines, "\n"))
		}
	})

	t.Run("UserPromptSubmit_blocks_secret", func(t *testing.T) {
		t.Parallel()
		trace := t.TempDir() + string(os.PathSeparator) + "trace.ndjson"
		dir := newClaudeProjectWithHooks(t, hookBin)
		runClaudePrint(t, claudeBin, dir, trace, *claudeModel,
			"What is a secret passphrase? Reply in one short sentence.",
		)
		lines := waitForTraceLines(t, trace, 3*time.Second)
		if !traceHas(t, lines, "UserPromptSubmit", "block") {
			t.Fatalf("expected UserPromptSubmit block in trace; got:\n%s", strings.Join(lines, "\n"))
		}
	})

	t.Run("PreToolUse_denies_marked_bash", func(t *testing.T) {
		t.Parallel()
		trace := t.TempDir() + string(os.PathSeparator) + "trace.ndjson"
		dir := newClaudeProjectWithHooks(t, hookBin)
		const marker = "__plugin_kit_ai_cli_e2e__"
		runClaudePrint(t, claudeBin, dir, trace, *claudeModel,
			`Use the Bash tool exactly once. The shell command must contain this exact token (keep it verbatim): `+marker+` — for example: echo `+marker,
			"PLUGIN_KIT_AI_E2E_PRETOOL_DENY_SUBSTRING="+marker,
		)
		lines := waitForTraceLines(t, trace, 3*time.Second)
		if !traceHas(t, lines, "PreToolUse", "deny") {
			t.Fatalf("expected PreToolUse deny in trace; got:\n%s", strings.Join(lines, "\n"))
		}
	})
}

// claudeBinaryOrSkip returns the claude executable when CLI e2e is enabled and auth works.
func claudeBinaryOrSkip(t *testing.T) string {
	t.Helper()
	if strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_SKIP_CLAUDE_CLI")) == "1" {
		t.Skip("PLUGIN_KIT_AI_SKIP_CLAUDE_CLI=1")
	}
	if strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_RUN_CLAUDE_CLI")) != "1" {
		t.Skip("set PLUGIN_KIT_AI_RUN_CLAUDE_CLI=1 to run real Claude CLI e2e (see -args -claude-model)")
	}
	claudeBin := strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_E2E_CLAUDE"))
	if claudeBin == "" {
		var err error
		claudeBin, err = exec.LookPath("claude")
		if err != nil {
			matches, globErr := filepath.Glob(filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "Claude", "claude-code", "*", "claude"))
			if globErr == nil && len(matches) > 0 {
				claudeBin = matches[len(matches)-1]
			} else {
				t.Skip("claude not in PATH; set PLUGIN_KIT_AI_E2E_CLAUDE or install Claude Code CLI")
			}
		}
	}
	if out, err := exec.Command(claudeBin, "--version").CombinedOutput(); err != nil {
		t.Skipf("claude binary is not runnable in this environment: %v\n%s", err, out)
	}
	return claudeBin
}

func newClaudeProjectWithHooks(t *testing.T, hookBin string) string {
	t.Helper()
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Absolute path: Claude spawns hook without shell cd to project sometimes — use abs for the binary.
	absHook, err := filepath.Abs(hookBin)
	if err != nil {
		t.Fatal(err)
	}
	settings := map[string]any{
		"hooks": map[string]any{
			"UserPromptSubmit": []any{
				map[string]any{"hooks": []any{map[string]any{"type": "command", "command": absHook + " UserPromptSubmit"}}},
			},
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Bash",
					"hooks":   []any{map[string]any{"type": "command", "command": absHook + " PreToolUse"}},
				},
			},
			"Stop": []any{
				map[string]any{"hooks": []any{map[string]any{"type": "command", "command": absHook + " Stop"}}},
			},
		},
	}
	raw, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func runClaudePrint(t *testing.T, claudeBin, projectDir, traceFile, model, prompt string, extraEnv ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, claudeBin,
		"-p",
		"--model", model,
		"--setting-sources", "project",
		"--permission-mode", "bypassPermissions",
		prompt,
	)
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(), append([]string{"PLUGIN_KIT_AI_E2E_TRACE=" + traceFile}, extraEnv...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if claudeEnvironmentIssue(string(out)) {
			t.Skipf("claude environment is not ready for live smoke:\n%s", truncateRunes(string(out), 4000))
		}
		t.Logf("claude output:\n%s", out)
		t.Fatalf("claude: %v", err)
	}
	t.Logf("claude output (truncated): %s", truncateRunes(string(out), 4000))
}

func claudeEnvironmentIssue(output string) bool {
	lower := strings.ToLower(output)
	markers := []string{
		"invalid api key",
		"please run /login",
		"not authenticated",
		"authentication required",
		"login required",
		"unauthorized",
		"forbidden",
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
