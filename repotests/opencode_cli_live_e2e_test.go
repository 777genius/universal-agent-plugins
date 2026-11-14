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

// OpenCode CLI real-model e2e. Example:
//
//	PLUGIN_KIT_AI_RUN_OPENCODE_CLI=1 go test ./repotests -run TestOpenCodeCLIPluginLoadSmoke -v -args -opencode-models=openai/gpt-5.1-codex-mini,openai/gpt-5.3-codex-spark,openai/gpt-5.4-mini
var opencodeModels = flag.String("opencode-models", "openai/gpt-5.1-codex-mini,openai/gpt-5.3-codex-spark,openai/gpt-5.4-mini", "comma-separated opencode run --model values for CLI e2e")

func TestOpenCodeCLIPluginLoadSmoke(t *testing.T) {
	opencodeBin := openCodeCLIBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	for _, model := range openCodeModelsOrSkip(t, opencodeBin) {
		model := model
		t.Run(openCodeModelTestName(model), func(t *testing.T) {
			workDir := newOpenCodeSmokeWorkspace(t, pluginKitAIBin, openCodeSmokeWorkspaceOptions{})
			markerPath := filepath.Join(t.TempDir(), "opencode-plugin-marker.json")
			env := append(newIsolatedOpenCodeEnv(t), "PLUGIN_KIT_AI_OPENCODE_SMOKE_MARKER="+markerPath)
			out := runOpenCodeCommand(t, opencodeBin, workDir, env,
				"run",
				"Reply with exactly OK.",
				"--model", model,
				"--format", "json",
				"--dangerously-skip-permissions",
			)
			assertOpenCodePluginMarker(t, markerPath)
			if !strings.Contains(out, `"type":"step_start"`) {
				t.Fatalf("opencode plugin smoke output missing step_start event:\n%s", out)
			}
		})
	}
}

type openCodeSmokeWorkspaceOptions struct{}

type openCodeJSONEvent struct {
	Type  string `json:"type"`
	Error *struct {
		Name string `json:"name"`
		Data struct {
			Message      string `json:"message"`
			ResponseBody string `json:"responseBody"`
		} `json:"data"`
	} `json:"error,omitempty"`
}

func openCodeCLIBinaryOrSkip(t *testing.T) string {
	t.Helper()
	if strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_SKIP_OPENCODE_CLI")) == "1" {
		t.Skip("PLUGIN_KIT_AI_SKIP_OPENCODE_CLI=1")
	}
	if strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_RUN_OPENCODE_CLI")) != "1" {
		t.Skip("set PLUGIN_KIT_AI_RUN_OPENCODE_CLI=1 to run real OpenCode CLI e2e (see -args -opencode-models)")
	}
	opencodeBin := strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_E2E_OPENCODE"))
	if opencodeBin == "" {
		var err error
		opencodeBin, err = exec.LookPath("opencode")
		if err != nil {
			t.Skip("opencode not in PATH; set PLUGIN_KIT_AI_E2E_OPENCODE or install OpenCode CLI")
		}
	}
	if out, err := exec.Command(opencodeBin, "--version").CombinedOutput(); err != nil {
		t.Skipf("OpenCode CLI is not runnable in this environment: %v\n%s", err, out)
	}
	return opencodeBin
}

func openCodeModelsOrSkip(t *testing.T, opencodeBin string) []string {
	t.Helper()
	list := make([]string, 0, 3)
	seen := make(map[string]struct{})
	for _, raw := range strings.Split(*opencodeModels, ",") {
		model := strings.TrimSpace(raw)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		list = append(list, model)
	}
	if len(list) == 0 {
		t.Fatal("opencode live smoke requires at least one model in -opencode-models")
	}
	for _, model := range list {
		ensureOpenCodeModelAvailable(t, opencodeBin, model)
	}
	return list
}

func ensureOpenCodeModelAvailable(t *testing.T, opencodeBin, want string) {
	t.Helper()
	out, err := exec.Command(opencodeBin, "models", "openai").CombinedOutput()
	if err != nil {
		t.Skipf("opencode models openai failed in this environment: %v\n%s", err, out)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == want {
			return
		}
	}
	t.Skipf("opencode models openai does not list %q in this build:\n%s", want, out)
}

