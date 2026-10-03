package usecase_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	vp "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodeprofile"
)

// This is the external core callback seam, backed by the real parser and
// filesystem state/stager. It deliberately does not replace NewLocal S2 proof.
type observedCoreAdapter struct {
	testEffectLocalAdapter
	observe         bool
	omitObservation bool
	calls           int
	afterVerify     func()
	verifyError     error
	repairAbsent    bool
	interruptFirst  bool
	effects         int
	absenceEffect   domain.NativeEffectState
}

type nilObservationCallbackAdapter struct {
	observedCoreAdapter
	afterEffect func(domain.ActivationRequest, domain.ActivationOutcome)
}

func (a *nilObservationCallbackAdapter) Activate(ctx context.Context, env clients.Env, req domain.ActivationRequest) (domain.ActivationOutcome, error) {
	a.calls++
	if err := ctx.Err(); err != nil {
		return domain.ActivationOutcome{}, err
	}
	outcome, err := a.observedCoreAdapter.Activate(ctx, env, req)
	if err == nil && !req.VerifyOnly && a.afterEffect != nil {
		a.afterEffect(req, outcome)
	}
	return outcome, err
}

// PR393 RED condition: a first selected registration commits real profile bytes,
// then its callback cancels with nil observations. Apply must expose cancellation
// and retain durable pending authority rather than acknowledge success and resend.
func TestNilObservationCallbackCancellation(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(fmt.Sprint(canceled), func(t *testing.T) {
			root := localProcessRoot(t)
			profilePath := filepath.Join(root, "profile", "settings.json")
			foreign := []byte(`{"chat.pluginLocations":{"/TEST-foreign":false},"foreign.setting":"original"}`)
			if err := os.WriteFile(profilePath, foreign, 0600); err != nil {
				t.Fatal(err)
			}
			a := &nilObservationCallbackAdapter{observedCoreAdapter: observedCoreAdapter{testEffectLocalAdapter: testEffectLocalAdapter{testLocalAdapter: testLocalAdapter{Adapter: vscode.New()}, root: root}}}
			engine := localTestEngine(t, filepath.Join(root, "state"), a)
			prepared, err := engine.Prepare(t.Context(), installerRequest(root))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = prepared.Close() }()
			store := statev2.Store{Path: filepath.Join(root, "state", "state-v2.json")}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var pending domain.ClientBinding
			var effectProfile []byte
			a.afterEffect = func(req domain.ActivationRequest, outcome domain.ActivationOutcome) {
				state, err := store.Load()
				if err != nil {
					t.Fatal(err)
				}
				pending = localOnlyBinding(t, state)
				if req.Plan.SelectedDelivery.IsZero() || req.Plan.LocalEntryObservation != nil || pending.LocalEntryObservation != nil || outcome.LocalEntryObservation != nil || outcome.NativeEffect != domain.NativeEffectCommitted {
					t.Fatal("fixture did not reach selected nil-observation committed callback")
				}
				if pending.NativeActivationAttempt == "" || pending.PendingNativeIntent == nil || pending.PendingNativeIntent.LocalEntryObservation != nil || pending.PendingNativeIntent.Direction != domain.NativeIntentRegister {
					t.Fatal("real callback lacks durable first-registration pending evidence")
				}
				if err := pending.PendingNativeIntent.Validate(pending); err != nil {
					t.Fatal(err)
				}
				effectProfile, err = os.ReadFile(profilePath)
				if err != nil {
					t.Fatal(err)
				}
				f := mustFacts(t, pending)
				id := vp.Identity{SettingsPath: f.SettingsPath, ProfileID: f.ProfileIdentity, PluginRoot: f.Registration.Selector, PackageID: f.Registration.ObjectID, PackageDigest: f.CanonicalDigest, ProjectionDigest: f.ProjectionDigest}
				if result, err := vp.VerifyRecordedEntry(effectProfile, id, true); err != nil || result.Receipt == nil {
					t.Fatalf("callback lacks actual parser-verified registration: %v", err)
				}
				if !bytes.Contains(effectProfile, []byte(`"foreign.setting":"original"`)) || !bytes.Contains(effectProfile, []byte(`"/TEST-foreign":false`)) {
					t.Fatal("registration lost foreign profile bytes")
				}
				if canceled {
					cancel()
				}
			}
			_, applyErr := engine.Apply(ctx, prepared, confirmedDecision())
			if a.calls != 1 || effectProfile == nil {
				t.Fatal("fixture did not execute exactly one real registration callback")
			}
			if canceled && !errors.Is(applyErr, context.Canceled) {
				t.Fatalf("committed nil-observation callback cancellation must reach public Apply: got %v", applyErr)
			}
			if !canceled && applyErr != nil {
				t.Fatalf("normal nil-observation callback failed: %v", applyErr)
			}
			state, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			live := localOnlyBinding(t, state)
			profile, err := os.ReadFile(profilePath)
			if err != nil || !bytes.Equal(profile, effectProfile) || live.LocalEntryObservation != nil {
				t.Fatalf("Apply changed committed profile or nil receipt: %v", err)
			}
			if !canceled {
				if live.NativeActivationAttempt != "" || live.PendingNativeIntent != nil || !live.SelectedDelivery.OwnsProfileEntry(live.NativeObjects) {
					t.Fatal("normal nil-observation registration was not acknowledged")
				}
				return
			}
			if live.NativeActivationAttempt != pending.NativeActivationAttempt || !reflect.DeepEqual(live.PendingNativeIntent, pending.PendingNativeIntent) || live.SelectedDelivery.OwnsProfileEntry(live.NativeObjects) {
				t.Fatal("cancellation cleared pending evidence or acknowledged selector ownership")
			}
			stateBeforeRetry, err := os.ReadFile(store.Path)
			if err != nil {
				t.Fatal(err)
			}
			service := localGroupService(t, root, a)
			input := localGroupInput(t, root)
			input.InstallationID = state.Installations[0].InstallationID
			input.Confirmed = true
			if _, err := service.Repair(t.Context(), input); err == nil || !strings.Contains(err.Error(), "pending Local attempt") || a.calls != 1 {
				t.Fatalf("pending cancellation allowed blind callback resend: calls=%d err=%v", a.calls, err)
			}
			stateAfterRetry, err := os.ReadFile(store.Path)
			if err != nil || !bytes.Equal(stateBeforeRetry, stateAfterRetry) {
				t.Fatalf("refused retry changed durable pending evidence: %v", err)
			}
			profile, err = os.ReadFile(profilePath)
			if err != nil || !bytes.Equal(profile, effectProfile) {
				t.Fatalf("refused retry changed committed foreign/profile bytes: %v", err)
			}
		})
	}
}

