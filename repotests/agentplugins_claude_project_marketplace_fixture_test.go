package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

func claudeProjectMarketplaceID(project, managed string) string {
	return commandsPluginName + "@" + claude.ProjectMarketplaceName(filepath.Base(managed), project)
}

func claudeProjectMarketplaceCache(f *claudeProjectNativeFixture, managed string) string {
	marketplace := claude.ProjectMarketplaceName(filepath.Base(managed), f.project)
	return filepath.Join(f.base.ConfigDir, "plugins", "cache", marketplace, commandsPluginName, "1.0.0")
}

func requireClaudeProjectActive(t *testing.T, result map[string]any) {
	t.Helper()
	outcome, err := commandsTargetOutcome(claudePreparedEnvelope(result))
	if err != nil {
		t.Fatal(err)
	}
	plan, _ := outcome["plan"].(map[string]any)
	activation, _ := outcome["activation"].(map[string]any)
	if plan["client_id"] != "claude" || plan["scope"] != "project" ||
		activation["activation"] != "active" || activation["verification"] != "installation_verified" || activation["activation_attested"] == true {
		t.Fatalf("project installation lacks native activation evidence: %v", outcome)
	}
}

func claudeProjectCapture(t *testing.T, f *claudeProjectNativeFixture, label, project string, wantExit int, args ...string) []byte {
	t.Helper()
	result := captureClaudeDiagnostic(f.client, project, f.base.env(filepath.Dir(f.client)), args)
	nativeWrite(t, filepath.Join(f.base.Root, label+".stdout.log"), result.Stdout, 0600)
	nativeWrite(t, filepath.Join(f.base.Root, label+".stderr.log"), result.Stderr, 0600)
	nativeJSON(t, filepath.Join(f.base.Root, label+".json"), map[string]any{"argv": args, "cwd": project,
		"exit": result.Exit, "error": result.Error, "deadline": result.Deadline, "stdout_limit": result.StdoutLimit, "stderr_limit": result.StderrLimit})
	if result.Exit != wantExit || result.Deadline || result.StdoutLimit || result.StderrLimit || wantExit == 0 && result.Error != "" {
		t.Fatal("bounded native observation failed; raw evidence retained")
	}
	f.requireForeign(t)
	return result.Stdout
}

// plugin list is a global inventory: B can see A's registration, but it must be
// disabled there and plugin details must refuse it. A's exact enabled entry and
// native details prove discovery without changing project trust or using a model.
func claudeProjectMarketplaceObservation(t *testing.T, f *claudeProjectNativeFixture, label, managed, command string, present bool) {
	t.Helper()
	for _, selected := range []struct {
		suffix, project string
		active          bool
	}{{"A", f.project, present}, {"B", f.otherProject, false}} {
		body := claudeProjectCapture(t, f, label+"-native-"+selected.suffix, selected.project, 0, "plugin", "list", "--json")
		if err := validateClaudeMarketplaceListing(body, f, managed, present, selected.active); err != nil {
			t.Fatal(err)
		}
	}
	if !present {
		return
	}
	id := claudeProjectMarketplaceID(f.project, managed)
	details := claudeProjectCapture(t, f, label+"-native-A-details", f.project, 0, "plugin", "details", id)
	skills, err := claudeDiscoveredSkills(string(details), claudeProjectMarketplaceCache(f, managed))
	if err != nil {
		t.Fatal(err)
	}
	if err := commandsCheckSkills(skills, command); err != nil {
		t.Fatal(err)
	}
	claudeProjectCapture(t, f, label+"-native-B-details", f.otherProject, 1, "plugin", "details", id)
}

func validateClaudeMarketplaceListing(body []byte, f *claudeProjectNativeFixture, managed string, present, active bool) error {
	value, err := shared.DecodeUniqueJSONValue(json.NewDecoder(bytes.NewReader(body)))
	rows, ok := value.([]any)
	if err != nil || !json.Valid(body) || !ok {
		return fmt.Errorf("native plugin list is not a unique whole JSON array")
	}
	count := 0
	for _, item := range rows {
		row, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("native plugin list row is not an object")
		}
		id, _ := row["id"].(string)
		if !strings.HasPrefix(id, commandsPluginName+"@") {
			continue
		}
		count++
		if !present || managed == "" || id != claudeProjectMarketplaceID(f.project, managed) {
			return fmt.Errorf("unexpected or duplicate native project plugin identity")
		}
		if err := validateClaudeMarketplaceRow(row, f, managed, active); err != nil {
			return err
		}
	}
	if present && count != 1 || !present && count != 0 {
		return fmt.Errorf("native project registration absent or duplicated")
	}
	return nil
}

func validateClaudeMarketplaceRow(row map[string]any, f *claudeProjectNativeFixture, managed string, active bool) error {
	if row["scope"] != "project" || row["enabled"] != active || row["projectPath"] != f.project ||
		row["version"] != "1.0.0" || row["installPath"] != claudeProjectMarketplaceCache(f, managed) {
		return fmt.Errorf("wrong project-scoped native registration")
	}
	for _, key := range []string{"errors", "notes"} {
		if value, exists := row[key]; exists {
			items, ok := value.([]any)
			if !ok || len(items) != 0 {
				return fmt.Errorf("native candidate has malformed/nonempty %s", key)
			}
		}
	}
	if value, exists := row["readFromFolder"]; exists && value != filepath.Join(managed, ".agentplugins-plugin") {
		return fmt.Errorf("native source folder differs")
	}
	if value, exists := row["projectEnabled"]; exists && value != active {
		return fmt.Errorf("native project enabled metadata differs")
	}
	return nil
}

func claudeProjectInitializeUntrusted(t *testing.T, f *claudeProjectNativeFixture) {
	t.Helper()
	claudeProjectMarketplaceObservation(t, f, "untrusted-baseline", "", "", false)
	profile := filepath.Join(f.base.ConfigDir, ".claude.json")
	body, err := os.ReadFile(profile)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Projects map[string]struct {
			Trusted bool `json:"hasTrustDialogAccepted"`
		} `json:"projects"`
	}
	if json.Unmarshal(body, &data) != nil {
		t.Fatal("native project profile is not valid JSON")
	}
	for _, entry := range data.Projects {
		if entry.Trusted {
			t.Fatal("credential-free installation fixture unexpectedly trusts a project")
		}
	}
	f.foreign[profile] = nativeSHA(t, profile)
}
