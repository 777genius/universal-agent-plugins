package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

// This positive discovery fixture authors documented trust in its own original
// disposable profile. It never changes production trust or the untrusted gate.
func TestAgentpluginsClaudeNativeTrustedProjectDiscovery(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CLAUDE_PROJECT_NATIVE_E2E") != "1" {
		t.Skip("opt-in source trusted project discovery")
	}
	nativeRequireDisposable(t)
	f := newClaudeProjectNativeFixture(t)
	trust, err := json.Marshal(map[string]any{"projects": map[string]any{
		f.project:      map[string]any{"hasTrustDialogAccepted": true},
		f.otherProject: map[string]any{"hasTrustDialogAccepted": true},
	}})
	if err != nil {
		t.Fatal(err)
	}
	f.keep(t, filepath.Join(f.base.ConfigDir, ".claude.json"), string(trust))
	nativeJSON(t, filepath.Join(f.base.Root, "trust-fixture.json"), map[string]any{"authored_before_baseline": true, "config_path": filepath.Join(f.base.ConfigDir, ".claude.json"), "configuration": json.RawMessage(trust), "production_trust_mutation": false})
	copyTree(t, commandsFixtureSource(t), f.base.PackageRoot)
	stages := map[string]nativeStage{}
	for _, name := range []string{"install", "native_project_discovery", "remove", "scope_preservation"} {
		stages[name] = nativeStage{Status: "not_evaluated", Reason: "not reached"}
	}
	stages["native_mcp_initialization"] = nativeStage{Status: "not_evaluated", Reason: "skills-only listing/details fixture"}
	stages["model_use"] = nativeStage{Status: "not_evaluated", Reason: "no session/model/account or trust-dialog execution"}
	evidence := claudeProjectNativeIdentity(t, f)
	evidence["project_contract"], evidence["native_surface"] = "trusted_project_discovery_v1", "documented fixture-authored exact project trust; genuine plugin list/details and owned skill bytes; no MCP/model/authentication claim"
	defer f.writeEvidence(t, evidence, stages)
	c := commandsNativeClient{target: "claude", pkg: f.base.PackageRoot, run: f.run}
	nativeAttempt(stages, "install", "trusted-add-installer.json")
	added := c.succeed(t, "trusted-add", "add", c.pkg)
	outcome, err := commandsTargetOutcome(claudePreparedEnvelope(added))
	if err != nil {
		t.Fatal(err)
	}
	plan, _ := outcome["plan"].(map[string]any)
	activation, _ := outcome["activation"].(map[string]any)
	if plan["client_id"] != "claude" || plan["scope"] != "project" || activation["activation"] != "active" || activation["verification"] != "installation_verified" || activation["activation_attested"] == true {
		t.Fatalf("trusted native discovery did not reconcile exact project identity: %v", outcome)
	}
	physical, _ := plan["physical_artifact_id"].(string)
	if physical == "" {
		t.Fatal("exact project artifact missing")
	}
	managed := filepath.Join(f.project, ".claude", "skills", physical)
	nativeRequireContained(t, f.project, managed)
	stages["install"] = nativeStage{Status: "passed", Reason: "public UAP project add reconciled genuine exact trusted native identity without attestation"}
	nativeAttempt(stages, "native_project_discovery", "trusted-installed-native-A.stdout.log")
	state := filepath.Join(f.project, ".uap", "state-v2.json")
	before := nativeSHA(t, state)
	claudeTrustedProjectObservation(t, f, "trusted-installed", managed, true)
	if nativeSHA(t, state) != before {
		t.Fatal("native observation changed installer state")
	}
	stages["native_project_discovery"] = nativeStage{Status: "passed", Reason: "fresh enabled project-scope listing has exact version/path and native details expose exact guide/ordinary command skills and bytes; no A plugin in trusted B"}
	nativeAttempt(stages, "remove", "trusted-remove-installer.json")
	c.succeed(t, "trusted-remove", "remove", commandsPluginName)
	if _, err := os.Lstat(managed); !os.IsNotExist(err) {
		t.Fatalf("owned project directory survived removal: %v", err)
	}
	claudeTrustedProjectObservation(t, f, "trusted-removed", managed, false)
	stages["remove"] = nativeStage{Status: "passed", Reason: "owned project directory absent and fresh trusted A/B native listings expose no removed plugin"}
	f.requireForeign(t)
	stages["scope_preservation"] = nativeStage{Status: "passed", Reason: "original user profile/trust and disabled foreign user/A/B settings, skills and cache bytes remained exact"}
}

func claudeTrustedProjectObservation(t *testing.T, f *claudeProjectNativeFixture, label, managed string, present bool) {
	t.Helper()
	capture := func(suffix, project string, args ...string) []byte {
		t.Helper()
		result := captureClaudeDiagnostic(f.client, project, f.base.env(filepath.Dir(f.client)), args)
		prefix := label + "-native-" + suffix
		nativeWrite(t, filepath.Join(f.base.Root, prefix+".stdout.log"), result.Stdout, 0600)
		nativeWrite(t, filepath.Join(f.base.Root, prefix+".stderr.log"), result.Stderr, 0600)
		nativeJSON(t, filepath.Join(f.base.Root, prefix+".json"), map[string]any{"argv": args, "cwd": project, "exit": result.Exit, "error": result.Error, "deadline": result.Deadline, "stdout_limit": result.StdoutLimit, "stderr_limit": result.StderrLimit})
		if result.Exit != 0 || result.Error != "" || result.Deadline || result.StdoutLimit || result.StderrLimit {
			t.Fatal("bounded trusted native observation failed; raw evidence retained")
		}
		f.requireForeign(t)
		return result.Stdout
	}
	for _, selected := range []struct {
		suffix, project string
		want            bool
	}{{"A", f.project, present}, {"B", f.otherProject, false}} {
		body := capture(selected.suffix, selected.project, "plugin", "list", "--json")
		value, err := shared.DecodeUniqueJSONValue(json.NewDecoder(bytes.NewReader(body)))
		rows, ok := value.([]any)
		if err != nil || !json.Valid(body) || !ok {
			t.Fatal("native trusted plugin list is not a unique whole JSON array")
		}
		count := 0
		for _, item := range rows {
			row, ok := item.(map[string]any)
			if !ok {
				t.Fatal("native plugin list row is not an object")
			}
			if row["id"] == "(suppressed)@skills-dir" {
				t.Fatal("trusted control returned native project suppression")
			}
			if row["id"] != claudeCommandsPluginID {
				continue
			}
			for _, key := range []string{"errors", "notes"} {
				if value, exists := row[key]; exists {
					items, ok := value.([]any)
					if !ok || len(items) != 0 {
						t.Fatal("trusted native candidate has malformed/nonempty errors or notes")
					}
				}
			}
			count++
			if !selected.want || row["scope"] != "project" || row["enabled"] != true || row["version"] != "1.0.0" || row["installPath"] != managed {
				t.Fatalf("wrong trusted project native identity: %+v", row)
			}
			if project, exists := row["projectPath"]; exists && project != f.project {
				t.Fatal("native project attribution differs")
			}
		}
		if count != 0 && !selected.want || count != 1 && selected.want {
			t.Fatal("native trusted project identity absent or duplicated")
		}
	}
	if present {
		details := capture("A-details", f.project, "plugin", "details", claudeCommandsPluginID)
		skills, err := claudeDiscoveredSkills(string(details), managed)
		if err != nil {
			t.Fatal(err)
		}
		if err := commandsCheckSkills(skills, "echo"); err != nil {
			t.Fatal(err)
		}
	}
}