// A real host handoff may write Store state, but it cannot replace frozen
// ownership authority and then have a nil-observation activation acknowledge it.
func TestNilObservationHostCallbackCannotReplaceBindingAuthority(t *testing.T) {
	for _, downgradeSelection := range []bool{false, true} {
		t.Run(fmt.Sprint(downgradeSelection), func(t *testing.T) {
			root := localProcessRoot(t)
			profilePath := filepath.Join(root, "profile", "settings.json")
			foreign := []byte(`{"chat.pluginLocations":{"/TEST-foreign":false},"foreign.setting":"original"}`)
			if err := os.WriteFile(profilePath, foreign, 0600); err != nil {
				t.Fatal(err)
			}
			foreignDirectory := filepath.Join(root, "TEST-foreign-package")
			if err := os.Mkdir(foreignDirectory, 0700); err != nil {
				t.Fatal(err)
			}
			foreignFile := filepath.Join(foreignDirectory, "keep.txt")
			if err := os.WriteFile(foreignFile, []byte("foreign-owned"), 0600); err != nil {
				t.Fatal(err)
			}
			store := statev2.Store{Path: filepath.Join(root, "state", "state-v2.json")}
			a := &nilObservationCallbackAdapter{observedCoreAdapter: observedCoreAdapter{testEffectLocalAdapter: testEffectLocalAdapter{testLocalAdapter: testLocalAdapter{Adapter: vscode.New()}, root: root}}}
			var pending domain.ClientBinding
			handoffs := 0
			engine := facadeEngine(t, root, a, installer.Config{OnCommittedBinding: func(_ context.Context, facts installer.BindingFacts) error {
				handoffs++
				state, err := store.Load()
				if err != nil {
					return err
				}
				pending = localOnlyBinding(t, state)
				if pending.ClientBindingID != facts.BindingID || pending.NativeActivationAttempt == "" || pending.PendingNativeIntent == nil || pending.LocalEntryObservation != nil {
					t.Fatal("handoff did not reach the frozen selected nil-observation binding")
				}
				if downgradeSelection {
					return nil // The downgrade callback runs only after the real native write.
				}
				changed := pending
				changed.NativeObjects = append([]domain.NativeObjectOwnership(nil), pending.NativeObjects...)
				replaced := false
				for i := range changed.NativeObjects {
					if changed.NativeObjects[i].Kind == "managed_package_directory" {
						changed.NativeObjects[i].Path = foreignDirectory
						replaced = true
					}
				}
				if !replaced {
					t.Fatal("handoff fixture lacks committed package authority")
				}
				state.Installations[0].Clients[pending.ClientBindingID] = changed
				return store.Save(state)
			}})
			if downgradeSelection {
				a.afterEffect = func(req domain.ActivationRequest, outcome domain.ActivationOutcome) {
					if outcome.NativeEffect != domain.NativeEffectCommitted || outcome.LocalEntryObservation != nil || req.Plan.SelectedDelivery.IsZero() {
						t.Fatal("downgrade callback did not follow a selected committed nil-observation effect")
					}
					state, err := store.Load()
					if err != nil {
						t.Fatal(err)
					}
					pending = localOnlyBinding(t, state)
					changed := pending
					changed.SelectedDelivery = domain.SelectedDelivery{}
					changed.PendingNativeIntent = nil
					state.Installations[0].Clients[pending.ClientBindingID] = changed
					if err := store.Save(state); err != nil {
						t.Fatal(err)
					}
				}
			}
			prepared, err := engine.Prepare(t.Context(), installerRequest(root))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = prepared.Close() }()
			_, applyErr := engine.Apply(t.Context(), prepared, confirmedDecision())
			if handoffs != 1 || a.calls != 1 {
				t.Fatalf("fixture did not execute one real handoff and activation: handoffs=%d activation=%d", handoffs, a.calls)
			}
			if applyErr == nil || !strings.Contains(applyErr.Error(), "activation callback changed frozen binding authority") {
				t.Fatalf("public Apply acknowledged callback-replaced nil-observation authority: %v", applyErr)
			}
			state, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			live := localOnlyBinding(t, state)
			wantPending := pending.PendingNativeIntent
			if downgradeSelection {
				wantPending = nil // The host already removed it; refusal must retain the attempt.
			}
			if live.NativeActivationAttempt != pending.NativeActivationAttempt || !reflect.DeepEqual(live.PendingNativeIntent, wantPending) || live.LocalEntryObservation != nil || live.SelectedDelivery.IsZero() != downgradeSelection {
				t.Fatal("refusal cleared the attempt or hid the callback's committed state change")
			}
			for _, object := range live.NativeObjects {
				if object.ObjectID == "TEST-profile-entry" {
					t.Fatal("refusal acknowledged returned selector ownership")
				}
			}
			profile, err := os.ReadFile(profilePath)
			if err != nil {
				t.Fatal(err)
			}
			f := mustFacts(t, pending)
			id := vp.Identity{SettingsPath: f.SettingsPath, ProfileID: f.ProfileIdentity, PluginRoot: f.Registration.Selector, PackageID: f.Registration.ObjectID, PackageDigest: f.CanonicalDigest, ProjectionDigest: f.ProjectionDigest}
			if result, err := vp.VerifyRecordedEntry(profile, id, true); err != nil || result.Receipt == nil {
				t.Fatalf("refusal hid or rolled back the already committed profile effect: %v", err)
			}
			if !bytes.Contains(profile, []byte(`"foreign.setting":"original"`)) || !bytes.Contains(profile, []byte(`"/TEST-foreign":false`)) {
				t.Fatal("activation lost foreign profile entries")
			}
			if body, err := os.ReadFile(foreignFile); err != nil || string(body) != "foreign-owned" {
				t.Fatalf("activation changed the foreign directory claimed by the callback: %v", err)
			}
			stateBeforeRetry, err := os.ReadFile(store.Path)
			if err != nil {
				t.Fatal(err)
			}
			service := localGroupService(t, root, a)
			input := localGroupInput(t, root)
			input.InstallationID, input.Confirmed = state.Installations[0].InstallationID, true
			if _, err := service.Repair(t.Context(), input); err == nil || a.calls != 1 || handoffs != 1 {
				t.Fatalf("pending callback tampering allowed blind activation resend: %v", err)
			}
			if after, err := os.ReadFile(store.Path); err != nil || !bytes.Equal(stateBeforeRetry, after) {
				t.Fatalf("refused retry changed pending state: %v", err)
			}
			if after, err := os.ReadFile(profilePath); err != nil || !bytes.Equal(profile, after) {
				t.Fatalf("refused retry changed committed profile: %v", err)
			}
		})
	}
}

