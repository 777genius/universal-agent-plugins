package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func claudeNativeHealthContractFixture(t *testing.T) (*claudeNativeFixture, []string, []string, map[string]any) {
	t.Helper()
	root := t.TempDir()
	f := &claudeNativeFixture{Root: root, Home: filepath.Join(root, "home"), ConfigDir: filepath.Join(root, "claude-config"),
		StateHome: filepath.Join(root, "installer-state"), PackageRoot: filepath.Join(root, "fixture"), diagnosticClient: filepath.Join(root, "verified-claude")}
	for _, path := range []string{f.Home, f.ConfigDir, f.StateHome, f.PackageRoot, filepath.Join(root, "xdg-state")} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	nativeWrite(t, filepath.Join(f.ConfigDir, "settings.json"), []byte("original fixture bytes\n"), 0600)
	nativeWrite(t, filepath.Join(root, "xdg-state", "sentinel"), []byte("original state bytes\n"), 0600)
	body := `{"command":"add","result":"failure","data":{"plugin":"claude-native-proof","status":"external_partial_failure","dry_run":false,"failed":1,"succeeded":0,"targets":[{"target":"claude","status":"external_failed","error":{"stage":"activation","message":"claude Code MCP health output is not recognized"},"output":{"scope":"user","plugin":"claude-native-proof","result":{"failure":{"stage":"activation","message":"claude Code MCP health output is not recognized"}}}}]}}`
	var result map[string]any
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatal(err)
	}
	return f, []string{"add", f.PackageRoot, "--target", "claude"}, []string{"HOME=" + f.Home, "USERPROFILE=" + f.Home, "PATH=" + root,
		"ANTHROPIC_AUTH_TOKEN=inert-secret", "ANTHROPIC_BASE_URL=http://model.invalid", "CLAUDE_CONFIG_DIR=unselected"}, result
}

