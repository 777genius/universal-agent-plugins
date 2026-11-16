package installer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/pi"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// A confirmed installation records exact-project Pi trust. Repeating add on
// an installation made before that setup completes it through the confirmed
// repair path; preview writes nothing, a confirmed update re-records the new
// owned content and removal revokes it.
func TestPiProjectTrustFacadeCompletesOldInstallationAndRevokes(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	user := filepath.Join(root, "home", ".pi", "agent")
	project, source := filepath.Join(root, "project"), filepath.Join(root, "source")
	write := func(path, value string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(source, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"test-pi-trust","version":"1.0.0","extensions":{"com.universal-agent-plugins.pi":{"version":1,"source":"native"}}}`)
	write(filepath.Join(source, "native", "index.ts"), "export default function(pi) {}\n")
	if err := os.MkdirAll(user, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	registry, err := clients.NewRegistry(pi.NewProject())
	if err != nil {
		t.Fatal(err)
	}
	engine, err := New(Config{ProjectRoot: project, StateRoot: filepath.Join(project, ".uap"), Registry: registry, HelperExecutable: helper, TrustedLocalPackages: true})
	if err != nil {
		t.Fatal(err)
	}
	installationID := "TEST-pi-project-trust"
	// Prepare is the host's read-only preview; only a confirmed Apply acts.
	apply := func(operation Operation, confirmed bool) Result {
		t.Helper()
		req := Request{Operation: operation, Scope: domain.ScopeProject, ProjectRoot: project, ClientID: "pi", ClientConfigRoot: user, InstallationID: installationID}
		if operation != OpRemove {
			req.PackageRoot = source
		}
		prepared, err := engine.Prepare(context.Background(), req)
		if err != nil {
			t.Fatal(operation, err)
		}
		var result Result
		var applyErr error
		if confirmed {
			result, applyErr = engine.Apply(context.Background(), prepared, Decision{Confirmed: true})
		}
		if closeErr := prepared.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		if applyErr != nil {
			t.Fatal(operation, applyErr)
		}
		return result
	}
	hook := filepath.Join(user, "extensions", "agentplugins-project-trust")
	entry, record := filepath.Join(hook, "index.ts"), filepath.Join(hook, "projects.json")
	recorded := func(stage, artifact string) {
		t.Helper()
		body, err := os.ReadFile(record)
		if err != nil {
			t.Fatal(stage, err)
		}
		var document struct {
			Projects map[string]map[string]struct {
				Extensions map[string]string `json:"extensions"`
			} `json:"projects"`
		}
		if err := json.Unmarshal(body, &document); err != nil {
			t.Fatal(stage, err)
		}
		owner, ok := document.Projects[project][artifact]
		if !ok || len(document.Projects) != 1 || len(owner.Extensions) != 1 || !strings.HasPrefix(owner.Extensions[artifact], "sha256:") {
			t.Fatalf("%s: project trust not recorded for %s: %s", stage, artifact, body)
		}
		if info, err := os.Stat(entry); err != nil || !info.Mode().IsRegular() {
			t.Fatal(stage, "trust hook missing", err)
		}
	}
	absent := func(stage string) {
		t.Helper()
		for _, path := range []string{entry, record} {
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Fatalf("%s: %s exists: %v", stage, path, err)
			}
		}
	}
	ready := func(result Result) bool {
		return slices.ContainsFunc(result.ManualActions, func(action string) bool {
			return strings.Contains(action, "Pi trust hook trusts this exact project")
		})
	}

	installed := apply(OpInstall, true)
	artifact := filepath.Base(installed.Binding.TargetPath)
	if installed.Outcome != OutcomeCompleted || installed.Client.Activation != string(domain.ActivationPrepared) || !ready(installed) {
		t.Fatal("confirmed project install", installed.Outcome, installed.Client.Activation, installed.ManualActions)
	}
	recorded("install", artifact)

	// Simulate an installation saved before UAP set up Pi trust.
	for _, path := range []string{entry, record} {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	apply(OpInstall, false)
	absent("preview")
	if repeated := apply(OpInstall, true); repeated.Outcome != OutcomeCompleted && repeated.Outcome != OutcomeUnchanged || !ready(repeated) {
		t.Fatal("confirmed repeat did not complete trust setup", repeated.Outcome, repeated.ManualActions)
	}
	recorded("repeat", artifact)
	before, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	if again := apply(OpInstall, true); !again.NoChange || !ready(again) {
		t.Fatal("completed trust setup repeated its repair", again.Outcome, again.NoChange, again.ManualActions)
	}
	if after, err := os.ReadFile(record); err != nil || string(after) != string(before) {
		t.Fatal("unchanged repeat rewrote the trust record", err)
	}

	// A confirmed UAP update re-records the changed owned content.
	write(filepath.Join(source, "native", "index.ts"), "export default function(pi) { /* updated */ }\n")
	if updated := apply(OpUpdate, true); updated.Outcome != OutcomeCompleted || !ready(updated) {
		t.Fatal("confirmed update lost owned project trust", updated.Outcome, updated.ManualActions)
	}
	recorded("update", artifact)
	if after, err := os.ReadFile(record); err != nil || string(after) == string(before) {
		t.Fatal("update did not refresh the recorded content digest", err)
	}

	if removed := apply(OpRemove, true); removed.Outcome != OutcomeCompleted {
		t.Fatal("remove", removed.Outcome)
	}
	absent("remove")
}
