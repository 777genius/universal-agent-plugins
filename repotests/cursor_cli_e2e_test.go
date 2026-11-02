package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

var cursorModel = flag.String("cursor-model", "composer-2", "cursor-agent --model for CLI e2e")

func TestCursorCLIOutputFormatsSmoke(t *testing.T) {
	cursorBin := cursorBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	workDir := newCursorSmokeWorkspace(t, pluginKitAIBin, cursorSmokeWorkspaceOptions{
		RuleBody: `---
description: "Cursor CLI output format smoke"
alwaysApply: true
---

- When asked to reply with OK, answer with exactly OK.`,
	})

	jsonOut := runCursorCommand(t, cursorBin, workDir, nil, "-p", "Reply with OK.", "--model", *cursorModel, "--print", "--output-format", "json", "--trust")
	assertCursorJSONResult(t, jsonOut, "OK")

	streamOut := runCursorCommand(t, cursorBin, workDir, nil, "-p", "Reply with OK.", "--model", *cursorModel, "--print", "--output-format", "stream-json", "--trust")
	assertCursorStreamResult(t, streamOut, "OK", false)
}

func TestCursorCLISharedAgentsSmoke(t *testing.T) {
	cursorBin := cursorBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	workDir := newCursorSmokeWorkspace(t, pluginKitAIBin, cursorSmokeWorkspaceOptions{
		RuleBody: `---
description: "Cursor CLI shared agents smoke"
alwaysApply: true
---

- Do not modify files for this task.`,
		AgentsBody: "# Shared Cursor Test Instructions\n\nReturn exactly `CURSOR_AGENTS_OK` when asked for the shared instruction token.\n",
	})
	before := snapshotTree(t, workDir)

	out := runCursorCommand(t, cursorBin, workDir, nil, "-p", "Return the shared instruction token only.", "--model", *cursorModel, "--print", "--output-format", "json", "--trust")
	assertCursorJSONResult(t, out, "CURSOR_AGENTS_OK")
	after := snapshotTree(t, workDir)
	if before != after {
		t.Fatalf("workspace changed during Cursor AGENTS smoke\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
}

func TestCursorCLIMCPSmoke(t *testing.T) {
	cursorBin := cursorBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	mcpBin := buildCursorMCPSmokeServer(t)
	marker := filepath.Join(t.TempDir(), "cursor-mcp-marker.json")
	workDir := newCursorSmokeWorkspace(t, pluginKitAIBin, cursorSmokeWorkspaceOptions{
		RuleBody: `---
description: "Cursor CLI MCP smoke"
alwaysApply: true
---

- When explicitly asked to use the MCP tool release_checks, do so exactly once.`,
		PortableMCPYAML: fmt.Sprintf("api_version: v1\n\nservers:\n  release-checks:\n    type: stdio\n    stdio:\n      command: %q\n", filepath.ToSlash(mcpBin)),
	})

	env := []string{"PLUGIN_KIT_AI_CURSOR_MCP_MARKER=" + marker}
	streamOut := runCursorCommand(t, cursorBin, workDir, env, "-p", "Use the MCP tool release_checks exactly once with token CURSOR_MCP_OK, then answer DONE.", "--model", *cursorModel, "--print", "--output-format", "stream-json", "--force", "--approve-mcps", "--trust")
	assertCursorStreamResult(t, streamOut, "DONE", true)

	body, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read MCP marker: %v", err)
	}
	if !strings.Contains(string(body), "CURSOR_MCP_OK") || !strings.Contains(string(body), "release_checks") {
		t.Fatalf("unexpected MCP marker:\n%s", body)
	}
}

type cursorSmokeWorkspaceOptions struct {
	RuleBody        string
	AgentsBody      string
	PortableMCPYAML string
}

func cursorBinaryOrSkip(t *testing.T) string {
	t.Helper()
	if strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_SKIP_CURSOR_CLI")) == "1" {
		t.Skip("PLUGIN_KIT_AI_SKIP_CURSOR_CLI=1")
	}
	if strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_RUN_CURSOR_CLI")) != "1" {
		t.Skip("set PLUGIN_KIT_AI_RUN_CURSOR_CLI=1 to run real Cursor CLI e2e (see -args -cursor-model)")
	}
	cursorBin := strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_E2E_CURSOR"))
	if cursorBin == "" {
		var err error
		cursorBin, err = exec.LookPath("cursor-agent")
		if err != nil {
			if cursorBin, err = exec.LookPath("cursor"); err != nil {
				const bundledCursor = "/Applications/Cursor.app/Contents/Resources/app/bin/cursor"
				if _, statErr := os.Stat(bundledCursor); statErr == nil {
					cursorBin = bundledCursor
				} else {
					t.Skip("cursor-agent/cursor not in PATH; set PLUGIN_KIT_AI_E2E_CURSOR or install Cursor CLI")
				}
			}
		}
	}
	if out, err := exec.Command(cursorVersionCommand(cursorBin)[0], cursorVersionCommand(cursorBin)[1:]...).CombinedOutput(); err != nil {
		t.Skipf("Cursor CLI is not runnable in this environment: %v\n%s", err, out)
	}
	return cursorBin
}

func cursorVersionCommand(cursorBin string) []string {
	if filepath.Base(cursorBin) == "cursor" {
		return []string{cursorBin, "--version"}
	}
	return []string{cursorBin, "--version"}
}

