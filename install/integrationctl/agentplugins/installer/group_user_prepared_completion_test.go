package installer

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodeprofile"
)

type userPreparedUpdateFixture struct {
	prepared *PreparedOperation
	got      usecase.GroupResult
	state    domain.StateFileV2
	callback bool
}

func newUserPreparedUpdateFixture(t *testing.T) userPreparedUpdateFixture {
	t.Helper()
	root := t.TempDir()
	id := "user-group-prepared-proof"
	prepared := &PreparedOperation{
		req: Request{Operation: OpUpdate, Scope: domain.ScopeUser, InstallationID: id},
		plan: Plan{InstallationID: id},
	}
	got := usecase.GroupResult{InstallationID: id, Phase: usecase.GroupPhaseCompleted, Mutated: true}
	installed := domain.Installation{InstallationID: id, Clients: map[string]domain.ClientBinding{},
		DataReceipts: map[string]domain.DataReceipt{}}
	for _, clientID := range []domain.ClientID{domain.ClientCodex, domain.ClientVSCode} {
		config := filepath.Join(root, string(clientID), "config")
		active := filepath.Join(root, "managed", string(clientID))
		client := domain.DetectedClient{ClientID: clientID, ConfigRoot: config}
		plan := domain.DeliveryPlan{
			ClientID: clientID, Scope: domain.ScopeUser,
			TargetContext: domain.TargetContext{Scope: domain.ScopeUser, UserConfigRoot: config},
			ActivePath: active, PhysicalArtifactID: "artifact-" + string(clientID),
			NativeRegistryRoot: config,
		}
		var observation *domain.LocalEntryObservation
		var objects []domain.NativeObjectOwnership
		if clientID == domain.ClientVSCode {
			enabled := true
			digest := "sha256:" + strings.Repeat("a", 64)
			projection := "sha256:" + strings.Repeat("b", 64)
			facts := domain.LocalDeliveryFacts{
				ProfileRoot: config, SettingsPath: filepath.Join(config, "settings.json"),
				ProfileIdentity: config, SettingsIdentity: filepath.Join(config, "settings.json"),
				Tuple: domain.LocalQualifiedTuple{VSCodeVersion: "TEST-vscode", CopilotVersion: "TEST-copilot",
					TargetOS: "linux", TargetShell: "bash", QualificationID: "TEST-local"},
				NativeStop: true, CanonicalDigest: digest, ProjectionDigest: projection,
				Registration: domain.OwnedProfileEntry{ObjectID: "TEST-owned-entry", Selector: active, DesiredValue: &enabled},
			}
			var err error
			plan.SelectedDelivery, err = domain.NewLocalDelivery(facts)
			if err != nil {
				t.Fatal(err)
			}
			native, err := vscodeprofile.Plan(vscodeprofile.Request{
				Settings: []byte("{}"), Action: vscodeprofile.Install,
				Identity: vscodeprofile.Identity{
					SettingsPath: facts.SettingsPath, ProfileID: facts.ProfileIdentity,
					PluginRoot: active, PackageID: facts.Registration.ObjectID,
					PackageDigest: digest, ProjectionDigest: projection,
				},
			})
			if err != nil || native.Receipt == nil {
				t.Fatalf("TEST native receipt setup: %v", err)
			}
			observation, err = domain.NewLocalEntryObservation(domain.LocalEntryObservationFacts{
				RevisionBasis: plan.SelectedDelivery,
				Enabled: native.Receipt.Enabled, ReceiptDigest: native.Receipt.Digest,
			})
			if err != nil {
				t.Fatal(err)
			}
			objects = []domain.NativeObjectOwnership{facts.Registration.Ownership(facts.SettingsPath)}
		}
		bindingID := domain.ComputeClientBindingID(id, string(clientID), string(domain.ScopeUser), active)
		receiptID := "receipt-" + string(clientID)
		installed.DataReceipts[receiptID] = domain.DataReceipt{DataReceiptID: receiptID, Scope: string(domain.ScopeUser),
			Locator: filepath.Join(root, "plugin-data", string(clientID)), State: domain.DataReceiptOwned}
		prepared.req.Targets = append(prepared.req.Targets, ClientTarget{ClientID: string(clientID), ClientConfigRoot: config})
		prepared.clients = append(prepared.clients, client)
		prepared.plan.Targets = append(prepared.plan.Targets, PlanTarget{
			ClientID: string(clientID), ConfigRoot: config, TargetPath: active, BindingID: bindingID,
			SelectedDelivery: plan.SelectedDelivery,
		})
		activation := domain.ActivationActive
		verification := domain.VerificationInstalled
		actions := []string(nil)
		if clientID == domain.ClientVSCode {
			activation = domain.ActivationPrepared
			verification = domain.VerificationPackageValid
			actions = []string{"TEST Local reload; activation not attested"}
		}
		outcome := domain.ActivationOutcome{
			Activation: activation, Authentication: domain.AuthenticationNotChecked,
			Policy: domain.PolicyAllowed, Verification: verification,
			NativeEffect: domain.NativeEffectUnchanged, UserActions: actions,
		}
		got.Targets = append(got.Targets, usecase.AddResult{
			InstallationID: id, Plan: plan, Activation: outcome,
			GroupPhase: usecase.GroupTargetExternalCompleted,
		})
		installed.Clients[bindingID] = domain.ClientBinding{
			ClientBindingID: bindingID, DataReceiptID: receiptID, ClientID: string(clientID), Scope: string(domain.ScopeUser),
			TargetLocator: active, PhysicalArtifact: plan.PhysicalArtifactID, NativeProfileRoot: config,
			SelectedDelivery: plan.SelectedDelivery, NativeObjects: objects, LocalEntryObservation: observation,
			Materialization: domain.MaterializationMaterialized, Activation: outcome.Activation,
			Authentication: outcome.Authentication, Policy: outcome.Policy, Verification: outcome.Verification,
		}
	}
	return userPreparedUpdateFixture{
		prepared: prepared, got: got,
		state: domain.StateFileV2{Installations: []domain.Installation{installed}},
		callback: true,
	}
}

