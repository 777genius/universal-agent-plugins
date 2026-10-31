package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type preparedFileStore struct {
	state domain.StateFileV2
	saves int
	fail  error
}

func (s *preparedFileStore) Load() (domain.StateFileV2, error) {
	out := s.state
	out.Installations = append([]domain.Installation(nil), out.Installations...)
	for i := range out.Installations {
		out.Installations[i].Clients = map[string]domain.ClientBinding{}
		for key, b := range s.state.Installations[i].Clients {
			b.NativeObjects = append([]domain.NativeObjectOwnership(nil), b.NativeObjects...)
			out.Installations[i].Clients[key] = b
		}
	}
	return out, nil
}
func (s *preparedFileStore) Save(state domain.StateFileV2) error {
	if s.fail != nil {
		return s.fail
	}
	s.saves++
	s.state = state
	return nil
}

type preparedFileActivator struct {
	observedActivator
	available        bool
	requests         []domain.ActivationRequest
	marker           string
	capabilityCalls  int
	mutateCapability func(domain.DeliveryPlan)
	mutateActivation func(*domain.ActivationRequest)
}

func (a *preparedFileActivator) VerifierAvailable(_ domain.DetectedClient, plan domain.DeliveryPlan, _ string) bool {
	a.capabilityCalls++
	if a.mutateCapability != nil {
		a.mutateCapability(plan)
	}
	return a.available
}
func (a *preparedFileActivator) Activate(_ context.Context, r domain.ActivationRequest) (domain.ActivationOutcome, error) {
	a.requests = append(a.requests, r)
	if a.mutateActivation != nil {
		a.mutateActivation(&r)
	}
	if a.marker != "" {
		if err := os.WriteFile(a.marker, []byte("actual inert effect"), 0600); err != nil {
			return domain.ActivationOutcome{}, err
		}
	}
	return a.outcome, a.err
}

type preparedFileFixture struct {
	service  Service
	input    AddInput
	plan     domain.DeliveryPlan
	binding  domain.ClientBinding
	delivery domain.StagedDelivery
	store    *preparedFileStore
	actor    *preparedFileActivator
	id       string
}

