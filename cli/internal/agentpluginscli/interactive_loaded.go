package agentpluginscli

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
)

type loadedTargetSession struct {
	app         App
	ctx         context.Context
	loaded      loadedPackage
	clientMap   map[domain.ClientID]domain.DetectedClient
	physicalID  string
	intents     []map[domain.ClientID]domain.InstallIntent
	preflighter interface {
		PreflightActivation(domain.ActivationRequest) error
	}
}

func (app App) compatibleLoadedTargets(ctx context.Context, loaded loadedPackage, detected []domain.DetectedClient, intents ...map[domain.ClientID]domain.InstallIntent) ([]domain.DetectedClient, []targetSkip) {
	session := newLoadedTargetSession(ctx, app, loaded, detected, intents...)
	if session == nil {
		skipped := make([]targetSkip, 0, len(detected))
		for _, client := range detected {
			skipped = append(skipped, targetSkip{client.ClientID, "package planning is not configured"})
		}
		return nil, skipped
	}
	return session.classifyAll(detected)
}

func newLoadedTargetSession(ctx context.Context, app App, loaded loadedPackage, detected []domain.DetectedClient, intents ...map[domain.ClientID]domain.InstallIntent) *loadedTargetSession {
	if app.Planner == nil {
		return nil
	}
	session := &loadedTargetSession{
		app: app, ctx: ctx, loaded: loaded, clientMap: detectedClientMap(detected),
		physicalID: domain.ComputePhysicalArtifactID(loaded.envelope.Manifest.Name, "00000000-0000-4000-8000-000000000000"),
		intents:    intents,
	}
	session.restorePersistedPrepareIntents()
	if preflighter, ok := app.Lifecycle.Activator.(interface {
		PreflightActivation(domain.ActivationRequest) error
	}); ok {
		session.preflighter = preflighter
	}
	return session
}

func (session *loadedTargetSession) restorePersistedPrepareIntents() {
	if len(session.intents) == 0 || session.intents[0] == nil || session.app.StateStore == nil {
		return
	}
	state, err := session.app.StateStore.Load()
	if err != nil {
		return
	}
	installation, ok := locallyMatchedInstallation(state, session.loaded.envelope.Manifest.Name)
	if !ok {
		return
	}
	for _, binding := range installation.Clients {
		if binding.InstallIntent == domain.InstallIntentPrepare {
			session.intents[0][domain.ClientID(binding.ClientID)] = binding.InstallIntent
		}
	}
	for _, preference := range installation.InstallPreferences {
		session.intents[0][preference.ClientID] = preference.InstallIntent
	}
}

func (session *loadedTargetSession) classifyAll(detected []domain.DetectedClient) ([]domain.DetectedClient, []targetSkip) {
	var skipped []targetSkip
	compatible := make([]domain.DetectedClient, 0, len(detected))
	for _, client := range detected {
		classified, skip, ok := session.classify(client)
		if skip != nil {
			skipped = append(skipped, *skip)
			continue
		}
		if ok {
			compatible = append(compatible, classified)
		}
	}
	return compatible, skipped
}

func (session *loadedTargetSession) classify(client domain.DetectedClient) (domain.DetectedClient, *targetSkip, bool) {
	if requiresPersonalMapping(client.ClientID) && session.loaded.chatGPTPreparation && session.loaded.localChatGPTMapping == nil {
		client.DisplayName += " (prepare personal marketplace; register in ChatGPT, then install and verify tools)"
		return client, nil, true
	}
	candidate := cloneLoadedPackage(session.loaded)
	if err := prepareLoadedPackageForClient(&candidate, client.ClientID); err != nil {
		return client, &targetSkip{client.ClientID, "package binding preparation failed; ask the package publisher to check its client mapping"}, false
	}
	plan, err := session.app.Planner.Plan(session.ctx, domain.PlanRequest{
		Envelope: candidate.envelope, Client: client, Scope: domain.ScopeUser, PhysicalArtifactID: session.physicalID,
		Detected: session.clientMap,
	})
	if err != nil || plan.Status == domain.PlanUnsupported {
		return client, &targetSkip{client.ClientID, unsupportedLoadedReason(err, plan)}, false
	}
	if skip := session.applyPrepareIntent(&plan, &client); skip != nil {
		return client, skip, false
	}
	return session.preflight(client, plan)
}