func (f *userPreparedUpdateFixture) changeBinding(clientID domain.ClientID, change func(*domain.ClientBinding)) {
	key := f.prepared.plan.Targets[0].BindingID
	if clientID == domain.ClientVSCode {
		key = f.prepared.plan.Targets[1].BindingID
	}
	binding := f.state.Installations[0].Clients[key]
	change(&binding)
	f.state.Installations[0].Clients[key] = binding
}

func (f userPreparedUpdateFixture) finish(err error, storeErr error) (Result, error) {
	var callback func(context.Context, BindingFacts) error
	if f.callback {
		callback = func(context.Context, BindingFacts) error { return nil }
	}
	engine := Engine{cfg: Config{OnCommittedBinding: callback},
		store: groupReportingStore{state: f.state, err: storeErr}}
	return engine.finishGroup(f.prepared, Result{}, true, f.got, err)
}

func TestUserGroupPreparedUpdateCompletesOnlyFiles(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "unchanged", true: "revised files"}[changed], func(t *testing.T) {
			f := newUserPreparedUpdateFixture(t)
			f.got.Mutated = changed
			f.prepared.plan.NoChange = true // A stale preview cannot override an observed write.
			result, err := f.finish(nil, nil)
			expected := OutcomeUnchanged
			if changed {
				expected = OutcomeCompleted
			}
			if err != nil || result.Outcome != expected || result.Mutated != changed ||
				result.NoChange == changed || len(result.Targets) != 2 {
				t.Fatalf("finished file operation %+v: %v", result, err)
			}
			if result.Targets[1].ClientID != "vscode" ||
				result.Targets[1].Activation != "prepared" ||
				result.Targets[1].Verification != "package_validated" {
				t.Fatalf("prepared Local was promoted or lost: %+v", result.Targets)
			}
			if len(result.ManualActions) != 1 || result.ManualActions[0] != "TEST Local reload; activation not attested" {
				t.Fatalf("prepared Local reload/trust actions lost: %v", result.ManualActions)
			}
			if result.Targets[0].ClientID != "codex" || result.Targets[0].Activation != "active" {
				t.Fatalf("Codex lifecycle was changed: %+v", result.Targets)
			}
		})
	}
}