func newPreparedFileFixture(t *testing.T) *preparedFileFixture {
	t.Helper()
	base := t.TempDir()
	project := filepath.Join(base, "A")
	root := filepath.Join(project, ".uap", "managed", "clients", "qoder")
	active := filepath.Join(root, "demo-physical")
	f := &preparedFileFixture{id: "prepared-file-install"}
	f.input = AddInput{Client: domain.DetectedClient{ClientID: domain.ClientQoder, ConfigRoot: filepath.Join(base, "user")}, Scope: domain.ScopeProject, InstallationID: f.id, Confirmed: true}
	f.plan = domain.DeliveryPlan{ClientID: domain.ClientQoder, Scope: domain.ScopeProject, DeclaredName: "demo", PhysicalArtifactID: "demo-physical", TargetRoot: root, ActivePath: active, NativeRegistryRoot: root, TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project, UserConfigRoot: f.input.Client.ConfigRoot}}
	object := domain.NativeObjectOwnership{Kind: "managed_package_directory", ObjectID: "package:qoder:demo-physical", LogicalName: "demo", Path: active, ManagedDigest: "sha256:recorded"}
	f.binding = domain.ClientBinding{ClientBindingID: domain.ComputeClientBindingID(f.id, "qoder", "project", active), ClientID: "qoder", Scope: "project", PhysicalArtifact: "demo-physical", TargetLocator: active, NativeProfileRoot: root, Materialization: domain.MaterializationMaterialized, Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, Authentication: domain.AuthenticationNotChecked, Policy: domain.PolicyAllowed, NativeObjects: []domain.NativeObjectOwnership{object}}
	f.delivery = domain.StagedDelivery{ClientID: domain.ClientQoder, OwnedBase: root, ActivePath: active, ArtifactDigest: object.ManagedDigest, NativeObjects: append([]domain.NativeObjectOwnership(nil), f.binding.NativeObjects...)}
	f.actor = &preparedFileActivator{available: true, observedActivator: observedActivator{outcome: domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, Authentication: domain.AuthenticationNotChecked, Policy: domain.PolicyAllowed, NativeEffect: domain.NativeEffectUnchanged, UserActions: []string{"review the independent native import"}}}}
	f.store = &preparedFileStore{state: domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{{InstallationID: f.id, Clients: map[string]domain.ClientBinding{f.binding.ClientBindingID: f.binding}}}}}
	f.service = Service{ProjectRoot: project, Activator: f.actor, StateStore: f.store, Now: func() time.Time { return time.Unix(10, 0) }}
	return f
}
func (f *preparedFileFixture) session(post bool) *repairSession {
	state, _ := f.store.Load()
	binding := state.Installations[0].Clients[f.binding.ClientBindingID]
	if post {
		binding.Verification = domain.VerificationPackageValid
		state.Installations[0].Clients[binding.ClientBindingID] = binding
		f.store.state = state
	}
	return &repairSession{service: f.service, ctx: context.Background(), input: f.input, plan: f.plan, result: AddResult{InstallationID: f.id, Plan: f.plan, Mutated: post}, state: state, index: 0, installation: state.Installations[0], clientKey: binding.ClientBindingID, client: binding}
}
func TestPreparedProjectFileVerifierUsesExactFreshReadOnlyDelivery(t *testing.T) {
	for _, post := range []bool{false, true} {
		t.Run(fmt.Sprint(post), func(t *testing.T) {
			f := newPreparedFileFixture(t)
			session := f.session(post)
			var out AddResult
			var err error
			if post {
				out, err = session.verifyRepairedProjectFiles(f.delivery)
			} else {
				observed, verifyErr := f.service.verifyClientReadOnly(t.Context(), f.input, session.result, f.binding)
				if verifyErr != nil {
					t.Fatal(verifyErr)
				}
				out, err = session.correctIntactLifecycle(observed)
			}
			if err != nil || out.Mutated != post || out.NoChange == post || out.Activation.Activation != domain.ActivationPrepared || out.Activation.Verification != domain.VerificationInstalled {
				t.Fatalf("file result %+v %v", out, err)
			}
			if len(f.actor.requests) != 1 {
				t.Fatal("no fresh verifier")
			}
			r := f.actor.requests[0]
			if !r.VerifyOnly || !reflect.DeepEqual(r.Client, f.input.Client) || !reflect.DeepEqual(r.Plan, f.plan) || !reflect.DeepEqual(r.Delivery, f.delivery) || !reflect.DeepEqual(r.PreviousNativeObjects, f.binding.NativeObjects) {
				t.Fatalf("verifier lost exact authority %+v", r)
			}
			state, _ := f.store.Load()
			saved := state.Installations[0].Clients[f.binding.ClientBindingID]
			if saved.Verification != domain.VerificationInstalled || saved.NativeActivationAttempt != "" || saved.PendingNativeIntent != nil || !reflect.DeepEqual(saved.NativeObjects, f.binding.NativeObjects) {
				t.Fatalf("file proof became native ownership %+v", saved)
			}
		})
	}
}
func TestPreparedProjectFileVerifierRejectsContradictoryOutcomesWithoutAdoption(t *testing.T) {
	for _, post := range []bool{false, true} {
		for _, kind := range []string{"committed", "committed-error", "uncertain", "unknown-effect", "objects", "local-observation", "activation-attested", "auth-attested", "authoritative", "no-actions", "active", "unverified", "blocked", "auth-change", "error"} {
			t.Run(fmt.Sprintf("post%t/%s", post, kind), func(t *testing.T) {
				f := newPreparedFileFixture(t)
				session := f.session(post)
				sentinel := errors.New("inert verifier failure")
				switch kind {
				case "committed", "committed-error":
					f.actor.outcome.NativeEffect = domain.NativeEffectCommitted
					f.actor.marker = filepath.Join(t.TempDir(), "inert-effect")
					if kind == "committed-error" {
						f.actor.err = sentinel
					}
				case "uncertain":
					f.actor.outcome.NativeEffect = domain.NativeEffectUncertain
				case "unknown-effect":
					f.actor.outcome.NativeEffect = "future"
				case "objects":
					f.actor.outcome.NativeObjects = []domain.NativeObjectOwnership{{Kind: "foreign", Path: "outside"}}
				case "local-observation":
					f.actor.outcome.LocalEntryObservation = &domain.LocalEntryObservation{}
				case "activation-attested":
					f.actor.outcome.ActivationAttested = true
				case "auth-attested":
					f.actor.outcome.AuthenticationAttested = true
				case "authoritative":
					f.actor.outcome.AuthoritativeObservation = true
				case "no-actions":
					f.actor.outcome.UserActions = nil
				case "active":
					f.actor.outcome.Activation = domain.ActivationActive
				case "unverified":
					f.actor.outcome.Verification = domain.VerificationPackageValid
				case "blocked":
					f.actor.outcome.Policy = domain.PolicyBlocked
				case "auth-change":
					f.actor.outcome.Authentication = domain.AuthenticationComplete
				case "error":
					f.actor.err = sentinel
				}
				before, _ := json.Marshal(f.store.state)
				var out AddResult
				var err error
				if post {
					out, err = session.verifyRepairedProjectFiles(f.delivery)
				} else {
					out, err = session.repairNative()
				}
				if err == nil || out.NoChange || out.Mutated != (post || kind == "committed" || kind == "committed-error") || len(f.actor.requests) != 1 || !f.actor.requests[0].VerifyOnly {
					t.Fatalf("contradiction hidden %+v %v calls%d", out, err, len(f.actor.requests))
				}
				if f.actor.err != nil && !errors.Is(err, sentinel) {
					t.Fatalf("cause lost %v", err)
				}
				if out.Activation.NativeEffect != f.actor.outcome.NativeEffect {
					t.Fatal("reported effect rewritten")
				}
				after, _ := json.Marshal(f.store.state)
				if string(before) != string(after) || f.store.saves != 0 {
					t.Fatal("contradictory outcome changed saved facts/receipts")
				}
				if f.actor.marker != "" {
					if body, err := os.ReadFile(f.actor.marker); err != nil || string(body) != "actual inert effect" {
						t.Fatal("committed claim lacks actual fixture effect", err)
					}
				}
			})
		}
	}
}
func TestPreparedProjectFileVerifierRefusesMalformedAuthorityBeforeCapability(t *testing.T) {
	for _, kind := range []string{"input-scope", "plan-scope", "context-scope", "context-root", "configured-root", "user-root", "client", "selection", "binding-client", "binding-scope", "binding-id", "binding-path", "binding-physical", "binding-profile", "binding-activation", "attempt", "intent", "receipt-digest", "receipt-path", "extra-object", "delivery-root"} {
		t.Run(kind, func(t *testing.T) {
			f := newPreparedFileFixture(t)
			switch kind {
			case "input-scope":
				f.input.Scope = domain.ScopeUser
			case "plan-scope":
				f.plan.Scope = domain.ScopeUser
			case "context-scope":
				f.plan.TargetContext.Scope = domain.ScopeUser
			case "context-root":
				f.plan.TargetContext.ProjectRoot = ""
			case "configured-root":
				f.service.ProjectRoot += "-other"
			case "user-root":
				f.plan.TargetContext.UserConfigRoot += "-other"
			case "client":
				f.plan.ClientID = domain.ClientGrok
			case "selection":
				if err := json.Unmarshal([]byte(`{"mode":"unknown-project-test"}`), &f.plan.SelectedDelivery); err != nil {
					t.Fatal(err)
				}
			case "binding-client":
				f.binding.ClientID = "grok"
			case "binding-scope":
				f.binding.Scope = "user"
			case "binding-id":
				f.binding.ClientBindingID = "other"
			case "binding-path":
				f.binding.TargetLocator += "-other"
			case "binding-physical":
				f.binding.PhysicalArtifact = "other"
			case "binding-profile":
				f.binding.NativeProfileRoot += "-other"
			case "binding-activation":
				f.binding.Activation = domain.ActivationActive
			case "attempt":
				f.binding.NativeActivationAttempt = "unacknowledged"
			case "intent":
				f.binding.PendingNativeIntent = &domain.PendingNativeIntent{}
			case "receipt-digest":
				f.delivery.ArtifactDigest = "sha256:foreign"
			case "receipt-path":
				f.delivery.NativeObjects[0].Path = "foreign"
			case "extra-object":
				f.delivery.NativeObjects = append(f.delivery.NativeObjects, domain.NativeObjectOwnership{Kind: "foreign"})
			case "delivery-root":
				f.delivery.OwnedBase += "-other"
			}
			if !preparedProjectFileCandidate(f.input, f.plan) {
				t.Fatal("malformed explicit project escaped candidate gate")
			}
			out, err := f.service.observePreparedProjectFiles(t.Context(), f.input, f.plan, f.id, f.binding, f.delivery)
			var refusal *preparedProjectVerificationError
			if err == nil || !errors.As(err, &refusal) || f.actor.capabilityCalls != 0 || len(f.actor.requests) != 0 || out.Activation != "" {
				t.Fatalf("malformed authority reached fallback/verifier %+v %v", out, err)
			}
		})
	}
}
func TestPreparedProjectFileVerifierUnavailableNeverCallsInheritedGate(t *testing.T) {
	f := newPreparedFileFixture(t)
	f.actor.available = false
	f.input.BackendExecutable = "/inert/qoder-static-auth-hint-must-not-enable"
	out, err := f.service.verifyClientReadOnly(t.Context(), f.input, AddResult{InstallationID: f.id, Plan: f.plan}, f.binding)
	if err != nil || out.Activation != "" || len(f.actor.requests) != 0 || f.actor.capabilityCalls != 1 {
		t.Fatalf("unavailable verifier executed %+v %v", out, err)
	}
	f.service.Activator = &observedActivator{}
	out, err = f.service.verifyClientReadOnly(t.Context(), f.input, AddResult{InstallationID: f.id, Plan: f.plan}, f.binding)
	if err != nil || out.Activation != "" {
		t.Fatalf("missing capability enabled verifier %+v %v", out, err)
	}
	for _, id := range []domain.ClientID{domain.ClientClaude, domain.ClientCodex} {
		f.input.Client.ClientID = id
		f.plan.ClientID = id
		if preparedProjectFileCandidate(f.input, f.plan) {
			t.Fatalf("other lifecycle borrowed prepared contract %s", id)
		}
	}
	f = newPreparedFileFixture(t)
	f.input.Scope = domain.ScopeUser
	f.plan.Scope = domain.ScopeUser
	f.plan.TargetContext = domain.TargetContext{Scope: domain.ScopeUser, UserConfigRoot: f.input.Client.ConfigRoot}
	if preparedProjectFileCandidate(f.input, f.plan) {
		t.Fatal("user route changed")
	}
}
func TestPreparedProjectFilePostCommitPersistenceFailureRetainsMutation(t *testing.T) {
	f := newPreparedFileFixture(t)
	session := f.session(true)
	sentinel := errors.New("inert persistence failure")
	f.store.fail = sentinel
	before, _ := json.Marshal(f.store.state)
	out, err := session.verifyRepairedProjectFiles(f.delivery)
	if !errors.Is(err, sentinel) || !out.Mutated || out.NoChange || len(f.actor.requests) != 1 {
		t.Fatalf("postcommit error hid mutation %+v %v", out, err)
	}
	after, _ := json.Marshal(f.store.state)
	if string(before) != string(after) {
		t.Fatal("failed persistence fabricated verification state")
	}
}