// The read-only native projection is existing confirmed selector ownership,
// independently established by the install fixture. No settings bool adopts it.
func (a *observedCoreAdapter) ProjectActiveNative(_ context.Context, _ string, _ domain.PackageEnvelope, plan domain.DeliveryPlan, _ string) ([]domain.NativeObjectOwnership, error) {
	f, _ := plan.SelectedDelivery.LocalFacts()
	return []domain.NativeObjectOwnership{f.Registration.Ownership(f.SettingsPath)}, nil
}

func (a *observedCoreAdapter) VerifierAvailable(domain.DetectedClient, domain.DeliveryPlan, string) bool {
	return true
}
func (a *observedCoreAdapter) Activate(ctx context.Context, env clients.Env, req domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if !a.observe {
		outcome, err := a.testEffectLocalAdapter.Activate(ctx, env, req)
		if a.interruptFirst && err == nil {
			outcome.NativeEffect = domain.NativeEffectUncertain
			return outcome, errors.New("TEST interrupted after native effect before acknowledgement")
		}
		return outcome, err
	}
	a.calls++
	f, _ := req.Plan.SelectedDelivery.LocalFacts()
	if !req.Plan.SelectedDelivery.OwnsProfileEntry(req.PreviousNativeObjects) {
		return domain.ActivationOutcome{}, errors.New("TEST verifier lacks independent owned predecessor")
	}
	snapshot, err := env.NativeConfig.ReadExactFile(f.SettingsPath)
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	enabled := *f.Registration.DesiredValue
	if req.Plan.LocalEntryObservation != nil {
		enabled = req.Plan.LocalEntryObservation.Facts().Enabled
	}
	id := vp.Identity{SettingsPath: f.SettingsPath, ProfileID: f.ProfileIdentity, PluginRoot: f.Registration.Selector, PackageID: f.Registration.ObjectID, PackageDigest: f.CanonicalDigest, ProjectionDigest: f.ProjectionDigest}
	result, effect, err := a.verifyOrRestoreCoreEntry(env, req, snapshot.Body, id, enabled)
	if err != nil {
		return domain.ActivationOutcome{NativeEffect: effect}, err
	}
	observation, err := domain.NewLocalEntryObservation(domain.LocalEntryObservationFacts{RevisionBasis: req.Plan.SelectedDelivery, Enabled: result.Receipt.Enabled, ReceiptDigest: result.Receipt.Digest})
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	if a.omitObservation {
		observation = nil
	}
	if a.afterVerify != nil {
		a.afterVerify()
	}
	return domain.ActivationOutcome{LocalEntryObservation: observation, Activation: domain.ActivationActive, Authentication: domain.AuthenticationNotRequired, Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled, NativeEffect: effect, NativeObjects: []domain.NativeObjectOwnership{f.Registration.Ownership(f.SettingsPath)}}, a.verifyError
}

