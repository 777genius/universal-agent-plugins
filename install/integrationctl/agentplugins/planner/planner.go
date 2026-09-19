package planner

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type Planner struct {
	ManagedRoot string
	Paths       ports.PathPolicy
	// Registry supplies the client adapters that decide where a package lands
	// and what a plan still asks of the user. It is injected by the composition
	// root and never defaulted to "every client".
	Registry *clients.Registry
}

var errPathPolicyRequired = errors.New("planner path policy is required")

// ChatGPTAppBindingAction describes registration and package-author
// responsibilities. The wording is owned by clients/chatgpt.AppBindingAction;
// the facade keeps a copy so planner production code never imports a concrete
// adapter. facade_actions_test.go locks the two strings together.
const ChatGPTAppBindingAction = "this package is not ready for ChatGPT. Connect its remote MCP server in ChatGPT Plugins developer mode; full plugin installation also needs the publisher's registered connection mapping (.app.json). You do not need to create this file. Setup: https://developers.openai.com/plugins/build/plugins"

// DetectedPhysicalClient returns a genuinely detected client that can address
// an installed physical binding. Copilot and VS Code share one backend, so an
// installed binding owned by either logical client can be maintained through
// the other when that is the surface actually present on this machine. The
// returned client keeps its real identity; callers must not use this to claim
// that an explicitly requested, undetected logical client is installed.
func DetectedPhysicalClient(bindingClient domain.ClientID, detected map[domain.ClientID]domain.DetectedClient) (domain.DetectedClient, bool) {
	if client, ok := detected[bindingClient]; ok && client.Status == domain.DetectionDetected {
		return client, true
	}
	for _, sibling := range domain.BackendSiblings(bindingClient) {
		if client, ok := detected[sibling]; ok && client.Status == domain.DetectionDetected {
			return client, true
		}
	}
	return domain.DetectedClient{}, false
}

// Plan resolves the request into a delivery plan and applies its install
// intent, so a caller never has to remember the second step. An empty intent is
// the automatic one and leaves the plan unchanged.
func (planner Planner) Plan(ctx context.Context, request domain.PlanRequest) (domain.DeliveryPlan, error) {
	if planner.Paths == nil {
		return domain.DeliveryPlan{}, errPathPolicyRequired
	}
	if planner.Registry == nil {
		return domain.DeliveryPlan{}, clients.ErrRegistryRequired
	}
	plan, err := planner.plan(ctx, request)
	if err != nil {
		return plan, err
	}
	if err := ApplyInstallIntent(planner.Registry, &plan, request.InstallIntent); err != nil {
		return plan, err
	}
	return plan, nil
}

// plan is the generic pipeline. Every client-specific decision in it is reached
// through the registry, and the stages are ordered so that the warnings and
// user actions a plan carries keep the order they are rendered in.
func (planner Planner) plan(ctx context.Context, request domain.PlanRequest) (domain.DeliveryPlan, error) {
	definition, plan, err := planner.startPlan(ctx, request)
	if err != nil {
		return domain.DeliveryPlan{}, err
	}
	input := clients.PlanInput{
		Envelope: request.Envelope, Client: request.Client,
		Detected: request.Detected, Intent: request.InstallIntent,
	}
	planner.setNativeRegistry(&plan, input)
	if !admissible(definition, request, &plan) {
		return plan, nil
	}
	if precondition, ok := clients.As[clients.PlanPrecondition](planner.Registry, plan.ClientID); ok {
		if err := precondition.CheckPlanPrecondition(input, &plan); err != nil {
			return plan, err
		}
		if plan.Status == domain.PlanUnsupported {
			return plan, nil
		}
	}
	target, err := planner.ResolveTarget(ctx, request.Client, request.Scope, request.PhysicalArtifactID)
	if err != nil {
		return domain.DeliveryPlan{}, err
	}
	plan.TargetAnchor, plan.TargetRoot, plan.ActivePath = target.TargetAnchor, target.TargetRoot, target.ActivePath
	plan.Components = componentDecisions(request.Envelope, definition.Capabilities)
	if err := planner.qualify(input, definition.Capabilities, &plan); err != nil {
		return plan, err
	}
	if plan.Status == domain.PlanUnsupported {
		return plan, nil
	}
	appendAuthenticationActions(&plan)
	refiner, ok := clients.As[clients.PlanRefiner](planner.Registry, plan.ClientID)
	if !ok {
		return plan, nil
	}
	if err := refiner.RefinePlan(ctx, input, &plan); err != nil {
		return plan, err
	}
	return plan, nil
}

