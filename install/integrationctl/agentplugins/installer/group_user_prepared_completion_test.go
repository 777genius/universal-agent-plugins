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
		req:  Request{Operation: OpUpdate, Scope: domain.ScopeUser, InstallationID: id},
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
			ActivePath:    active, PhysicalArtifactID: "artifact-" + string(clientID),
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
				Enabled:       native.Receipt.Enabled, ReceiptDigest: native.Receipt.Digest,
			})
			if err != nil {
				t.Fatal(err)
			}
			objects = []domain.NativeObjectOwnership{facts.Registration.Ownership(facts.SettingsPath)}
		}
		bindingID := domain.ComputeClientBindingID(id, string(clientID), string(domain.ScopeUser), active)
		receiptID := "receipt-" + string(clientID)
		installed.DataReceipts[receiptID] = domain.DataReceipt{DataReceiptID: receiptID, Scope: string(domain.ScopeUser),
			Locator: filepath.Join(root, "plugin-data", string(clientID)), State: domain.DataReceiptOwned,
			OwnershipDigest: "sha256:" + strings.Repeat("c", 64)}
		// Preview carries the selected authority before real staging seals
		// its projection digest. The final core Plan below carries that seal.
		previewPlan := plan
		if clientID == domain.ClientVSCode {
			facts, _ := plan.SelectedDelivery.LocalFacts()
			facts.ProjectionDigest = ""
			var err error
			previewPlan.SelectedDelivery, err = domain.NewLocalDelivery(facts)
			if err != nil {
				t.Fatal(err)
			}
		}
		prepared.req.Targets = append(prepared.req.Targets, ClientTarget{ClientID: string(clientID), ClientConfigRoot: config})
		prepared.clients = append(prepared.clients, client)
		prepared.deliveries = append(prepared.deliveries, preparedDeliveryPlan{plan: previewPlan})
		prepared.plan.Targets = append(prepared.plan.Targets, PlanTarget{
			ClientID: string(clientID), ConfigRoot: config, TargetPath: active, BindingID: bindingID,
			SelectedDelivery: previewPlan.SelectedDelivery,
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
			// Core returns the original planned view; the store below keeps
			// the separately sealed/native-acknowledged selected revision.
			InstallationID: id, Plan: previewPlan, Activation: outcome,
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
		state:    domain.StateFileV2{Installations: []domain.Installation{installed}},
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
			localID := f.prepared.plan.Targets[1].BindingID
			if f.got.Targets[1].Plan.SelectedDelivery.ProjectionDigest() != "" ||
				f.state.Installations[0].Clients[localID].SelectedDelivery.ProjectionDigest() == "" {
				t.Fatal("fixture did not separate pre-staging core plan from committed projection")
			}
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
		{"target incomplete", func(f *userPreparedUpdateFixture) {
			f.got.Targets[1].GroupPhase = usecase.GroupTargetExternalNotAttempted
		}},
		{"target failed", func(f *userPreparedUpdateFixture) {
			f.got.Targets[1].Failure = &usecase.GroupTargetFailure{Stage: "verification"}
		}},
		{"missing result", func(f *userPreparedUpdateFixture) { f.got.Targets = f.got.Targets[:1] }},
		{"duplicate result", func(f *userPreparedUpdateFixture) { f.got.Targets[0] = f.got.Targets[1] }},
		{"wrong target binding", func(f *userPreparedUpdateFixture) { f.prepared.plan.Targets[1].BindingID += "-stale" }},
		{"missing frozen delivery", func(f *userPreparedUpdateFixture) { f.prepared.deliveries = nil }},
		{"replaced frozen delivery", func(f *userPreparedUpdateFixture) {
			f.prepared.deliveries[1].plan.ActivePath += "-foreign"
		}},
		{"replaced frozen selection", func(f *userPreparedUpdateFixture) {
			facts, _ := f.prepared.deliveries[1].plan.SelectedDelivery.LocalFacts()
			facts.CanonicalDigest = "sha256:" + strings.Repeat("c", 64)
			replaced, err := domain.NewLocalDelivery(facts)
			if err != nil {
				t.Fatal(err)
			}
			f.prepared.deliveries[1].plan.SelectedDelivery = replaced
			f.prepared.plan.Targets[1].SelectedDelivery = replaced
		}},
		{"replaced staged digest", func(f *userPreparedUpdateFixture) {
			facts, _ := f.prepared.deliveries[1].plan.SelectedDelivery.LocalFacts()
			facts.ProjectionDigest = "sha256:" + strings.Repeat("c", 64)
			replaced, err := domain.NewLocalDelivery(facts)
			if err != nil {
				t.Fatal(err)
			}
			f.prepared.deliveries[1].plan.SelectedDelivery = replaced
			f.prepared.plan.Targets[1].SelectedDelivery = replaced
		}},
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
		{"indeterminate receipt ownership", func(f *userPreparedUpdateFixture) {
			b := f.state.Installations[0].Clients[f.prepared.plan.Targets[1].BindingID]
			receipt := f.state.Installations[0].DataReceipts[b.DataReceiptID]
			receipt.State = domain.DataReceiptUnknown
			f.state.Installations[0].DataReceipts[b.DataReceiptID] = receipt
		}},
		{"missing receipt ownership digest", func(f *userPreparedUpdateFixture) {
			b := f.state.Installations[0].Clients[f.prepared.plan.Targets[1].BindingID]
			receipt := f.state.Installations[0].DataReceipts[b.DataReceiptID]
			receipt.OwnershipDigest = ""
			f.state.Installations[0].DataReceipts[b.DataReceiptID] = receipt
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
		{"unverified Local", func(f *userPreparedUpdateFixture) {
			f.got.Targets[1].Activation.Verification = domain.VerificationNotRun
		}},
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
		}},
		{"blocked policy", func(f *userPreparedUpdateFixture) { f.got.Targets[1].Activation.Policy = domain.PolicyBlocked }},
		{"unknown effect", func(f *userPreparedUpdateFixture) {
			f.got.Targets[1].Activation.NativeEffect = domain.NativeEffectUncertain
		}},
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
				result.NoChange || result.Reason == "" {
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

// A real user Codex CLI without a usable listing executable reports manual
// activation with package-only verification. That does not undo a proven file
// Update or manufacture native Codex activation.
func setUserGroupManualCodex(f *userPreparedUpdateFixture) {
	outcome := &f.got.Targets[0].Activation
	outcome.Activation = domain.ActivationManual
	outcome.Verification = domain.VerificationPackageValid
	outcome.UserActions = []string{"install the prepared plugin in Codex Plugins"}
	f.changeBinding(domain.ClientCodex, func(b *domain.ClientBinding) {
		b.Activation = outcome.Activation
		b.Verification = outcome.Verification
	})
}

func TestUserGroupPreparedUpdateManualCodexFileCompletion(t *testing.T) {
	f := newUserPreparedUpdateFixture(t)
	setUserGroupManualCodex(&f)
	result, err := f.finish(nil, nil)
	if err != nil || result.Outcome != OutcomeCompleted || !result.Mutated || result.NoChange ||
		len(result.Targets) != 2 {
		t.Fatalf("proved user file Update was not completed: %+v %v", result, err)
	}
	if len(result.ManualActions) != 2 ||
		result.ManualActions[0] != "install the prepared plugin in Codex Plugins" ||
		result.ManualActions[1] != "TEST Local reload; activation not attested" {
		t.Fatalf("manual Codex or Local next actions lost: %v", result.ManualActions)
	}
	if result.Targets[0].Activation != string(domain.ActivationManual) ||
		result.Targets[0].Verification != string(domain.VerificationPackageValid) ||
		result.Targets[1].Activation != string(domain.ActivationPrepared) ||
		result.Targets[1].Verification != string(domain.VerificationPackageValid) {
		t.Fatalf("manual Codex or prepared Local was falsely activated: %+v", result.Targets)
	}
}

func TestUserGroupPreparedUpdateManualCodexCannotBorrowUnprovenFileCompletion(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*userPreparedUpdateFixture)
	}{
		{"missing manual action", func(f *userPreparedUpdateFixture) {
			f.got.Targets[0].Activation.UserActions = nil
		}},
		{"package verification missing", func(f *userPreparedUpdateFixture) {
			f.got.Targets[0].Activation.Verification = domain.VerificationNotRun
		}},
		{"manual activation attested", func(f *userPreparedUpdateFixture) {
			f.got.Targets[0].Activation.ActivationAttested = true
		}},
		{"manual authentication attested", func(f *userPreparedUpdateFixture) {
			f.got.Targets[0].Activation.AuthenticationAttested = true
		}},
		{"unknown native effect", func(f *userPreparedUpdateFixture) {
			f.got.Targets[0].Activation.NativeEffect = domain.NativeEffectUncertain
		}},
		{"blocked policy", func(f *userPreparedUpdateFixture) {
			f.got.Targets[0].Activation.Policy = domain.PolicyBlocked
		}},
		{"saved Codex not manual", func(f *userPreparedUpdateFixture) {
			f.changeBinding(domain.ClientCodex, func(b *domain.ClientBinding) {
				b.Activation = domain.ActivationActive
			})
		}},
		{"unresolved native attempt", func(f *userPreparedUpdateFixture) {
			f.changeBinding(domain.ClientCodex, func(b *domain.ClientBinding) {
				b.NativeActivationAttempt = "unreconciled"
			})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newUserPreparedUpdateFixture(t)
			setUserGroupManualCodex(&f)
			tc.change(&f)
			result, err := f.finish(nil, nil)
			if err == nil || result.Outcome != OutcomeIncomplete || !result.Mutated || result.NoChange {
				t.Fatalf("unproven manual Codex file completion accepted: %+v %v", result, err)
			}
		})
	}
}

func TestUserGroupPreparedUpdateRepeatedIdenticalFrozenDeliveries(t *testing.T) {
	f := newUserPreparedUpdateFixture(t)
	setUserGroupManualCodex(&f)
	// The real host group preview can capture multiple identical plans per
	// selected client. Repeating proof must not manufacture activation.
	f.prepared.deliveries = append(f.prepared.deliveries, f.prepared.deliveries[0], f.prepared.deliveries[1])
	result, err := f.finish(nil, nil)
	if err != nil || result.Outcome != OutcomeCompleted || !result.Mutated || result.NoChange ||
		len(result.Targets) != 2 || len(result.ManualActions) != 2 {
		t.Fatalf("identical frozen deliveries rejected: %+v %v", result, err)
	}
	if result.Targets[0].Activation != string(domain.ActivationManual) ||
		result.Targets[1].Activation != string(domain.ActivationPrepared) {
		t.Fatalf("file completion incorrectly attested native activation: %+v", result.Targets)
	}
}

func TestUserGroupPreparedUpdateConflictingRepeatedFrozenDeliveryFailsClosed(t *testing.T) {
	for _, clientID := range []domain.ClientID{domain.ClientCodex, domain.ClientVSCode} {
		t.Run(string(clientID), func(t *testing.T) {
			f := newUserPreparedUpdateFixture(t)
			setUserGroupManualCodex(&f)
			i := 0
			if clientID == domain.ClientVSCode {
				i = 1
			}
			conflicting := f.prepared.deliveries[i]
			conflicting.plan.NativeRegistryRoot += "-foreign"
			f.prepared.deliveries = append(f.prepared.deliveries, conflicting)
			result, err := f.finish(nil, nil)
			if err == nil || result.Outcome != OutcomeIncomplete || !result.Mutated || result.NoChange {
				t.Fatalf("conflicting frozen delivery accepted: %+v %v", result, err)
			}
		})
	}
}
