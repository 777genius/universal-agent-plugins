package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// These fixtures exercise the post-projection verification boundary with an
// inert fresh probe and real state/profile files. They do not stand in for the
// unchanged CLI tests' full native projection or actual OpenCode execution.
type materializationTestStore struct {
	path         string
	saves, loads int
	loadErr      error
}

func (s *materializationTestStore) Load() (domain.StateFileV2, error) {
	s.loads++
	if s.loadErr != nil {
		return domain.StateFileV2{}, s.loadErr
	}
	body, err := os.ReadFile(s.path)
	if err != nil {
		return domain.StateFileV2{}, err
	}
	var state domain.StateFileV2
	err = json.Unmarshal(body, &state)
	return state, err
}

func (s *materializationTestStore) Save(state domain.StateFileV2) error {
	s.saves++
	return s.write(state)
}

func (s *materializationTestStore) write(state domain.StateFileV2) error {
	body, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, body, 0600)
}

type materializationTestProbe struct {
	outcome        domain.ActivationOutcome
	err            error
	requests       []domain.ActivationRequest
	setupCalls     int
	setupAvailable bool
}

func (p *materializationTestProbe) Activate(_ context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	p.requests = append(p.requests, request)
	if !request.VerifyOnly {
		return domain.ActivationOutcome{}, errors.New("acting probe is forbidden")
	}
	return cloneMaterializationOutcome(p.outcome), p.err
}

func (*materializationTestProbe) Deactivate(context.Context, domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	return domain.DeactivationOutcome{}, errors.New("deactivation is forbidden")
}

func (*materializationTestProbe) VerifierAvailable(domain.DetectedClient, domain.DeliveryPlan, string) bool {
	return true
}
func (p *materializationTestProbe) PreparedSetupVerifierAvailable(domain.DetectedClient, domain.DeliveryPlan, string) bool {
	return p.setupAvailable
}
func (p *materializationTestProbe) CanCompletePreparedSetup(domain.ActivationRequest, domain.ActivationOutcome) bool {
	p.setupCalls++
	return true
}

type materializationTestOptIn struct {
	*materializationTestProbe
	calls    int
	request  domain.ActivationRequest
	callback func(domain.ActivationRequest, domain.ActivationOutcome) bool
}

func (*materializationTestOptIn) PreparedMaterializationVerifierAvailable(domain.DetectedClient) bool {
	return true
}

func (p *materializationTestOptIn) PreparedMaterializationVerified(request domain.ActivationRequest, observed domain.ActivationOutcome) bool {
	p.calls++
	p.request = request
	if p.callback != nil {
		return p.callback(request, observed)
	}
	return request.VerifyOnly && request.Replacing && request.DeclaredName == "reviewed" &&
		slices.Contains(observed.UserActions, "fresh inert materialization")
}

type materializationTestFixture struct {
	service Service
	input   AddInput
	plan    domain.DeliveryPlan
	binding domain.ClientBinding
	store   *materializationTestStore
	probe   *materializationTestOptIn
	profile string
}