func newOpenCodeSmokeWorkspace(t *testing.T, pluginKitAIBin string, opts openCodeSmokeWorkspaceOptions) string {
	t.Helper()
	root := RepoRoot(t)
	workDir := filepath.Join(t.TempDir(), "opencode-cli-live")
	copyTree(t, filepath.Join(root, "examples", "plugins", "opencode-basic"), workDir)
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", workDir))
	runCmd(t, root, exec.Command(pluginKitAIBin, "validate", workDir, "--platform", "opencode", "--strict"))
	return workDir
}

func newIsolatedOpenCodeEnv(t *testing.T) []string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "opencode-home")
	seedOpenCodeAuthOrSkip(t, filepath.Join(root, ".local", "share", "opencode"))
	return []string{
		"HOME=" + root,
		"XDG_CONFIG_HOME=" + filepath.Join(root, ".config"),
		"XDG_DATA_HOME=" + filepath.Join(root, ".local", "share"),
		"XDG_STATE_HOME=" + filepath.Join(root, ".local", "state"),
		"XDG_CACHE_HOME=" + filepath.Join(root, ".cache"),
		"OPENAI_API_KEY=",
		"NO_COLOR=1",
	}
}

func seedOpenCodeAuthOrSkip(t *testing.T, destDir string) {
	t.Helper()
	src := filepath.Join(os.Getenv("HOME"), ".local", "share", "opencode", "auth.json")
	body, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("opencode auth.json not found; run `opencode providers login -p openai` first")
		}
		t.Fatalf("read opencode auth.json: %v", err)
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destDir, "auth.json"), body, 0o600); err != nil {
		t.Fatalf("seed opencode auth.json: %v", err)
	}
}

func runOpenCodeCommand(t *testing.T, opencodeBin, workDir string, extraEnv []string, args ...string) string {
	t.Helper()
	for attempt := 1; attempt <= 2; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		cmd := exec.CommandContext(ctx, opencodeBin, args...)
		cmd.Dir = workDir
		cmd.Env = append(os.Environ(), extraEnv...)
		out, err := cmd.CombinedOutput()
		cancel()
		text := string(out)
		if ctx.Err() == context.DeadlineExceeded {
			if attempt < 2 {
				t.Logf("opencode %q timed out on attempt %d, retrying once", strings.Join(args, " "), attempt)
				continue
			}
			t.Fatalf("opencode %q timed out:\n%s", strings.Join(args, " "), truncateRunes(text, 4000))
		}
		if envErr := openCodeJSONError(text); envErr != "" {
			if openCodeEnvironmentIssue(envErr) {
				t.Skipf("opencode environment is not ready for live smoke:\n%s", truncateRunes(text, 4000))
			}
			t.Fatalf("opencode %q reported an error event:\n%s", strings.Join(args, " "), text)
		}
		if err != nil {
			t.Fatalf("opencode %q: %v\n%s", strings.Join(args, " "), err, text)
		}
		t.Logf("opencode %s output: %s", strings.Join(args, " "), truncateRunes(text, 4000))
		return text
	}
	t.Fatal("unreachable")
	return ""
}

func openCodeJSONError(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var event openCodeJSONEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event.Type != "error" || event.Error == nil {
			continue
		}
		msg := strings.TrimSpace(event.Error.Data.Message)
		if msg != "" {
			if body := strings.TrimSpace(event.Error.Data.ResponseBody); body != "" {
				return msg + "\n" + body
			}
			return msg
		}
		return line
	}
	return ""
}

func openCodeEnvironmentIssue(output string) bool {
	lower := strings.ToLower(output)
	markers := []string{
		"insufficient_quota",
		"quota exceeded",
		"check your plan and billing details",
		"authentication",
		"unauthorized",
		"invalid api key",
		"api key",
		"credential",
		"login required",
		"provider",
		"model_not_found",
		"does not exist",
		"model unavailable",
		"rate limit",
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func openCodeModelTestName(model string) string {
	name := model
	replacer := strings.NewReplacer("/", "_", ".", "_", "-", "_")
	return replacer.Replace(name)
}

func assertOpenCodePluginMarker(t *testing.T, path string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read OpenCode plugin marker: %v", err)
	}
	var doc struct {
		Directory string `json:"directory"`
		Worktree  string `json:"worktree"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse OpenCode plugin marker: %v\n%s", err, body)
	}
	if strings.TrimSpace(doc.Directory) == "" {
		t.Fatalf("OpenCode plugin marker missing directory:\n%s", body)
	}
}
