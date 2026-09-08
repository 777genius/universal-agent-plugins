package agentpluginscli

import (
	"context"
	"errors"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func TestDoctorNativeProjectionVerificationIsExactReadOnlyAndFailClosed(t *testing.T) {
	for _, clientID := range []domain.ClientID{domain.ClientGemini, domain.ClientOpenCode, domain.ClientCline, domain.ClientWindsurf} {
		t.Run(string(clientID), func(t *testing.T) {
			activator := &doctorProjectionActivator{err: errors.New("owned projection differs")}
			installation := domain.Installation{InstallationID: "installation-id", DeclaredName: "demo"}
			binding := domain.ClientBinding{
				ClientID: string(clientID), Scope: string(domain.ScopeUser), PhysicalArtifact: "demo-physical",
				NativeObjects: []domain.NativeObjectOwnership{{ObjectID: "native:demo", Kind: "owned_native_projection", LogicalName: "demo", ManagedDigest: "sha256:native"}},
			}
			target := domain.DeliveryTarget{TargetRoot: "/managed", ActivePath: "/managed/demo"}
			findings := checkNativeProjectionIntegrity(context.Background(), App{Lifecycle: usecase.Service{Activator: activator}},
				domain.DetectedClient{ClientID: clientID, Status: domain.DetectionDetected, ConfigRoot: "/client-config"},
				target, installation, binding, "sha256:package")

			if len(findings) != 1 || findings[0].Status != "degraded" || findings[0].Code != "native_projection_changed" || findings[0].RecoveryAction == "" {
				t.Fatalf("findings = %+v", findings)
			}
			if len(activator.requests) != 1 {
				t.Fatalf("verification calls = %d", len(activator.requests))
			}
			request := activator.requests[0]
			if !request.VerifyOnly || request.Delivery.ActivePath != target.ActivePath || request.Delivery.ArtifactDigest != "sha256:package" ||
				len(request.Delivery.NativeObjects) != 1 || len(request.PreviousNativeObjects) != 1 {
				t.Fatalf("verification request = %+v", request)
			}
		})
	}
}

func TestDoctorNativeProjectionWithoutConfigVisibilityIsInconclusive(t *testing.T) {
	activator := &doctorProjectionActivator{}
	findings := checkNativeProjectionIntegrity(context.Background(), App{Lifecycle: usecase.Service{Activator: activator}},
		domain.DetectedClient{ClientID: domain.ClientGemini, Status: domain.DetectionDetected}, domain.DeliveryTarget{},
		domain.Installation{InstallationID: "installation-id", DeclaredName: "demo"},
		domain.ClientBinding{ClientID: string(domain.ClientGemini), NativeObjects: []domain.NativeObjectOwnership{{Kind: "owned_native_projection"}}}, "sha256:package")
	if len(findings) != 1 || findings[0].Status != "unknown" || findings[0].Code != "native_projection_not_checked" || len(activator.requests) != 0 {
		t.Fatalf("findings = %+v, calls = %d", findings, len(activator.requests))
	}
}

type doctorProjectionActivator struct {
	requests []domain.ActivationRequest
	err      error
}

func (activator *doctorProjectionActivator) Activate(_ context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	activator.requests = append(activator.requests, request)
	return domain.ActivationOutcome{}, activator.err
}

func (*doctorProjectionActivator) Deactivate(context.Context, domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	return domain.DeactivationOutcome{}, nil
}
