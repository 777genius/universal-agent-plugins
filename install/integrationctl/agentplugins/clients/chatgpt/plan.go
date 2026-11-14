package chatgpt

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// AppBindingAction describes registration and package-author responsibilities.
const AppBindingAction = "this package is not ready for ChatGPT. Connect its remote MCP server in ChatGPT Plugins developer mode; full plugin installation also needs the publisher's registered connection mapping (.app.json). You do not need to create this file. Setup: https://developers.openai.com/plugins/build/plugins"

var (
	_ clients.LocalPreparationAuthorizer = (*Adapter)(nil)
	_ clients.PlanQualifier              = (*Adapter)(nil)
	_ clients.PlanRefiner                = (*Adapter)(nil)
	_ clients.PreparationRefiner         = (*Adapter)(nil)
	_ clients.CompatibilityLimiter       = (*Adapter)(nil)
)

// AuthorizeLocalPreparation accepts the user's own Context7 registration
// receipt in place of pinned catalog evidence. The receipt authorizes creating
// the local prepared package; it says nothing about the remote Plugins registry,
// which this installer cannot observe.
func (*Adapter) AuthorizeLocalPreparation(in clients.PlanInput, plan *domain.DeliveryPlan) (bool, error) {
	mapping := in.Envelope.LocalChatGPTMapping
	if mapping == nil {
		return false, nil
	}
	if plan.Scope != domain.ScopeUser {
		return false, fmt.Errorf("ChatGPT preparation supports user scope only")
	}
	if err := mapping.ValidatePackage(in.Envelope); err != nil {
		return false, err
	}
	binding, ok := in.Envelope.App.Bindings[mapping.Server]
	if !in.Envelope.App.Enabled || len(in.Envelope.App.Bindings) != 1 || !ok || binding.ID != mapping.AppID {
		return false, fmt.Errorf("personal ChatGPT mapping projection does not match receipt")
	}
	plan.LocalPreparationAuthorized = true
	plan.PersonalChatGPTPreparation = true
	plan.Authentication = domain.AuthenticationNotRequired
	plan.Warnings = shared.AppendUnique(plan.Warnings, "personal_registration_requires_account_install")
	return true, nil
}

// QualifyPlan admits a package only when every remote MCP server it declares is
// bound to a registered app: without that binding ChatGPT has nothing to
// connect to, and a delivered package would be inert.
func (adapter *Adapter) QualifyPlan(in clients.PlanInput, plan *domain.DeliveryPlan) error {
	envelope := in.Envelope
	if envelope.CatalogEvidence != nil {
		compatibility, ok := envelope.CatalogEvidence.Compatibility[string(adapter.ID())]
		if ok && compatibility.AppBinding != nil && shared.ComponentKindPresent(plan.Components, domain.ComponentApp) {
			// App bindings are added only from validated signed Directory
			// compatibility evidence. This authorizes local preparation, not a
			// claim about the unobservable remote Plugins registry.
			plan.LocalPreparationAuthorized = true
		}
	}
	missing := missingAppBindings(envelope)
	if !unboundPackage(envelope) && len(missing) == 0 {
		return nil
	}
	plan.Status = domain.PlanUnsupported
	plan.Activation = domain.ActivationFailed
	plan.Warnings = shared.AppendUnique(plan.Warnings, "chatgpt_app_binding_required")
	action := AppBindingAction
	if len(missing) > 0 {
		action += " for: " + strings.Join(missing, ", ")
	}
	plan.UserActions = shared.AppendUnique(plan.UserActions, action)
	return nil
}

func (*Adapter) RefinePlan(_ context.Context, _ clients.PlanInput, plan *domain.DeliveryPlan) error {
	action := "install the prepared skills-only plugin from ChatGPT Plugins, then start a new chat"
	if shared.ComponentKindPresent(plan.Components, domain.ComponentApp) {
		action = "install the prepared plugin from ChatGPT Plugins, verify its registered app connection, then start a new chat"
	}
	plan.UserActions = shared.AppendUnique(plan.UserActions, action)
	return nil
}

// RefinePreparation is reachable only through a validated personal mapping: a
// prepared ChatGPT package is the user's own marketplace entry, not a catalog
// delivery.
func (*Adapter) RefinePreparation(plan *domain.DeliveryPlan) error {
	if !plan.PersonalChatGPTPreparation {
		return fmt.Errorf("ChatGPT preparation requires a validated Context7 personal mapping")
	}
	plan.Status = domain.PlanReady
	plan.Activation = domain.ActivationPrepared
	plan.Authentication = domain.AuthenticationNotRequired
	plan.Verification = domain.VerificationPackageValid
	plan.UserActions = []string{domain.ChatGPTMappedPreparationAction}
	return nil
}

// ClientLimitations records that preparation is manual and that the remote app
// registration is never checked from here.
func (*Adapter) ClientLimitations(envelope domain.PackageEnvelope) []string {
	limitations := []string{"chatgpt_manual_preparation_only", "remote_app_registration_not_checked"}
	if unboundPackage(envelope) || len(missingAppBindings(envelope)) > 0 {
		limitations = append(limitations, "chatgpt_app_binding_required")
	}
	return limitations
}

// ComponentLimitations bounds each item by what a hosted, manually activated
// client can actually take: a bound remote MCP server, or nothing.
func (*Adapter) ComponentLimitations(envelope domain.PackageEnvelope, item domain.ComponentDecision) ([]string, []string) {
	switch item.Kind {
	case domain.ComponentMCPServer:
		server := envelope.MCP.Servers[item.Name]
		binding, mapped := envelope.App.Bindings[item.Name]
		switch {
		case !envelope.App.Enabled || !mapped || binding.ID == "":
			return []string{"chatgpt_app_binding_required"}, nil
		case server.Type != "streamable-http" && server.Type != "sse":
			return []string{"chatgpt_remote_mcp_required"}, nil
		default:
			return nil, []string{"remote_app_registration_not_checked"}
		}
	case domain.ComponentApp:
		return nil, []string{"remote_app_registration_not_checked"}
	}
	return nil, nil
}

// unboundPackage reports a package that carries MCP servers or an app section
// without an enabled app component to bind them.
func unboundPackage(envelope domain.PackageEnvelope) bool {
	return !envelope.App.Enabled &&
		(len(envelope.MCP.Servers) > 0 || envelope.App.Present || envelope.App.Declared)
}

// missingAppBindings names the declared MCP servers that no app binding covers.
func missingAppBindings(envelope domain.PackageEnvelope) []string {
	missing := make([]string, 0)
	for name := range envelope.MCP.Servers {
		if _, ok := envelope.App.Bindings[name]; !ok {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}
