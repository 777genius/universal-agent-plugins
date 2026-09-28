package usecase

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

type uncertainNativeActivator struct {
	inner           ports.ClientActivator
	store           transaction.StateStore
	t               *testing.T
	observedAttempt string
}

type postNativeVerificationError struct{ inner ports.ClientActivator }

type uncertainNativeDeactivator struct {
	inner ports.ClientActivator
	store transaction.StateStore
	t     *testing.T
	seen  string
}

func (a *uncertainNativeDeactivator) Activate(ctx context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	return a.inner.Activate(ctx, request)
}

func (a *uncertainNativeDeactivator) Deactivate(_ context.Context, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	a.t.Helper()
	state, err := a.store.Load()
	if err != nil {
		a.t.Fatal(err)
	}
	a.seen = onlyBinding(state.Installations[0]).NativeActivationAttempt
	if a.seen == "" {
		a.t.Fatal("native removal began before durable attempt marker")
	}
	if err := os.Remove(filepath.Join(request.Client.ConfigRoot, "skills", "docs", "SKILL.md")); err != nil {
		a.t.Fatal(err)
	}
	return domain.DeactivationOutcome{}, errors.New("injected partial native removal")
}

func (a postNativeVerificationError) Activate(ctx context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	outcome, err := a.inner.Activate(ctx, request)
	if err != nil || request.Client.ClientID != domain.ClientKiro || request.VerifyOnly {
		return outcome, err
	}
	outcome.Activation = domain.ActivationFailed
	outcome.Verification = domain.VerificationFailed
	return outcome, errors.New("injected ACP verification failure after native commit")
}

func (a postNativeVerificationError) Deactivate(ctx context.Context, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	return a.inner.Deactivate(ctx, request)
}

func (a *uncertainNativeActivator) Activate(_ context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	a.t.Helper()
	state, err := a.store.Load()
	if err != nil {
		a.t.Fatal(err)
	}
	a.observedAttempt = onlyBinding(state.Installations[0]).NativeActivationAttempt
	if a.observedAttempt == "" {
		a.t.Fatal("native mutation began before durable attempt marker")
	}
	path := filepath.Join(request.Client.ConfigRoot, "skills", "docs")
	if err := os.MkdirAll(path, 0o700); err != nil {
		a.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte("partial foreign effect\n"), 0o644); err != nil {
		a.t.Fatal(err)
	}
	return domain.ActivationOutcome{Activation: domain.ActivationFailed, Authentication: domain.AuthenticationNotChecked, Policy: domain.PolicyAllowed, Verification: domain.VerificationFailed, NativeEffect: domain.NativeEffectUncertain}, errors.New("injected uncertain native effect")
}

func (a *uncertainNativeActivator) Deactivate(ctx context.Context, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	return a.inner.Deactivate(ctx, request)
}

type observeNativeActivations struct {
	inner  ports.ClientActivator
	cancel context.CancelFunc
	seen   []domain.ActivationRequest
}

