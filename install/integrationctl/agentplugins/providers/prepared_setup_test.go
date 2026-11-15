package providers

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type setupIdentityOnly struct{}

func (setupIdentityOnly) ID() domain.ClientID { return "codex" }

type setupOptIn struct {
	setupIdentityOnly
	calls     int
	allow     bool
	available bool
}

func (adapter *setupOptIn) VerifierAvailable(client domain.DetectedClient, plan domain.DeliveryPlan, executable string) bool {
	return adapter.available && client.ClientID == adapter.ID() && plan.Scope == domain.ScopeProject && executable == "/native"
}

type setupVerifierOnly struct{ setupIdentityOnly }

func (setupVerifierOnly) VerifierAvailable(domain.DetectedClient, domain.DeliveryPlan, string) bool {
	return true
}

func TestPreparedSetupVerifierRequiresOptInAndExactScopedVerifier(t *testing.T) {
	client := domain.DetectedClient{ClientID: "codex"}
	plan := domain.DeliveryPlan{Scope: domain.ScopeProject}
	registry, err := clients.NewRegistry(setupVerifierOnly{})
	if err != nil {
		t.Fatal(err)
	}
	if (Activator{Registry: registry}).PreparedSetupVerifierAvailable(client, plan, "/native") {
		t.Fatal("legacy verifier gained setup opt-in")
	}
	adapter := &setupOptIn{available: true}
	registry, err = clients.NewRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	actor := Activator{Registry: registry}
	if !actor.PreparedSetupVerifierAvailable(client, plan, "/native") || adapter.calls != 0 {
		t.Fatal("scoped read-only capability was not queried independently")
	}
	plan.Scope = domain.ScopeUser
	if actor.PreparedSetupVerifierAvailable(client, plan, "/native") {
		t.Fatal("another scope borrowed setup verification")
	}
}

func (adapter *setupOptIn) CanCompletePreparedSetup(request domain.ActivationRequest, observed domain.ActivationOutcome) bool {
	adapter.calls++
	return adapter.allow && request.DeclaredName == "reviewed" && observed.Activation == domain.ActivationPrepared
}

func TestPreparedSetupDispatchRequiresRegisteredExplicitOptIn(t *testing.T) {
	request := domain.ActivationRequest{Client: domain.DetectedClient{ClientID: "codex"}, DeclaredName: "reviewed"}
	observed := domain.ActivationOutcome{Activation: domain.ActivationPrepared}
	if (Activator{}).CanCompletePreparedSetup(request, observed) {
		t.Fatal("missing registry admitted setup")
	}
	registry, err := clients.NewRegistry(setupIdentityOnly{})
	if err != nil {
		t.Fatal(err)
	}
	if (Activator{Registry: registry}).CanCompletePreparedSetup(request, observed) {
		t.Fatal("adapter without capability admitted setup")
	}
	adapter := &setupOptIn{allow: true}
	registry, err = clients.NewRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	activator := Activator{Registry: registry}
	if !activator.CanCompletePreparedSetup(request, observed) || adapter.calls != 1 {
		t.Fatal("exact opt-in was not dispatched")
	}
	request.Client.ClientID = "claude"
	if activator.CanCompletePreparedSetup(request, observed) || adapter.calls != 1 {
		t.Fatal("another adapter borrowed setup authority")
	}
	request.Client.ClientID = "codex"
	adapter.allow = false
	if activator.CanCompletePreparedSetup(request, observed) {
		t.Fatal("declined setup admitted")
	}
}
