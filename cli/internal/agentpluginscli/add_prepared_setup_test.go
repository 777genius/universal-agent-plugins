package agentpluginscli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

type setupConfirmPort struct {
	allow       bool
	unavailable bool
	calls       int
	request     domain.ActivationRequest
}

func (*setupConfirmPort) Activate(context.Context, domain.ActivationRequest) (domain.ActivationOutcome, error) {
	panic("classification must not activate")
}
func (*setupConfirmPort) Deactivate(context.Context, domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	panic("classification must not deactivate")
}
func (port *setupConfirmPort) PreparedSetupVerifierAvailable(domain.DetectedClient, domain.DeliveryPlan, string) bool {
	return !port.unavailable
}

func (port *setupConfirmPort) CanCompletePreparedSetup(request domain.ActivationRequest, _ domain.ActivationOutcome) bool {
	port.calls++
	port.request = request
	return port.allow && request.VerifyOnly
}

func TestInteractivePreparedSetupUsesNativeOptInAndNormalConfirmation(t *testing.T) {
	for _, scenario := range []string{"automatic", "empty-default", "declined-capability", "unavailable-verifier", "manual-activation", "prepare-intent", "planned-prepare-intent", "explicit-auth", "unverified", "attested", "not-reviewed"} {
		t.Run(scenario, func(t *testing.T) {
			port := &setupConfirmPort{allow: scenario != "declined-capability", unavailable: scenario == "unavailable-verifier"}
			service := usecase.Service{Activator: port}
			input := usecase.AddInput{Client: domain.DetectedClient{ClientID: "claude"}, Envelope: domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "reviewed"}}, BackendExecutable: "/selected/native"}
			planned := usecase.AddResult{RequiresConfirmation: true, Plan: domain.DeliveryPlan{ActivePath: "/selected/owned", TargetRoot: "/selected", InstallIntent: domain.InstallIntentAutomatic}, Activation: domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, Policy: domain.PolicyAllowed, Authentication: domain.AuthenticationNotRequired}}
			switch scenario {
			case "manual-activation":
				planned.Activation.Activation = domain.ActivationManual
			case "prepare-intent":
				input.InstallIntent = domain.InstallIntentPrepare
			case "planned-prepare-intent":
				planned.Plan.InstallIntent = domain.InstallIntentPrepare
			case "explicit-auth":
				input.AuthComplete = true
			case "unverified":
				planned.Activation.Verification = domain.VerificationNotRun
			case "attested":
				planned.Activation.ActivationAttested = true
			case "not-reviewed":
				planned.RequiresConfirmation = false
			}
			automatic := canConfirmPreparedSetup(service, input, planned)
			if automatic != (scenario == "automatic" || scenario == "empty-default") {
				t.Fatalf("automatic setup=%v for %s", automatic, scenario)
			}
			app := App{Terminal: true}
			opts := &options{format: "human"}
			manual := shouldResumeInteractiveLifecycle(app, opts, planned, false, input.AuthComplete, automatic)
			if scenario == "automatic" || scenario == "empty-default" {
				if manual {
					t.Fatal("automatic native setup was intercepted by manual handoff")
				}
				if !port.request.VerifyOnly || port.request.DeclaredName != "reviewed" || port.request.BackendExecutable != "/selected/native" || port.request.Delivery.ActivePath != "/selected/owned" {
					t.Fatalf("setup classification lost selected identity: %+v", port.request)
				}
				var output bytes.Buffer
				command := &cobra.Command{}
				command.SetIn(strings.NewReader("y\n"))
				command.SetOut(&output)
				command.SetErr(&output)
				confirmed, err := confirmPlannedAdd(context.Background(), command, app, opts, loadedPackage{}, planned, false, false, automatic)
				if err != nil || !confirmed || strings.Count(output.String(), "Apply this plan?") != 1 || strings.Contains(output.String(), "attestation") {
					t.Fatalf("automatic setup confirmation=%v,%v: %q", confirmed, err, output.String())
				}
			} else if scenario != "explicit-auth" && !manual {
				t.Fatal("existing manual lifecycle handoff changed")
			}
		})
	}
}