func versionedNativeSkill(t *testing.T, input *AddInput, version string) {
	t.Helper()
	body := []byte("---\nname: docs\ndescription: docs " + version + "\n---\n")
	if err := os.WriteFile(filepath.Join(input.Envelope.SnapshotRoot, "skills", "docs", "SKILL.md"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	skill := input.Envelope.Skills["docs"]
	skill.Raw = body
	input.Envelope.Skills["docs"] = skill
}

func (a *observeNativeActivations) Activate(ctx context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	a.seen = append(a.seen, request)
	outcome, err := a.inner.Activate(ctx, request)
	if a.cancel != nil && len(a.seen) == 1 {
		a.cancel()
	}
	return outcome, err
}

func (a *observeNativeActivations) Deactivate(ctx context.Context, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	return a.inner.Deactivate(ctx, request)
}

type injectGeminiForeignSkill struct {
	inner ports.ClientActivator
	done  bool
}

func (a *injectGeminiForeignSkill) Activate(ctx context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if request.Client.ClientID == domain.ClientGemini && !a.done {
		a.done = true
		body, err := os.ReadFile(filepath.Join(request.Delivery.ActivePath, "skills", "docs", "SKILL.md"))
		if err != nil {
			return domain.ActivationOutcome{}, err
		}
		target := filepath.Join(request.Client.ConfigRoot, "skills", "docs")
		if err := os.MkdirAll(target, 0o700); err != nil {
			return domain.ActivationOutcome{}, err
		}
		if err := os.WriteFile(filepath.Join(target, "SKILL.md"), body, 0o644); err != nil {
			return domain.ActivationOutcome{}, err
		}
	}
	return a.inner.Activate(ctx, request)
}

func (a *injectGeminiForeignSkill) Deactivate(ctx context.Context, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	return a.inner.Deactivate(ctx, request)
}

func TestPublicS1GeminiFailedFirstRetriesCompleteDesiredWithoutAdoptingForeignBytes(t *testing.T) {
	service, store, _ := serviceFixture(t)
	service.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Stager: service.Stager})
	client := domain.DetectedClient{ClientID: domain.ClientGemini, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".gemini")}
	input := kiroPowerInput(t, client, "1.0.0", "sha256:gemini-retry-tree", "sha256:gemini-retry-manifest")
	input.Confirmed = true
	inner := service.Activator
	service.Activator = &injectGeminiForeignSkill{inner: inner}
	first, err := service.Add(context.Background(), input)
	if err == nil {
		t.Fatalf("foreign desired-identical skill was adopted: %+v", first)
	}
	foreign := filepath.Join(client.ConfigRoot, "skills", "docs")
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyBinding(state.Installations[0])
	if len(binding.NativeObjects) != 1 || binding.NativeActivationAttempt != "" {
		t.Fatalf("failed-first state claimed external ownership or retained a resolved attempt: %+v", binding)
	}
	if _, err := os.Stat(filepath.Join(foreign, "SKILL.md")); err != nil {
		t.Fatalf("foreign skill was changed: %v", err)
	}
	if err := os.RemoveAll(foreign); err != nil {
		t.Fatal(err)
	}
	service.Activator = inner
	input.OperationID = "gemini-retry-second"
	second, err := service.Add(context.Background(), input)
	if err != nil || !second.Mutated {
		t.Fatalf("retry did not deliver missing native skill: %+v, %v", second, err)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding = onlyBinding(state.Installations[0])
	if len(binding.NativeObjects) != 2 || len(binding.Receipts) != 1 || binding.NativeActivationAttempt != "" {
		t.Fatalf("retry persisted incomplete receipts or replaced package: %+v", binding)
	}
	if _, err := os.Stat(filepath.Join(foreign, "SKILL.md")); err != nil {
		t.Fatalf("retry did not install skill: %v", err)
	}
	input.OperationID = "gemini-retry-third"
	third, err := service.Add(context.Background(), input)
	if err != nil || third.Mutated {
		t.Fatalf("verified repeat should be read-only: %+v, %v", third, err)
	}
	state, err = store.Load()
	if err != nil || len(onlyBinding(state.Installations[0]).Receipts) != 1 {
		t.Fatalf("verified repeat created a package receipt: %v, %+v", err, state)
	}
}