type observedCoreReconciler struct {
	adapter *observedCoreAdapter
	store   statev2.Store
}

func (r observedCoreReconciler) ReconcileNativeIntent(ctx context.Context, intent domain.PendingNativeIntent) (domain.ActivationOutcome, error) {
	state, err := r.store.Load()
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	binding := onlyLocalBinding(state)
	return r.adapter.Activate(ctx, clients.Env{NativeConfig: nativeconfig.New()}, domain.ActivationRequest{
		Plan:                  domain.DeliveryPlan{SelectedDelivery: intent.Delivery, LocalEntryObservation: intent.LocalEntryObservation.Clone()},
		PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), binding.NativeObjects...), VerifyOnly: true,
	})
}

type observedCountingStore struct {
	statev2.Store
	saves           int
	fail            bool
	visibleFailures int
	thirdState      *domain.StateFileV2
}

func (s *observedCountingStore) Save(state domain.StateFileV2) error {
	s.saves++
	if s.thirdState != nil {
		if err := s.Store.Save(*s.thirdState); err != nil {
			return err
		}
		return errors.New("TEST third visible state")
	}
	if s.visibleFailures > 0 {
		s.visibleFailures--
		if err := s.Store.Save(state); err != nil {
			return err
		}
		return errors.New("TEST save error after atomic visibility")
	}
	if s.fail {
		return errors.New("TEST acknowledgement storage failure")
	}
	return s.Store.Save(state)
}