func TestUserGroupPreparedUpdateRetainsFailClosedHandoff(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*userPreparedUpdateFixture)
	}{
		{"without host callback", func(f *userPreparedUpdateFixture) { f.callback = false }},
		{"install is not update", func(f *userPreparedUpdateFixture) { f.prepared.req.Operation = OpInstall }},
		{"project scope", func(f *userPreparedUpdateFixture) { f.prepared.req.Scope = domain.ScopeProject }},
		{"group not completed", func(f *userPreparedUpdateFixture) { f.got.Phase = usecase.GroupPhaseManagedCommitted }},
		{"wrong installation", func(f *userPreparedUpdateFixture) { f.got.InstallationID = "another" }},
		{"target incomplete", func(f *userPreparedUpdateFixture) { f.got.Targets[1].GroupPhase = usecase.GroupTargetExternalNotAttempted }},
		{"target failed", func(f *userPreparedUpdateFixture) {
			f.got.Targets[1].Failure = &usecase.GroupTargetFailure{Stage: "verification"}
		}},
		{"missing result", func(f *userPreparedUpdateFixture) { f.got.Targets = f.got.Targets[:1] }},
		{"duplicate result", func(f *userPreparedUpdateFixture) { f.got.Targets[0] = f.got.Targets[1] }},
		{"wrong target binding", func(f *userPreparedUpdateFixture) { f.prepared.plan.Targets[1].BindingID += "-stale" }},
		{"wrong client context", func(f *userPreparedUpdateFixture) { f.got.Targets[1].Plan.TargetContext.UserConfigRoot += "-foreign" }},
		{"wrong profile", func(f *userPreparedUpdateFixture) { f.got.Targets[1].Plan.NativeRegistryRoot += "-foreign" }},
		{"wrong artifact", func(f *userPreparedUpdateFixture) { f.got.Targets[1].Plan.PhysicalArtifactID += "-foreign" }},
		{"wrong saved locator", func(f *userPreparedUpdateFixture) {
			f.changeBinding(domain.ClientVSCode, func(b *domain.ClientBinding) { b.TargetLocator += "-foreign" })
		}},
		{"forged user Local native root", func(f *userPreparedUpdateFixture) {
			f.changeBinding(domain.ClientVSCode, func(b *domain.ClientBinding) { b.NativeProfileRoot = "foreign" })
		}},
		{"missing binding", func(f *userPreparedUpdateFixture) {
			delete(f.state.Installations[0].Clients, f.prepared.plan.Targets[1].BindingID)
		}},
		{"missing data receipt", func(f *userPreparedUpdateFixture) {
			b := f.state.Installations[0].Clients[f.prepared.plan.Targets[1].BindingID]
			delete(f.state.Installations[0].DataReceipts, b.DataReceiptID)
		}},
		{"stale data receipt", func(f *userPreparedUpdateFixture) {
			f.changeBinding(domain.ClientVSCode, func(b *domain.ClientBinding) { b.DataReceiptID = "stale" })
		}},
		{"duplicate binding", func(f *userPreparedUpdateFixture) {
			b := f.state.Installations[0].Clients[f.prepared.plan.Targets[1].BindingID]
			b.ClientBindingID = "another-binding"
			f.state.Installations[0].Clients[b.ClientBindingID] = b
		}},
		{"pending native attempt", func(f *userPreparedUpdateFixture) {
			f.changeBinding(domain.ClientVSCode, func(b *domain.ClientBinding) { b.NativeActivationAttempt = "unreconciled" })
		}},
		{"pending native intent", func(f *userPreparedUpdateFixture) {
			f.changeBinding(domain.ClientVSCode, func(b *domain.ClientBinding) { b.PendingNativeIntent = &domain.PendingNativeIntent{} })
		}},
		{"unverified Local", func(f *userPreparedUpdateFixture) { f.got.Targets[1].Activation.Verification = domain.VerificationNotRun }},
		{"missing native observation", func(f *userPreparedUpdateFixture) {
			f.changeBinding(domain.ClientVSCode, func(b *domain.ClientBinding) { b.LocalEntryObservation = nil })
		}},
		{"missing native owner", func(f *userPreparedUpdateFixture) {
			f.changeBinding(domain.ClientVSCode, func(b *domain.ClientBinding) { b.NativeObjects = nil })
		}},
		{"missing selected Local delivery", func(f *userPreparedUpdateFixture) {
			f.got.Targets[1].Plan.SelectedDelivery = domain.SelectedDelivery{}
			f.prepared.plan.Targets[1].SelectedDelivery = domain.SelectedDelivery{}
			f.changeBinding(domain.ClientVSCode, func(b *domain.ClientBinding) { b.SelectedDelivery = domain.SelectedDelivery{} })
		}},,
		{"blocked policy", func(f *userPreparedUpdateFixture) { f.got.Targets[1].Activation.Policy = domain.PolicyBlocked }},
		{"unknown effect", func(f *userPreparedUpdateFixture) { f.got.Targets[1].Activation.NativeEffect = domain.NativeEffectUncertain }},
		{"activation attestation", func(f *userPreparedUpdateFixture) { f.got.Targets[1].Activation.ActivationAttested = true }},
		{"missing prepared guidance", func(f *userPreparedUpdateFixture) { f.got.Targets[1].Activation.UserActions = nil }},
		{"incorrect saved status", func(f *userPreparedUpdateFixture) {
			f.changeBinding(domain.ClientVSCode, func(b *domain.ClientBinding) { b.Activation = domain.ActivationActive })
		}},
		{"wrong prepared product", func(f *userPreparedUpdateFixture) {
			f.got.Targets[0].Activation.Activation = domain.ActivationPrepared
			f.got.Targets[0].Activation.UserActions = []string{"TEST unqualified"}
			f.changeBinding(domain.ClientCodex, func(b *domain.ClientBinding) { b.Activation = domain.ActivationPrepared })
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newUserPreparedUpdateFixture(t)
			tc.change(&f)
			result, err := f.finish(nil, nil)
			if err == nil || result.Outcome != OutcomeIncomplete || !result.Mutated ||
				result.NoChange || result.Reason != "committed binding is not activated" {
				t.Fatalf("unproven completion accepted: %+v %v", result, err)
			}
		})
	}
}

func TestUserGroupPreparedUpdateCannotHideFailureOrUnknownState(t *testing.T) {
	for _, failure := range []error{nil, errors.New("TEST activation failure")} {
		f := newUserPreparedUpdateFixture(t)
		result, err := f.finish(failure, errors.New("TEST state failure"))
		if err == nil || result.Outcome != OutcomeIncomplete || !result.Mutated || result.NoChange {
			t.Fatalf("state or activation error promoted: %+v %v", result, err)
		}
		if failure != nil && !errors.Is(err, failure) {
			t.Fatalf("actual activation error lost: %v", err)
		}
	}
}