func newMaterializationTestFixture(t *testing.T) *materializationTestFixture {
	t.Helper()
	root := t.TempDir()
	f := &materializationTestFixture{profile: filepath.Join(root, "profile")}
	client := domain.DetectedClient{ClientID: domain.ClientOpenCode, ConfigRoot: root, ExecutablePath: filepath.Join(root, "opencode"), Surfaces: []domain.ClientSurface{{ID: "original"}}}
	f.input = AddInput{Client: client, Scope: domain.ScopeUser, InstallIntent: domain.InstallIntentAutomatic, BackendExecutable: client.ExecutablePath}
	f.input.Envelope.Manifest.Name = "reviewed"
	f.plan = domain.DeliveryPlan{ClientID: client.ClientID, Scope: domain.ScopeUser, InstallIntent: domain.InstallIntentAutomatic, TargetRoot: root, ActivePath: filepath.Join(root, "package"), NativeRegistryRoot: root, TargetContext: domain.TargetContext{Scope: domain.ScopeUser, UserConfigRoot: root}, Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "local"}}, UserActions: []string{"original plan action"}}
	objects := []domain.NativeObjectOwnership{{Kind: "managed_package_directory", ObjectID: "package", Path: f.plan.ActivePath, ManagedDigest: "sha256:original"}, {Kind: "native_fixture", ObjectID: "local", Path: filepath.Join(root, "config"), ManagedDigest: "sha256:native"}}
	f.binding = domain.ClientBinding{ClientBindingID: "original-binding", ClientID: string(client.ClientID), Scope: string(domain.ScopeUser), TargetLocator: f.plan.ActivePath, NativeProfileRoot: root, Materialization: domain.MaterializationMaterialized, Activation: domain.ActivationActive, Verification: domain.VerificationInstalled, Policy: domain.PolicyAllowed, NativeObjects: slices.Clone(objects)}
	f.store = &materializationTestStore{path: filepath.Join(root, "state.json")}
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{{InstallationID: "original-installation", Clients: map[string]domain.ClientBinding{f.binding.ClientBindingID: f.binding}}}}
	if err := f.store.write(state); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.profile, []byte("original profile"), 0600); err != nil {
		t.Fatal(err)
	}
	f.probe = &materializationTestOptIn{materializationTestProbe: &materializationTestProbe{setupAvailable: true, outcome: domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, Policy: domain.PolicyAllowed, NativeEffect: domain.NativeEffectUnchanged, NativeObjects: slices.Clone(objects), UserActions: []string{"fresh inert materialization"}, LocalActions: []string{"original local action"}}}}
	f.service = Service{StateStore: f.store, profileCheck: f.checkProfile}
	f.service.Activator = profileActivator{ClientActivator: f.probe, check: f.checkProfile}
	return f
}

func (f *materializationTestFixture) checkProfile() error {
	body, err := os.ReadFile(f.profile)
	if err != nil {
		return err
	}
	if string(body) != "original profile" {
		return errors.New("original profile changed")
	}
	return nil
}

func (f *materializationTestFixture) inspect(ctx context.Context) (bool, error) {
	return f.service.inspectOwnedVerification(ctx, f.input, f.plan, f.binding, true)
}

func materializationTestBytes(t *testing.T, paths ...string) [][]byte {
	t.Helper()
	bodies := make([][]byte, 0, len(paths))
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		bodies = append(bodies, body)
	}
	return bodies
}

func TestOwnedPreparedMaterializationUsesFreshReadOnlyPacket(t *testing.T) {
	f := newMaterializationTestFixture(t)
	before := materializationTestBytes(t, f.store.path, f.profile)
	complete, err := f.inspect(t.Context())
	if err != nil || !complete || len(f.probe.requests) != 1 || f.probe.calls != 1 || f.probe.setupCalls != 0 {
		t.Fatalf("fresh materialization was not proved: complete=%v probe=%d classifier=%d err=%v", complete, len(f.probe.requests), f.probe.calls, err)
	}
	if !reflect.DeepEqual(f.probe.request, f.probe.requests[0]) || !f.probe.request.VerifyOnly {
		t.Fatalf("classifier did not receive the exact read-only verifier authority: %+v", f.probe.request)
	}
	if f.store.saves != 0 || !reflect.DeepEqual(before, materializationTestBytes(t, f.store.path, f.profile)) {
		t.Fatal("materialization proof persisted readiness or changed original files")
	}
}

func TestOwnedPreparedMaterializationKeepsActiveAndExplicitOptInBoundaries(t *testing.T) {
	for _, variant := range []string{"active", "setup only", "stale active", "stored guidance"} {
		t.Run(variant, func(t *testing.T) {
			f := newMaterializationTestFixture(t)
			want := variant == "active"
			switch variant {
			case "active":
				f.probe.outcome.Activation = domain.ActivationActive
			case "setup only":
				f.service.Activator = profileActivator{ClientActivator: f.probe.materializationTestProbe, check: f.checkProfile}
			case "stale active", "stored guidance":
				f.probe.outcome.UserActions = nil
				if variant == "stale active" {
					f.probe.outcome.Verification = domain.VerificationFailed
				}
				f.plan.UserActions = []string{"fresh inert materialization"}
			}
			complete, err := f.inspect(t.Context())
			if err != nil || complete != want || f.probe.setupCalls != 0 || len(f.probe.requests) != 1 {
				t.Fatalf("wrong existing readiness boundary: complete=%v want=%v err=%v", complete, want, err)
			}
			if variant != "stored guidance" && f.probe.calls != 0 {
				t.Fatal("ineligible outcome invoked materialization classifier")
			}
		})
	}
}