func unsupportedLoadedReason(err error, plan domain.DeliveryPlan) string {
	reason := "package is unsupported for this client; ask the package publisher for supported components"
	if err != nil {
		reason = "package planning failed; ask the package publisher to check compatibility for this client"
	}
	for _, warning := range plan.Warnings {
		if warning == "chatgpt_app_binding_required" {
			return clientplanner.ChatGPTAppBindingAction
		}
	}
	return reason
}

func (session *loadedTargetSession) applyPrepareIntent(plan *domain.DeliveryPlan, client *domain.DetectedClient) *targetSkip {
	if len(session.intents) == 0 || session.intents[0][client.ClientID] != domain.InstallIntentPrepare {
		return nil
	}
	if err := clientplanner.ApplyInstallIntent(session.app.ClientRegistry, plan, domain.InstallIntentPrepare); err != nil {
		return &targetSkip{client.ClientID, "persisted preparation cannot serve this package"}
	}
	if requiresPersonalMapping(client.ClientID) {
		client.DisplayName += " (prepare personal marketplace; install and verify tools in ChatGPT)"
		return nil
	}
	client.DisplayName += " (prepare configuration; authenticate and verify tools in " + domain.ClientDisplayName(client.ClientID) + ")"
	return nil
}

func (session *loadedTargetSession) preflight(client domain.DetectedClient, plan domain.DeliveryPlan) (domain.DetectedClient, *targetSkip, bool) {
	if session.preflighter == nil {
		return client, nil, true
	}
	err := session.preflighter.PreflightActivation(domain.ActivationRequest{
		Client: client, Plan: plan, BackendExecutable: backendExecutable(client, session.clientMap), VerifyOnly: true,
	})
	if err == nil {
		return client, nil, true
	}
	return session.preflightFailure(client, plan)
}

func (session *loadedTargetSession) preflightFailure(client domain.DetectedClient, plan domain.DeliveryPlan) (domain.DetectedClient, *targetSkip, bool) {
	reason := "automatic activation/verification preflight failed; resolve this client's verification prerequisites before retrying --target " + string(client.ClientID)
	if allowsHostedPrepare(client.ClientID) && len(session.intents) > 0 && session.intents[0] != nil {
		if prepared, ok := session.tryHostedPrepare(client, plan); ok {
			return prepared, nil, true
		}
	}
	if allowsHostedPrepare(client.ClientID) {
		reason = "this CLI cannot automatically check MCP connections in your " + domain.ClientDisplayName(client.ClientID) + " setup. Nothing was installed in " + domain.ClientDisplayName(client.ClientID) + ". Use another listed client, or connect the server in " + domain.ClientDisplayName(client.ClientID) + ": https://kiro.dev/docs/mcp/"
	}
	return client, &targetSkip{client.ClientID, reason}, false
}

func (session *loadedTargetSession) tryHostedPrepare(client domain.DetectedClient, plan domain.DeliveryPlan) (domain.DetectedClient, bool) {
	if prepareErr := clientplanner.ApplyInstallIntent(session.app.ClientRegistry, &plan, domain.InstallIntentPrepare); prepareErr != nil {
		return client, false
	}
	prepareErr := session.preflighter.PreflightActivation(domain.ActivationRequest{Client: client, Plan: plan, VerifyOnly: true})
	if prepareErr != nil {
		return client, false
	}
	session.intents[0][client.ClientID] = domain.InstallIntentPrepare
	client.DisplayName += " (prepare configuration; automatic MCP verification unavailable; authenticate and verify tools in " + domain.ClientDisplayName(client.ClientID) + ")"
	return client, true
}
