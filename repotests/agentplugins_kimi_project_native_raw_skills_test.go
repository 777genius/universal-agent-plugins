//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const kimiRawPackage = "skill-b0a5b0253f26020d7acb797250495459c1bd758fe440530b3db1347e"
const kimiRawSourceKey = "github-skill:anthropics/skills:frontend-design:skills/frontend-design"
const kimiRawRevision = "ca45fc496a3436251f191354091d557da31c15b5"
const kimiRawSource = "https://github.com/anthropics/skills/tree/" + kimiRawRevision + "/skills/frontend-design"

// Reuse the original closed command/capture owner and existing native collector.
// The mixed plugin lifecycle and its conservative MCP oracle run afterwards.
func (f *kimiProjectNativeFixture) rawSkillsCycle(t *testing.T) {
	t.Helper()
	public := map[string]string{}
	defer func() {
		body, err := json.Marshal(public)
		if err != nil {
			t.Error(err)
			return
		}
		f.keep(t, "raw-skills-public.json", body)
	}()
	f.rawNames = []string{"frontend-design"}
	f.observe(t, "raw-empty", false)
	args := []string{f.installer, "skills", "add", kimiRawSource, "--skill", "frontend-design", "--target", "kimi", "--scope", "project", "--project-root", f.projects["a"], "--no-ui", "--yes", "--format", "json"}
	var first domain.Installation
	var binding domain.ClientBinding
	var continuity map[string]string
	for attempt, stage := range []string{"raw-add", "raw-repeat"} {
		result := f.rawSkillsCommand(t, public, stage, args)
		if err := kimiRawPublicReady(result.Stdout); err != nil {
			t.Fatal(err)
		}
		installation, selected := f.rawSkillsReceipt(t)
		native := filepath.Join(f.projects["a"], ".kimi-code", "skills", "frontend-design", "SKILL.md")
		body, err := projectNativeRuntimeRead(native, kimiProjectNativeLimit)
		if err != nil {
			t.Fatal(err)
		}
		skill, err := conformance.ParseSkill(f.ctx, "frontend-design", body)
		if err != nil {
			t.Fatal(err)
		}
		f.rawSkill = map[string]any{"name": skill.Name, "description": skill.Description, "path": native, "source": "project", "sha256": nativeSHA(t, native)}
		current := map[string]string{}
		for _, path := range []string{filepath.Join(f.projects["a"], ".uap", "state-v2.json"), filepath.Join(f.projects["a"], "uap.toml"), filepath.Join(f.projects["a"], "uap.lock"), native} {
			current[path] = nativeSHA(t, path)
		}
		for path, digest := range kimiProjectTree(t, selected.TargetLocator) {
			current[path] = digest
		}
		if attempt == 0 {
			first, binding, continuity = installation, selected, current
		} else if first.InstallationID != installation.InstallationID || !reflect.DeepEqual(first.Source, installation.Source) || !reflect.DeepEqual(binding, selected) || !reflect.DeepEqual(continuity, current) {
			t.Fatal("Kimi raw repeat changed installation/binding, source, state, lock or payload")
		}
		f.observe(t, stage, false)
	}
	removed := f.rawSkillsCommand(t, public, "raw-remove", []string{f.installer, "remove", "frontend-design", "--target", "kimi", "--scope", "project", "--project-root", f.projects["a"], "--yes", "--format", "json"})
	removeResult, err := kimiProjectObject(removed.Stdout)
	if err != nil || removeResult["schema_version"] != json.Number("1") || removeResult["command"] != "agentplugins remove" || removeResult["result"] != "success" || removeResult["outcome"] != "completed" || removeResult["scope"] != "project" {
		t.Fatal("Kimi raw saved removal public result differs", err)
	}
	results, ok := removeResult["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatal("Kimi raw saved removal result is ambiguous")
	}
	result, ok := results[0].(map[string]any)
	if !ok || result["installation_id"] != first.InstallationID || result["client_id"] != "kimi" || result["operation"] != "remove" || result["outcome"] != "completed" {
		t.Fatal("Kimi raw saved removal selected identity differs")
	}
	for _, path := range []string{binding.TargetLocator, filepath.Dir(f.rawSkill["path"].(string))} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("Kimi removed raw Skill survives", path, err)
		}
	}
	state := f.rawState(t)
	for _, installation := range state.Installations {
		if installation.InstallationID == first.InstallationID && len(installation.Clients) != 0 {
			t.Fatal("Kimi removed raw Skill retains active clients")
		}
	}
	f.rawSkill = nil
	f.observe(t, "raw-removed", false)
	f.rawNames = nil
	f.evidence["raw_skills"] = map[string]any{"source": first.Source, "installation_id": first.InstallationID, "binding_id": binding.ClientBindingID, "public_add_repeat_ready": true, "same_public_argv": true, "native_a_b_c_user_inventory": true, "removed": true, "native_shutdown_evidence": "existing independent collector commands; production Ready also requires natural finite-process completion", "model_qualified": false, "mcp_activation_qualified": false}
}