func TestOwnedPreparedMaterializationRejectsIncompleteEvidence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*domain.ClientBinding, *domain.ActivationOutcome)
	}{
		{"not materialized", func(b *domain.ClientBinding, _ *domain.ActivationOutcome) {
			b.Materialization = domain.MaterializationAbsent
		}},
		{"pending attempt", func(b *domain.ClientBinding, _ *domain.ActivationOutcome) { b.NativeActivationAttempt = "unsettled" }},
		{"pending intent", func(b *domain.ClientBinding, _ *domain.ActivationOutcome) {
			b.PendingNativeIntent = &domain.PendingNativeIntent{}
		}},
		{"missing objects", func(_ *domain.ClientBinding, o *domain.ActivationOutcome) { o.NativeObjects = nil }},
		{"changed object", func(_ *domain.ClientBinding, o *domain.ActivationOutcome) { o.NativeObjects[0].Path += "-foreign" }},
		{"missing effect", func(_ *domain.ClientBinding, o *domain.ActivationOutcome) { o.NativeEffect = "" }},
		{"committed", func(_ *domain.ClientBinding, o *domain.ActivationOutcome) {
			o.NativeEffect = domain.NativeEffectCommitted
		}},
		{"uncertain", func(_ *domain.ClientBinding, o *domain.ActivationOutcome) {
			o.NativeEffect = domain.NativeEffectUncertain
		}},
		{"unverified", func(_ *domain.ClientBinding, o *domain.ActivationOutcome) { o.Verification = domain.VerificationNotRun }},
		{"policy unknown", func(_ *domain.ClientBinding, o *domain.ActivationOutcome) { o.Policy = "" }},
		{"policy blocked", func(_ *domain.ClientBinding, o *domain.ActivationOutcome) { o.Policy = domain.PolicyBlocked }},
		{"activation attested", func(_ *domain.ClientBinding, o *domain.ActivationOutcome) { o.ActivationAttested = true }},
		{"authentication attested", func(_ *domain.ClientBinding, o *domain.ActivationOutcome) { o.AuthenticationAttested = true }},
		{"authoritative negative", func(_ *domain.ClientBinding, o *domain.ActivationOutcome) { o.AuthoritativeObservation = true }},
		{"local receipt", func(_ *domain.ClientBinding, o *domain.ActivationOutcome) {
			o.LocalEntryObservation = &domain.LocalEntryObservation{}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newMaterializationTestFixture(t)
			tc.change(&f.binding, &f.probe.outcome)
			if preparedMaterializationCandidate(f.binding, f.probe.outcome) {
				t.Fatal("incomplete evidence passed the pure candidate predicate")
			}
			changeMaterializationTestState(t, f, func(state *domain.StateFileV2) { state.Installations[0].Clients[f.binding.ClientBindingID] = f.binding })
			complete, _ := f.inspect(t.Context())
			if complete || f.probe.calls != 0 {
				t.Fatal("incomplete evidence reached materialization classifier")
			}
		})
	}
}

func TestOwnedPreparedMaterializationPreservesUnclosedProbeCause(t *testing.T) {
	f := newMaterializationTestFixture(t)
	cause := errors.New("inert original close failure")
	f.probe.err = &domain.UnclosedClientProbe{Operation: "inert observation", Err: cause}
	complete, err := f.inspect(t.Context())
	if complete || !errors.Is(err, cause) || !domain.IsUnclosedClientProbe(err) || f.probe.calls != 0 {
		t.Fatalf("native failure was classified or lost: complete=%v err=%v", complete, err)
	}
}

