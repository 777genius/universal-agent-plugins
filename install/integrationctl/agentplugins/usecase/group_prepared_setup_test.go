package usecase

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/plannertest"
)

type groupSetupActors map[domain.ClientID]*existingSetupActor

func (actors groupSetupActors) VerifierAvailable(client domain.DetectedClient, _ domain.DeliveryPlan, _ string) bool {
	return actors[client.ClientID] != nil
}
func (actors groupSetupActors) PreparedSetupVerifierAvailable(client domain.DetectedClient, plan domain.DeliveryPlan, executable string) bool {
	return actors.VerifierAvailable(client, plan, executable)
}
func (groupSetupActors) AutomaticallyActivates(domain.ActivationRequest) bool { return true }
func (actors groupSetupActors) CanCompletePreparedSetup(request domain.ActivationRequest, observed domain.ActivationOutcome) bool {
	return actors[request.Client.ClientID].CanCompletePreparedSetup(request, observed)
}
func (actors groupSetupActors) Activate(ctx context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	return actors[request.Client.ClientID].Activate(ctx, request)
}
func (groupSetupActors) Deactivate(context.Context, domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	panic("unused")
}

type groupSetupFixture struct {
	service  Service
	store    statev2.Store
	inputs   []AddInput
	actors   groupSetupActors
	before   domain.StateFileV2
	inodes   map[string]os.FileInfo
	foreign  string
	identity *groupSetupIdentity
}

// The native namespace is inert in this use-case fixture; package ownership is
// still checked by the real stager, including changes made from callbacks.
type groupSetupIdentity struct {
	service Service
	foreign bool
}

func (o *groupSetupIdentity) ObserveNativeIdentity(ctx context.Context, _ domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	if o.foreign {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityUnmanaged}, nil
	}
	return o.service.filesystemIdentityObservation(ctx, plan, managed)
}

func (o *groupSetupIdentity) ObservePreparedIdentity(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	observed, err := o.ObserveNativeIdentity(ctx, client, plan, managed)
	observed.NativeDiscoveryState = domain.NativeIdentityAbsent
	observed.ReceiptReconciled = observed.State == domain.NativeIdentityManaged && err == nil
	return observed, err
}

