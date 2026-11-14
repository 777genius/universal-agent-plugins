//go:build linux

package grok

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type enablementNativeFixture struct {
	fixture       enablementFixture
	calls         int
	wrongReadback bool
	afterSecond   func()
}

func (*enablementNativeFixture) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	return legacyports.CommandResult{}, errors.New("unexpected ordinary native call")
}
func (*enablementNativeFixture) DuplexCapability() error { return nil }
func (native *enablementNativeFixture) RunReadonlyDuplexWithPlannedShutdown(ctx context.Context, command legacyports.Command, inputs legacyports.ReadonlyDuplexInputs, exchange func(io.Writer, io.Reader) error) error {
	if err := validateEnablementFixtureInputs(command, inputs); err != nil {
		return err
	}
	native.calls++
	body, err := os.ReadFile(native.fixture.config)
	if err != nil {
		return err
	}
	_, lists, err := membershipDocument(body)
	if err != nil {
		return err
	}
	enabled := membershipContains(lists["enabled"], native.fixture.record.PluginID)
	if native.calls == 2 && native.wrongReadback {
		enabled = !enabled
	}
	record := native.fixture.record
	first := map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"protocolVersion": 1,
		"_meta": map[string]any{"agentVersion": "1.0.70", "currentWorkingDirectory": record.ProjectRoot}}}
	second := map[string]any{"jsonrpc": "2.0", "id": 2, "result": map[string]any{"result": map[string]any{"plugins": []any{
		map[string]any{"id": record.PluginID, "root": record.ArtifactPath, "name": record.PluginName, "scope": "project", "version": "", "enabled": enabled, "trusted": false},
	}}}}
	var frames bytes.Buffer
	encoder := json.NewEncoder(&frames)
	if err := encoder.Encode(first); err != nil {
		return err
	}
	if err := encoder.Encode(second); err != nil {
		return err
	}
	fixture := enablementDuplexFixture{frames: frames.String()}
	err = fixture.RunReadonlyDuplexWithPlannedShutdown(ctx, command, inputs, exchange)
	if native.calls == 2 && native.afterSecond != nil {
		native.afterSecond()
	}
	return err
}