// startPlan builds the plan every client starts from: the declarative
// capabilities of the client, plus the package identity the request names.
func (planner Planner) startPlan(ctx context.Context, request domain.PlanRequest) (domain.ClientDefinition, domain.DeliveryPlan, error) {
	if err := ctx.Err(); err != nil {
		return domain.ClientDefinition{}, domain.DeliveryPlan{}, err
	}
	if err := planner.Paths.ValidateLeafID(request.PhysicalArtifactID); err != nil {
		return domain.ClientDefinition{}, domain.DeliveryPlan{}, fmt.Errorf("invalid physical artifact id: %w", err)
	}
	definition, ok := domain.ClientDefinitionFor(request.Client.ClientID)
	if !ok {
		return domain.ClientDefinition{}, domain.DeliveryPlan{}, fmt.Errorf("unsupported client %q", request.Client.ClientID)
	}
	capabilities := definition.Capabilities
	return definition, domain.DeliveryPlan{
		ClientID:    request.Client.ClientID,
		Scope:       request.Scope,
		Status:      statusFor(capabilities.ActivationMode),
		PackageMode: capabilities.PackageMode,
		Activation:  activationFor(capabilities.ActivationMode),
		// A package loaded without catalog evidence has unknown authentication
		// requirements. Only affirmative per-client evidence may mark it as not
		// required.
		Authentication:     domain.AuthenticationNotChecked,
		Policy:             domain.PolicyAllowed,
		Verification:       domain.VerificationPackageValid,
		PhysicalArtifactID: request.PhysicalArtifactID,
		DeclaredName:       request.Envelope.Manifest.Name,
		DeclaredVersion:    request.Envelope.Manifest.Version,
	}, nil
}

// setNativeRegistry records the client registry this delivery is made against.
// It runs before any rejection so that even an unsupported plan carries the
// locators a caller needs to reason about what is already installed.
func (planner Planner) setNativeRegistry(plan *domain.DeliveryPlan, input clients.PlanInput) {
	plan.NativeRegistryRoot = input.Client.ConfigRoot
	plan.NativeRegistryExecutable = input.Client.ExecutablePath
	if layout, ok := clients.As[clients.NativeRegistryLayout](planner.Registry, plan.ClientID); ok {
		plan.NativeRegistryRoot, plan.NativeRegistryExecutable = layout.NativeRegistry(input)
	}
}

// admissible applies the two rejections that need nothing but the declarative
// registry: the client has to be here, and it has to support the scope.
func admissible(definition domain.ClientDefinition, request domain.PlanRequest, plan *domain.DeliveryPlan) bool {
	if request.Client.Status != domain.DetectionDetected && !definition.PlansWithoutHostPresence {
		rejectPlan(plan, "client_not_detected")
		return false
	}
	if !supportsScope(definition.Capabilities.Scopes, request.Scope) {
		rejectPlan(plan, "scope_not_supported")
		return false
	}
	return true
}

// qualify turns the component selection into a verdict: the client's own
// authorization of a local preparation, otherwise the pinned catalog, then the
// package diagnostics, then the client's admission rules, then the generic
// "nothing usable is left" check.
func (planner Planner) qualify(input clients.PlanInput, capabilities domain.ClientCapabilities, plan *domain.DeliveryPlan) error {
	authorized := false
	if authorizer, ok := clients.As[clients.LocalPreparationAuthorizer](planner.Registry, plan.ClientID); ok {
		granted, err := authorizer.AuthorizeLocalPreparation(input, plan)
		if err != nil {
			return err
		}
		authorized = granted
	}
	if !authorized {
		applyCatalogCompatibility(plan, input.Envelope.CatalogEvidence)
	}
	hasComponentErrors := applyDiagnostics(plan, input.Envelope, capabilities)
	if qualifier, ok := clients.As[clients.PlanQualifier](planner.Registry, plan.ClientID); ok {
		if err := qualifier.QualifyPlan(input, plan); err != nil {
			return err
		}
	}
	applyComponentVerdict(plan, hasComponentErrors)
	return nil
}

