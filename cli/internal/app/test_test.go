package app

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/scaffold"
)

func TestPluginServiceTestClaudeStableEventsUpdateAndMatchGoldens(t *testing.T) {
	restorePluginTestHelpers(t)
	dir := runtimeTestProjectRoot(t, "claude")
	writeBootstrapProjectFile(t, dir, filepath.Join("fixtures", "claude", "Stop.json"), `{"session_id":"s","cwd":"/tmp","hook_event_name":"Stop"}`)
	writeBootstrapProjectFile(t, dir, filepath.Join("fixtures", "claude", "PreToolUse.json"), `{"session_id":"s","cwd":"/tmp","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"echo hi"}}`)
	writeBootstrapProjectFile(t, dir, filepath.Join("fixtures", "claude", "UserPromptSubmit.json"), `{"session_id":"s","cwd":"/tmp","hook_event_name":"UserPromptSubmit","prompt":"hello"}`)

	var svc PluginService
	updated, err := svc.Test(context.Background(), PluginTestOptions{
		Root:         dir,
		Platform:     "claude",
		All:          true,
		UpdateGolden: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Passed {
		t.Fatalf("expected update run to pass:\n%s", strings.Join(updated.Lines, "\n"))
	}
	for _, event := range []string{"Stop", "PreToolUse", "UserPromptSubmit"} {
		for _, suffix := range []string{".stdout", ".stderr", ".exitcode"} {
			if _, err := os.Stat(filepath.Join(dir, "goldens", "claude", event+suffix)); err != nil {
				t.Fatalf("missing golden %s%s: %v", event, suffix, err)
			}
		}
	}

	matched, err := svc.Test(context.Background(), PluginTestOptions{
		Root:     dir,
		Platform: "claude",
		All:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !matched.Passed {
		t.Fatalf("expected match run to pass:\n%s", strings.Join(matched.Lines, "\n"))
	}
	if matched.Summary.Total != 3 || matched.Summary.Passed != 3 || matched.Summary.GoldenMatched != 3 {
		t.Fatalf("summary = %+v", matched.Summary)
	}
	for _, tc := range matched.Cases {
		if tc.GoldenStatus != "matched" {
			t.Fatalf("golden status for %s = %q", tc.Event, tc.GoldenStatus)
		}
	}
}

func TestPluginServiceTestCodexNotifyReportsGoldenMismatch(t *testing.T) {
	restorePluginTestHelpers(t)
	dir := runtimeTestProjectRoot(t, "codex-runtime")
	writeBootstrapProjectFile(t, dir, filepath.Join("fixtures", "codex-runtime", "Notify.json"), `{"client":"codex-tui"}`)
	writeBootstrapProjectFile(t, dir, filepath.Join("goldens", "codex-runtime", "Notify.stdout"), "unexpected\n")
	writeBootstrapProjectFile(t, dir, filepath.Join("goldens", "codex-runtime", "Notify.stderr"), "")
	writeBootstrapProjectFile(t, dir, filepath.Join("goldens", "codex-runtime", "Notify.exitcode"), "0\n")

	var svc PluginService
	result, err := svc.Test(context.Background(), PluginTestOptions{
		Root:     dir,
		Platform: "codex-runtime",
		Event:    "Notify",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed {
		t.Fatalf("expected mismatch run to fail:\n%s", strings.Join(result.Lines, "\n"))
	}
	if len(result.Cases) != 1 {
		t.Fatalf("cases = %d", len(result.Cases))
	}
	if result.Cases[0].GoldenStatus != "mismatch" {
		t.Fatalf("golden status = %q", result.Cases[0].GoldenStatus)
	}
	if !contains(result.Cases[0].Mismatches, "stdout") {
		t.Fatalf("mismatches = %#v", result.Cases[0].Mismatches)
	}
	if len(result.Cases[0].MismatchInfo) == 0 || result.Cases[0].MismatchInfo[0].Field != "stdout" {
		t.Fatalf("mismatch info = %#v", result.Cases[0].MismatchInfo)
	}
	if result.Summary.Failed != 1 || result.Summary.GoldenMismatch != 1 {
		t.Fatalf("summary = %+v", result.Summary)
	}
	if !strings.Contains(strings.Join(result.Lines, "\n"), "expected=") {
		t.Fatalf("lines missing mismatch details:\n%s", strings.Join(result.Lines, "\n"))
	}
}

func TestProcessGoldenAssertionsRejectsPartialGoldenConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeBootstrapProjectFile(t, dir, filepath.Join("Notify.stdout"), "ok\n")

	status, _, mismatches, info, failure := processGoldenAssertions(dir, "Notify", "ok\n", "", 0, false)
	if status != "mismatch" {
		t.Fatalf("status = %q", status)
	}
	if failure != "golden files are partially configured" {
		t.Fatalf("failure = %q", failure)
	}
	if len(mismatches) != 1 || mismatches[0] != "golden_files" {
		t.Fatalf("mismatches = %#v", mismatches)
	}
	if len(info) != 1 || info[0].Field != "golden_files" {
		t.Fatalf("mismatch info = %#v", info)
	}
}

func TestRuntimeTestInvocationCodexLowercasesEvent(t *testing.T) {
	t.Parallel()
	args, stdin, err := runtimeTestInvocation("./bin/demo", "Notify", "argv_json", []byte(`{"ok":true}`), "codex-runtime")
	if err != nil {
		t.Fatal(err)
	}
	if len(stdin) != 0 {
		t.Fatalf("stdin = %q", stdin)
	}
	if len(args) != 3 || args[1] != "notify" {
		t.Fatalf("args = %#v", args)
	}
}

func TestResolveRuntimeTestPlatformGeminiRequestedReturnsRuntimeGuidance(t *testing.T) {
	t.Parallel()
	_, err := resolveRuntimeTestPlatform([]string{"gemini"}, "gemini")
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{
		"Gemini uses its dedicated runtime gate instead",
		"go mod tidy",
		"go test ./...",
		"plugin-kit-ai validate . --platform gemini --strict",
		"plugin-kit-ai inspect . --target gemini",
		"plugin-kit-ai capabilities --mode runtime --platform gemini",
		"make test-gemini-runtime",
		"gemini extensions link .",
		"make test-gemini-runtime-live",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q:\n%s", want, err)
		}
	}
}

func TestResolveRuntimeTestPlatformGeminiAutoDetectReturnsRuntimeGuidance(t *testing.T) {
	t.Parallel()
	_, err := resolveRuntimeTestPlatform([]string{"gemini"}, "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Gemini uses its dedicated runtime gate instead") {
		t.Fatalf("error = %q", err)
	}
	for _, want := range []string{
		"plugin-kit-ai inspect . --target gemini",
		"make test-gemini-runtime",
		"make test-gemini-runtime-live",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q:\n%s", want, err)
		}
	}
}

func TestPluginServiceExecHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_PLUGIN_TEST_HELPER") != "1" {
		return
	}
	args := os.Args
	sep := -1
	for i, arg := range args {
		if arg == "--" {
			sep = i
			break
		}
	}
	if sep == -1 || sep+2 >= len(args) {
		os.Exit(2)
	}
	cmdArgs := args[sep+2:]
	stdin, _ := io.ReadAll(os.Stdin)
	switch {
	case len(cmdArgs) == 1 && cmdArgs[0] == "Stop":
		if !strings.Contains(string(stdin), `"hook_event_name":"Stop"`) {
			os.Exit(3)
		}
		_, _ = os.Stdout.Write([]byte("{}"))
	case len(cmdArgs) == 1 && cmdArgs[0] == "PreToolUse":
		if !strings.Contains(string(stdin), `"hook_event_name":"PreToolUse"`) || !strings.Contains(string(stdin), `"tool_name":"Bash"`) {
			os.Exit(3)
		}
		_, _ = os.Stdout.Write([]byte("{}"))
	case len(cmdArgs) == 1 && cmdArgs[0] == "UserPromptSubmit":
		if !strings.Contains(string(stdin), `"hook_event_name":"UserPromptSubmit"`) || !strings.Contains(string(stdin), `"prompt":"hello"`) {
			os.Exit(3)
		}
		_, _ = os.Stdout.Write([]byte("{}"))
	case len(cmdArgs) == 2 && cmdArgs[0] == "notify":
		if !strings.Contains(cmdArgs[1], `"client":"codex-tui"`) {
			os.Exit(3)
		}
	default:
		os.Exit(2)
	}
	os.Exit(0)
}

func restorePluginTestHelpers(t *testing.T) {
	t.Helper()
	prev := testCommandContext
	testCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmdArgs := []string{"-test.run=TestPluginServiceExecHelperProcess", "--", name}
		cmdArgs = append(cmdArgs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], cmdArgs...)
		cmd.Env = append(os.Environ(), "GO_WANT_PLUGIN_TEST_HELPER=1")
		return cmd
	}
	t.Cleanup(func() {
		testCommandContext = prev
	})
}

func runtimeTestProjectRoot(t *testing.T, platform string) string {
	t.Helper()
	dir := t.TempDir()
	if err := scaffold.Write(dir, scaffold.Data{
		ProjectName: "demo",
		Platform:    platform,
		Runtime:     "shell",
	}, false); err != nil {
		t.Fatal(err)
	}
	renderRuntimeTestTarget(t, dir, platform)
	return dir
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func renderRuntimeTestTarget(t *testing.T, root, target string) {
	t.Helper()
	result, err := pluginmanifest.Generate(root, target)
	if err != nil {
		t.Fatal(err)
	}
	if err := pluginmanifest.WriteArtifacts(root, result.Artifacts); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		mustChmodBootstrapExecutable(t, filepath.Join(root, "bin", "demo"))
		mustChmodBootstrapExecutable(t, filepath.Join(root, "scripts", "main.sh"))
	}
}
