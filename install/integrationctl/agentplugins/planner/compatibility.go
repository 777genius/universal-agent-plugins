package planner

import (
	"errors"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// ErrUnknownCompatibilityClient is deliberately independent of the supplied
// target text, which may contain secrets or an arbitrarily large string.
var ErrUnknownCompatibilityClient = errors.New("unknown compatibility client")

// ComponentCompatibility describes static adapter support, never activation.
// Index is one-based within Kind, ordered by the input's component map keys
// (including invalid inventory entries). Names are intentionally not returned:
// even malformed component keys and diagnostic items can contain secrets/paths.
type ComponentCompatibility struct {
	Kind        domain.ComponentKind `json:"kind"`
	Index       int                  `json:"index"`
	Support     domain.SupportLevel  `json:"support"`
	Limitations []string             `json:"limitations,omitempty"`
}

// ClientCompatibility separates registry metadata from conservative decisions
// about this envelope. Capabilities are an independent copy of the registry;
// its generic extension support does not establish any namespace's semantics.
// Limitations are fixed codes, never raw diagnostics or configuration data.
type ClientCompatibility struct {
	ClientID     domain.ClientID           `json:"client_id"`
	Capabilities domain.ClientCapabilities `json:"capabilities"`
	Components   []ComponentCompatibility  `json:"components"`
	Limitations  []string                  `json:"limitations"`
}

// Compatibility evaluates explicitly requested clients without host discovery,
// I/O, lifecycle planning, or catalog/trust evaluation. Duplicate targets are
// collapsed and results sorted by ClientID; an empty list returns an empty
// result. Any unknown target fails the whole request with a fixed error.
//
// The envelope is already interpreted input, not a request to decode or validate
// a package. Component-only envelopes are accepted. Known component errors are
// retained without suppressing valid siblings. Neither a support level nor
// registry activation metadata is evidence of installation, authentication,
// OAuth, runtime readiness, client version compatibility, or publication.
func Compatibility(registry *clients.Registry, envelope domain.PackageEnvelope, targetIDs []domain.ClientID) ([]ClientCompatibility, error) {
	if registry == nil {
		return nil, clients.ErrRegistryRequired
	}
	targets := make(map[domain.ClientID]domain.ClientCapabilities)
	for _, id := range targetIDs {
		if _, exists := targets[id]; exists {
			continue
		}
		capabilities, ok := Capabilities(id)
		if !ok {
			return nil, ErrUnknownCompatibilityClient
		}
		targets[id] = capabilities
	}
	result := make([]ClientCompatibility, 0, len(targets))
	ids := make([]domain.ClientID, 0, len(targets))
	for id := range targets {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		limiter, _ := clients.As[clients.CompatibilityLimiter](registry, id)
		result = append(result, compatibilityFor(envelope, targets[id], limiter))
	}
	return result, nil
}

func compatibilityFor(envelope domain.PackageEnvelope, capabilities domain.ClientCapabilities, limiter clients.CompatibilityLimiter) ClientCompatibility {
	result := newClientCompatibility(capabilities)
	if capabilities.ActivationMode == domain.ActivationByUser {
		result.Limitations = append(result.Limitations, "manual_activation_required")
	}
	if limiter != nil {
		result.Limitations = append(result.Limitations, limiter.ClientLimitations(envelope)...)
	}
	// Reuse lifecycle component choices, then apply authoring-only uncertainty
	// bounds. No detected client or DeliveryPlan is constructed.
	decisions, invalid, invalidKind := compatibilityInventory(envelope, capabilities)
	result.Limitations = applyErrorDiagnostics(envelope, invalid, invalidKind, result.Limitations)
	includeMissingInvalid(&decisions, invalid)
	sortCompatibilityDecisions(decisions)
	result.Components = compatibilityComponents(envelope, limiter, decisions, invalid, invalidKind)
	sort.Strings(result.Limitations)
	return result
}

func newClientCompatibility(capabilities domain.ClientCapabilities) ClientCompatibility {
	return ClientCompatibility{
		ClientID: capabilities.ClientID, Capabilities: capabilities,
		Components:  make([]ComponentCompatibility, 0),
		Limitations: []string{"static_adapter_support_only", "installation_not_checked", "authentication_not_checked", "runtime_not_checked", "client_version_not_checked", "catalog_publication_not_checked"},
	}
}

func compatibilityInventory(envelope domain.PackageEnvelope, capabilities domain.ClientCapabilities) ([]domain.ComponentDecision, map[domain.ComponentKind]map[string]bool, map[domain.ComponentKind]bool) {
	invalid, invalidKind := collectInvalidComponents(envelope)
	return componentDecisions(envelope, capabilities), invalid, invalidKind
}

func collectInvalidComponents(envelope domain.PackageEnvelope) (map[domain.ComponentKind]map[string]bool, map[domain.ComponentKind]bool) {
	invalid := make(map[domain.ComponentKind]map[string]bool)
	// Empty names are exact item identities, never a kind-wide sentinel.
	invalidKind := map[domain.ComponentKind]bool{
		domain.ComponentSkill: envelope.Inventory.InvalidSkillsRoot,
	}
	for name := range envelope.MCP.InvalidServer {
		markInvalidComponent(invalid, domain.ComponentMCPServer, name)
	}
	for _, name := range envelope.Inventory.InvalidMCPServer {
		markInvalidComponent(invalid, domain.ComponentMCPServer, name)
	}
	for _, name := range envelope.Inventory.InvalidSkills {
		markInvalidComponent(invalid, domain.ComponentSkill, name)
	}
	return invalid, invalidKind
}

func markInvalidComponent(invalid map[domain.ComponentKind]map[string]bool, kind domain.ComponentKind, name string) {
	if invalid[kind] == nil {
		invalid[kind] = make(map[string]bool)
	}
	invalid[kind][name] = true
}

func applyErrorDiagnostics(envelope domain.PackageEnvelope, invalid map[domain.ComponentKind]map[string]bool, invalidKind map[domain.ComponentKind]bool, limitations []string) []string {
	for _, diagnostic := range envelope.Diagnostics {
		if diagnostic.Severity != domain.SeverityError {
			continue
		}
		limitations = appendUnique(limitations, "input_has_errors")
		kind, kindWide := compatibilityDiagnosticKind(diagnostic.Boundary)
		if kindWide {
			invalidKind[domain.ComponentMCPServer] = true
			continue
		}
		if kind != "" {
			markInvalidComponent(invalid, kind, diagnostic.Item)
		}
	}
	return limitations
}

func sortCompatibilityDecisions(decisions []domain.ComponentDecision) {
	sort.Slice(decisions, func(i, j int) bool {
		if decisions[i].Kind != decisions[j].Kind {
			return decisions[i].Kind < decisions[j].Kind
		}
		return decisions[i].Name < decisions[j].Name
	})
}

func compatibilityDiagnosticKind(boundary domain.FailureBoundary) (domain.ComponentKind, bool) {
	switch boundary {
	case domain.BoundarySkill:
		return domain.ComponentSkill, false
	case domain.BoundaryMCP:
		// This boundary describes the MCP document, not a server key.
		return "", true
	case domain.BoundaryMCPServer:
		return domain.ComponentMCPServer, false
	case domain.BoundaryApp:
		return domain.ComponentApp, false
	case domain.BoundaryExtension:
		return domain.ComponentExtension, false
	}
	return "", false
}

func includeMissingInvalid(decisions *[]domain.ComponentDecision, invalid map[domain.ComponentKind]map[string]bool) {
	for kind, names := range invalid {
		for name := range names {
			found := false
			for _, item := range *decisions {
				if item.Kind == kind && item.Name == name {
					found = true
					break
				}
			}
			if !found {
				*decisions = append(*decisions, decision(kind, name, domain.SupportUnsupported))
			}
		}
	}
}

func compatibilityComponents(envelope domain.PackageEnvelope, limiter clients.CompatibilityLimiter, decisions []domain.ComponentDecision, invalid map[domain.ComponentKind]map[string]bool, invalidKind map[domain.ComponentKind]bool) []ComponentCompatibility {
	counts := make(map[domain.ComponentKind]int)
	components := make([]ComponentCompatibility, 0, len(decisions))
	for _, item := range decisions {
		components = append(components, refineComponentCompatibility(envelope, limiter, item, invalid, invalidKind, counts))
	}
	return components
}

func refineComponentCompatibility(envelope domain.PackageEnvelope, limiter clients.CompatibilityLimiter, item domain.ComponentDecision, invalid map[domain.ComponentKind]map[string]bool, invalidKind map[domain.ComponentKind]bool, counts map[domain.ComponentKind]int) ComponentCompatibility {
	counts[item.Kind]++
	component := ComponentCompatibility{Kind: item.Kind, Index: counts[item.Kind], Support: item.Support}
	reject := func(code string) {
		component.Support = domain.SupportUnsupported
		component.Limitations = appendUnique(component.Limitations, code)
	}
	if item.Reason != "" {
		component.Limitations = append(component.Limitations, item.Reason)
	}
	if invalid[item.Kind][item.Name] || invalidKind[item.Kind] {
		reject("invalid_component")
	}
	applyKindCompatibility(envelope, item, reject)
	if limiter != nil {
		rejected, notes := limiter.ComponentLimitations(envelope, item)
		for _, code := range rejected {
			reject(code)
		}
		component.Limitations = append(component.Limitations, notes...)
	}
	return component
}

func applyKindCompatibility(envelope domain.PackageEnvelope, item domain.ComponentDecision, reject func(string)) {
	switch item.Kind {
	case domain.ComponentExtension:
		// The registry has no namespace-specific interpreter contract.
		reject("extension_semantics_not_established")
	case domain.ComponentMCPServer:
		if envelope.MCP.Present && !envelope.MCP.Enabled {
			reject("component_disabled")
		}
	case domain.ComponentApp:
		if !envelope.App.Enabled || envelope.App.Bindings[item.Name].ID == "" {
			reject("invalid_component")
		}
	}
}
