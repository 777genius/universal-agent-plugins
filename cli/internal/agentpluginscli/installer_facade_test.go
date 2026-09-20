package agentpluginscli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestExplicitLocalCodexAddUsesInstallerFacade(t *testing.T) {
	client := fixtureClient(t, domain.ClientCodex)
	client.ExecutablePath = filepath.Join(t.TempDir(), "codex")
	fixture := newCLIFixture(t, []domain.DetectedClient{client})
	stateRoot := filepath.Join(fixture.root, "data")
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	facade, err := installer.New(installer.Config{
		StateRoot: stateRoot, StateFile: fixture.store.Path,
		LockFile: filepath.Join(stateRoot, "mutation.lock"), OperationsDir: fixture.operations,
		PluginDataBase: filepath.Join(stateRoot, "plugin-data"), ManagedRoot: fixture.app.ManagedRoot,
		TempRoot: filepath.Join(stateRoot, "installer-tmp"), Registry: fixture.app.ClientRegistry,
		HelperExecutable: helper,
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.app.Installer = facade
	// A raw lifecycle call would fail. Success therefore proves that the
	// qualified explicit local path crossed the public facade boundary.
	fixture.app.Lifecycle.Planner = nil
	fixture.app.Lifecycle.Targets = nil
	plugin := writeCLIPlugin(t)
	writeCLIMCP(t, plugin)

	stdout, _, err := fixture.execute(false, "add", plugin, "--target", "codex", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "add")
	if !strings.Contains(stdout, `"client_id":"codex"`) || !strings.Contains(stdout, `"dry_run":false`) {
		t.Fatalf("facade add output = %s", stdout)
	}
	state, err := fixture.store.Load()
	if err != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 {
		t.Fatalf("facade add state = %+v, err = %v", state, err)
	}
	var installedClient domain.ClientBinding
	for _, binding := range state.Installations[0].Clients {
		installedClient = binding
	}
	if installedClient.ClientID != "codex" {
		t.Fatalf("facade add client = %+v", installedClient)
	}
	// The first non-interactive Codex add truthfully stops at manual activation.
	// Model the user completing that out-of-process step before asserting the
	// facade's repeat behavior; an incomplete lifecycle is a resume, not a no-op.
	for key, binding := range state.Installations[0].Clients {
		binding.Activation = domain.ActivationActive
		binding.Authentication = domain.AuthenticationNotRequired
		binding.Verification = domain.VerificationInstalled
		state.Installations[0].Clients[key] = binding
	}
	if err := fixture.store.Save(state); err != nil {
		t.Fatal(err)
	}
	stdout, _, err = fixture.execute(false, "add", plugin, "--target", "codex", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"no_change":true`) {
		t.Fatalf("facade repeat output = %s", stdout)
	}
	repeated, err := fixture.store.Load()
	if err != nil || len(repeated.Installations) != 1 || len(repeated.Installations[0].Clients) != 1 {
		t.Fatalf("facade repeat state = %+v, err = %v", repeated, err)
	}
}