func TestPreparedProjectFileFailureCannotBecomeAuthoritativePreviewPersistence(t *testing.T) {
	f := newPreparedFileFixture(t)
	f.actor.outcome.NativeEffect = domain.NativeEffectCommitted
	f.actor.outcome.AuthoritativeObservation = true
	f.actor.marker = filepath.Join(t.TempDir(), "preview-effect")
	session := &applySession{service: f.service, ctx: t.Context(), input: f.input, result: AddResult{InstallationID: f.id, Plan: f.plan}, installationID: f.id, clientBindingID: f.binding.ClientBindingID}
	before, _ := json.Marshal(f.store.state)
	_, out, err := session.persistReadOnlyObservation(f.binding)
	if err == nil || !out.Mutated || out.NoChange || out.Activation.NativeEffect != domain.NativeEffectCommitted || len(f.actor.requests) != 1 {
		t.Fatalf("preview hid actual effect: %+v %v", out, err)
	}
	after, _ := json.Marshal(f.store.state)
	if string(before) != string(after) || f.store.saves != 0 {
		t.Fatal("invalid file-only outcome became authoritative saved evidence")
	}
}

func TestPreparedProjectFileVerifierCannotMutateFrozenOwnershipPacket(t *testing.T) {
	for _, post := range []bool{false, true} {
		for _, boundary := range []string{"capability", "activation"} {
			t.Run(fmt.Sprintf("post%t/%s", post, boundary), func(t *testing.T) {
				f := newPreparedFileFixture(t)
				session := f.session(post)
				if !post {
					session.client.Verification = domain.VerificationPackageValid
				}
				session.plan.PreviousNativeObjects = session.client.NativeObjects
				session.result.Plan = session.plan
				beforeObjects := append([]domain.NativeObjectOwnership(nil), session.client.NativeObjects...)
				mutate := func(objects []domain.NativeObjectOwnership) {
					objects[0].ManagedDigest = "sha256:unowned-mutated-input"
					objects[0].Path = "foreign-input-path"
				}
				if boundary == "capability" {
					f.actor.mutateCapability = func(plan domain.DeliveryPlan) { mutate(plan.PreviousNativeObjects) }
				} else {
					f.actor.mutateActivation = func(request *domain.ActivationRequest) {
						mutate(request.Delivery.NativeObjects)
						mutate(request.Plan.PreviousNativeObjects)
						mutate(request.PreviousNativeObjects)
					}
				}
				var out AddResult
				var err error
				if post {
					out, err = session.verifyRepairedProjectFiles(f.delivery)
				} else {
					var observed domain.ActivationOutcome
					observed, err = f.service.verifyClientReadOnly(t.Context(), f.input, session.result, session.client)
					if err == nil {
						out, err = session.correctIntactLifecycle(observed)
					}
				}
				if err != nil || out.Activation.Verification != domain.VerificationInstalled {
					t.Fatalf("valid observer outcome failed: %+v %v", out, err)
				}
				if boundary == "capability" && (len(f.actor.requests) != 1 || !reflect.DeepEqual(f.actor.requests[0].Plan.PreviousNativeObjects, beforeObjects) || !reflect.DeepEqual(f.actor.requests[0].Delivery.NativeObjects, beforeObjects)) {
					t.Fatal("classifier mutation contaminated activation packet")
				}
				state, _ := f.store.Load()
				if !reflect.DeepEqual(session.client.NativeObjects, beforeObjects) || !reflect.DeepEqual(session.plan.PreviousNativeObjects, beforeObjects) || !reflect.DeepEqual(state.Installations[0].Clients[f.binding.ClientBindingID].NativeObjects, beforeObjects) {
					t.Fatal("external verifier mutated or persisted unowned input receipt")
				}
			})
		}
	}
}
