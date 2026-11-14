package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func claudeProjectPreparedStages() map[string]nativeStage {
	stages := map[string]nativeStage{}
	for _, name := range []string{"install", "unchanged_add", "unchanged_update", "invocation_change_update", "intact_repair", "absent_repair", "remove", "project_scope_preservation", "native_suppression_boundary", "prepared_command_projection", "prepared_explicit_command_withheld"} {
		stages[name] = nativeStage{Status: "not_evaluated", Reason: "not reached"}
	}
	for _, name := range []string{"native_skill_discovery", "command_skill_discovery"} {
		stages[name] = nativeStage{Status: "blocked", Reason: "project trust is not granted; filesystem projection does not prove native discovery"}
	}
	for _, name := range []string{"native_mcp_initialization", "real_model_command_use", "explicit_invocation_enforced"} {
		stages[name] = nativeStage{Status: "not_evaluated", Reason: "no trust, MCP initialization, model invocation or explicit invocation enforcement is exercised"}
	}
	return stages
}

func validateClaudePreparedStages(stages map[string]nativeStage) error {
	expected := claudeProjectPreparedStages()
	if len(stages) != len(expected) {
		return fmt.Errorf("prepared evidence has an unexpected stage set")
	}
	for name, contract := range expected {
		stage, ok := stages[name]
		want := contract.Status
		if want == "not_evaluated" && contract.Reason == "not reached" {
			want = "passed"
		}
		if !ok || stage.Status != want || stage.Reason == "" {
			return fmt.Errorf("prepared evidence stage %s is %q, want %q with reason", name, stage.Status, want)
		}
	}
	return nil
}

// The public first-add envelope is per-target; repeats may use the established
// single-result shape. Normalize only the assertion view, never recorded output.
func claudePreparedEnvelope(result map[string]any) map[string]any {
	data, _ := result["data"].(map[string]any)
	if _, ok := data["targets"]; ok {
		return result
	}
	if outcome, ok := data["result"].(map[string]any); ok {
		return map[string]any{"data": map[string]any{"targets": []any{map[string]any{"output": map[string]any{"result": outcome}}}}}
	}
	return result
}

func requireClaudePreparedMutation(t *testing.T, label string, result map[string]any, want bool) {
	t.Helper()
	commandsRequireMutated(t, label, claudePreparedEnvelope(result), want)
}

func requireClaudePreparedResult(t *testing.T, result map[string]any) {
	t.Helper()
	out, err := commandsTargetOutcome(claudePreparedEnvelope(result))
	if err != nil {
		t.Fatal(err)
	}
	activation, _ := out["activation"].(map[string]any)
	if activation["activation"] != "prepared" || activation["verification"] != "installation_verified" || activation["activation_attested"] == true {
		t.Fatalf("native suppression manufactured active result: %+v", activation)
	}
	actions, _ := activation["user_actions"].([]any)
	if len(actions) == 0 {
		t.Fatal("prepared result omitted fresh native guidance")
	}
}

// The public persisted binding provides file authority; a suppression sentinel
// deliberately supplies no package id or path. It cannot be used as ownership.
func (f *claudeProjectNativeFixture) preparedPath(t *testing.T, label string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(f.project, ".uap", "state-v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state domain.StateFileV2
	if err := json.Unmarshal(body, &state); err != nil {
		t.Fatal(err)
	}
	var selected *domain.ClientBinding
	for _, installation := range state.Installations {
		if installation.DeclaredName != commandsPluginName {
			continue
		}
		for _, binding := range installation.Clients {
			if binding.ClientID == "claude" && binding.Scope == "project" {
				if selected != nil {
					t.Fatal("ambiguous project binding")
				}
				copied := binding
				selected = &copied
			}
		}
	}
	if selected == nil || selected.Activation != domain.ActivationPrepared || selected.Verification != domain.VerificationInstalled || filepath.Dir(selected.TargetLocator) != filepath.Join(f.project, ".claude", "skills") {
		t.Fatalf("unrecognized prepared binding=%+v", selected)
	}
	nativeRequireContained(t, f.project, selected.TargetLocator)
	owned := 0
	for _, object := range selected.NativeObjects {
		if object.Kind == "managed_package_directory" && object.Path == selected.TargetLocator && object.ManagedDigest != "" {
			owned++
		}
	}
	if owned != 1 {
		t.Fatal("project binding lacks unique owned directory receipt")
	}
	nativeJSON(t, filepath.Join(f.base.Root, label+".json"), map[string]any{"binding_id": selected.ClientBindingID, "scope": selected.Scope, "target_path": selected.TargetLocator, "activation": selected.Activation, "verification": selected.Verification, "evidence_kind": "owned_files_not_native_discovery"})
	return selected.TargetLocator
}

func (f *claudeProjectNativeFixture) requirePreparedSkills(t *testing.T, managed, command string) {
	t.Helper()
	root := filepath.Join(managed, ".agentplugins-runtime", "skills")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() {
			files[entry.Name()] = filepath.Join(root, entry.Name(), "SKILL.md")
		}
	}
	if err := commandsCheckSkills(files, command); err != nil {
		t.Fatalf("prepared filesystem projection: %v", err)
	}
}