func TestOwnedPreparedMaterializationDetachesClassifierInputs(t *testing.T) {
	f := newMaterializationTestFixture(t)
	before := materializationTestBytes(t, f.store.path, f.profile)
	f.probe.callback = func(request domain.ActivationRequest, observed domain.ActivationOutcome) bool {
		request.Client.Surfaces[0].ID = "changed"
		request.Plan.Components[0].Name = "changed"
		request.Plan.UserActions[0] = "changed"
		request.Plan.PreviousNativeObjects[0].Path = "changed"
		request.Delivery.NativeObjects[0].Path = "changed"
		request.PreviousNativeObjects[0].Path = "changed"
		observed.NativeObjects[0].Path = "changed"
		observed.UserActions[0] = "changed"
		observed.LocalActions[0] = "changed"
		return true
	}
	complete, err := f.inspect(t.Context())
	if err != nil || !complete {
		t.Fatalf("detached callback changed authority: %v", err)
	}
	if f.input.Client.Surfaces[0].ID != "original" || f.plan.Components[0].Name != "local" || f.plan.UserActions[0] != "original plan action" ||
		f.binding.NativeObjects[0].Path != f.plan.ActivePath || f.probe.outcome.NativeObjects[0].Path != f.plan.ActivePath ||
		f.probe.outcome.UserActions[0] != "fresh inert materialization" || f.probe.outcome.LocalActions[0] != "original local action" ||
		!reflect.DeepEqual(before, materializationTestBytes(t, f.store.path, f.profile)) || f.store.saves != 0 {
		t.Fatal("classifier mutated original packet or retained state")
	}
}

func changeMaterializationTestState(t *testing.T, f *materializationTestFixture, change func(*domain.StateFileV2)) {
	t.Helper()
	state, err := f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	change(&state)
	if err := f.store.write(state); err != nil {
		t.Fatal(err)
	}
}

func TestOwnedPreparedMaterializationRejectsClassifierStateProfileOrContextDrift(t *testing.T) {
	for _, variant := range []string{"binding", "other state", "profile", "cancel"} {
		t.Run(variant, func(t *testing.T) {
			f := newMaterializationTestFixture(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			f.probe.callback = func(domain.ActivationRequest, domain.ActivationOutcome) bool {
				switch variant {
				case "binding":
					changeMaterializationTestState(t, f, func(state *domain.StateFileV2) {
						b := state.Installations[0].Clients[f.binding.ClientBindingID]
						b.UpdatedAt = "changed during callback"
						state.Installations[0].Clients[b.ClientBindingID] = b
					})
				case "other state":
					changeMaterializationTestState(t, f, func(state *domain.StateFileV2) { state.Installations[0].UpdatedAt = "changed during callback" })
				case "profile":
					if err := os.WriteFile(f.profile, []byte("changed during callback"), 0600); err != nil {
						t.Fatal(err)
					}
				case "cancel":
					cancel()
				}
				return true
			}
			complete, err := f.inspect(ctx)
			if complete || err == nil || f.probe.calls != 1 || f.store.saves != 0 {
				t.Fatalf("changed callback authority admitted: complete=%v err=%v calls=%d", complete, err, f.probe.calls)
			}
			if variant == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatalf("lost cancellation: %v", err)
			}
		})
	}
}

func TestOwnedPreparedMaterializationRequiresUniqueOriginalBinding(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		t.Run(fmt.Sprint(duplicate), func(t *testing.T) {
			f := newMaterializationTestFixture(t)
			changeMaterializationTestState(t, f, func(state *domain.StateFileV2) {
				if duplicate {
					state.Installations = append(state.Installations, state.Installations[0])
				} else {
					delete(state.Installations[0].Clients, f.binding.ClientBindingID)
				}
			})
			complete, err := f.inspect(t.Context())
			if complete || err == nil || f.probe.calls != 0 {
				t.Fatalf("lost original binding accepted: %v", err)
			}
		})
	}
}

