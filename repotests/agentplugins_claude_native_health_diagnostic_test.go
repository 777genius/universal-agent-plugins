package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const claudeNativeHealthError = "claude Code MCP health output is not recognized"

type claudeNativeDiagnosticRun func(context.Context, legacyports.Command) (legacyports.CommandResult, error)

// This captures a later observation, never the missing original activation
// response. It leaves the first installer result and all qualification gates intact.
func (f *claudeNativeFixture) captureNativeHealthFailure(ctx context.Context, t *testing.T, label string, args, environment []string, result map[string]any) {
	t.Helper()
	report, logs := f.nativeHealthDiagnostics(ctx, label, args, environment, result, func(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
		return claude.RunClaudeListCommand(ctx, process.OS{}, command)
	})
	if report == nil {
		return
	}
	for name, body := range logs {
		if err := os.WriteFile(filepath.Join(f.Root, name), body, 0600); err != nil {
			t.Logf("Claude post-failure diagnostic stream was not saved: %s", boundedClaudeDiagnosticError(err))
		}
	}
	body, err := json.MarshalIndent(report, "", "  ")
	if err == nil && len(body) > 2*claudeDiagnosticLimit {
		err = errors.New("claude post-failure diagnostic report exceeds its bound")
	}
	if err == nil {
		err = os.WriteFile(filepath.Join(f.Root, "failure-user-mcp-diagnostics.json"), body, 0600)
	}
	if err != nil {
		t.Logf("Claude post-failure diagnostic report was not saved: %s", boundedClaudeDiagnosticError(err))
	}
}

func (f *claudeNativeFixture) nativeHealthDiagnosticSelected(label string, args []string, result map[string]any) bool {
	if label != "add-v1" && label != "A-install" {
		return false
	}
	if !filepath.IsAbs(f.Root) || !filepath.IsAbs(f.diagnosticClient) || f.Root != filepath.Clean(f.Root) {
		return false
	}
	if f.Home != filepath.Join(f.Root, "home") || f.ConfigDir != filepath.Join(f.Root, "claude-config") ||
		f.StateHome != filepath.Join(f.Root, "installer-state") || f.PackageRoot != filepath.Join(f.Root, "fixture") {
		return false
	}
	if !slices.Equal(args, []string{"add", f.PackageRoot, "--target", "claude"}) {
		return false
	}
	return claudeNativeHealthFailedTarget(result)
}

func claudeNativeHealthFailedTarget(result map[string]any) bool {
	data, _ := result["data"].(map[string]any)
	if result["command"] != "add" || result["result"] != "failure" || data["plugin"] != "claude-native-proof" ||
		data["status"] != "external_partial_failure" || data["dry_run"] != false || data["failed"] != float64(1) || data["succeeded"] != float64(0) {
		return false
	}
	targets, _ := data["targets"].([]any)
	if len(targets) != 1 {
		return false
	}
	target, _ := targets[0].(map[string]any)
	output, _ := target["output"].(map[string]any)
	native, _ := output["result"].(map[string]any)
	return target["target"] == "claude" && target["status"] == "external_failed" &&
		output["plugin"] == "claude-native-proof" && output["scope"] == "user" &&
		claudeNativeHealthFailure(target["error"]) && claudeNativeHealthFailure(native["failure"])
}

func claudeNativeHealthFailure(value any) bool {
	failure, _ := value.(map[string]any)
	return failure["stage"] == "activation" && failure["message"] == claudeNativeHealthError
}

func (f *claudeNativeFixture) nativeHealthDiagnosticCommand(environment []string) (legacyports.Command, error) {
	env, err := claudeFailureMCPEnvironment(f.ConfigDir, append([]string(nil), environment...))
	if err != nil {
		return legacyports.Command{}, err
	}
	homeKey := "HOME="
	if runtime.GOOS == "windows" {
		homeKey = "USERPROFILE="
	}
	if !slices.Contains(env, homeKey+f.Home) {
		return legacyports.Command{}, errors.New("claude diagnostic home differs from the original fixture")
	}
	return legacyports.Command{Argv: []string{f.diagnosticClient}, Env: env, Dir: f.Home, StdoutLimitBytes: claudeDiagnosticLimit}, nil
}