func newCursorSmokeWorkspace(t *testing.T, pluginKitAIBin string, opts cursorSmokeWorkspaceOptions) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin", "plugin.yaml"), []byte("api_version: v1\nname: \"cursor-cli-smoke\"\nversion: \"0.1.0\"\ndescription: \"Cursor CLI live smoke workspace\"\ntargets:\n  - \"cursor\"\n  - \"cursor-workspace\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(opts.AgentsBody) != "" {
		path := filepath.Join(dir, "AGENTS.md")
		if err := os.WriteFile(path, []byte(opts.AgentsBody), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if strings.TrimSpace(opts.RuleBody) != "" {
		path := filepath.Join(dir, "plugin", "targets", "cursor-workspace", "rules", "project.mdc")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(opts.RuleBody+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if strings.TrimSpace(opts.PortableMCPYAML) != "" {
		path := filepath.Join(dir, "plugin", "mcp", "servers.yaml")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(opts.PortableMCPYAML), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	root := RepoRoot(t)
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", dir))
	runCmd(t, root, exec.Command(pluginKitAIBin, "validate", dir, "--platform", "cursor", "--strict"))
	runCmd(t, root, exec.CommandContext(context.Background(), pluginKitAIBin, "validate", dir, "--platform", "cursor-workspace", "--strict"))
	return dir
}

func buildCursorMCPSmokeServer(t *testing.T) string {
	t.Helper()
	root := RepoRoot(t)
	name := "cursor-cli-mcp-smoke"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	out := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("go", "build", "-o", out, "./repotests/testdata/cursor_cli_mcp_smoke")
	cmd.Dir = root
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build cursor CLI MCP smoke helper: %v\n%s", err, b)
	}
	return out
}

func runCursorCommand(t *testing.T, cursorBin, workDir string, extraEnv []string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if filepath.Base(cursorBin) == "cursor" {
		args = append([]string{"agent"}, args...)
	}
	cmd := exec.CommandContext(ctx, cursorBin, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), extraEnv...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("cursor-agent %q timed out:\n%s", strings.Join(args, " "), out)
	}
	if err != nil {
		text := string(out)
		if cursorEnvironmentIssue(text) {
			t.Skipf("cursor-agent environment is not ready for live smoke:\n%s", truncateRunes(text, 4000))
		}
		t.Fatalf("cursor-agent %q: %v\n%s", strings.Join(args, " "), err, out)
	}
	text := string(out)
	t.Logf("cursor-agent %s output: %s", strings.Join(args, " "), truncateRunes(text, 4000))
	return text
}

func cursorEnvironmentIssue(output string) bool {
	lower := strings.ToLower(output)
	markers := []string{
		"not logged in",
		"login required",
		"authentication",
		"unauthorized",
		"model unavailable",
		"model not found",
		"cannot use this model",
		"available models:",
		"access denied",
		"failed to fetch models",
		"usage limit",
		"switch to a different model",
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func assertCursorJSONResult(t *testing.T, raw, wantSubstring string) {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &doc); err != nil {
		t.Fatalf("parse cursor json output: %v\n%s", err, raw)
	}
	if v, ok := doc["is_error"].(bool); ok && v {
		t.Fatalf("cursor json output is_error=true:\n%s", raw)
	}
	if s, ok := doc["session_id"].(string); !ok || strings.TrimSpace(s) == "" {
		t.Fatalf("cursor json output missing session_id:\n%s", raw)
	}
	result := strings.TrimSpace(fmt.Sprint(doc["result"]))
	if result == "" || result == "<nil>" {
		t.Fatalf("cursor json output missing result:\n%s", raw)
	}
	if !strings.Contains(result, wantSubstring) {
		t.Fatalf("cursor result %q missing %q", result, wantSubstring)
	}
}

func assertCursorStreamResult(t *testing.T, raw, wantSubstring string, requireToolCall bool) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) == 0 {
		t.Fatalf("empty cursor stream-json output")
	}
	var foundResult, foundUser, foundInit, foundToolCall bool
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			t.Fatalf("parse cursor stream-json line: %v\n%s", err, line)
		}
		typ := strings.TrimSpace(fmt.Sprint(doc["type"]))
		switch typ {
		case "result":
			foundResult = true
			if !strings.Contains(strings.TrimSpace(fmt.Sprint(doc["result"])), wantSubstring) {
				t.Fatalf("cursor result event missing %q:\n%s", wantSubstring, line)
			}
		case "user":
			foundUser = true
		case "tool_call":
			foundToolCall = true
		case "system":
			if strings.TrimSpace(fmt.Sprint(doc["subtype"])) == "init" {
				foundInit = true
			}
		}
	}
	if !foundResult {
		t.Fatalf("cursor stream-json missing result event:\n%s", raw)
	}
	if !foundUser {
		t.Fatalf("cursor stream-json missing user event:\n%s", raw)
	}
	if !foundInit {
		t.Fatalf("cursor stream-json missing system/init event:\n%s", raw)
	}
	if requireToolCall && !foundToolCall {
		t.Fatalf("cursor stream-json missing tool_call event:\n%s", raw)
	}
}

func snapshotTree(t *testing.T, root string) string {
	t.Helper()
	var lines []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines = append(lines, filepath.ToSlash(rel)+"\n"+string(body))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(lines, "\n---\n")
}