func TestPublicS2CanceledGroupRetainsUntouchedNativeReceiptsAndRetriesOnlyMissingPeer(t *testing.T) {
	service, store, _ := serviceFixture(t)
	service.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Stager: service.Stager})
	gemini := domain.DetectedClient{ClientID: domain.ClientGemini, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".gemini")}
	kiro := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
	v1Gemini := kiroPowerInput(t, gemini, "1.0.0", "sha256:group-native-v1", "sha256:group-native-manifest-v1")
	v1Kiro := kiroPowerInput(t, kiro, "1.0.0", "sha256:group-native-v1", "sha256:group-native-manifest-v1")
	versionedNativeSkill(t, &v1Gemini, "v1")
	versionedNativeSkill(t, &v1Kiro, "v1")
	if _, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{v1Gemini, v1Kiro}, OperationGroupID: "public-native-v1", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	prior := map[string]domain.ClientBinding{}
	for _, binding := range before.Installations[0].Clients {
		prior[binding.ClientID] = binding
	}
	v2Gemini := kiroPowerInput(t, gemini, "2.0.0", "sha256:group-native-v2", "sha256:group-native-manifest-v2")
	v2Kiro := kiroPowerInput(t, kiro, "2.0.0", "sha256:group-native-v2", "sha256:group-native-manifest-v2")
	versionedNativeSkill(t, &v2Gemini, "v2")
	versionedNativeSkill(t, &v2Kiro, "v2")
	ctx, cancel := context.WithCancel(context.Background())
	inner := service.Activator
	service.Activator = &observeNativeActivations{inner: inner, cancel: cancel}
	_, err = service.UpdateGroup(ctx, GroupInput{Targets: []AddInput{v2Gemini, v2Kiro}, CompatibilityChecks: []AddInput{v2Gemini, v2Kiro}, OperationGroupID: "public-native-v2-cancel", Confirmed: true})
	if err == nil {
		t.Fatal("group cancellation was ignored")
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	var first, second domain.ClientBinding
	for _, binding := range state.Installations[0].Clients {
		if binding.ClientID == string(domain.ClientGemini) {
			first = binding
		} else if binding.ClientID == string(domain.ClientKiro) {
			second = binding
		}
	}
	if first.NativeObjects[1].ManagedDigest == prior[first.ClientID].NativeObjects[1].ManagedDigest || second.NativeObjects[1].ManagedDigest != prior[second.ClientID].NativeObjects[1].ManagedDigest {
		t.Fatalf("partial group receipts do not match actual activation: first=%+v second=%+v", first.NativeObjects, second.NativeObjects)
	}
	if second.NativeObjects[0].ManagedDigest == prior[second.ClientID].NativeObjects[0].ManagedDigest {
		t.Fatal("second package revision did not commit before cancellation")
	}
	for _, tc := range []struct{ root, want string }{{gemini.ConfigRoot, "v2"}, {kiro.ConfigRoot, "v1"}} {
		body, err := os.ReadFile(filepath.Join(tc.root, "skills", "docs", "SKILL.md"))
		if err != nil || !strings.Contains(string(body), tc.want) {
			t.Fatalf("native skill at %s = %q, %v", tc.root, body, err)
		}
	}
	observer := &observeNativeActivations{inner: inner}
	service.Activator = observer
	if _, err := service.UpdateGroup(context.Background(), GroupInput{Targets: []AddInput{v2Gemini, v2Kiro}, CompatibilityChecks: []AddInput{v2Gemini, v2Kiro}, OperationGroupID: "public-native-v2-retry", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	if len(observer.seen) != 2 || !observer.seen[0].VerifyOnly || observer.seen[1].VerifyOnly {
		if len(observer.seen) == 2 {
			t.Fatalf("retry verification flags = %v/%v; second prior digest=%s desired digest=%s", observer.seen[0].VerifyOnly, observer.seen[1].VerifyOnly, observer.seen[1].PreviousNativeObjects[1].ManagedDigest, observer.seen[1].Delivery.NativeObjects[1].ManagedDigest)
		}
		t.Fatalf("retry observed %d peers", len(observer.seen))
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range state.Installations[0].Clients {
		if binding.NativeObjects[1].ManagedDigest == prior[binding.ClientID].NativeObjects[1].ManagedDigest || len(binding.Receipts) != 2 {
			t.Fatalf("retry did not converge peer %s: %+v", binding.ClientID, binding)
		}
	}
}

func TestPublicS2UncertainNativeAttemptBlocksRetryAndRemoval(t *testing.T) {
	service, store, _ := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientGemini, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".gemini")}
	input := kiroPowerInput(t, client, "1.0.0", "sha256:uncertain-tree", "sha256:uncertain-manifest")
	input.Confirmed = true
	inject := &uncertainNativeActivator{inner: service.Activator, store: store, t: t}
	service.Activator = inject
	_, err := service.Add(context.Background(), input)
	if err == nil || inject.observedAttempt == "" {
		t.Fatalf("uncertain effect was not retained: %v", err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyBinding(state.Installations[0])
	if binding.NativeActivationAttempt != inject.observedAttempt || len(binding.NativeObjects) != 1 {
		t.Fatalf("uncertain effect falsely claimed ownership: %+v", binding)
	}
	service.Activator = inject.inner
	input.OperationID = "uncertain-native-retry"
	if _, err := service.Add(context.Background(), input); err == nil {
		t.Fatal("retry mutated unresolved native attempt")
	}
	if _, err := service.Remove(context.Background(), RemoveInput{Selector: state.Installations[0].InstallationID, Client: client, Scope: domain.ScopeUser, Confirmed: true, OperationID: "uncertain-native-remove"}); err == nil {
		t.Fatal("remove guessed ownership after uncertain effect")
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if onlyBinding(state.Installations[0]).NativeActivationAttempt != inject.observedAttempt {
		t.Fatal("recovery marker was lost")
	}
	if body, err := os.ReadFile(filepath.Join(client.ConfigRoot, "skills", "docs", "SKILL.md")); err != nil || string(body) != "partial foreign effect\n" {
		t.Fatalf("uncertain external effect was changed: %q, %v", body, err)
	}
}

func TestPublicS2VerificationFailureAfterCommittedNativeWriteKeepsReceipts(t *testing.T) {
	service, store, _ := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
	input := kiroPowerInput(t, client, "1.0.0", "sha256:kiro-post-native-tree", "sha256:kiro-post-native-manifest")
	input.Confirmed = true
	inner := service.Activator
	service.Activator = postNativeVerificationError{inner: inner}
	_, err := service.Add(context.Background(), input)
	if err == nil {
		t.Fatal("post-native verification error was ignored")
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyBinding(state.Installations[0])
	if len(binding.NativeObjects) != 2 || binding.NativeActivationAttempt != "" || binding.Activation != domain.ActivationFailed {
		t.Fatalf("committed native effect was lost after verification failed: %+v", binding)
	}
	if _, err := os.Stat(filepath.Join(client.ConfigRoot, "skills", "docs", "SKILL.md")); err != nil {
		t.Fatalf("committed native skill is absent: %v", err)
	}
	service.Activator = inner
	input.OperationID = "kiro-post-native-retry"
	result, err := service.Add(context.Background(), input)
	if err != nil || result.Activation.NativeEffect != domain.NativeEffectUnchanged {
		t.Fatalf("verification retry should only read existing native skill: %+v, %v", result, err)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(onlyBinding(state.Installations[0]).Receipts) != 1 {
		t.Fatal("verification retry created another directory receipt")
	}
}

func TestPublicS2UncertainNativeRemovalRetainsAttemptAndManagedPackage(t *testing.T) {
	service, store, _ := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
	input := kiroPowerInput(t, client, "1.0.0", "sha256:kiro-remove-tree", "sha256:kiro-remove-manifest")
	input.Confirmed = true
	added, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	inject := &uncertainNativeDeactivator{inner: service.Activator, store: store, t: t}
	service.Activator = inject
	_, err = service.Remove(context.Background(), RemoveInput{Selector: added.InstallationID, Client: client, Scope: domain.ScopeUser, Confirmed: true})
	if err == nil || inject.seen == "" {
		t.Fatalf("partial native removal was not held for recovery: %v", err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyBinding(state.Installations[0])
	if binding.NativeActivationAttempt != inject.seen || len(binding.NativeObjects) != 2 || len(binding.Receipts) != 1 {
		t.Fatalf("partial removal changed authoritative ownership: %+v", binding)
	}
	if _, err := os.Stat(added.Plan.ActivePath); err != nil {
		t.Fatalf("managed package was removed after partial native failure: %v", err)
	}
	service.Activator = inject.inner
	if _, err := service.Remove(context.Background(), RemoveInput{Selector: added.InstallationID, Client: client, Scope: domain.ScopeUser, Confirmed: true}); err == nil {
		t.Fatal("retry removed package despite unresolved native removal")
	}
}

// A valid package digest does not authorize removal from a different native
// profile. Both remove entry points must reject before adapter deactivation.
func TestPublicS2CodexRemovalRejectsDifferentBoundProfile(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		t.Run(fmt.Sprint("grouped=", grouped), func(t *testing.T) {
			service, store, _ := serviceFixture(t)
			profileA := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "profile-a")}
			input := addInput(t, profileA, "https://example.com/codex-profile-owned")
			input.Confirmed = true
			added, err := service.Add(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			before, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if got := onlyBinding(before.Installations[0]).NativeProfileRoot; got != profileA.ConfigRoot {
				t.Fatalf("fresh profile root was not persisted before activation: %q", got)
			}
			profileB := profileA
			profileB.ConfigRoot = filepath.Join(t.TempDir(), "profile-b")
			remove := RemoveInput{Selector: added.InstallationID, Client: profileB, Scope: domain.ScopeUser, ExternalUninstalled: true, Confirmed: true}
			if grouped {
				_, err = service.RemoveGroup(context.Background(), RemoveGroupInput{Selector: added.InstallationID, Targets: []RemoveInput{remove}, Confirmed: true})
			} else {
				_, err = service.Remove(context.Background(), remove)
			}
			if err == nil || !strings.Contains(err.Error(), "native profile root") {
				t.Fatalf("wrong-profile removal error = %v", err)
			}
			if _, statErr := os.Stat(added.Plan.ActivePath); statErr != nil {
				t.Fatalf("wrong-profile removal touched managed package: %v", statErr)
			}
			after, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatal("wrong-profile removal changed durable state")
			}
		})
	}
}
