package usecase

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// The existing inert runner discovers the real projected manifest and MCP
// definition. Only the connection status changes; no installer outcome is faked.
type unreadyClaudeRunner struct{ fakeClaudeLifecycleRunner }

func (r *unreadyClaudeRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	result, err := r.fakeClaudeLifecycleRunner.Run(ctx, command)
	if err == nil && len(command.Argv) == 4 && command.Argv[1] == "mcp" {
		result.Stdout = []byte(strings.Replace(string(result.Stdout), "✔ Connected", "✘ Failed to connect", 1))
	}
	result.NoChildStarted = true
	return result, err
}

type countedClaudePreferenceFiles struct{ writes int }

func (*countedClaudePreferenceFiles) ReadNoFollow(path string) ([]byte, os.FileMode, bool, error) {
	snapshot, err := nativeconfig.New().ReadExactFile(path)
	return snapshot.Body, snapshot.Mode, snapshot.Exists, err
}

func (f *countedClaudePreferenceFiles) WriteAtomic(path string, body []byte, mode os.FileMode) error {
	f.writes++
	return atomicfile.Write(path, body, mode)
}

func (*countedClaudePreferenceFiles) RemoveNoFollow(string) error {
	return errors.New("inert Claude preference removal is not allowed")
}

type claudePreparedRepeatFixture struct {
	service Service
	store   statev2.Store
	input   AddInput
	runner  *unreadyClaudeRunner
	files   *countedClaudePreferenceFiles
	added   GroupResult
}