func newGroupSetupFixture(t *testing.T) groupSetupFixture {
	t.Helper()
	service, store, _ := serviceFixture(t)
	project := t.TempDir()
	service.ProjectRoot = project
	store.Path, store.ProjectRoot = filepath.Join(project, ".uap", "state-v2.json"), project
	service.StateStore, service.Kernel.StateStore = store, store
	service.CheckNamespace = func() error { return service.Paths.RequireContainedChild(project, store.Path) }
	planner := plannertest.NewPlanner(clientplanner.Planner{ProjectRoot: project, ManagedRoot: filepath.Join(project, ".uap", "managed")})
	service.Planner, service.Targets = planner, planner
	f := groupSetupFixture{service: service, store: store, actors: groupSetupActors{}, inodes: map[string]os.FileInfo{}}
	f.identity = &groupSetupIdentity{service: service}
	f.service.NativeObserver = f.identity
	for _, id := range []domain.ClientID{domain.ClientClaude, domain.ClientCodex} {
		client := domain.DetectedClient{ClientID: id, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "profile"), ExecutablePath: filepath.Join(t.TempDir(), string(id))}
		input := addInput(t, client, "https://example.invalid/group-prepared-setup")
		input.Scope, input.BackendExecutable = domain.ScopeProject, client.ExecutablePath
		skill := []byte("---\nname: guide\ndescription: Fixture guide\n---\nUse the fixture guide.\n")
		skillDir := filepath.Join(input.Envelope.SnapshotRoot, "skills", "guide")
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), skill, 0644); err != nil {
			t.Fatal(err)
		}
		input.Envelope.Skills = map[string]domain.Skill{"guide": {Name: "guide", Description: "Fixture guide", RelativePath: "skills/guide/SKILL.md", Raw: skill}}
		input.Envelope.Inventory.Skills = []string{"guide"}
		f.inputs = append(f.inputs, input)
		f.actors[id] = &existingSetupActor{allow: true, marker: filepath.Join(t.TempDir(), "native-setup"), observed: domain.ActivationOutcome{
			Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, Authentication: domain.AuthenticationNotChecked,
			Policy: domain.PolicyAllowed, NativeEffect: domain.NativeEffectUnchanged,
		}}
	}
	f.service.Activator = f.actors
	if _, err := f.service.AddGroup(t.Context(), GroupInput{Targets: f.inputs, Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		// Model a historical unknown authentication fact, not new native proof.
		binding.Authentication = domain.AuthenticationNotChecked
		state.Installations[0].Clients[key] = binding
		f.inodes[binding.TargetLocator], err = os.Stat(binding.TargetLocator)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	f.before = state
	f.foreign = filepath.Join(project, "foreign-project-settings")
	if err := os.WriteFile(f.foreign, []byte("preserve unrelated settings"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, actor := range f.actors {
		actor.needsSetup, actor.reads, actor.acts = true, 0, 0
	}
	return f
}

func (f groupSetupFixture) assertPreservedPackage(t *testing.T) {
	t.Helper()
	state, err := f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, original := range f.before.Installations[0].Clients {
		binding := state.Installations[0].Clients[key]
		if !reflect.DeepEqual(original.Receipts, binding.Receipts) || original.DataReceiptID != binding.DataReceiptID || !reflect.DeepEqual(original.PackageRevision, binding.PackageRevision) {
			t.Fatal("native setup changed package revision or ownership receipts")
		}
		info, err := os.Stat(binding.TargetLocator)
		if err != nil || !os.SameFile(f.inodes[binding.TargetLocator], info) {
			t.Fatalf("native setup replaced package: %v", err)
		}
	}
	if body, err := os.ReadFile(f.foreign); err != nil || string(body) != "preserve unrelated settings" {
		t.Fatalf("native setup changed unrelated settings: %q %v", body, err)
	}
}

func TestGroupPreparedSetupIsReviewedAndCompletesWithoutRestaging(t *testing.T) {
	for _, update := range []bool{false, true} {
		t.Run(map[bool]string{false: "add", true: "update"}[update], func(t *testing.T) {
			f := newGroupSetupFixture(t)
			apply := f.service.AddGroup
			if update {
				apply = f.service.UpdateGroup
			}
			before, err := os.ReadFile(f.store.Path)
			if err != nil {
				t.Fatal(err)
			}
			for _, mode := range []string{"online-preview", "declined", "offline-dry-run"} {
				input := GroupInput{Targets: f.inputs, DryRun: mode != "declined", OnlinePreview: mode == "online-preview"}
				preview, err := apply(t.Context(), input)
				if err != nil {
					t.Fatal(err)
				}
				for _, target := range preview.Targets {
					if mode != "offline-dry-run" && (!target.RequiresConfirmation || target.NoChange || target.Activation.Activation != domain.ActivationPrepared || !strings.Contains(strings.Join(target.Plan.UserActions, " "), "Repair native setup")) {
						t.Fatalf("native setup missing from %s review: %+v", mode, target)
					}
				}
				for _, actor := range f.actors {
					if actor.acts != 0 || actor.reads != btoi(mode != "offline-dry-run") {
						t.Fatalf("preview performed effects or offline probe: reads=%d acts=%d mode=%s", actor.reads, actor.acts, mode)
					}
					actor.reads = 0
				}
				if after, err := os.ReadFile(f.store.Path); err != nil || string(after) != string(before) {
					t.Fatalf("preview changed saved state: %v", err)
				}
			}
			result, err := apply(t.Context(), GroupInput{Targets: f.inputs, Confirmed: true})
			if err != nil {
				t.Fatal(err)
			}
			if !result.Mutated || len(result.Receipts) != 0 {
				t.Fatalf("intact setup was restaged or not reported: %+v", result)
			}
			for _, target := range result.Targets {
				if target.RequiresConfirmation || target.NoChange || !target.Mutated || target.Activation.Activation != domain.ActivationActive || target.Activation.Authentication != domain.AuthenticationNotRequired || target.Activation.Verification != domain.VerificationInstalled {
					t.Fatalf("group setup did not finish: %+v", target)
				}
				actor := f.actors[target.Plan.ClientID]
				if actor.acts != 1 {
					t.Fatalf("setup acting calls=%d", actor.acts)
				}
				if _, err := os.Stat(actor.marker); err != nil {
					t.Fatal(err)
				}
			}
			f.assertPreservedPackage(t)
			settled, err := os.ReadFile(f.store.Path)
			if err != nil {
				t.Fatal(err)
			}
			repeated, err := apply(t.Context(), GroupInput{Targets: f.inputs, Confirmed: true})
			if err != nil || repeated.Mutated {
				t.Fatalf("ready repeat changed native setup: %+v %v", repeated, err)
			}
			for _, target := range repeated.Targets {
				if !target.NoChange || f.actors[target.Plan.ClientID].acts != 1 {
					t.Fatalf("ready repeat acted: %+v", target)
				}
			}
			if after, err := os.ReadFile(f.store.Path); err != nil || string(after) != string(settled) {
				t.Fatalf("ready repeat rewrote state: %v", err)
			}
		})
	}
}

func TestGroupPreparedSetupRefusesUnclosedCancelledAndChangedAuthority(t *testing.T) {
	for _, scenario := range []string{"unclosed", "canceled", "foreign", "package-changed", "binding-changed"} {
		t.Run(scenario, func(t *testing.T) {
			f := newGroupSetupFixture(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			sentinel := errors.New("native probe did not close")
			if scenario == "unclosed" {
				f.actors[domain.ClientClaude].observeErr = &domain.UnclosedClientProbe{Operation: "inert group observation", Err: sentinel}
				f.actors[domain.ClientClaude].observed.NativeEffect = domain.NativeEffectUncertain
			}
			if scenario == "canceled" {
				cancel()
			}
			if scenario == "foreign" {
				f.identity.foreign = true
			}
			changed := false
			input := GroupInput{Targets: f.inputs, Confirmed: true, Progress: func(event GroupProgressEvent) {
				if changed || event.Phase != GroupProgressConfigured {
					return
				}
				changed = true
				if scenario == "package-changed" {
					if err := os.WriteFile(filepath.Join(event.Result.Plan.ActivePath, "foreign-change"), []byte("keep"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if scenario == "binding-changed" {
					state, err := f.store.Load()
					if err != nil {
						t.Fatal(err)
					}
					for key, binding := range state.Installations[0].Clients {
						binding.NativeActivationAttempt = "foreign-attempt"
						state.Installations[0].Clients[key] = binding
					}
					if err := f.store.Save(state); err != nil {
						t.Fatal(err)
					}
				}
			}}
			_, err := f.service.AddGroup(ctx, input)
			if err == nil || scenario == "unclosed" && (!domain.IsUnclosedClientProbe(err) || !errors.Is(err, sentinel)) || scenario == "canceled" && !errors.Is(err, context.Canceled) {
				t.Fatalf("refusal lost its reason: %v", err)
			}
			if (scenario == "package-changed" || scenario == "binding-changed") && !changed {
				t.Fatalf("test never reached the callback that changed authority: %v", err)
			}
			for _, actor := range f.actors {
				if actor.acts != 0 {
					t.Fatal("refused setup performed native effects")
				}
			}
			f.assertPreservedPackage(t)
		})
	}
}

func TestGroupNativeObservationDoesNotReplaceExplicitOrUnprovedAuthentication(t *testing.T) {
	for _, scenario := range []string{"pending", "failed", "complete", "unverified", "blocked", "attested"} {
		t.Run(scenario, func(t *testing.T) {
			f := newGroupSetupFixture(t)
			state, err := f.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			want := domain.AuthenticationNotChecked
			switch scenario {
			case "pending":
				want = domain.AuthenticationPending
			case "failed":
				want = domain.AuthenticationFailed
			case "complete":
				want = domain.AuthenticationComplete
			}
			for key, binding := range state.Installations[0].Clients {
				binding.Authentication = want
				state.Installations[0].Clients[key] = binding
			}
			if err := f.store.Save(state); err != nil {
				t.Fatal(err)
			}
			for _, actor := range f.actors {
				actor.observed.Activation, actor.observed.Authentication = domain.ActivationActive, domain.AuthenticationNotRequired
				switch scenario {
				case "unverified":
					actor.observed.Verification = domain.VerificationNotRun
				case "blocked":
					actor.observed.Policy = domain.PolicyBlocked
				case "attested":
					actor.observed.AuthenticationAttested = true
				}
			}
			result, err := f.service.AddGroup(t.Context(), GroupInput{Targets: f.inputs, Confirmed: true})
			if err != nil {
				t.Fatal(err)
			}
			for _, target := range result.Targets {
				if target.Activation.Authentication != want || f.actors[target.Plan.ClientID].acts != 0 {
					t.Fatalf("group replaced authentication or acted without setup: %+v", target)
				}
			}
			state, err = f.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			for _, binding := range state.Installations[0].Clients {
				if binding.Authentication != want {
					t.Fatalf("group persisted unproved authentication: %+v", binding)
				}
			}
			f.assertPreservedPackage(t)
		})
	}
}