func (f *claudeNativeFixture) nativeHealthDiagnostics(parent context.Context, label string, args, environment []string, original map[string]any, run claudeNativeDiagnosticRun) (map[string]any, map[string][]byte) {
	if f.diagnosticAttempted || parent.Err() != nil || !f.nativeHealthDiagnosticSelected(label, args, original) {
		return nil, nil
	}
	f.diagnosticAttempted = true
	report := map[string]any{"installer_label": label, "original_installer_error": claudeNativeHealthError,
		"original_failure_preserved": true, "original_get_response": false, "qualification": false,
		"config_root": f.ConfigDir, "context_budget_ms": 10000, "descendant_grace_ms": 5000}
	logs := map[string][]byte{}
	command, err := f.nativeHealthDiagnosticCommand(environment)
	if err != nil {
		report["diagnostic_error"] = boundedClaudeDiagnosticError(err)
		return report, logs
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	before, err := f.nativeHealthDiagnosticProfiles()
	if err != nil {
		report["before_error"] = boundedClaudeDiagnosticError(err)
		return report, logs
	}
	report["before_diagnostic"] = before
	records := claudeNativeHealthRecords(ctx, command, run, logs)
	report["diagnostics"] = records
	after, err := f.nativeHealthDiagnosticProfiles()
	if err != nil {
		report["after_error"] = boundedClaudeDiagnosticError(err)
	} else {
		report["after_diagnostic"] = after
		report["diagnostic_profiles_unchanged"] = reflect.DeepEqual(before, after)
	}
	return report, logs
}

func claudeNativeHealthRecords(ctx context.Context, command legacyports.Command, run claudeNativeDiagnosticRun, logs map[string][]byte) []map[string]any {
	records := make([]map[string]any, 0, 3)
	for _, name := range []string{"local", "remote-http", "remote-sse"} {
		if ctx.Err() != nil {
			break
		}
		probe := command
		probe.Argv = []string{command.Argv[0], "mcp", "get", "plugin:claude-native-proof:" + name}
		probe.Env = append([]string(nil), command.Env...)
		started := time.Now()
		result, err := run(ctx, probe)
		captured := claudeDiagnosticResult{Stdout: result.Stdout, Stderr: result.Stderr, Exit: result.ExitCode,
			Deadline: errors.Is(ctx.Err(), context.DeadlineExceeded), StdoutLimit: errors.Is(err, process.ErrStdoutLimitExceeded), Elapsed: time.Since(started)}
		if err != nil {
			captured.Error = boundedClaudeDiagnosticError(err)
		}
		record, streams := claudeDiagnosticRecord("failure-user-"+name+"-mcp-get", probe.Argv[0], probe.Dir, probe.Argv[1:], captured)
		record["post_failure"], record["original_get_response"] = true, false
		record["cleanup_complete"], record["no_child_started"] = result.CleanupComplete, result.NoChildStarted
		record["stdout_limit_bytes"], record["stderr_retained_limit_bytes"] = claudeDiagnosticLimit, 32*1024
		delete(record, "stderr_limit") // The existing runner exposes retained bytes, not a truncation flag.
		record["stderr_truncation"] = "not_reported_by_runner"
		records = append(records, record)
		for filename, body := range streams {
			logs[filename] = body
		}
		if err != nil || result.ExitCode != 0 || !result.CleanupComplete {
			break // Never launch a later probe after unknown or failed process closure.
		}
	}
	return records
}

// These are the same fixture-owned profile inventories used by the A/B test.
// They are observations around a later diagnostic, never a new acceptance baseline.
func (f *claudeNativeFixture) nativeHealthDiagnosticProfiles() (map[string]any, error) {
	result := map[string]any{}
	for _, name := range []string{"home", "claude-config", "claude-config-B", "installer-state", "installer-state-B", "xdg-config", "xdg-data", "xdg-cache", "xdg-state", "tmp", "fixture"} {
		root := filepath.Join(f.Root, name)
		info, err := os.Lstat(root)
		if errors.Is(err, os.ErrNotExist) {
			result[name] = map[string]any{"exists": false}
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("claude diagnostic profile %s is not an original directory", name)
		}
		files, err := claudeProfileFilesResult(root)
		if err != nil {
			return nil, err
		}
		result[name] = map[string]any{"exists": true, "files": files}
	}
	return result, nil
}
