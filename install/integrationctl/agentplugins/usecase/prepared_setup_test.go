package usecase

import (
	"context"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type preparedSetupBaseActivator struct{}

func (preparedSetupBaseActivator) Activate(context.Context, domain.ActivationRequest) (domain.ActivationOutcome, error) {
	panic("classification must not activate")
}
func (preparedSetupBaseActivator) Deactivate(context.Context, domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	panic("classification must not deactivate")
}

type preparedSetupProbe struct {
	preparedSetupBaseActivator
	calls   int
	allow   bool
	request domain.ActivationRequest
}

func (*preparedSetupProbe) PreparedSetupVerifierAvailable(domain.DetectedClient, domain.DeliveryPlan, string) bool {
	return true
}

func (probe *preparedSetupProbe) CanCompletePreparedSetup(request domain.ActivationRequest, observed domain.ActivationOutcome) bool {
	probe.calls++
	probe.request = request
	request.Plan.Components[0].Name = "changed by classifier"
	request.Delivery.NativeObjects[0].Path = "changed by classifier"
	request.PreviousNativeObjects[0].Path = "changed by classifier"
	observed.UserActions[0] = "changed by classifier"
	return probe.allow
}

func TestPreparedSetupClassificationRequiresFreshProofAndDetachedPacket(t *testing.T) {
	for _, scenario := range []string{"admitted", "no-capability", "declined", "active", "unverified", "blocked", "uncertain-effect"} {
		t.Run(scenario, func(t *testing.T) {
			object := domain.NativeObjectOwnership{ObjectID: "owned", Kind: "skill_directory", Path: "/owned"}
			delivery := domain.StagedDelivery{ActivePath: "/owned", NativeObjects: []domain.NativeObjectOwnership{object}}
			observed := domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, Policy: domain.PolicyAllowed, UserActions: []string{"finish setup"}}
			probe := &preparedSetupProbe{allow: scenario != "declined"}
			session := repairSession{service: Service{Activator: probe}, input: AddInput{BackendExecutable: "/native", Envelope: domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "reviewed"}}}, client: domain.ClientBinding{NativeObjects: []domain.NativeObjectOwnership{object}}, result: AddResult{Plan: domain.DeliveryPlan{Components: []domain.ComponentDecision{{Name: "selected"}}}}}
			switch scenario {
			case "no-capability":
				session.service.Activator = preparedSetupBaseActivator{}
			case "active":
				observed.Activation = domain.ActivationActive
			case "unverified":
				observed.Verification = domain.VerificationNotRun
			case "blocked":
				observed.Policy = domain.PolicyBlocked
			case "uncertain-effect":
				observed.NativeEffect = domain.NativeEffectUncertain
			}
			allowed := session.canCompletePreparedSetup(delivery, observed)
			if allowed != (scenario == "admitted") {
				t.Fatalf("admitted=%v for %s", allowed, scenario)
			}
			expectedCalls := 0
			if scenario == "admitted" || scenario == "declined" {
				expectedCalls = 1
			}
			if probe.calls != expectedCalls {
				t.Fatalf("unproved setup reached classifier: calls=%d", probe.calls)
			}
			if probe.calls > 0 && (probe.request.DeclaredName != "reviewed" || probe.request.BackendExecutable != "/native" || probe.request.VerifyOnly) {
				t.Fatalf("classification differs from acting setup packet: %+v", probe.request)
			}
			if delivery.NativeObjects[0].Path != "/owned" || session.client.NativeObjects[0].Path != "/owned" || session.result.Plan.Components[0].Name != "selected" || observed.UserActions[0] != "finish setup" {
				t.Fatal("classifier mutated reviewed setup facts")
			}
		})
	}
}