func applyDiagnostics(plan *domain.DeliveryPlan, envelope domain.PackageEnvelope, capabilities domain.ClientCapabilities) bool {
	hasComponentErrors := false
	for _, diagnostic := range envelope.Diagnostics {
		plan.Diagnostics = append(plan.Diagnostics, diagnostic)
		plan.Warnings = appendUnique(plan.Warnings, diagnostic.Code)
		if diagnostic.Severity == domain.SeverityError && invalidatesComponent(diagnostic.Boundary, capabilities) {
			hasComponentErrors = true
		}
	}
	return hasComponentErrors
}

// invalidatesComponent reports whether a failed boundary destroys a component
// this client could otherwise have taken. The app boundary counts only for a
// client that delivers app bindings at all; for every other client an app
// diagnostic describes a part of the package it never looks at.
func invalidatesComponent(boundary domain.FailureBoundary, capabilities domain.ClientCapabilities) bool {
	switch boundary {
	case domain.BoundaryMCP, domain.BoundaryMCPServer, domain.BoundarySkill, domain.BoundaryExtension:
		return true
	case domain.BoundaryApp:
		return capabilities.AppSupport != domain.SupportUnsupported
	}
	return false
}

func applyComponentVerdict(plan *domain.DeliveryPlan, hasComponentErrors bool) {
	switch {
	case !hasComponents(plan.Components) && hasComponentErrors:
		rejectPlan(plan, "no_valid_components")
	case hasComponents(plan.Components) && !hasSupportedComponent(plan.Components):
		rejectPlan(plan, "no_supported_components")
	}
}

func appendAuthenticationActions(plan *domain.DeliveryPlan) {
	switch plan.Authentication {
	case domain.AuthenticationPending:
		plan.UserActions = append(plan.UserActions, "complete authentication for this plugin in the selected client")
	case domain.AuthenticationNotChecked:
		plan.UserActions = append(plan.UserActions, "verify the plugin's authentication requirements before using it")
	}
}

func rejectPlan(plan *domain.DeliveryPlan, warning string) {
	plan.Status = domain.PlanUnsupported
	plan.Activation = domain.ActivationFailed
	plan.Warnings = append(plan.Warnings, warning)
}

func applyCatalogCompatibility(plan *domain.DeliveryPlan, evidence *domain.CatalogEvidence) {
	if evidence == nil {
		plan.Warnings = appendUnique(plan.Warnings, "authentication_not_catalog_verified")
		return
	}
	compatibility, ok := evidence.Compatibility[string(plan.ClientID)]
	if !ok {
		plan.Status = domain.PlanUnsupported
		plan.Activation = domain.ActivationFailed
		plan.Warnings = appendUnique(plan.Warnings, "client_compatibility_not_catalog_verified")
		plan.UserActions = append(plan.UserActions, "choose a client present in the pinned catalog evidence")
		return
	}
	switch compatibility.Authentication {
	case domain.AuthenticationRequirementNotRequired:
		plan.Authentication = domain.AuthenticationNotRequired
	case domain.AuthenticationRequirementRequired:
		plan.Authentication = domain.AuthenticationPending
	default:
		plan.Authentication = domain.AuthenticationNotChecked
		plan.Warnings = appendUnique(plan.Warnings, "authentication_requirement_unknown")
	}
	if compatibility.Package == "unsupported" {
		plan.Status = domain.PlanUnsupported
		plan.Activation = domain.ActivationFailed
		plan.Warnings = appendUnique(plan.Warnings, "catalog_client_unsupported")
		plan.UserActions = append(plan.UserActions, "choose a client marked compatible by the pinned catalog")
	} else if !catalogPackageMatches(compatibility.Package, plan.PackageMode) {
		plan.Status = domain.PlanUnsupported
		plan.Activation = domain.ActivationFailed
		plan.Warnings = appendUnique(plan.Warnings, "catalog_package_mode_mismatch")
		plan.UserActions = append(plan.UserActions, "update agentplugins or choose a client whose catalog package mode is supported")
	}
	if compatibility.Verification == "schema_only" || compatibility.Verification == "not_tested" {
		plan.Warnings = appendUnique(plan.Warnings, "catalog_"+compatibility.Verification)
	}
	if !hasTrustedRuntimePass(compatibility, plan.ClientID) && plan.Status != domain.PlanUnsupported {
		plan.Warnings = appendUnique(plan.Warnings, "catalog_runtime_not_tested")
		plan.UserActions = append(plan.UserActions, "verify the plugin in the selected client before relying on it")
	}
}

