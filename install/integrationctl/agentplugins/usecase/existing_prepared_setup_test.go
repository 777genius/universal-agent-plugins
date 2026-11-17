package usecase

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/plannertest"
)

type existingSetupActor struct {
	needsSetup, allow bool
	observed          domain.ActivationOutcome
	observeErr        error
	marker            string
	reads, acts       int
	onRead            func()
	// settled, when set, is the saved lifecycle that an acting pass re-observes
	// without any effect, like an intact native projection.
	settled *domain.ActivationOutcome
}

func (a *existingSetupActor) VerifierAvailable(domain.DetectedClient, domain.DeliveryPlan, string) bool {
	return true
}
func (a *existingSetupActor) PreparedSetupVerifierAvailable(domain.DetectedClient, domain.DeliveryPlan, string) bool {
	return true
}
func (a *existingSetupActor) AutomaticallyActivates(domain.ActivationRequest) bool { return true }
func (a *existingSetupActor) CanCompletePreparedSetup(r domain.ActivationRequest, _ domain.ActivationOutcome) bool {
	return a.allow && r.Plan.Scope == domain.ScopeProject && r.Delivery.ActivePath == r.Plan.ActivePath && len(r.PreviousNativeObjects) > 0
}
func (a *existingSetupActor) Activate(_ context.Context, r domain.ActivationRequest) (domain.ActivationOutcome, error) {
	effect := domain.NativeEffectUnchanged
	var objects []domain.NativeObjectOwnership
	if r.VerifyOnly {
		a.reads++
		if a.onRead != nil {
			a.onRead()
		}
		if a.needsSetup {
			return a.observed, a.observeErr
		}
	} else {
		a.acts++
		if a.settled != nil && a.needsSetup {
			outcome := *a.settled
			outcome.NativeEffect = domain.NativeEffectUnchanged
			return outcome, nil
		}
		effect = domain.NativeEffectCommitted
		objects = append([]domain.NativeObjectOwnership(nil), r.Delivery.NativeObjects...)
		if a.needsSetup {
			if err := os.WriteFile(a.marker, []byte("exact owned setup"), 0600); err != nil {
				return domain.ActivationOutcome{}, err
			}
			a.needsSetup = false
		}
	}
	return domain.ActivationOutcome{Activation: domain.ActivationActive, Verification: domain.VerificationInstalled, Authentication: domain.AuthenticationNotRequired, Policy: domain.PolicyAllowed, NativeEffect: effect, NativeObjects: objects}, nil
}
func (*existingSetupActor) Deactivate(context.Context, domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	panic("unused")
}

// A legacy read-only verifier does not implicitly opt into the new setup path.
type existingLegacyVerifier struct{ inner *existingSetupActor }

func (a existingLegacyVerifier) VerifierAvailable(domain.DetectedClient, domain.DeliveryPlan, string) bool {
	return true
}
func (a existingLegacyVerifier) AutomaticallyActivates(domain.ActivationRequest) bool { return true }
func (a existingLegacyVerifier) Activate(ctx context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	return a.inner.Activate(ctx, request)
}
func (a existingLegacyVerifier) Deactivate(ctx context.Context, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	return a.inner.Deactivate(ctx, request)
}

// existingProjectSetupFixture returns a project-scoped service and confirmed
// Claude add input whose fresh verifier observes a completable Prepared setup.
func existingProjectSetupFixture(t *testing.T, source string) (Service, statev2.Store, AddInput, domain.ActivationOutcome) {
	t.Helper()
	service, store, _ := serviceFixture(t)
	project := t.TempDir()
	service.ProjectRoot = project
	store.Path, store.ProjectRoot = filepath.Join(project, ".uap", "state-v2.json"), project
	service.StateStore, service.Kernel.StateStore = store, store
	service.CheckNamespace = func() error { return service.Paths.RequireContainedChild(project, store.Path) }
	planner := plannertest.NewPlanner(clientplanner.Planner{ProjectRoot: project, ManagedRoot: filepath.Join(project, ".uap", "managed")})
	service.Planner, service.Targets = planner, planner
	client := domain.DetectedClient{ClientID: domain.ClientClaude, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "profile"), ExecutablePath: "/TEST/claude"}
	input := addInput(t, client, source)
	input.Scope = domain.ScopeProject
	input.BackendExecutable = client.ExecutablePath
	input.Confirmed = true
	observed := domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, Authentication: domain.AuthenticationNotChecked, Policy: domain.PolicyAllowed, NativeEffect: domain.NativeEffectUnchanged}
	return service, store, input, observed
}

