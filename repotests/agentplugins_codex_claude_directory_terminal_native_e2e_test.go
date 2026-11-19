package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"
)

type nativeTerminalPluginReport struct {
	Status                      string   `json:"status"`
	Plugin                      string   `json:"plugin"`
	Scope                       string   `json:"scope"`
	SelectedTargets             []string `json:"selected_targets"`
	Confirmations               int      `json:"confirmations"`
	UnchangedBeforeConfirmation bool     `json:"unchanged_before_confirmation"`
	ProgressObserved            bool     `json:"progress_observed"`
	InstallationCompleted       bool     `json:"installation_completed"`
	ClosureVerified             bool     `json:"closure_verified"`
	ModelRequest                *bool    `json:"model_request"`
	AuthenticationQualified     *bool    `json:"authentication_qualified"`
	NativeReadinessQualified    *bool    `json:"native_readiness_qualified"`
	Result                      struct {
		InstallationID string `json:"installation_id"`
	} `json:"result"`
}

// This is a terminal/consent contract. Native discovery and full installation
// ownership remain independently required by the existing Directory lifecycle.
func validateNativeTerminalPluginReport(body []byte) (string, error) {
	if len(body) > nativeTerminalDriverLogLimit {
		return "", fmt.Errorf("terminal plugin evidence exceeds its 1 MiB bound")
	}
	var report nativeTerminalPluginReport
	if err := json.Unmarshal(body, &report); err != nil {
		return "", fmt.Errorf("decode terminal plugin evidence: %w", err)
	}
	if report.Status != "passed" || report.Plugin != directorySitePlugin || report.Scope != "user" ||
		!reflect.DeepEqual(report.SelectedTargets, []string{"claude", "codex"}) || report.Confirmations != 1 ||
		!report.UnchangedBeforeConfirmation || !report.ProgressObserved || !report.InstallationCompleted || !report.ClosureVerified ||
		report.ModelRequest == nil || *report.ModelRequest || report.AuthenticationQualified == nil || *report.AuthenticationQualified ||
		report.NativeReadinessQualified == nil || *report.NativeReadinessQualified || report.Result.InstallationID == "" {
		return "", fmt.Errorf("terminal plugin keyboard/consent/completion/closure contract is incomplete")
	}
	return report.Result.InstallationID, nil
}

func readNativeTerminalPluginReport(t *testing.T, root string) string {
	t.Helper()
	file, err := os.Open(filepath.Join(root, "native-terminal-plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close terminal plugin evidence: %v", err)
		}
	}()
	if info, err := file.Stat(); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("terminal plugin evidence is not a regular file: %v", err)
	}
	body, err := io.ReadAll(io.LimitReader(file, nativeTerminalDriverLogLimit+1))
	if err != nil {
		t.Fatal(err)
	}
	installationID, err := validateNativeTerminalPluginReport(body)
	if err != nil {
		t.Fatal(err)
	}
	return installationID
}

func nativeTerminalDirectoryPluginInstall(t *testing.T, f *nativeFixture, env []string, installer string) string {
	t.Helper()
	if runtime.GOOS != "linux" || os.Getenv("AGENTPLUGINS_NATIVE_BROWSER_LIVE") != "1" {
		t.Fatal("terminal Directory proof requires the current-source Linux native UI lane")
	}
	python := nativeBinary(t, "AGENTPLUGINS_NATIVE_BROWSER_PYTHON")
	source := os.Getenv("AGENTPLUGINS_NATIVE_BROWSER_SOURCE")
	driver := filepath.Join(source, "scripts", "terminal-ui", "native_plugin.py")
	if info, err := os.Stat(driver); !filepath.IsAbs(source) || err != nil || !info.Mode().IsRegular() {
		t.Fatalf("terminal Directory tooling file unavailable: %s: %v", driver, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	command := nativeTerminalDriverCommand(ctx, python, driver, "--installer", installer, "--root", f.Root, "--project", f.Project)
	command.Dir = source
	command.Env = append(append([]string(nil), env...), "TERM=xterm-256color", "CI=1",
		"AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED=1", "GITHUB_ACTIONS=true", "RUNNER_ENVIRONMENT=github-hosted")
	var output nativeTerminalDriverOutput
	command.Stdout, command.Stderr = &output, &output
	err := command.Run()
	nativeWrite(t, filepath.Join(f.Root, "native-terminal-plugin-driver.log"), output.Bytes(), 0600)
	if output.overflow {
		t.Fatal("terminal Directory driver diagnostic exceeded its 1 MiB evidence bound")
	}
	if err != nil || ctx.Err() != nil {
		t.Fatalf("terminal Directory install failed: %v; deadline=%v\n%s", err, ctx.Err(), nativeBrowserReportTail(output.Bytes()))
	}
	return readNativeTerminalPluginReport(t, f.Root)
}

func TestNativeTerminalDirectoryPluginReportContract(t *testing.T) {
	valid := map[string]any{
		"status": "passed", "plugin": directorySitePlugin, "scope": "user", "selected_targets": []string{"claude", "codex"},
		"confirmations": 1, "unchanged_before_confirmation": true, "progress_observed": true,
		"installation_completed": true, "closure_verified": true, "model_request": false,
		"authentication_qualified": false, "native_readiness_qualified": false,
		"result": map[string]any{"installation_id": "selected-context7"},
	}
	encode := func(value map[string]any) []byte {
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	if installationID, err := validateNativeTerminalPluginReport(encode(valid)); err != nil || installationID != "selected-context7" {
		t.Fatalf("valid report lost its exact installation identity: %s, %v", installationID, err)
	}
	for _, field := range []string{"status", "plugin", "scope", "selected_targets", "confirmations", "unchanged_before_confirmation",
		"progress_observed", "installation_completed", "closure_verified", "model_request", "authentication_qualified", "native_readiness_qualified", "result"} {
		t.Run("missing-"+field, func(t *testing.T) {
			item := make(map[string]any, len(valid))
			for key, value := range valid {
				if key != field {
					item[key] = value
				}
			}
			if _, err := validateNativeTerminalPluginReport(encode(item)); err == nil {
				t.Fatalf("missing %s was accepted", field)
			}
		})
	}
	for _, item := range []struct {
		name, field string
		value       any
	}{
		{"failure", "status", "failed"}, {"other-source", "plugin", "other"}, {"other-scope", "scope", "project"},
		{"duplicate-target", "selected_targets", []string{"claude", "claude"}}, {"extra-target", "selected_targets", []string{"claude", "codex", "other"}},
		{"multiple-confirmations", "confirmations", 2}, {"preconfirmation-write", "unchanged_before_confirmation", false},
		{"no-progress", "progress_observed", false}, {"incomplete", "installation_completed", false}, {"live-process", "closure_verified", false},
		{"model", "model_request", true}, {"authentication", "authentication_qualified", true},
		{"readiness-promotion", "native_readiness_qualified", true},
		{"missing-visible-identity", "result", map[string]any{}},
	} {
		t.Run(item.name, func(t *testing.T) {
			original := valid[item.field]
			valid[item.field] = item.value
			defer func() { valid[item.field] = original }()
			if _, err := validateNativeTerminalPluginReport(encode(valid)); err == nil {
				t.Fatal("invalid terminal plugin contract was accepted")
			}
		})
	}
	if _, err := validateNativeTerminalPluginReport([]byte("not-json")); err == nil {
		t.Fatal("malformed evidence was accepted")
	}
	if _, err := validateNativeTerminalPluginReport(make([]byte, nativeTerminalDriverLogLimit+1)); err == nil {
		t.Fatal("oversized evidence was accepted")
	}
}