func (f *claudeProjectNativeFixture) requireSuppression(t *testing.T, label string, want bool) {
	t.Helper()
	for _, selected := range []struct {
		suffix, cwd string
		want        bool
	}{{"A", f.project, want}, {"B", f.otherProject, false}} {
		result := captureClaudeDiagnostic(f.client, selected.cwd, f.base.env(filepath.Dir(f.client)), []string{"plugin", "list", "--json"})
		prefix := label + "-native-" + selected.suffix
		nativeWrite(t, filepath.Join(f.base.Root, prefix+".stdout.log"), result.Stdout, 0600)
		nativeWrite(t, filepath.Join(f.base.Root, prefix+".stderr.log"), result.Stderr, 0600)
		nativeJSON(t, filepath.Join(f.base.Root, prefix+".json"), map[string]any{"argv": []string{f.client, "plugin", "list", "--json"}, "cwd": selected.cwd, "exit": result.Exit, "error": result.Error, "deadline": result.Deadline, "stdout_limit": result.StdoutLimit, "stderr_limit": result.StderrLimit, "elapsed_ms": result.Elapsed.Milliseconds(), "expected_suppression": selected.want})
		if result.Exit != 0 || result.Error != "" || result.Deadline || result.StdoutLimit || result.StderrLimit {
			t.Fatalf("bounded native listing failed: %+v", result)
		}
		if err := validateClaudeProjectSuppression(result.Stdout, selected.want); err != nil {
			t.Fatal(err)
		}
	}
	f.requireForeign(t)
}

func validateClaudeProjectSuppression(body []byte, want bool) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	value, err := shared.DecodeUniqueJSONValue(decoder)
	if err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing native list output")
	}
	rows, ok := value.([]any)
	if !ok {
		return fmt.Errorf("native list is not an array")
	}
	count := 0
	for _, value := range rows {
		row, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("native list row is not an object")
		}
		if row["id"] == claudeCommandsPluginID {
			return fmt.Errorf("untrusted project unexpectedly loaded exact plugin")
		}
		if row["id"] != "(suppressed)@skills-dir" {
			id, idOK := row["id"].(string)
			scope, scopeOK := row["scope"].(string)
			_, enabledOK := row["enabled"].(bool)
			path, pathOK := row["installPath"].(string)
			if !idOK || id == "" || !scopeOK || scope == "" || !enabledOK || !pathOK || !filepath.IsAbs(path) || filepath.Clean(path) != path {
				return fmt.Errorf("malformed unrelated native row")
			}
			continue
		}
		count++
		notes, ok := row["notes"].([]any)
		if len(row) != 6 || row["version"] != "unknown" || row["scope"] != "project" || row["enabled"] != false || row["installPath"] != "" || !ok || len(notes) != 1 || notes[0] != "1 project-scope directory under ./.claude/skills/ that may load as a plugin was skipped because this workspace was not trusted when plugins were scanned. After accepting the trust dialog, run /reload-plugins (or relaunch) to load what qualifies." {
			return fmt.Errorf("unrecognized native suppression=%+v", row)
		}
	}
	expected := 0
	if want {
		expected = 1
	}
	if count != expected {
		return fmt.Errorf("native suppression count=%d,want=%d", count, expected)
	}
	return nil
}