// Reproduces an older CLI's saved Active marketplace binding after exact setup
// ceases to be ready. Only fresh scoped verification plus adapter opt-in may
// authorize normal confirmed add/update to finish setup on the intact package.
func TestExistingScopedVerifierCompletesPreparedSetupThroughNormalApply(t *testing.T) {
	for _, mode := range []string{"confirmed-add", "confirmed-update", "prepared-binding", "preview", "dry-run", "online-preview", "declined", "denied", "unclosed", "no-opt-in", "binding-changed"} {
		t.Run(mode, func(t *testing.T) {
			service, store, input, observed := existingProjectSetupFixture(t, "https://example.invalid/existing-scoped-setup")
			client := input.Client
			actor := &existingSetupActor{allow: true, marker: filepath.Join(t.TempDir(), "setup-effect"), observed: observed}
			service.Activator = actor
			added, err := service.Add(t.Context(), input)
			if err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(added.Plan.ActivePath)
			if err != nil {
				t.Fatal(err)
			}
			before, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			saved := onlyBinding(before.Installations[0])
			actor.needsSetup = true
			actor.reads = 0
			actor.acts = 0
			sentinel := errors.New("unclosed native probe")
			switch mode {
			case "prepared-binding":
				for key, b := range before.Installations[0].Clients {
					b.Activation = domain.ActivationPrepared
					before.Installations[0].Clients[key] = b
				}
				if err := store.Save(before); err != nil {
					t.Fatal(err)
				}
			case "preview":
				input.Confirmed = false
			case "dry-run":
				input.DryRun = true
			case "online-preview":
				input.DryRun, input.OnlinePreview = true, true
			case "declined":
				actor.allow = false
			case "denied":
				actor.observed.Policy = domain.PolicyBlocked
			case "unclosed":
				actor.observed.NativeEffect = domain.NativeEffectUncertain
				actor.observeErr = &domain.UnclosedClientProbe{Operation: "inert scoped verification", Err: sentinel}
			case "no-opt-in":
				service.Activator = existingLegacyVerifier{inner: actor}
			case "binding-changed":
				actor.onRead = func() {
					state, loadErr := store.Load()
					if loadErr != nil {
						t.Fatal(loadErr)
					}
					for key, binding := range state.Installations[0].Clients {
						binding.NativeActivationAttempt = "concurrent-attempt"
						state.Installations[0].Clients[key] = binding
					}
					if saveErr := store.Save(state); saveErr != nil {
						t.Fatal(saveErr)
					}
				}
			}
			var out AddResult
			if mode == "confirmed-update" {
				out, err = service.Update(t.Context(), input)
			} else {
				out, err = service.Add(t.Context(), input)
			}
			if mode == "unclosed" {
				var unclosed *domain.UnclosedClientProbe
				if !errors.As(err, &unclosed) || !errors.Is(err, sentinel) {
					t.Fatalf("unclosed cause lost: %v", err)
				}
			} else if mode == "binding-changed" {
				if err == nil {
					t.Fatal("changed binding authorized setup")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			acting := mode == "confirmed-add" || mode == "confirmed-update" || mode == "prepared-binding"
			wantRead := mode != "dry-run" && (mode != "no-opt-in" || domain.ShouldReadOnlyVerifyForScope(client.ClientID, input.Scope, input.BackendExecutable, input.InstallIntent))
			if actor.reads != btoi(wantRead) || actor.acts != btoi(acting) {
				t.Fatalf("reads=%d acts=%d outcome=%+v", actor.reads, actor.acts, out)
			}
			if acting {
				if out.Activation.Activation != domain.ActivationActive || out.NoChange || !out.Mutated {
					t.Fatalf("setup was not completed: %+v", out)
				}
				if _, err := os.Stat(actor.marker); err != nil {
					t.Fatalf("missing setup effect: %v", err)
				}
			} else {
				if _, err := os.Stat(actor.marker); !os.IsNotExist(err) {
					t.Fatalf("readonly/refused route acted: %v", err)
				}
				if (mode == "preview" || mode == "online-preview") && (!out.RequiresConfirmation || out.NoChange || out.Activation.Activation != domain.ActivationPrepared) {
					t.Fatalf("setup was not reviewable: %+v", out)
				}
			}
			currentInfo, err := os.Stat(added.Plan.ActivePath)
			if err != nil || !os.SameFile(info, currentInfo) {
				t.Fatalf("setup replaced package: %v", err)
			}
			after, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			current := onlyBinding(after.Installations[0])
			if len(current.Receipts) != len(saved.Receipts) || current.DataReceiptID != saved.DataReceiptID {
				t.Fatal("setup changed package or data ownership")
			}
			if (mode == "preview" || mode == "dry-run" || mode == "online-preview") && current.Activation != domain.ActivationActive {
				t.Fatal("preview persisted readiness")
			}
		})
	}
}

// Regression: closing the completion attempt's own marker reported a mutation
// and rewrote updated_at although the repeated add changed nothing.
func TestExistingPreparedSetupWithoutEffectIsNoChange(t *testing.T) {
	service, store, input, observed := existingProjectSetupFixture(t, "https://example.invalid/unchanged-prepared-setup")
	actor := &existingSetupActor{allow: true, observed: observed}
	service.Activator = actor
	if _, err := service.Add(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	saved := lifecycleOutcome(onlyBinding(before.Installations[0]))
	actor.settled, actor.needsSetup, actor.acts = &saved, true, 0
	out, err := service.Add(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}
	if actor.acts != 1 || out.Mutated || !out.NoChange || out.Activation.Activation != domain.ActivationActive {
		t.Fatalf("unchanged completion reported a change: acts=%d %+v", actor.acts, out)
	}
	after, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("unchanged completion rewrote state:\nbefore %+v\nafter  %+v", before.Installations[0], after.Installations[0])
	}
}

func btoi(v bool) int {
	if v {
		return 1
	}
	return 0
}