// Fold only new raw public command data into one bounded member. Keep the
// original fixture's 120-capture bound and the outer reader's 80-member bound.
// A deferred transfer also retains the exact failed command before Goexit.
func (f *kimiProjectNativeFixture) rawSkillsCommand(t *testing.T, public map[string]string, label string, argv []string) ports.CommandResult {
	t.Helper()
	defer func() {
		for _, suffix := range []string{".stdout.log", ".stderr.log", ".command.json"} {
			name := label + suffix
			if body, exists := f.files[name]; exists {
				public[name] = string(body)
				delete(f.files, name)
			}
		}
	}()
	return f.command(t, label, f.projects["a"], f.env(), argv, 30*time.Second, false)
}

func (f *kimiProjectNativeFixture) rawState(t *testing.T) domain.StateFileV2 {
	t.Helper()
	body, err := projectNativeRuntimeRead(filepath.Join(f.projects["a"], ".uap", "state-v2.json"), kimiProjectNativeLimit)
	if err != nil {
		t.Fatal(err)
	}
	state, err := kimiProjectProofState(body)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func (f *kimiProjectNativeFixture) rawSkillsReceipt(t *testing.T) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	var selected []domain.Installation
	for _, installation := range f.rawState(t).Installations {
		if installation.DeclaredName == kimiRawPackage {
			selected = append(selected, installation)
		}
	}
	if len(selected) != 1 {
		t.Fatal("Kimi raw Skill installation is ambiguous")
	}
	installation := selected[0]
	if installation.Source.StableSourceKey != kimiRawSourceKey || installation.Source.Repository != "anthropics/skills" || installation.Source.ResolvedRevision != kimiRawRevision || installation.Source.PackageSubpath != "skills/frontend-design" || installation.Source.SourceBindingID == "" || installation.Source.TreeDigest == "" || len(installation.Clients) != 1 || !reflect.DeepEqual(installation.Package.Inventory.Skills, []string{"frontend-design"}) || len(installation.Package.Inventory.MCPServers) != 0 {
		t.Fatal("Kimi raw source pin or pure-Skill package differs")
	}
	var binding domain.ClientBinding
	for _, row := range installation.Clients {
		binding = row
	}
	if binding.ClientID != "kimi" || binding.Scope != "project" || binding.NativeProfileRoot != filepath.Join(f.projects["a"], ".kimi-code") || binding.Activation != domain.ActivationActive || binding.Authentication != domain.AuthenticationNotRequired || binding.Policy != domain.PolicyAllowed || binding.Verification != domain.VerificationInstalled || binding.ClientBindingID != domain.ComputeClientBindingID(installation.InstallationID, "kimi", "project", binding.TargetLocator) || binding.PhysicalArtifact != domain.ComputePhysicalArtifactID(kimiRawPackage, installation.InstallationID) || binding.TargetLocator != filepath.Join(f.projects["a"], ".uap", "managed", "clients", "kimi", binding.PhysicalArtifact) || binding.PackageRevision == nil {
		t.Fatal("Kimi raw Skill binding is not genuinely Ready", binding)
	}
	native := filepath.Join(f.projects["a"], ".kimi-code", "skills", "frontend-design")
	owned := 0
	for _, object := range binding.NativeObjects {
		if object.Path == native && object.LogicalName == "frontend-design" && object.ManagedDigest != "" && object.ProtectionClass == "managed" {
			owned++
		}
	}
	managedBody, err := projectNativeRuntimeRead(filepath.Join(binding.TargetLocator, "skills", "frontend-design", "SKILL.md"), kimiProjectNativeLimit)
	if err != nil {
		t.Fatal(err)
	}
	nativeBody, err := projectNativeRuntimeRead(filepath.Join(native, "SKILL.md"), kimiProjectNativeLimit)
	if err != nil || owned != 1 || !bytes.Equal(managedBody, nativeBody) {
		t.Fatal("Kimi raw selected native/managed bytes or ownership differ", err)
	}
	return installation, binding
}

func kimiRawPublicReady(body []byte) error {
	value, err := kimiProjectObject(body)
	if err != nil {
		return err
	}
	items, ok := value["items"].([]any)
	if value["schema_version"] != json.Number("1") || value["command"] != "skills add" || value["result"] != "success" || value["status"] != "succeeded" || value["scope"] != "project" || value["readiness"] != "ready" || !ok || len(items) != 1 {
		return fmt.Errorf("Kimi raw public Skills result is not Ready")
	}
	if actions, exists := value["next_actions"]; exists {
		rows, ok := actions.([]any)
		if !ok || len(rows) != 0 {
			return fmt.Errorf("Kimi raw public result retains pending actions")
		}
	}
	item, ok := items[0].(map[string]any)
	if !ok || item["targetId"] != "kimi" || item["status"] != "succeeded" {
		return fmt.Errorf("Kimi raw public Skill item differs")
	}
	return nil
}
