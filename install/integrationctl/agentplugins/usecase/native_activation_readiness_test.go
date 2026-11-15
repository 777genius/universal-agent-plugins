package usecase

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

type readinessTestActivator struct {
	providers.Activator
	required bool
	err      error
	requests []domain.ActivationRequest
}

func (a *readinessTestActivator) RequiresActivation(context.Context, domain.DetectedClient, domain.DeliveryPlan) (bool, error) {
	return a.required, a.err
}

func (a *readinessTestActivator) Activate(ctx context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	a.requests = append(a.requests, request)
	out, err := a.Activator.Activate(ctx, request)
	if err == nil && !request.VerifyOnly && a.required {
		a.required = false
		out.NativeEffect = domain.NativeEffectCommitted
	}
	return out, err
}

func TestNativeActivationReadinessRestoresConfigurationWithoutReplacingPackage(t *testing.T) {
	service, store, _ := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
	input := kiroPowerInput(t, client, "1.0.0", "sha256:readiness-tree", "sha256:readiness-manifest")
	input.Confirmed = true
	if _, err := service.Add(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	before := onlyBinding(state.Installations[0])
	a := &readinessTestActivator{Activator: service.Activator.(providers.Activator), required: true}
	service.Activator = a
	input.Confirmed = false
	preview, err := service.Add(t.Context(), input)
	if err != nil || !preview.RequiresConfirmation || len(a.requests) != 0 {
		t.Fatalf("unconfirmed readiness mutated: %+v %v", preview, err)
	}
	input.Confirmed = true
	input.OperationID = "readiness-only-reactivation"
	result, err := service.Add(t.Context(), input)
	if err != nil || !result.Mutated || result.NoChange || a.required {
		t.Fatalf("reactivation: %+v %v", result, err)
	}
	if len(a.requests) != 1 || a.requests[0].VerifyOnly || !a.requests[0].PackageUnchanged {
		t.Fatalf("wrong native request: %+v", a.requests)
	}
	after, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyBinding(after.Installations[0])
	if binding.TargetLocator != before.TargetLocator || !reflect.DeepEqual(binding.Receipts, before.Receipts) || !reflect.DeepEqual(binding.NativeObjects, before.NativeObjects) {
		t.Fatal("configuration repair replaced immutable package or ownership")
	}
	result, err = service.Add(t.Context(), input)
	if err != nil || result.Mutated || !a.requests[len(a.requests)-1].VerifyOnly {
		t.Fatalf("settled repeated add: %+v %v", result, err)
	}
}

func TestNativeActivationReadinessErrorNeverRetriesActivation(t *testing.T) {
	service, store, _ := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
	input := kiroPowerInput(t, client, "1.0.0", "sha256:readiness-error-tree", "sha256:readiness-error-manifest")
	input.Confirmed = true
	if _, err := service.Add(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	cause := errors.New("unsafe native configuration")
	a := &readinessTestActivator{Activator: service.Activator.(providers.Activator), required: true, err: cause}
	service.Activator = a
	result, err := service.Add(t.Context(), input)
	if !errors.Is(err, cause) || result.Mutated || len(a.requests) != 0 {
		t.Fatalf("readiness error activated: %+v %v", result, err)
	}
	after, err := store.Load()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("readiness refusal changed state")
	}
}

func TestNativeActivationReadinessOptionalContractPreservesDelivery(t *testing.T) {
	delivery := domain.StagedDelivery{NativeObjects: []domain.NativeObjectOwnership{{ObjectID: "unchanged"}}}
	service := Service{Activator: &observedActivator{}}
	got, complete, settled, err := service.nativeDeliveryReadiness(t.Context(), AddInput{}, domain.DeliveryPlan{}, delivery)
	if err != nil || !complete || settled || !reflect.DeepEqual(got, delivery) {
		t.Fatal("absent optional capability changed completeness or ownership")
	}
}
