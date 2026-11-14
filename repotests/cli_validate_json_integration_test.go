package pluginkitairepo_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPluginKitAIValidateJSONReportsWarningsAndFailures(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)

	t.Run("success with warnings", func(t *testing.T) {
		plugRoot := t.TempDir()

		initCmd := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "codex-runtime", "-o", plugRoot)
		if out, err := initCmd.CombinedOutput(); err != nil {
			t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
		}
		bootstrapGeneratedGoPlugin(t, plugRoot)

		manifestPath := filepath.Join(plugRoot, authoredRel("plugin.yaml"))
		body, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatal(err)
		}
		body = append(body, []byte("extra_field: true\n")...)
		if err := os.WriteFile(manifestPath, body, 0o644); err != nil {
			t.Fatal(err)
		}

		validateCmd := exec.Command(pluginKitAIBin, "validate", plugRoot, "--platform", "codex-runtime", "--format", "json")
		validateCmd.Env = append(os.Environ(), "GOWORK=off")
		out, err := validateCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("plugin-kit-ai validate --format json: %v\n%s", err, out)
		}

		var report struct {
			Format            string           `json:"format"`
			SchemaVersion     int              `json:"schema_version"`
			RequestedPlatform string           `json:"requested_platform"`
			Outcome           string           `json:"outcome"`
			Platform          string           `json:"platform"`
			OK                bool             `json:"ok"`
			StrictMode        bool             `json:"strict_mode"`
			StrictFailed      bool             `json:"strict_failed"`
			WarningCount      int              `json:"warning_count"`
			FailureCount      int              `json:"failure_count"`
			Checks            []string         `json:"checks"`
			Warnings          []map[string]any `json:"warnings"`
			Failures          []map[string]any `json:"failures"`
		}
		if err := json.Unmarshal(out, &report); err != nil {
			t.Fatalf("parse validate json: %v\n%s", err, out)
		}
		if report.Format != "plugin-kit-ai/validate-report" || report.SchemaVersion != 1 {
			t.Fatalf("contract = %#v", report)
		}
		if report.RequestedPlatform != "codex-runtime" || report.Outcome != "passed" {
			t.Fatalf("requested platform/outcome = %#v", report)
		}
		if report.Platform != "codex-runtime" {
			t.Fatalf("platform = %q", report.Platform)
		}
		if !report.OK || report.StrictMode || report.StrictFailed {
			t.Fatalf("summary = %#v", report)
		}
		if report.WarningCount == 0 || report.FailureCount != 0 {
			t.Fatalf("counts = %#v", report)
		}
		if len(report.Checks) == 0 {
			t.Fatalf("checks = %#v", report.Checks)
		}
		if len(report.Warnings) == 0 {
			t.Fatalf("warnings = %#v", report.Warnings)
		}
		if report.Failures == nil || len(report.Failures) != 0 {
			t.Fatalf("failures = %#v", report.Failures)
		}
	})

	t.Run("failure still prints json", func(t *testing.T) {
		missingRoot := t.TempDir()
		validateCmd := exec.Command(pluginKitAIBin, "validate", missingRoot, "--format", "json")
		validateCmd.Env = append(os.Environ(), "GOWORK=off")
		out, err := validateCmd.CombinedOutput()
		if err == nil {
			t.Fatal("expected validate failure")
		}

		var report struct {
			Format        string           `json:"format"`
			SchemaVersion int              `json:"schema_version"`
			Outcome       string           `json:"outcome"`
			OK            bool             `json:"ok"`
			StrictMode    bool             `json:"strict_mode"`
			StrictFailed  bool             `json:"strict_failed"`
			WarningCount  int              `json:"warning_count"`
			FailureCount  int              `json:"failure_count"`
			Checks        []string         `json:"checks"`
			Warnings      []map[string]any `json:"warnings"`
			Failures      []struct {
				Kind string `json:"kind"`
				Path string `json:"path"`
			} `json:"failures"`
		}
		if err := json.Unmarshal(out, &report); err != nil {
			t.Fatalf("parse failure json: %v\n%s", err, out)
		}
		if report.Format != "plugin-kit-ai/validate-report" || report.SchemaVersion != 1 || report.Outcome != "failed" {
			t.Fatalf("contract = %#v", report)
		}
		if report.OK || report.StrictMode || report.StrictFailed {
			t.Fatalf("summary = %#v", report)
		}
		if report.WarningCount != 0 || report.FailureCount != 1 {
			t.Fatalf("counts = %#v", report)
		}
		if report.Checks == nil || len(report.Checks) != 0 {
			t.Fatalf("checks = %#v", report.Checks)
		}
		if report.Warnings == nil || len(report.Warnings) != 0 {
			t.Fatalf("warnings = %#v", report.Warnings)
		}
		if len(report.Failures) != 1 {
			t.Fatalf("failures = %#v", report.Failures)
		}
		if report.Failures[0].Kind != "manifest_missing" || report.Failures[0].Path != authoredSlash("plugin.yaml") {
			t.Fatalf("failure = %#v", report.Failures[0])
		}
	})

	t.Run("strict warning failure is explicit", func(t *testing.T) {
		plugRoot := t.TempDir()

		initCmd := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "codex-runtime", "-o", plugRoot)
		if out, err := initCmd.CombinedOutput(); err != nil {
			t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
		}
		bootstrapGeneratedGoPlugin(t, plugRoot)

		manifestPath := filepath.Join(plugRoot, authoredRel("plugin.yaml"))
		body, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatal(err)
		}
		body = append(body, []byte("extra_field: true\n")...)
		if err := os.WriteFile(manifestPath, body, 0o644); err != nil {
			t.Fatal(err)
		}

		validateCmd := exec.Command(pluginKitAIBin, "validate", plugRoot, "--platform", "codex-runtime", "--strict", "--format", "json")
		validateCmd.Env = append(os.Environ(), "GOWORK=off")
		out, err := validateCmd.CombinedOutput()
		if err == nil {
			t.Fatal("expected strict validation failure")
		}

		var report struct {
			Format            string           `json:"format"`
			SchemaVersion     int              `json:"schema_version"`
			RequestedPlatform string           `json:"requested_platform"`
			Outcome           string           `json:"outcome"`
			OK                bool             `json:"ok"`
			StrictMode        bool             `json:"strict_mode"`
			StrictFailed      bool             `json:"strict_failed"`
			WarningCount      int              `json:"warning_count"`
			FailureCount      int              `json:"failure_count"`
			Warnings          []map[string]any `json:"warnings"`
			Failures          []map[string]any `json:"failures"`
		}
		if err := json.Unmarshal(out, &report); err != nil {
			t.Fatalf("parse strict warning json: %v\n%s", err, out)
		}
		if report.Format != "plugin-kit-ai/validate-report" || report.SchemaVersion != 1 {
			t.Fatalf("contract = %#v", report)
		}
		if report.RequestedPlatform != "codex-runtime" || report.Outcome != "failed_strict_warnings" {
			t.Fatalf("requested platform/outcome = %#v", report)
		}
		if report.OK || !report.StrictMode || !report.StrictFailed {
			t.Fatalf("summary = %#v", report)
		}
		if report.WarningCount == 0 || report.FailureCount != 0 {
			t.Fatalf("counts = %#v", report)
		}
		if len(report.Warnings) == 0 || len(report.Failures) != 0 {
			t.Fatalf("warnings/failures = %#v", report)
		}
	})
}