func hasTrustedRuntimePass(compatibility domain.CatalogCompatibility, client domain.ClientID) bool {
	for _, evidence := range compatibility.Evidence {
		if evidence.Level == "runtime" && evidence.Outcome == "passed" && evidence.Client == client && evidence.HasTrustedEligibilityProvenance() {
			return true
		}
	}
	return false
}

func catalogPackageMatches(value string, mode domain.PackageMode) bool {
	switch value {
	case "native":
		return mode == domain.PackageNative
	case "projected":
		return mode == domain.PackageProjection
	case "prepared":
		return mode == domain.PackagePrepared
	default:
		return false
	}
}

func appendUnique(values []string, value string) []string {
	return shared.AppendUnique(values, value)
}

func (planner Planner) ResolveTarget(
	ctx context.Context,
	client domain.DetectedClient,
	scope domain.InstallScope,
	physicalArtifactID string,
) (domain.DeliveryTarget, error) {
	if planner.Paths == nil {
		return domain.DeliveryTarget{}, errPathPolicyRequired
	}
	if planner.Registry == nil {
		return domain.DeliveryTarget{}, clients.ErrRegistryRequired
	}
	if err := ctx.Err(); err != nil {
		return domain.DeliveryTarget{}, err
	}
	if err := planner.Paths.ValidateLeafID(physicalArtifactID); err != nil {
		return domain.DeliveryTarget{}, fmt.Errorf("invalid physical artifact id: %w", err)
	}
	capabilities, ok := Capabilities(client.ClientID)
	if !ok {
		return domain.DeliveryTarget{}, fmt.Errorf("unsupported client %q", client.ClientID)
	}
	if !supportsScope(capabilities.Scopes, scope) {
		return domain.DeliveryTarget{}, fmt.Errorf("scope %q is unsupported for %s", scope, client.ClientID)
	}
	targetAnchor, targetRoot, err := planner.targetRoot(client, capabilities.PackageMode)
	if err != nil {
		return domain.DeliveryTarget{}, err
	}
	activePath := filepath.Join(targetRoot, physicalArtifactID)
	if err := planner.Paths.RequireContainedChild(targetRoot, activePath); err != nil {
		return domain.DeliveryTarget{}, fmt.Errorf("unsafe client target path: %w", err)
	}
	return domain.DeliveryTarget{TargetAnchor: targetAnchor, TargetRoot: targetRoot, ActivePath: activePath}, nil
}

func Capabilities(clientID domain.ClientID) (domain.ClientCapabilities, bool) {
	definition, ok := domain.ClientDefinitionFor(clientID)
	return definition.Capabilities, ok
}