func TestClaudeNativeHealthDiagnosticPreservesFailureAndOriginalContext(t *testing.T) {
	for _, label := range []string{"add-v1", "A-install"} {
		t.Run(label, func(t *testing.T) {
			f, args, env, original := claudeNativeHealthContractFixture(t)
			before, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			originalEnv := append([]string(nil), env...)
			calls := 0
			report, logs := f.nativeHealthDiagnostics(context.Background(), label, args, env, original, func(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
				name := []string{"local", "remote-http", "remote-sse"}[calls]
				calls++
				wantEnv := []string{"HOME=" + f.Home, "PATH=" + f.Root, "USERPROFILE=" + f.Home, "CLAUDE_CONFIG_DIR=" + f.ConfigDir,
					"DISABLE_AUTOUPDATER=1", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1"}
				if !slices.Equal(command.Argv, []string{f.diagnosticClient, "mcp", "get", "plugin:claude-native-proof:" + name}) ||
					command.Dir != f.Home || !slices.Equal(command.Env, wantEnv) || command.StdoutLimitBytes != 1<<20 || command.DiscardStdout {
					t.Fatal("diagnostic escaped its selected original context")
				}
				if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 10*time.Second {
					t.Fatal("diagnostic lost its shared finite context")
				}
				command.Env[0] = "changed child copy"
				nativeWrite(t, filepath.Join(f.ConfigDir, "settings.json"), []byte("later diagnostic effect\n"), 0600)
				return legacyports.CommandResult{CleanupComplete: true, Stdout: []byte("unknown native format\r\n"), Stderr: []byte("separate diagnostic")}, nil
			})
			after, err := json.Marshal(original)
			if err != nil || string(before) != string(after) || !slices.Equal(env, originalEnv) || calls != 3 || len(logs) != 6 {
				t.Fatal("diagnostic changed original evidence or its finite call set")
			}
			claudeRequireNativeHealthReport(t, report, logs)
			if report["diagnostic_profiles_unchanged"] != false || reflect.DeepEqual(report["before_diagnostic"], report["after_diagnostic"]) {
				t.Fatal("later diagnostic effects replaced the original before snapshot")
			}
			if again, _ := f.nativeHealthDiagnostics(context.Background(), label, args, env, original, nil); again != nil {
				t.Fatal("a fixture attempted its diagnostic twice")
			}
		})
	}
}

func claudeRequireNativeHealthReport(t *testing.T, report map[string]any, logs map[string][]byte) {
	t.Helper()
	before, _ := report["before_diagnostic"].(map[string]any)
	after, _ := report["after_diagnostic"].(map[string]any)
	state, _ := before["xdg-state"].(map[string]any)
	files, _ := state["files"].(map[string]claudeProfileFile)
	if state["exists"] != true || files["sentinel"].Kind != "file" || !reflect.DeepEqual(state, after["xdg-state"]) {
		t.Fatal("diagnostic lost the original XDG state inventory")
	}
	if report["qualification"] != false || report["original_get_response"] != false || report["original_failure_preserved"] != true || report["original_installer_error"] != claudeNativeHealthError {
		t.Fatal("later observation qualified or replaced the original failure")
	}
	for _, row := range report["diagnostics"].([]map[string]any) {
		if row["qualification"] != false || row["post_failure"] != true || row["original_get_response"] != false || row["cleanup_complete"] != true || row["no_child_started"] != false {
			t.Fatal("closed diagnostic metadata was lost")
		}
		if row["stdout_limit_bytes"] != 1<<20 || row["stderr_retained_limit_bytes"] != 32*1024 || row["stderr_truncation"] != "not_reported_by_runner" {
			t.Fatal("diagnostic misrepresented the supervised runner's capture limits")
		}
		if _, claimed := row["stderr_limit"]; claimed || string(logs[row["stdout_file"].(string)]) != "unknown native format\r\n" || string(logs[row["stderr_file"].(string)]) != "separate diagnostic" {
			t.Fatal("separate raw streams were replaced or stderr completeness invented")
		}
	}
}

func TestClaudeNativeHealthDiagnosticRefusesOtherFailureOrContext(t *testing.T) {
	for _, mode := range []string{"B-install", "other-command", "other-target", "other-plugin", "other-error", "other-scope", "other-native-error", "unselected-config", "unselected-home", "duplicate-env", "nul-env", "canceled", "inventory-error"} {
		t.Run(mode, func(t *testing.T) {
			f, args, env, original := claudeNativeHealthContractFixture(t)
			label := "add-v1"
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			label, args, env = claudeMutateNativeHealthContext(t, mode, f, label, args, env, original, cancel)
			report, logs := f.nativeHealthDiagnostics(ctx, label, args, env, original, func(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
				t.Fatal("refused diagnostic started a native command")
				return legacyports.CommandResult{}, nil
			})
			if len(logs) != 0 || report != nil && (report["qualification"] != false || report["original_failure_preserved"] != true) {
				t.Fatal("refused capture acquired evidence or changed qualification")
			}
			if slices.Contains([]string{"unselected-home", "duplicate-env", "nul-env"}, mode) && (report == nil || report["diagnostic_error"] == nil) {
				t.Fatal("environment refusal was not retained")
			}
			if mode == "inventory-error" && (report == nil || report["before_error"] == nil) {
				t.Fatal("incomplete before inventory was not refused")
			}
		})
	}
}

func claudeMutateNativeHealthContext(t *testing.T, mode string, f *claudeNativeFixture, label string, args, env []string, original map[string]any, cancel context.CancelFunc) (string, []string, []string) {
	t.Helper()
	data := original["data"].(map[string]any)
	target := data["targets"].([]any)[0].(map[string]any)
	output := target["output"].(map[string]any)
	switch mode {
	case "B-install":
		label = mode
	case "other-command":
		args[0] = "repair"
	case "other-target":
		target["target"] = "other"
	case "other-plugin":
		data["plugin"] = "other"
	case "other-error":
		target["error"].(map[string]any)["message"] = "process cleanup unconfirmed"
	case "other-scope":
		output["scope"] = "project"
	case "other-native-error":
		output["result"].(map[string]any)["failure"].(map[string]any)["message"] = "unknown exit"
	case "unselected-config":
		f.ConfigDir = filepath.Join(f.Root, "claude-config-B")
	case "unselected-home":
		env[0], env[1] = "HOME="+f.Root, "USERPROFILE="+f.Root
	case "duplicate-env":
		env = append(env, "home="+f.Home)
	case "nul-env":
		env[2] = "PATH=x\x00y"
	case "canceled":
		cancel()
	case "inventory-error":
		if err := os.WriteFile(filepath.Join(f.Root, "claude-config-B"), []byte("not a profile directory"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return label, args, env
}

func TestClaudeNativeHealthDiagnosticStopsAfterUnclosedOrFailedProbe(t *testing.T) {
	for _, mode := range []string{"unclosed", "unclosed-with-error", "no-child", "output-limit", "nonzero-exit", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			f, args, env, original := claudeNativeHealthContractFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			report, logs := f.nativeHealthDiagnostics(ctx, "A-install", args, env, original, func(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
				calls++
				result := legacyports.CommandResult{Stdout: []byte("prefix"), Stderr: []byte("diagnostic")}
				err := errors.New("original diagnostic failure")
				switch mode {
				case "unclosed":
					err = nil
				case "no-child":
					result.NoChildStarted = true
				case "output-limit":
					result.CleanupComplete, err = true, process.ErrStdoutLimitExceeded
				case "nonzero-exit":
					result.CleanupComplete, result.ExitCode, err = true, 1, nil
				case "canceled":
					cancel()
					result.CleanupComplete, err = true, context.Canceled
				}
				return result, err
			})
			rows := report["diagnostics"].([]map[string]any)
			if calls != 1 || len(rows) != 1 || len(logs) != 2 || rows[0]["qualification"] != false || report["qualification"] != false {
				t.Fatal("failed or unclosed observation was followed by another native probe")
			}
			if rows[0]["no_child_started"] != (mode == "no-child") || rows[0]["cleanup_complete"] != slices.Contains([]string{"output-limit", "nonzero-exit", "canceled"}, mode) {
				t.Fatal("original process ownership facts were lost")
			}
			if rows[0]["stdout_limit"] != (mode == "output-limit") || !strings.HasSuffix(rows[0]["stdout_file"].(string), ".stdout.log") {
				t.Fatal("incomplete stream metadata was changed")
			}
		})
	}
}