func newLifecycleEnablementFixture(t *testing.T) enablementFixture {
	t.Helper()
	fixture := newEnablementFixture(t)
	prepareInventoryInputFixture(t, fixture.record.ProjectRoot)
	record, err := fixture.recovery.readRecord(fixture.record.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.recovery.removeRecord(record.OperationID, record.rawDigest); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func lifecycleEnablementInput(t *testing.T, fixture enablementFixture) (clients.Env, domain.ActivationRequest, *enablementNativeFixture) {
	t.Helper()
	state := fixture.loaded(t)
	binding := state.Installations[0].Clients[fixture.record.BindingID]
	previous := append([]domain.NativeObjectOwnership(nil), binding.NativeObjects...)
	binding.NativeActivationAttempt = fixture.record.OperationID
	if err := fixture.state.Save(enablementState(state, 0, binding)); err != nil {
		t.Fatal(err)
	}
	native := &enablementNativeFixture{fixture: fixture}
	executable := filepath.Join(filepath.Dir(fixture.record.ProjectRoot), "native")
	env := clients.Env{UserHome: filepath.Dir(fixture.record.UserRoot), NativeTransitions: &fixture.recovery, NativeConfig: fixture.recovery.Kernel, Runner: native}
	request := domain.ActivationRequest{ConfirmPluginEnable: true, NativeAttempt: domain.NativeAttemptIdentity{OperationID: fixture.record.OperationID,
		InstallationID: fixture.record.InstallationID, BindingID: fixture.record.BindingID, NativeRoot: binding.NativeProfileRoot},
		Client: domain.DetectedClient{ClientID: domain.ClientGrok, ConfigRoot: fixture.record.UserRoot, ExecutablePath: executable},
		Plan: domain.DeliveryPlan{Scope: domain.ScopeProject, TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: fixture.record.ProjectRoot, UserConfigRoot: fixture.record.UserRoot},
			NativeRegistryRoot: binding.NativeProfileRoot, PhysicalArtifactID: binding.PhysicalArtifact},
		Delivery:     domain.StagedDelivery{ActivePath: fixture.record.ArtifactPath, ArtifactDigest: fixture.record.PackageDigest},
		DeclaredName: fixture.record.PluginName, BackendExecutable: executable, PreviousNativeObjects: previous}
	return env, request, native
}

func TestEnablementOwnedEffectKeepsPreparedAndOwnsExactMember(t *testing.T) {
	fixture := newLifecycleEnablementFixture(t)
	env, request, native := lifecycleEnablementInput(t, fixture)
	baseline := domain.ActivationOutcome{Activation: domain.ActivationPrepared, NativeEffect: domain.NativeEffectUnchanged}
	outcome, err := enableProject(t.Context(), env, request, baseline)
	if err != nil || outcome.NativeEffect != domain.NativeEffectCommitted || outcome.Activation != domain.ActivationPrepared || native.calls != 2 {
		t.Fatalf("native enablement result: %+v, %v, calls=%d", outcome, err, native.calls)
	}
	body, _ := os.ReadFile(fixture.config)
	if !bytes.Equal(body, fixture.next) {
		t.Fatalf("foreign config changed: %s", body)
	}
	binding := fixture.loaded(t).Installations[0].Clients[fixture.record.BindingID]
	if binding.NativeActivationAttempt != "" || len(binding.NativeObjects) != 2 || binding.NativeObjects[1] != fixture.record.object() {
		t.Fatalf("durable exact ownership: %+v", binding)
	}
	pending, err := fixture.recovery.Pending()
	if err != nil || len(pending) != 0 {
		t.Fatalf("completed record retained: %v %v", pending, err)
	}
	env, request, native = lifecycleEnablementInput(t, fixture)
	outcome, err = enableProject(t.Context(), env, request, baseline)
	if err != nil || outcome.NativeEffect != domain.NativeEffectUnchanged || native.calls != 1 {
		t.Fatalf("repeat: %+v %v", outcome, err)
	}
	repeated, _ := os.ReadFile(fixture.config)
	if !bytes.Equal(repeated, body) {
		t.Fatal("repeated enablement changed native config")
	}
}

func TestEnablementReadbackFailureRecoversExactConfigAndState(t *testing.T) {
	fixture := newLifecycleEnablementFixture(t)
	env, request, native := lifecycleEnablementInput(t, fixture)
	native.wrongReadback = true
	outcome, err := enableProject(t.Context(), env, request, domain.ActivationOutcome{Activation: domain.ActivationPrepared, NativeEffect: domain.NativeEffectUnchanged})
	if err == nil || outcome.NativeEffect != domain.NativeEffectUncertain {
		t.Fatalf("unproved readback accepted: %+v %v", outcome, err)
	}
	pending, err := fixture.recovery.Pending()
	if err != nil || len(pending) != 1 || pending[0].Root == "" {
		t.Fatalf("pending record: %v %v", pending, err)
	}
	record, _ := os.ReadFile(pending[0].Root)
	if strings.Contains(string(record), "PRIVATE_TOKEN") || strings.Contains(string(record), "foreign") {
		t.Fatal("journal persisted native config content")
	}
	if err := recoverPrototypeRecords(t.Context(), fixture.recovery); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(fixture.config)
	if !bytes.Equal(body, fixture.original) || enablementStateDigest(fixture.loaded(t)) != fixture.record.BeforeStateDigest {
		t.Fatal("cold recovery did not restore exact predecessor")
	}
}

func TestEnablementForeignEditsRetainAttemptAndUserConfig(t *testing.T) {
	fixture := newLifecycleEnablementFixture(t)
	env, request, native := lifecycleEnablementInput(t, fixture)
	foreign := append(bytes.Clone(fixture.next), []byte("\n[late]\nforeign='preserved'\n")...)
	native.afterSecond = func() {
		if err := os.WriteFile(fixture.config, foreign, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := enableProject(t.Context(), env, request, domain.ActivationOutcome{NativeEffect: domain.NativeEffectUnchanged}); err == nil {
		t.Fatal("concurrent native version accepted")
	}
	if err := recoverPrototypeRecords(t.Context(), fixture.recovery); err == nil {
		t.Fatal("third native version overwritten")
	}
	body, _ := os.ReadFile(fixture.config)
	if !bytes.Equal(body, foreign) || fixture.loaded(t).Installations[0].Clients[fixture.record.BindingID].NativeActivationAttempt == "" {
		t.Fatal("foreign config or pending authority changed")
	}
}

func TestEnablementExplicitHomeAndDisabledPreferenceRefuseBeforeACP(t *testing.T) {
	for _, mode := range []string{"blank-home", "wrong-home", "disabled-name", "disabled-id"} {
		t.Run(mode, func(t *testing.T) {
			fixture := newLifecycleEnablementFixture(t)
			env, request, native := lifecycleEnablementInput(t, fixture)
			switch mode {
			case "blank-home":
				env.UserHome = ""
			case "wrong-home":
				env.UserHome += "-different"
			default:
				name := fixture.record.PluginName
				if mode == "disabled-id" {
					name = fixture.record.PluginID
				}
				body := []byte("[plugins]\nenabled=[]\ndisabled=['" + name + "']\n")
				if err := os.WriteFile(fixture.config, body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.ReadFile(fixture.config)
			outcome, err := enableProject(t.Context(), env, request, domain.ActivationOutcome{NativeEffect: domain.NativeEffectUnchanged})
			after, _ := os.ReadFile(fixture.config)
			if err == nil || native.calls != 0 || !bytes.Equal(before, after) || outcome.NativeEffect != domain.NativeEffectUnchanged {
				t.Fatalf("refusal changed native state: %+v %v", outcome, err)
			}
		})
	}
}

func TestPrototypeRemovalComponentIsNotPublicAdmission(t *testing.T) {
	fixture := newLifecycleEnablementFixture(t)
	env, request, _ := lifecycleEnablementInput(t, fixture)
	if _, err := enableProject(t.Context(), env, request, domain.ActivationOutcome{NativeEffect: domain.NativeEffectUnchanged}); err != nil {
		t.Fatal("private prototype setup", err)
	}
	env, request, native := lifecycleEnablementInput(t, fixture)
	state := fixture.loaded(t)
	binding := state.Installations[0].Clients[fixture.record.BindingID]
	identity := request.NativeAttempt
	identity.StateDigest = enablementStateDigest(state)
	removal := domain.DeactivationRequest{Client: request.Client, TargetContext: request.Plan.TargetContext,
		NativeRegistryRoot: binding.NativeProfileRoot, PhysicalArtifactID: binding.PhysicalArtifact, ManagedArtifactPath: binding.TargetLocator,
		DeclaredName: request.DeclaredName, NativeObjects: binding.NativeObjects, NativeAttempt: identity,
		BackendExecutable: request.BackendExecutable, Confirmed: true}
	originals := capturePrototypeFiles(t, fixture.config, fixture.state.Path)
	for _, confirmed := range []bool{false, true} {
		removal.Confirmed = confirmed
		outcome, err := New().Deactivate(t.Context(), env, removal)
		if !errors.Is(err, errEnablementUnqualified) || native.calls != 0 || outcome.ExternalRemovalComplete || outcome.NativeEffect != domain.NativeEffectUnchanged {
			t.Fatalf("direct removal admitted unqualified ACP: %+v %v", outcome, err)
		}
		assertPrototypeFilesUnchanged(t, originals)
	}
	// The private component remains testable with inert pipes. This does not
	// bypass either public Adapter method or establish native recovery safety.
	removal.Confirmed = true
	outcome, err := removeEnabledProject(t.Context(), env, removal, domain.DeactivationOutcome{NativeEffect: domain.NativeEffectUnchanged})
	if err != nil || outcome.NativeEffect != domain.NativeEffectCommitted || native.calls != 2 {
		t.Fatalf("private removal component: %+v %v", outcome, err)
	}
	if got, err := os.ReadFile(fixture.config); err != nil || !bytes.Equal(got, fixture.original) {
		t.Fatal("private member removal changed foreign bytes", err)
	}
	if err := fixture.recovery.Recover(t.Context()); !errors.Is(err, errEnablementUnqualified) {
		t.Fatal("prototype removal unexpectedly became public recovery", err)
	}
}