func TestOwnedPreparedMaterializationNonOptInAddsNoStateReads(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(fmt.Sprint(missing), func(t *testing.T) {
			f := newMaterializationTestFixture(t)
			f.probe.setupAvailable = false
			f.service.Activator = profileActivator{ClientActivator: f.probe.materializationTestProbe, check: f.checkProfile}
			f.store.loadErr = errors.New("state must not be read for non-opt-in fallback")
			if missing {
				f.service.StateStore = nil
			}
			complete, err := f.inspect(t.Context())
			if complete || err != nil || f.store.loads != 0 || len(f.probe.requests) != 1 || f.probe.calls != 0 {
				t.Fatalf("non-opt-in fallback changed: complete=%v err=%v reads=%d", complete, err, f.store.loads)
			}
		})
	}
}

type materializationAliasingStore struct{ state domain.StateFileV2 }

func (s *materializationAliasingStore) Load() (domain.StateFileV2, error) { return s.state, nil }
func (*materializationAliasingStore) Save(domain.StateFileV2) error {
	return errors.New("read-only inspection attempted state write")
}

func TestOwnedPreparedMaterializationFreezesAliasingStore(t *testing.T) {
	for _, variant := range []string{"native object", "other installation", "receipt pointer"} {
		t.Run(variant, func(t *testing.T) {
			f := newMaterializationTestFixture(t)
			state, err := f.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			state.TransactionReceipts = []domain.MutationReceipt{{EnvironmentAuthority: &domain.EnvironmentAuthority{EnvironmentID: "original"}}}
			alias := &materializationAliasingStore{state: state}
			f.service.StateStore = alias
			f.probe.callback = func(domain.ActivationRequest, domain.ActivationOutcome) bool {
				switch variant {
				case "native object":
					alias.state.Installations[0].Clients[f.binding.ClientBindingID].NativeObjects[0].Path += "-changed"
				case "other installation":
					alias.state.Installations[0].UpdatedAt = "changed in place"
				case "receipt pointer":
					alias.state.TransactionReceipts[0].EnvironmentAuthority.EnvironmentID = "changed in place"
				}
				return true
			}
			complete, err := f.inspect(t.Context())
			if complete || err == nil || f.probe.calls != 1 {
				t.Fatalf("aliasing store changed original comparison basis: complete=%v err=%v", complete, err)
			}
		})
	}
}

func TestOwnedBindingSeparatesReadinessFromMaterialization(t *testing.T) {
	// The opted-in verifier revalidates the original binding before and after Activate.
	const verifierStateReads = 2
	for _, tc := range []struct {
		name                string
		materializationOnly bool
		activation          domain.ActivationState
		want                bool
		classifications     int
		classifierReads     int
	}{
		{"prepared is not readiness", false, domain.ActivationPrepared, false, 0, 0},
		{"prepared materialization", true, domain.ActivationPrepared, true, 1, 2},
		{"active readiness", false, domain.ActivationActive, true, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newMaterializationTestFixture(t)
			f.probe.outcome.Activation = tc.activation
			before := materializationTestBytes(t, f.store.path, f.profile)
			complete, err := f.service.inspectOwnedVerification(t.Context(), f.input, f.plan, f.binding, tc.materializationOnly)
			if err != nil || complete != tc.want || len(f.probe.requests) != 1 || f.probe.calls != tc.classifications || f.probe.setupCalls != 0 {
				t.Fatalf("readiness/materialization boundary changed: complete=%v want=%v probes=%d classifiers=%d setup=%d err=%v", complete, tc.want, len(f.probe.requests), f.probe.calls, f.probe.setupCalls, err)
			}
			wantReads := verifierStateReads + tc.classifierReads
			if f.store.loads != wantReads || f.store.saves != 0 || !reflect.DeepEqual(before, materializationTestBytes(t, f.store.path, f.profile)) {
				t.Fatalf("observation changed retained state or access: reads=%d want=%d writes=%d", f.store.loads, wantReads, f.store.saves)
			}
		})
	}
}
