package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/copilot"
)

func TestProjectCopilotEngineRestoresTrustWithoutReplacingPackage(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &projectFilesFixture{base: base, project: filepath.Join(base, "A"), user: filepath.Join(base, "selected-copilot"), pkg: filepath.Join(base, "package"), runner: &projectFilesNoRunner{}}
	deathWrite(t, filepath.Join(f.project, "keep.txt"), "project control")
	deathWrite(t, filepath.Join(f.user, "config.json"), `{"trustedFolders":[],"foreign":"retained"}`)
	deathWrite(t, filepath.Join(f.pkg, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"copilot-trust","version":"1.0.0"}`)
	deathWrite(t, filepath.Join(f.pkg, "skills", "docs", "SKILL.md"), deathSkill("docs", "original"))
	registry, err := clients.NewRegistry(copilot.New())
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	f.cfg = Config{ProjectRoot: f.project, StateRoot: filepath.Join(f.project, ".uap"), Registry: registry, Runner: f.runner, TrustedLocalPackages: true, HelperExecutable: helper}
	f.req = Request{Scope: "project", ProjectRoot: f.project, Operation: OpInstall, PackageRoot: f.pkg, ClientID: "copilot", ClientConfigRoot: f.user, InstallationID: "00000000-0000-4000-8000-000000000375", RequiredComponents: []string{"skills"}}
	f.reopen(t)
	f.apply(t, OpInstall)
	before := f.binding(t)
	trustPath := filepath.Join(f.user, "config.json")
	assertCopilotEngineTrust(t, trustPath, f.project)
	deathWrite(t, trustPath, `{"trustedFolders":[],"foreign":"retained"}`)
	f.reopen(t)
	out := f.apply(t, OpInstall)
	assertCopilotEngineTrust(t, trustPath, f.project)
	after := f.binding(t)
	if !out.Mutated || out.Outcome == OutcomeUnchanged || before.TargetLocator != after.TargetLocator || !reflect.DeepEqual(before.Receipts, after.Receipts) || !reflect.DeepEqual(before.NativeObjects, after.NativeObjects) {
		t.Fatal("remembered-trust restoration did not reuse the existing package and exact receipts")
	}
	if f.runner.calls != 0 {
		t.Fatal("file-only trust restoration invoked a client or MCP process")
	}
}

func assertCopilotEngineTrust(t *testing.T, path, project string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var actual map[string]any
	if err := json.Unmarshal(body, &actual); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, map[string]any{"trustedFolders": []any{project}, "foreign": "retained"}) {
		t.Fatal("engine failed to remember only the selected project or changed foreign settings")
	}
}
