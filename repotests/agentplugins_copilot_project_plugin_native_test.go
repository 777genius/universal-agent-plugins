//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// This is a real pinned Copilot process, not just an inspected JSON overlay.
// It reuses the existing isolated executable/source acquisition and process
// supervisor but deliberately removes the bundled stdio requirement so the
// new whole-plugin registration, not legacy loose components, is selected.
func testCopilotNativeWholePluginProject(t *testing.T) {
	t.Helper()
	t.Setenv("AGENTPLUGINS_PROJECT_NATIVE_OUTPUT", t.TempDir())
	f := newCopilotProjectAttempt(t)
	defer f.cancel()
	f.preparePackage(t)

	// The fixture's original bin/probe identity was frozen by preparePackage.
	// Remove only this disposable source dependency after confirming its pin;
	// the standalone probe binary, installer and Copilot remain immutable.
	bundled := filepath.Join(f.base.Package, "bin", "probe")
	if err := os.Remove(filepath.Join(f.base.Package, "mcp.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(bundled); err != nil {
		t.Fatal(err)
	}
	delete(f.immutable, bundled)
	if err := os.Remove(filepath.Dir(bundled)); err != nil {
		t.Fatal(err)
	}
	f.check(t)

	args := []string{f.installer, "add", f.base.Package, "--no-save", "--scope", "project",
		"--project-root", f.base.Project, "--target", "copilot", "--yes", "--format", "json"}
	installed := f.command(t, "whole-plugin-add", f.base.Project, f.env(), args, 45*time.Second)
	var envelope struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(installed.Stdout, &envelope); err != nil || envelope.Result != "success" {
		t.Fatalf("public UAP whole-plugin installation failed: %v", err)
	}
	stateBody, err := projectNativeRuntimeRead(filepath.Join(f.base.Project, ".uap", "state-v2.json"), copilotProjectBodyLimit)
	if err != nil {
		t.Fatal(err)
	}
	var state domain.StateFileV2
	if err := json.Unmarshal(stateBody, &state); err != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 {
		t.Fatalf("UAP native project state differs: %v", err)
	}
	var binding domain.ClientBinding
	for _, value := range state.Installations[0].Clients {
		binding = value
	}
	if binding.ClientID != string(domain.ClientCopilot) || binding.Scope != string(domain.ScopeProject) ||
		binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
		t.Fatalf("whole-plugin binding incorrectly attested as runtime-active: %+v", binding)
	}
	registered := 0
	for _, object := range binding.NativeObjects {
		switch object.Kind {
		case "managed_package_directory":
		case "copilot_project_plugin_registration":
			registered++
		default:
			t.Fatalf("native whole-plugin installation duplicated loose components: %+v", object)
		}
	}
	if registered != 1 {
		t.Fatal("missing exact owned project-native registration receipt")
	}
	marketplace := shared.ManagedMarketplaceName(binding.PhysicalArtifact)
	spec := "native-proof@" + marketplace
	settings := filepath.Join(f.base.Project, ".github", "copilot", "settings.local.json")
	raw, err := projectNativeRuntimeRead(settings, copilotProjectBodyLimit)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Known map[string]struct {
			Source struct {
				Source string `json:"source"`
				Path   string `json:"path"`
			} `json:"source"`
		} `json:"extraKnownMarketplaces"`
		Enabled map[string]bool `json:"enabledPlugins"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	source, known := document.Known[marketplace]
	if !known || source.Source.Source != "directory" || source.Source.Path !=
		filepath.ToSlash(filepath.Join(".uap", "managed", "clients", "copilot", binding.PhysicalArtifact)) ||
		!document.Enabled[spec] {
		t.Fatalf("native project marketplace not registered at the exact relative UAP artifact")
	}
	if _, err := os.Lstat(filepath.Join(f.base.Project, ".github", "skills", "uapcopilota")); !os.IsNotExist(err) {
		t.Fatalf("native plugin duplicated a loose project skill: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(f.base.Project, ".github", "mcp.json")); !os.IsNotExist(err) {
		t.Fatalf("native plugin duplicated loose project MCP: %v", err)
	}

	list := f.command(t, "whole-plugin-list", f.base.Project, f.env(),
		[]string{f.client, "plugin", "list"}, 30*time.Second)
	if !bytes.Contains(list.Stdout, []byte("native-proof")) || !bytes.Contains(list.Stdout, []byte(marketplace)) {
		t.Fatalf("real Copilot did not list native project plugin: %s", list.Stdout)
	}
	skills := f.command(t, "whole-plugin-skills", f.base.Project, f.env(),
		[]string{f.client, "skill", "list"}, 30*time.Second)
	if !bytes.Contains(skills.Stdout, []byte("uapcopilota")) {
		t.Fatalf("real Copilot did not list native plugin skill: %s", skills.Stdout)
	}
	removed := f.command(t, "whole-plugin-remove", f.base.Project, f.env(),
		[]string{f.installer, "remove", "native-proof", "--scope", "project", "--project-root", f.base.Project,
			"--target", "copilot", "--format", "json"}, 45*time.Second)
	if err := json.Unmarshal(removed.Stdout, &envelope); err != nil || envelope.Result != "success" {
		t.Fatalf("public UAP native project remove failed: %v", err)
	}
	after, err := projectNativeRuntimeRead(settings, copilotProjectBodyLimit)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), marketplace) || strings.Contains(string(after), spec) {
		t.Fatal("removed UAP plugin remains in Copilot project settings")
	}
	afterList := f.command(t, "whole-plugin-after-remove", f.base.Project, f.env(),
		[]string{f.client, "plugin", "list"}, 30*time.Second)
	if bytes.Contains(afterList.Stdout, []byte(spec)) {
		t.Fatal("real Copilot still lists a removed UAP project plugin")
	}
	f.check(t)
	t.Logf("Pinned Copilot native whole-plugin E2E: public add, exact project receipt, plugin list, skill list, public remove and foreign authority preservation; plugin=%s", spec)
}