func observedCoreFixture(t *testing.T) (string, *observedCoreAdapter, usecase.Service, usecase.AddInput, *observedCountingStore) {
	t.Helper()
	root := localProcessRoot(t)
	a := &observedCoreAdapter{testEffectLocalAdapter: testEffectLocalAdapter{testLocalAdapter: testLocalAdapter{Adapter: vscode.New()}, root: root}}
	engine := localTestEngine(t, filepath.Join(root, "state"), a)
	prepared, err := engine.Prepare(t.Context(), installerRequest(root))
	if err != nil {
		t.Fatal(err)
	}
	installed, err := engine.Apply(t.Context(), prepared, confirmedDecision())
	_ = prepared.Close()
	if err != nil {
		t.Fatal(err)
	}
	a.observe = true
	service := localGroupService(t, root, a)
	registry, err := clients.NewRegistry(a)
	if err != nil {
		t.Fatal(err)
	}
	service.Stager = providers.Stager{Registry: registry, Paths: pathpolicy.Policy{}}
	input := localGroupInput(t, root)
	input.InstallationID = installed.InstallationID
	input.Confirmed = true
	store := &observedCountingStore{Store: statev2.Store{Path: filepath.Join(root, "state", "state-v2.json")}}
	service.StateStore = store
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	f := mustFacts(t, localOnlyBinding(t, state))
	key, err := jsonKey(f.Registration.Selector)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{/* TEST native edit */"foreign":42,"chat.pluginLocations":{"/TEST-foreign":false,` + key + `:false}}`)
	if err := os.WriteFile(f.SettingsPath, body, 0600); err != nil {
		t.Fatal(err)
	}
	return root, a, service, input, store
}

func jsonKey(value string) (string, error) {
	body, err := json.Marshal(value)
	return string(body), err
}

// Enum equality previously hid false receipt changes; prove one actual atomic
// Save and an identical repeat with stable bytes/times through public Repair.
func TestObservedRepairPersistsFalseOnce(t *testing.T) {
	_, adapter, service, input, store := observedCoreFixture(t)
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := localOnlyBinding(t, before)
	facts := mustFacts(t, binding)
	profileBefore, err := os.ReadFile(facts.SettingsPath)
	if err != nil {
		t.Fatal(err)
	}
	profileInfo, err := os.Stat(facts.SettingsPath)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Repair(t.Context(), input)
	if err != nil || !result.Mutated || store.saves != 1 {
		t.Fatalf("first receipt persistence: result=%+v saves=%d err=%v", result, store.saves, err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	current := localOnlyBinding(t, state)
	if current.LocalEntryObservation == nil || current.LocalEntryObservation.Facts().Enabled || current.PendingNativeIntent != nil || current.SelectedDelivery.OwnsProfileEntry(current.NativeObjects) != true {
		t.Fatal("false/ownership/acknowledgement lost")
	}
	desired := mustFacts(t, current)
	if !*desired.Registration.DesiredValue {
		t.Fatal("observed false replaced desired true")
	}
	stateBefore, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	stateInfo, err := os.Stat(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := service.Repair(t.Context(), input)
	if err != nil || repeated.Mutated || !repeated.NoChange || store.saves != 1 {
		t.Fatalf("repeat churned receipt: %+v saves=%d err=%v", repeated, store.saves, err)
	}
	stateAfter, _ := os.ReadFile(store.Path)
	profileAfter, _ := os.ReadFile(facts.SettingsPath)
	stateAfterInfo, _ := os.Stat(store.Path)
	profileAfterInfo, _ := os.Stat(facts.SettingsPath)
	if !bytes.Equal(stateBefore, stateAfter) || !bytes.Equal(profileBefore, profileAfter) || !os.SameFile(stateInfo, stateAfterInfo) || !os.SameFile(profileInfo, profileAfterInfo) || !stateInfo.ModTime().Equal(stateAfterInfo.ModTime()) || !profileInfo.ModTime().Equal(profileAfterInfo.ModTime()) {
		t.Fatal("repeat changed durable state/profile bytes or timestamps")
	}
	adapter.omitObservation = true
	if _, err := service.Repair(t.Context(), input); err == nil {
		t.Fatal("missing verification accepted as an observed repeat")
	}
	refusedState, _ := os.ReadFile(store.Path)
	refusedProfile, _ := os.ReadFile(facts.SettingsPath)
	if store.saves != 1 || !bytes.Equal(stateBefore, refusedState) || !bytes.Equal(profileBefore, refusedProfile) {
		t.Fatal("missing verification changed recorded authority or native bytes")
	}
}

// Errors, cancellation and failed acknowledgement must retain the exact old
// authority/attempt. The profile is an actual parser-verified disposable file.
func TestObservedActivationRetainsIntentOnUncertainty(t *testing.T) {
	for _, mode := range []string{"error", "cancel", "storage", "drift"} {
		t.Run(mode, func(t *testing.T) {
			_, a, service, input, store := observedCoreFixture(t)
			if _, err := service.Repair(t.Context(), input); err != nil {
				t.Fatal(err)
			}
			before, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			old := localOnlyBinding(t, before)
			facts := mustFacts(t, old)
			old.NativeActivationAttempt = "TEST-attempt"
			old.PendingNativeIntent = &domain.PendingNativeIntent{AttemptID: old.NativeActivationAttempt, Direction: domain.NativeIntentRegister, Delivery: old.SelectedDelivery, LocalEntryObservation: old.LocalEntryObservation.Clone()}
			before.Installations[0].Clients[old.ClientBindingID] = old
			if err := store.Store.Save(before); err != nil {
				t.Fatal(err)
			}
			profileBefore, _ := os.ReadFile(facts.SettingsPath)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch mode {
			case "error":
				a.verifyError = errors.New("TEST callback failed after verification")
			case "cancel":
				a.afterVerify = cancel
			case "storage":
				store.fail = true
			case "drift":
				a.afterVerify = func() {
					current, err := store.Load()
					if err != nil {
						t.Fatal(err)
					}
					for k, b := range current.Installations[0].Clients {
						b.NativeActivationAttempt = "TEST-replaced-attempt"
						b.PendingNativeIntent = &domain.PendingNativeIntent{AttemptID: b.NativeActivationAttempt, Direction: domain.NativeIntentRegister, Delivery: b.SelectedDelivery, LocalEntryObservation: b.LocalEntryObservation.Clone()}
						current.Installations[0].Clients[k] = b
					}
					if err := store.Store.Save(current); err != nil {
						t.Fatal(err)
					}
				}
			}
			calls := a.calls
			err = service.RecoverNativeIntent(ctx, input.InstallationID, old.ClientBindingID, old.NativeActivationAttempt, observedCoreReconciler{adapter: a, store: store.Store})
			if err == nil {
				t.Fatal("uncertain recovery acknowledged")
			}
			after, loadErr := store.Load()
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			live := localOnlyBinding(t, after)
			profileAfter, _ := os.ReadFile(facts.SettingsPath)
			if !live.LocalEntryObservation.Equal(old.LocalEntryObservation) || !bytes.Equal(profileBefore, profileAfter) || a.calls-calls != 1 {
				t.Fatal("uncertainty replaced authority, native bytes, or retried effect")
			}
			if live.PendingNativeIntent == nil || (mode == "drift" && live.NativeActivationAttempt != "TEST-replaced-attempt") || (mode != "drift" && live.NativeActivationAttempt != old.NativeActivationAttempt) {
				t.Fatal("pending uncertainty was cleared")
			}
		})
	}
}

// Preparation freezes recorded observation separately from the raw new plan;
// a late valid predecessor edit must be refused before any activation callback.
func TestObservedConfirmationFreezesAuthority(t *testing.T) {
	root, a, service, input, store := observedCoreFixture(t)
	if _, err := service.Repair(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	engine := localTestEngine(t, filepath.Join(root, "state"), a)
	prepared, err := engine.Prepare(t.Context(), installerRequest(root))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = prepared.Close() }()
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for k, b := range state.Installations[0].Clients {
		f := mustFacts(t, b)
		f.Tuple.QualificationID += "-late"
		basis, err := domain.NewLocalDelivery(f)
		if err != nil {
			t.Fatal(err)
		}
		o := b.LocalEntryObservation.Facts()
		o.RevisionBasis = basis
		b.LocalEntryObservation, err = domain.NewLocalEntryObservation(o)
		if err != nil {
			t.Fatal(err)
		}
		state.Installations[0].Clients[k] = b
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(store.Path)
	calls := a.calls
	if _, err := engine.Apply(t.Context(), prepared, confirmedDecision()); !errors.Is(err, installer.ErrPlanChanged) {
		t.Fatalf("changed receipt confirmed: %v", err)
	}
	after, _ := os.ReadFile(store.Path)
	if a.calls != calls || !bytes.Equal(before, after) {
		t.Fatal("confirmation drift reached callback or saved state")
	}
}

// Exercise the existing exact-old/exact-desired decision kernel with real
// filesystem storage; no native profile retry is authorized by write ambiguity.
func TestObservedAcknowledgementVisibility(t *testing.T) {
	for _, third := range []bool{false, true} {
		t.Run(fmt.Sprint(third), func(t *testing.T) {
			_, a, service, input, store := observedCoreFixture(t)
			before, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			binding := localOnlyBinding(t, before)
			if binding.LocalEntryObservation != nil {
				t.Fatal("first observation fixture is not historical nil")
			}
			binding.NativeActivationAttempt = "TEST-visible-attempt"
			binding.PendingNativeIntent = &domain.PendingNativeIntent{AttemptID: binding.NativeActivationAttempt, Direction: domain.NativeIntentRegister, Delivery: binding.SelectedDelivery, LocalEntryObservation: binding.LocalEntryObservation.Clone()}
			before.Installations[0].Clients[binding.ClientBindingID] = binding
			if err := store.Store.Save(before); err != nil {
				t.Fatal(err)
			}
			f := mustFacts(t, binding)
			profileBefore, _ := os.ReadFile(f.SettingsPath)
			if third {
				altered, err := store.Load()
				if err != nil {
					t.Fatal(err)
				}
				b := localOnlyBinding(t, altered)
				b.Activation = domain.ActivationManual
				altered.Installations[0].Clients[b.ClientBindingID] = b
				store.thirdState = &altered
			} else {
				store.visibleFailures = 1
			}
			store.saves = 0
			calls := a.calls
			err = service.RecoverNativeIntent(t.Context(), input.InstallationID, binding.ClientBindingID, binding.NativeActivationAttempt, observedCoreReconciler{adapter: a, store: store.Store})
			after, loadErr := store.Load()
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			live := localOnlyBinding(t, after)
			if third {
				if err == nil || live.PendingNativeIntent == nil || live.NativeActivationAttempt != binding.NativeActivationAttempt || store.saves != 1 {
					t.Fatalf("ambiguous state was acknowledged or retried: %v saves=%d", err, store.saves)
				}
			} else if err != nil || live.PendingNativeIntent != nil || live.NativeActivationAttempt != "" || store.saves != 2 {
				t.Fatalf("visible exact decision did not become durable: %v saves=%d", err, store.saves)
			}
			profileAfter, _ := os.ReadFile(f.SettingsPath)
			if third && !live.LocalEntryObservation.Equal(binding.LocalEntryObservation) || !third && (live.LocalEntryObservation == nil || live.LocalEntryObservation.Facts().Enabled || !reflect.DeepEqual(live.LocalEntryObservation.Facts().RevisionBasis, binding.SelectedDelivery)) || !bytes.Equal(profileBefore, profileAfter) || a.calls != calls+1 {
				t.Fatal("state visibility changed authority/profile or retried native verification")
			}
		})
	}
}

// Only actual parser absence is translated to the narrow core classification.
// Effects use the real parser/Plan and exact-file kernel in disposable profiles.
func (a *observedCoreAdapter) verifyOrRestoreCoreEntry(env clients.Env, req domain.ActivationRequest, body []byte, id vp.Identity, enabled bool) (vp.Result, domain.NativeEffectState, error) {
	result, err := vp.VerifyRecordedEntry(body, id, enabled)
	if !a.repairAbsent || !errors.Is(err, vp.ErrRecordedEntryAbsent) || req.Plan.LocalEntryObservation == nil {
		return result, domain.NativeEffectUnchanged, err
	}
	if a.verifyError != nil {
		return result, domain.NativeEffectUnchanged, a.verifyError
	}
	if req.VerifyOnly {
		if a.afterVerify != nil {
			a.afterVerify()
		}
		absent, err := domain.NewLocalEntryAbsence(req.Plan.SelectedDelivery)
		if err != nil {
			return result, domain.NativeEffectUnchanged, err
		}
		if a.absenceEffect != "" {
			return result, a.absenceEffect, absent
		}
		return result, domain.NativeEffectUnchanged, absent
	}
	o := req.Plan.LocalEntryObservation.Facts()
	receipt := &vp.Receipt{Version: "1", Selector: "chat.pluginLocations", Identity: id, Enabled: o.Enabled, Digest: o.ReceiptDigest}
	result, err = vp.Plan(vp.Request{Settings: body, Identity: id, Previous: receipt, Action: vp.Repair})
	if err != nil {
		return result, domain.NativeEffectUnchanged, err
	}
	file, err := env.NativeConfig.BeginExactFile(id.SettingsPath)
	if err != nil {
		return result, domain.NativeEffectUnchanged, err
	}
	defer func() { _ = file.Close() }()
	if err := file.Apply(result.Settings); err != nil {
		_ = file.Rollback()
		return result, domain.NativeEffectUncertain, err
	}
	a.effects++
	return result, domain.NativeEffectCommitted, nil
}

// Regression M1: complete projected ownership hid native absence. The public
// Repair boundary must require confirmation, then restore the recorded bool.
func TestObservedAbsentRepairDispatch(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			_, a, service, input, store := observedCoreFixture(t)
			state, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			binding := localOnlyBinding(t, state)
			f := mustFacts(t, binding)
			if enabled {
				key, err := jsonKey(f.Registration.Selector)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(f.SettingsPath, []byte(`{"chat.pluginLocations":{`+key+`:true}}`), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := service.Repair(t.Context(), input); err != nil {
				t.Fatal(err)
			}
			state, err = store.Load()
			if err != nil {
				t.Fatal(err)
			}
			recorded := localOnlyBinding(t, state).LocalEntryObservation.Clone()
			if recorded == nil || recorded.Facts().Enabled != enabled {
				t.Fatal("fixture did not record actual native bool")
			}
			absent := []byte(`{/* TEST late sibling */"foreign":43,"chat.pluginLocations":{"/TEST-late":false}}`)
			if err := os.WriteFile(f.SettingsPath, absent, 0600); err != nil {
				t.Fatal(err)
			}
			oldState, err := os.ReadFile(store.Path)
			if err != nil {
				t.Fatal(err)
			}
			a.repairAbsent = true
			for _, mode := range []string{"unconfirmed", "dry", "generic", "cancel", "uncertain", "unknown", "joined", "basis"} {
				t.Run(mode, func(t *testing.T) {
					request := input
					ctx, cancel := context.WithCancel(t.Context())
					defer cancel()
					switch mode {
					case "unconfirmed":
						request.Confirmed = false
					case "dry":
						request.DryRun = true
					case "generic":
						a.verifyError = errors.New("TEST unrelated verification failure")
					case "cancel":
						a.afterVerify = cancel
					case "uncertain":
						a.absenceEffect = domain.NativeEffectUncertain
					case "unknown":
						a.absenceEffect = domain.NativeEffectState("TEST-unknown")
					case "joined":
						absence, err := domain.NewLocalEntryAbsence(recorded.Facts().RevisionBasis)
						if err != nil {
							t.Fatal(err)
						}
						a.verifyError = errors.Join(absence, errors.New("TEST independent verification failure"))
					case "basis":
						stale, _ := recorded.Facts().RevisionBasis.LocalFacts()
						stale.Tuple.QualificationID += "-stale"
						basis, err := domain.NewLocalDelivery(stale)
						if err != nil {
							t.Fatal(err)
						}
						a.verifyError, err = domain.NewLocalEntryAbsence(basis)
						if err != nil {
							t.Fatal(err)
						}

					}
					result, err := service.Repair(ctx, request)
					if mode == "unconfirmed" && (err != nil || !result.RequiresConfirmation) || mode == "dry" && err != nil || (mode == "generic" || mode == "cancel" || mode == "uncertain" || mode == "unknown" || mode == "joined" || mode == "basis") && err == nil {
						t.Fatalf("absence refusal/confirmation contract: %+v %v", result, err)
					}
					a.verifyError = nil
					a.afterVerify = nil
					a.absenceEffect = ""
					profile, err := os.ReadFile(f.SettingsPath)
					if err != nil {
						t.Fatal(err)
					}
					state, err := os.ReadFile(store.Path)
					if err != nil {
						t.Fatal(err)
					}
					if a.effects != 0 || !bytes.Equal(profile, absent) || !bytes.Equal(oldState, state) {
						t.Fatal("unconfirmed/failed verification changed bytes or authority")
					}
				})
			}
			repaired, err := service.Repair(t.Context(), input)
			if err != nil || !repaired.Mutated || repaired.Activation.NativeEffect != domain.NativeEffectCommitted || a.effects != 1 {
				t.Fatalf("confirmed recorded absence could not be repaired: %+v effects=%d err=%v", repaired, a.effects, err)
			}
			body, err := os.ReadFile(f.SettingsPath)
			if err != nil {
				t.Fatal(err)
			}
			verified, err := vp.VerifyRecordedEntry(body, vp.Identity{SettingsPath: f.SettingsPath, ProfileID: f.ProfileIdentity, PluginRoot: f.Registration.Selector, PackageID: f.Registration.ObjectID, PackageDigest: f.CanonicalDigest, ProjectionDigest: f.ProjectionDigest}, enabled)
			if err != nil || verified.Receipt.Enabled != enabled || !bytes.Contains(body, []byte("/* TEST late sibling */")) || !bytes.Contains(body, []byte(`"/TEST-late":false`)) {
				t.Fatalf("restoration lost bool/foreign JSONC: %v", err)
			}
			live, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			b := localOnlyBinding(t, live)
			if !b.LocalEntryObservation.Equal(recorded) || b.PendingNativeIntent != nil || b.NativeActivationAttempt != "" {
				t.Fatal("restoration changed recorded basis/bool or failed acknowledgement")
			}
			before, err := os.ReadFile(store.Path)
			if err != nil {
				t.Fatal(err)
			}
			saves := store.saves
			repeat, err := service.Repair(t.Context(), input)
			after, readErr := os.ReadFile(store.Path)
			if err != nil || readErr != nil || repeat.Mutated || !repeat.NoChange || store.saves != saves || a.effects != 1 || !bytes.Equal(before, after) {
				t.Fatal("restored repeat churned or resent")
			}
		})
	}
}

type observedFirstRegisterReconciler struct {
	store  statev2.Store
	calls  int
	effect domain.NativeEffectState
	mode   string
	cancel context.CancelFunc
}

func (r *observedFirstRegisterReconciler) ReconcileNativeIntent(ctx context.Context, intent domain.PendingNativeIntent) (domain.ActivationOutcome, error) {
	r.calls++
	state, err := r.store.Load()
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	binding := onlyLocalBinding(state)
	if err := intent.Validate(binding); err != nil {
		return domain.ActivationOutcome{}, err
	}
	f, _ := intent.Delivery.LocalFacts()
	body, err := os.ReadFile(f.SettingsPath)
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	id := vp.Identity{SettingsPath: f.SettingsPath, ProfileID: f.ProfileIdentity, PluginRoot: f.Registration.Selector, PackageID: f.Registration.ObjectID, PackageDigest: f.CanonicalDigest, ProjectionDigest: f.ProjectionDigest}
	verified, err := vp.VerifyRecordedEntry(body, id, *f.Registration.DesiredValue)
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	basis := intent.Delivery
	if r.mode == "basis" {
		f.Tuple.QualificationID += "-foreign"
		basis, err = domain.NewLocalDelivery(f)
		if err != nil {
			return domain.ActivationOutcome{}, err
		}
	}
	observation, err := domain.NewLocalEntryObservation(domain.LocalEntryObservationFacts{RevisionBasis: basis, Enabled: verified.Receipt.Enabled, ReceiptDigest: verified.Receipt.Digest})
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	object := f.Registration.Ownership(f.SettingsPath)
	if r.mode == "selector" {
		object.LogicalName += "-foreign"
	}
	r.effect = domain.NativeEffectUnchanged
	outcome := domain.ActivationOutcome{LocalEntryObservation: observation, NativeEffect: r.effect, NativeObjects: []domain.NativeObjectOwnership{object}, Activation: domain.ActivationPrepared, Authentication: domain.AuthenticationNotRequired, Policy: domain.PolicyAllowed, Verification: domain.VerificationPackageValid}
	if r.mode == "error" {
		return outcome, errors.New("TEST failed verified recovery")
	}
	if r.mode == "cancel" {
		r.cancel()
	}
	return outcome, ctx.Err()
}

// Regression M2: first-register pending authority is independent of confirmed
// selector ownership. Real Store must accept the atomic receipt+selector ack.
func TestObservedFirstRegistrationRecovery(t *testing.T) {
	for _, mode := range []string{"ack", "selector", "basis", "error", "cancel", "attempt"} {
		t.Run(mode, func(t *testing.T) {
			root := localProcessRoot(t)
			a := &observedCoreAdapter{testEffectLocalAdapter: testEffectLocalAdapter{testLocalAdapter: testLocalAdapter{Adapter: vscode.New()}, root: root}, interruptFirst: true}
			engine := localTestEngine(t, filepath.Join(root, "state"), a)
			prepared, err := engine.Prepare(t.Context(), installerRequest(root))
			if err != nil {
				t.Fatal(err)
			}
			_, applyErr := engine.Apply(t.Context(), prepared, confirmedDecision())
			_ = prepared.Close()
			if applyErr == nil {
				t.Fatal("fixture did not interrupt real first-registration effect")
			}
			store := statev2.Store{Path: filepath.Join(root, "state", "state-v2.json")}
			state, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			b := localOnlyBinding(t, state)
			f := mustFacts(t, b)
			if b.PendingNativeIntent == nil || b.LocalEntryObservation != nil || b.SelectedDelivery.OwnsProfileEntry(b.NativeObjects) {
				t.Fatal("fixture pregranted selector/observation or lost pending registration")
			}
			profile, err := os.ReadFile(f.SettingsPath)
			if err != nil {
				t.Fatal(err)
			}
			if result, err := vp.VerifyRecordedEntry(profile, vp.Identity{SettingsPath: f.SettingsPath, ProfileID: f.ProfileIdentity, PluginRoot: f.Registration.Selector, PackageID: f.Registration.ObjectID, PackageDigest: f.CanonicalDigest, ProjectionDigest: f.ProjectionDigest}, true); err != nil || result.Receipt == nil {
				t.Fatalf("fixture lacks actual native effect: %v", err)
			}
			service := localGroupService(t, root, a)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			reconciler := &observedFirstRegisterReconciler{store: store, mode: mode, cancel: cancel}
			attempt := b.NativeActivationAttempt
			if mode == "attempt" {
				attempt += "-stale"
			}
			before, err := os.ReadFile(store.Path)
			if err != nil {
				t.Fatal(err)
			}
			err = service.RecoverNativeIntent(ctx, state.Installations[0].InstallationID, b.ClientBindingID, attempt, reconciler)
			after, readErr := os.ReadFile(store.Path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			afterProfile, readErr := os.ReadFile(f.SettingsPath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !bytes.Equal(profile, afterProfile) {
				t.Fatal("recovery rewrote native bytes")
			}
			if mode != "ack" {
				if err == nil || !bytes.Equal(before, after) {
					t.Fatalf("refused recovery replaced intent/authority: %v", err)
				}
				if mode == "attempt" && reconciler.calls != 0 {
					t.Fatal("stale attempt reached callback")
				}
				return
			}
			if err != nil {
				t.Fatalf("verified unchanged first registration cannot acknowledge: %v", err)
			}
			live, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			confirmed := localOnlyBinding(t, live)
			if reconciler.effect != domain.NativeEffectUnchanged || reconciler.calls != 1 || confirmed.LocalEntryObservation == nil || !confirmed.LocalEntryObservation.Facts().Enabled || !confirmed.SelectedDelivery.OwnsProfileEntry(confirmed.NativeObjects) || confirmed.PendingNativeIntent != nil || confirmed.NativeActivationAttempt != "" {
				t.Fatal("unchanged receipt/selector acknowledgement is incomplete")
			}
			if err := service.RecoverNativeIntent(t.Context(), state.Installations[0].InstallationID, b.ClientBindingID, attempt, reconciler); err == nil || reconciler.calls != 1 {
				t.Fatal("acknowledged attempt was resent")
			}
			repeat, err := os.ReadFile(store.Path)
			if err != nil || !bytes.Equal(after, repeat) {
				t.Fatal("repeat rewrote state")
			}
		})
	}
}
