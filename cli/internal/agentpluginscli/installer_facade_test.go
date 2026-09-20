package agentpluginscli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestInstallerFacadeApplyFailurePresentation(t *testing.T) {
	for _, tc := range []struct {
		name, stage, status string
		result              installer.Result
		err                 error
		doctor              bool
	}{
		{name: "preflight", stage: "preflight", status: "apply_failed", result: installer.Result{Outcome: installer.OutcomeConflict, Reason: "plan_changed"}, err: installer.ErrPlanChanged},
		{name: "state save", stage: "persist", status: "managed_commit_unknown", result: installer.Result{Outcome: installer.OutcomeIncomplete, Mutated: true}, err: errors.New("state save denied"), doctor: true},
		{name: "unknown committed state", stage: "persist", status: "managed_commit_unknown", result: installer.Result{Outcome: installer.OutcomeIncomplete, Mutated: true, Reason: "committed state is unknown", Recovery: installer.RecoveryReport{Unknown: []installer.PendingReceipt{{OperationID: "test"}}}}, err: errors.New("cannot load state"), doctor: true},
		{name: "pending recovery", stage: "persist", status: "managed_commit_unknown", result: installer.Result{Outcome: installer.OutcomeRecovery}, err: installer.ErrRecoveryRequired, doctor: true},
		{name: "activation", stage: "activation", status: "external_partial_failure", result: installer.Result{Outcome: installer.OutcomeIncomplete, Mutated: true, Client: installer.ClientResult{Activation: string(domain.ActivationFailed)}}, err: errors.New("activation denied")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loaded := loadedPackage{origin: domain.OriginModeDirect}
			loaded.envelope.Source.RequestedSource = "/tmp/test-plugin"
			result := facadeResultAddResult(installer.Plan{ClientID: "codex"}, tc.result, loaded.envelope)
			result.Failure = facadeApplyFailure(tc.result, tc.err)
			var stdout bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&stdout)
			if err := renderInstallerFacadeAdd(cmd, &options{format: "json"}, loaded, result, true, tc.err); err != nil {
				t.Fatal(err)
			}
			var document struct {
				Data addMultiResult `json:"data"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
				t.Fatal(err)
			}
			if document.Data.Status != tc.status || document.Data.Failed != 1 || len(document.Data.Targets) != 1 {
				t.Fatalf("unexpected failure envelope: %s", stdout.String())
			}
			target := document.Data.Targets[0]
			wantFailure := usecase.GroupTargetFailure{Stage: tc.stage, Message: tc.err.Error()}
			if tc.result.Reason != "" && tc.result.Reason != wantFailure.Message {
				wantFailure.Message = tc.result.Reason + ": " + wantFailure.Message
			}
			if target.Error == nil || *target.Error != wantFailure || target.Output.Result.Failure == nil || *target.Output.Result.Failure != wantFailure {
				t.Fatalf("lost failure cause/stage: %s", stdout.String())
			}
			if batchFailureNeedsDoctor(target) != tc.doctor {
				t.Fatalf("unexpected doctor decision: %+v", target)
			}
			if tc.doctor && (target.RetryCommand != "" || batchRetryCommandFor(target, "/tmp/test-plugin") != "" || !strings.Contains(strings.Join(batchAttentionLines(target), " "), "doctor before retrying")) {
				t.Fatalf("persistence uncertainty must require doctor without retry: %+v", target)
			}
		})
	}
}

func TestInstallerFacadeAppliedDeliverySupersedesPreview(t *testing.T) {
	preview := installer.Plan{ClientID: "claude", TargetPath: "/preview", Delivery: installer.DeliveryPlan{
		PhysicalArtifactID: "preview-artifact", Components: []installer.ComponentDecision{{Kind: "mcp_server", Name: "local", Support: "projected"}},
	}}
	actual := installer.DeliveryPlan{ActivePath: "/applied", PhysicalArtifactID: "applied-artifact", Components: []installer.ComponentDecision{{Kind: "mcp_server", Name: "local", Support: "unsupported", Reason: "managed_stdio_platform_unsupported"}}}
	for _, mutated := range []bool{false, true} {
		got := facadeResultAddResult(preview, installer.Result{Delivery: &actual, Mutated: mutated}, domain.PackageEnvelope{})
		if got.Plan.PhysicalArtifactID != actual.PhysicalArtifactID || got.Plan.ActivePath != actual.ActivePath || len(got.Plan.Components) != 1 || string(got.Plan.Components[0].Support) != "unsupported" || got.Plan.Components[0].Reason != "managed_stdio_platform_unsupported" {
			t.Fatalf("applied delivery was replaced by preview (mutated=%v): %+v", mutated, got.Plan)
		}
	}
	early := facadeResultAddResult(preview, installer.Result{}, domain.PackageEnvelope{})
	if early.Plan.PhysicalArtifactID != preview.Delivery.PhysicalArtifactID || early.Plan.ActivePath != preview.TargetPath {
		t.Fatalf("failure before planning lost preview: %+v", early.Plan)
	}
}

func TestExplicitLocalCodexAddUsesInstallerFacade(t *testing.T) {
	testExplicitLocalAddUsesInstallerFacade(t, domain.ClientCodex)
}

func TestExplicitLocalClaudeAddUsesInstallerFacade(t *testing.T) {
	testExplicitLocalAddUsesInstallerFacade(t, domain.ClientClaude)
}

func testExplicitLocalAddUsesInstallerFacade(t *testing.T, clientID domain.ClientID) {
	t.Helper()
	client := fixtureClient(t, clientID)
	client.ExecutablePath = filepath.Join(t.TempDir(), string(clientID))
	fixture := newCLIFixture(t, []domain.DetectedClient{client})
	stateRoot := filepath.Join(fixture.root, "data")
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var runner ports.CommandRunner
	if clientID == domain.ClientClaude {
		runner = &cliCommandRunner{run: func(_ int, command legacyports.Command) legacyports.CommandResult {
			if len(command.Argv) == 0 || command.Argv[0] != client.ExecutablePath {
				t.Fatalf("unexpected command: %v", command.Argv)
			}
			state, err := fixture.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			entries := []map[string]any{}
			for _, installation := range state.Installations {
				for _, binding := range installation.Clients {
					entries = append(entries, map[string]any{"id": "demo@skills-dir", "version": "1.0.0", "scope": "user", "enabled": true, "installPath": binding.TargetLocator})
				}
			}
			body, err := json.Marshal(entries)
			if err != nil {
				t.Fatal(err)
			}
			return legacyports.CommandResult{Stdout: body}
		}}
	}
	facade, err := installer.New(installer.Config{
		StateRoot: stateRoot, StateFile: fixture.store.Path,
		LockFile: filepath.Join(stateRoot, "mutation.lock"), OperationsDir: fixture.operations,
		PluginDataBase: filepath.Join(stateRoot, "plugin-data"), ManagedRoot: fixture.app.ManagedRoot,
		TempRoot: filepath.Join(stateRoot, "installer-tmp"), Registry: fixture.app.ClientRegistry,
		HelperExecutable: helper, Runner: runner,
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
	// A regular bin file has no executable bit on Windows (or on this fixture).
	// Re-inferring executables would change the CLI's already-assessed digest.
	if err := os.Mkdir(filepath.Join(plugin, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plugin, "bin", "data"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := fixture.execute(false, "add", plugin, "--target", string(clientID), "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "add")
	var document struct {
		Data addMultiResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &document); err != nil {
		t.Fatal(err)
	}
	if document.Data.Status != "completed" || len(document.Data.Targets) != 1 {
		t.Fatalf("first add lost target completion contract: %s", stdout)
	}
	added := document.Data.Targets[0].Output.Result
	if added.Plan.PhysicalArtifactID == "" || len(added.Plan.Components) != 1 || added.Plan.Components[0].Name != "demo" || added.Plan.Components[0].Kind != domain.ComponentMCPServer || added.RequiresConfirmation || !added.Mutated {
		t.Fatalf("first add lost provider plan or mutation facts: %s", stdout)
	}
	if !strings.Contains(stdout, `"client_id":"`+string(clientID)+`"`) || !strings.Contains(stdout, `"dry_run":false`) {
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
	if installedClient.ClientID != string(clientID) {
		t.Fatalf("facade add client = %+v", installedClient)
	}
	if added.Plan.PhysicalArtifactID != installedClient.PhysicalArtifact {
		t.Fatalf("facade output disagrees with committed artifact: plan=%+v binding=%+v", added.Plan, installedClient)
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
	stdout, _, err = fixture.execute(false, "add", plugin, "--target", string(clientID), "--format", "json")
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

func TestInstallerFacadeKeepsOfflineClientPlanning(t *testing.T) {
	for _, executable := range []string{"", "codex"} {
		t.Run(executable, func(t *testing.T) {
			client := fixtureClient(t, domain.ClientCodex)
			client.ExecutablePath = executable
			fixture := newCLIFixture(t, []domain.DetectedClient{client})
			facade, err := installer.New(installer.Config{StateRoot: filepath.Join(fixture.root, "facade"), Registry: fixture.app.ClientRegistry})
			if err != nil {
				t.Fatal(err)
			}
			fixture.app.Installer = facade
			plugin := writeCLIPlugin(t)
			writeCLIMCP(t, plugin)
			stdout, _, err := fixture.execute(false, "add", plugin, "--target", "codex", "--dry-run", "--format", "json")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stdout, `"status":"planned"`) || !strings.Contains(stdout, `"kind":"mcp_server"`) {
				t.Fatalf("offline plan = %s", stdout)
			}
		})
	}
}
