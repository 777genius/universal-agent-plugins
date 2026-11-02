package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/pi"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Exercises the real UAP facade and its shared transaction/receipt engine.
// A successful Prepared status does not claim Pi loaded an untrusted project.
func TestPiProjectExtensionFacadeIsolatedAAndB(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	user := filepath.Join(root, "home", ".pi", "agent")
	projects := []string{filepath.Join(root, "A"), filepath.Join(root, "B")}
	source := filepath.Join(root, "source")
	write := func(path, value string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(source, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"test-pi-project","version":"1.0.0","extensions":{"com.universal-agent-plugins.pi":{"version":1,"source":"native"}}}`)
	write(filepath.Join(source, "native", "index.ts"), "export default function(pi) {}\n")
	write(filepath.Join(user, "extensions", "foreign.ts"), "// user foreign\n")
	for _, project := range projects {
		write(filepath.Join(project, ".pi", "extensions", "foreign.ts"), "// project foreign\n")
	}
	beforeUser, err := os.ReadFile(filepath.Join(user, "extensions", "foreign.ts"))
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	engines := make([]*Engine, 0, 2)
	installed := make([]Result, 0, 2)
	for _, project := range projects {
		registry, err := clients.NewRegistry(pi.NewProject())
		if err != nil {
			t.Fatal(err)
		}
		engine, err := New(Config{ProjectRoot: project, StateRoot: filepath.Join(project, ".uap"), Registry: registry, HelperExecutable: helper, TrustedLocalPackages: true})
		if err != nil {
			t.Fatal(err)
		}
		engines = append(engines, engine)
		req := Request{Operation: OpInstall, Scope: domain.ScopeProject, ProjectRoot: project,
			PackageRoot: source, ClientID: "pi", ClientConfigRoot: user, InstallationID: "TEST-pi-project"}
		prepared, err := engine.Prepare(context.Background(), req)
		if err != nil {
			t.Fatal("project prepare", err)
		}
		result, applyErr := engine.Apply(context.Background(), prepared, Decision{Confirmed: true})
		if closeErr := prepared.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		if applyErr != nil || result.Outcome != OutcomeCompleted {
			t.Fatal("project apply", result, applyErr)
		}
		if result.Binding.Scope != string(domain.ScopeProject) || result.Binding.TargetPath == "" ||
			filepath.Dir(result.Binding.TargetPath) != filepath.Join(project, ".pi", "extensions") {
			t.Fatal("project binding escaped selected native discovery path")
		}
		if _, err := os.Stat(filepath.Join(result.Binding.TargetPath, "index.ts")); err != nil {
			t.Fatal("native Pi entrypoint not installed", err)
		}
		installed = append(installed, result)
	}
	targetA, targetB := installed[0].Binding.TargetPath, installed[1].Binding.TargetPath
	aBefore, err := os.ReadFile(filepath.Join(targetB, "native", "index.ts"))
	if err != nil {
		t.Fatal(err)
	}
	// A-only update must not affect B, the user's original Pi profile, or the
	// foreign file under either project's project-trust directory.
	write(filepath.Join(source, "native", "index.ts"), "export default function(pi) { /* A-updated */ }\n")
	change := Request{Operation: OpUpdate, Scope: domain.ScopeProject, ProjectRoot: projects[0],
		PackageRoot: source, ClientID: "pi", ClientConfigRoot: user, InstallationID: installed[0].InstallationID}
	prepared, err := engines[0].Prepare(context.Background(), change)
	if err != nil {
		t.Fatal("A update prepare", err)
	}
	updated, applyErr := engines[0].Apply(context.Background(), prepared, Decision{Confirmed: true})
	if closeErr := prepared.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if applyErr != nil || updated.Outcome != OutcomeCompleted {
		t.Fatal("A update", updated, applyErr)
	}
	revised, err := os.ReadFile(filepath.Join(targetA, "native", "index.ts"))
	if err != nil || !strings.Contains(string(revised), "A-updated") {
		t.Fatal("A update missing", err)
	}
	afterB, err := os.ReadFile(filepath.Join(targetB, "native", "index.ts"))
	if err != nil || string(afterB) != string(aBefore) {
		t.Fatal("B changed during A update", err)
	}
	if err := os.RemoveAll(targetA); err != nil {
		t.Fatal(err)
	}
	change.Operation = OpRepair
	prepared, err = engines[0].Prepare(context.Background(), change)
	if err != nil {
		t.Fatal("A repair prepare", err)
	}
	repaired, applyErr := engines[0].Apply(context.Background(), prepared, Decision{Confirmed: true})
	if closeErr := prepared.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if applyErr != nil || repaired.Outcome != OutcomeCompleted {
		t.Fatal("A repair", repaired, applyErr)
	}
	if _, err := os.Lstat(targetA); err != nil {
		t.Fatal("missing A repair", err)
	}
	remove := Request{Operation: OpRemove, Scope: domain.ScopeProject, ProjectRoot: projects[0],
		ClientID: "pi", ClientConfigRoot: user, InstallationID: installed[0].InstallationID}
	prepared, err = engines[0].Prepare(context.Background(), remove)
	if err != nil {
		t.Fatal("A remove prepare", err)
	}
	result, applyErr := engines[0].Apply(context.Background(), prepared, Decision{Confirmed: true})
	if closeErr := prepared.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if applyErr != nil || result.Outcome != OutcomeCompleted {
		t.Fatal("A remove", result, applyErr)
	}
	if _, err := os.Lstat(targetA); !os.IsNotExist(err) {
		t.Fatal("A survives removal", err)
	}
	if body, err := os.ReadFile(filepath.Join(user, "extensions", "foreign.ts")); err != nil || string(body) != string(beforeUser) {
		t.Fatal("user Pi profile changed", err)
	}
	if _, err := os.Lstat(targetB); err != nil {
		t.Fatal("B removed with A", err)
	}
	for _, project := range projects {
		if body, err := os.ReadFile(filepath.Join(project, ".pi", "extensions", "foreign.ts")); err != nil || string(body) != "// project foreign\n" {
			t.Fatal("foreign project extension changed", err)
		}
	}
}