// targetRoot asks the client where its package lives, and falls back to the
// managed root for every client that has no directory of its own. The
// containment check afterwards applies to both answers alike: an adapter
// chooses a location, it does not get to escape one.
func (planner Planner) targetRoot(client domain.DetectedClient, mode domain.PackageMode) (string, string, error) {
	if planner.Paths == nil {
		return "", "", errPathPolicyRequired
	}
	anchor, root, err := planner.clientTargetRoot(client, mode)
	if err != nil {
		return "", "", err
	}
	absoluteAnchor, err := filepath.Abs(anchor)
	if err != nil {
		return "", "", err
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	absoluteAnchor, absolute = filepath.Clean(absoluteAnchor), filepath.Clean(absolute)
	if err := planner.Paths.RequireContainedChild(absoluteAnchor, absolute); err != nil {
		return "", "", fmt.Errorf("unsafe client target root: %w", err)
	}
	return absoluteAnchor, absolute, nil
}

func (planner Planner) clientTargetRoot(client domain.DetectedClient, mode domain.PackageMode) (string, string, error) {
	if layout, ok := clients.As[clients.TargetLayout](planner.Registry, client.ClientID); ok {
		return layout.TargetRoot(client, mode, planner.ManagedRoot)
	}
	return shared.ManagedTargetRoot(client, mode, planner.ManagedRoot)
}

func componentDecisions(envelope domain.PackageEnvelope, capabilities domain.ClientCapabilities) []domain.ComponentDecision {
	decisions := make([]domain.ComponentDecision, 0, len(envelope.Skills)+len(envelope.MCP.Servers)+len(envelope.App.Bindings)+len(envelope.Manifest.Extensions))
	for _, name := range sortedKeys(envelope.Skills) {
		decisions = append(decisions, decision(domain.ComponentSkill, name, capabilities.SkillSupport))
	}
	for _, name := range sortedKeys(envelope.MCP.Servers) {
		decisions = append(decisions, mcpDecision(envelope, capabilities, name))
	}
	for _, name := range sortedKeys(envelope.App.Bindings) {
		decisions = append(decisions, decision(domain.ComponentApp, name, capabilities.AppSupport))
	}
	for _, name := range sortedKeys(envelope.Manifest.Extensions) {
		decisions = append(decisions, decision(domain.ComponentExtension, name, capabilities.ExtensionSupport))
	}
	return decisions
}

func mcpDecision(envelope domain.PackageEnvelope, capabilities domain.ClientCapabilities, name string) domain.ComponentDecision {
	server := envelope.MCP.Servers[name]
	support, ok := capabilities.MCPTransports[server.Type]
	if !ok {
		support = domain.SupportUnsupported
	}
	// A client that delivers app bindings reaches a mapped MCP server through
	// the binding instead of the transport, so the binding's support level is
	// what the server actually gets.
	if capabilities.AppSupport != domain.SupportUnsupported && envelope.App.Enabled {
		if _, mapped := envelope.App.Bindings[name]; mapped {
			support = capabilities.AppSupport
		}
	}
	value := decision(domain.ComponentMCPServer, name, support)
	if reason := unsupportedTransportReason(capabilities, server.Type, support); reason != "" {
		value.Reason = reason
	}
	return value
}

// unsupportedTransportReason names the transport when a client that does take
// MCP servers refuses this particular one, which is a more useful answer than
// the generic "component not supported by client". A client that takes no MCP
// transport at all keeps the generic reason.
func unsupportedTransportReason(capabilities domain.ClientCapabilities, transport string, support domain.SupportLevel) string {
	if transport != "sse" || support != domain.SupportUnsupported {
		return ""
	}
	for name, level := range capabilities.MCPTransports {
		if name != transport && level != domain.SupportUnsupported {
			return "declared_sse_not_supported_by_client"
		}
	}
	return ""
}

func decision(kind domain.ComponentKind, name string, support domain.SupportLevel) domain.ComponentDecision {
	if support == "" {
		support = domain.SupportUnsupported
	}
	value := domain.ComponentDecision{Kind: kind, Name: name, Support: support}
	if support == domain.SupportUnsupported {
		value.Reason = "component_not_supported_by_client"
	}
	return value
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func supportsScope(scopes []domain.InstallScope, requested domain.InstallScope) bool {
	for _, scope := range scopes {
		if scope == requested {
			return true
		}
	}
	return false
}

func statusFor(mode domain.ActivationMode) domain.PlanStatus {
	switch mode {
	case domain.ActivationAutomatic, domain.ActivationByClient:
		return domain.PlanReady
	case domain.ActivationByUser:
		return domain.PlanManualActivationRequired
	default:
		return domain.PlanUnsupported
	}
}

func activationFor(mode domain.ActivationMode) domain.ActivationState {
	switch mode {
	case domain.ActivationAutomatic, domain.ActivationByClient:
		return domain.ActivationActive
	case domain.ActivationByUser:
		return domain.ActivationManual
	default:
		return domain.ActivationFailed
	}
}

func hasComponents(decisions []domain.ComponentDecision) bool {
	return len(decisions) > 0
}

func hasSupportedComponent(decisions []domain.ComponentDecision) bool {
	for _, item := range decisions {
		if item.Support != domain.SupportUnsupported {
			return true
		}
	}
	return false
}