func newClaudePreparedRepeatFixture(t *testing.T) claudePreparedRepeatFixture {
	t.Helper()
	service, store, _ := serviceFixture(t)
	config, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	client := domain.DetectedClient{ClientID: domain.ClientClaude, Status: domain.DetectionDetected,
		ConfigRoot: config, ExecutablePath: filepath.Join(config, "inert-claude")}
	runner := &unreadyClaudeRunner{fakeClaudeLifecycleRunner: fakeClaudeLifecycleRunner{configRoot: config}}
	files := &countedClaudePreferenceFiles{}
	kernel := nativeconfig.NewWithFileIO(files)
	stager := providerstest.NewStager(providers.Stager{})
	service.Stager = stager
	service.Activator = providerstest.NewActivator(providers.Activator{Runner: runner, NativeConfig: &kernel})
	service.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Runner: runner, Stager: stager})
	input := codexSSEInput(t, client, false)
	input.BackendExecutable = client.ExecutablePath
	if err := os.WriteFile(filepath.Join(config, "settings.json"), []byte("{\"foreign\":\"preserve\"}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	added, err := service.AddGroup(t.Context(), GroupInput{Targets: []AddInput{input}, Confirmed: true})
	if err != nil || len(added.Targets) != 1 {
		t.Fatalf("install fixture: %+v %v", added, err)
	}
	outcome := added.Targets[0].Activation
	if !added.Mutated || outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled ||
		outcome.Authentication != domain.AuthenticationNotChecked || outcome.NativeEffect != domain.NativeEffectCommitted || files.writes != 1 {
		t.Fatalf("fixture did not install and approve exact selected MCP: %+v writes=%d", added, files.writes)
	}
	return claudePreparedRepeatFixture{service: service, store: store, input: input, runner: runner, files: files, added: added}
}

func TestClaudeUserPreparedGroupRepeatReportsObservedChanges(t *testing.T) {
	for _, mode := range []string{"unchanged", "approval-missing", "explicit-rejection"} {
		t.Run(mode, func(t *testing.T) {
			f := newClaudePreparedRepeatFixture(t)
			settings := filepath.Join(f.input.Client.ConfigRoot, "settings.json")
			switch mode {
			case "approval-missing":
				if err := os.WriteFile(settings, []byte("{\"foreign\":\"preserve\"}\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "explicit-rejection":
				if err := os.WriteFile(settings, []byte("{\"foreign\":\"preserve\",\"disabledMcpjsonServers\":[\"plugin:demo:events\"]}\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			beforeState, err := os.ReadFile(f.store.Path)
			if err != nil {
				t.Fatal(err)
			}
			beforeNative := codexTransportFiles(t, f.input.Client.ConfigRoot)
			stateInfo, err := os.Stat(f.store.Path)
			if err != nil {
				t.Fatal(err)
			}
			preferenceInfo, err := os.Stat(settings)
			if err != nil {
				t.Fatal(err)
			}
			writes, calls := f.files.writes, len(f.runner.commands)
			f.input.OperationID = "claude-prepared-repeat"
			result, err := f.service.UpdateGroup(t.Context(), GroupInput{Targets: []AddInput{f.input}, Confirmed: true})
			if err != nil || len(result.Targets) != 1 {
				t.Fatalf("unchanged update failed: %+v %v", result, err)
			}
			f.assertRepeatResult(t, result, mode == "approval-missing", writes)
			if len(f.runner.commands) <= calls {
				t.Fatal("repeat reused cached lifecycle without fresh native observations")
			}
			if mode != "approval-missing" {
				afterState, readErr := os.ReadFile(f.store.Path)
				afterInfo, statErr := os.Stat(f.store.Path)
				afterPreference, preferenceErr := os.Stat(settings)
				if readErr != nil || statErr != nil || preferenceErr != nil || string(beforeState) != string(afterState) ||
					!os.SameFile(stateInfo, afterInfo) || !os.SameFile(preferenceInfo, afterPreference) ||
					!reflect.DeepEqual(beforeNative, codexTransportFiles(t, f.input.Client.ConfigRoot)) {
					t.Fatalf("no-effect update rewrote owned state/native files: %v %v %v", readErr, statErr, preferenceErr)
				}
			}
		})
	}
}

func (f claudePreparedRepeatFixture) assertRepeatResult(t *testing.T, result GroupResult, wantMutation bool, writes int) {
	t.Helper()
	target := result.Targets[0]
	wantEffect, wantWrites := domain.NativeEffectUnchanged, writes
	if wantMutation {
		wantEffect, wantWrites = domain.NativeEffectCommitted, writes+1
	}
	if result.Mutated != wantMutation || target.Mutated != wantMutation || target.NoChange == wantMutation ||
		target.Activation.NativeEffect != wantEffect || f.files.writes != wantWrites {
		t.Errorf("mutation lacks matching observed effect: result=%+v config writes=%d->%d", result, writes, f.files.writes)
	}
	if target.Activation.Activation != domain.ActivationPrepared || target.Activation.Verification != domain.VerificationInstalled ||
		target.Activation.Authentication != domain.AuthenticationNotChecked || len(target.Activation.UserActions) == 0 {
		t.Fatal("unchanged reporting manufactured readiness or lost guidance")
	}
	if !strings.Contains(codexTransportFiles(t, f.input.Client.ConfigRoot)["settings.json"], "\"foreign\"") {
		t.Fatal("selected approval lost foreign preference")
	}
}

func TestPreparedUserCLIUnchangedRequiresExactNoEffectProof(t *testing.T) {
	f := newClaudePreparedRepeatFixture(t)
	state, err := f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	client := onlyBinding(state.Installations[0])
	plan := f.added.Targets[0].Plan
	delivery := domain.StagedDelivery{ClientID: f.input.Client.ClientID, ActivePath: client.TargetLocator,
		ArtifactDigest: managedDigest(client), NativeObjects: append([]domain.NativeObjectOwnership(nil), client.NativeObjects...)}
	observed := lifecycleOutcome(client)
	observed.NativeEffect, observed.NativeObjects = domain.NativeEffectUnchanged, append([]domain.NativeObjectOwnership(nil), client.NativeObjects...)
	for _, fault := range []string{"none", "empty-effect", "committed", "uncertain", "attested", "pending-intent", "digest", "path", "foreign-object", "policy", "authentication", "wrong-scope"} {
		t.Run(fault, func(t *testing.T) {
			in, binding, out, projected := f.input, client, observed, delivery
			switch fault {
			case "empty-effect":
				out.NativeEffect = ""
			case "committed":
				out.NativeEffect = domain.NativeEffectCommitted
			case "uncertain":
				out.NativeEffect = domain.NativeEffectUncertain
			case "attested":
				out.ActivationAttested = true
			case "pending-intent":
				binding.NativeActivationAttempt = "unresolved"
			case "digest":
				projected.ArtifactDigest = "sha256:foreign"
			case "path":
				projected.ActivePath = filepath.Join(t.TempDir(), "foreign")
			case "foreign-object":
				out.NativeObjects = nil
			case "policy":
				out.Policy = domain.PolicyBlocked
			case "authentication":
				out.Authentication = domain.AuthenticationComplete
			case "wrong-scope":
				in.Scope = domain.ScopeProject
			}
			if got := f.service.verifiedPreparedObservationUnchanged(in, plan, binding, projected, out, false); got != (fault == "none") {
				t.Fatalf("%s accepted=%v", fault, got)
			}
		})
	}
}
