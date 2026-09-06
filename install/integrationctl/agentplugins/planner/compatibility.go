package planner

import (
	"errors"
	"sort"

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
func Compatibility(envelope domain.PackageEnvelope, clients []domain.ClientID) ([]ClientCompatibility, error) {
	targets := make(map[domain.ClientID]domain.ClientCapabilities)
	for _, id := range clients {
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
		capabilities := targets[id]
		result = append(result, compatibilityFor(envelope, capabilities))
	}
	return result, nil
}

func compatibilityFor(envelope domain.PackageEnvelope, capabilities domain.ClientCapabilities) ClientCompatibility {
	result := ClientCompatibility{
		ClientID: capabilities.ClientID, Capabilities: capabilities,
		Components:  make([]ComponentCompatibility, 0),
		Limitations: []string{"static_adapter_support_only", "installation_not_checked", "authentication_not_checked", "runtime_not_checked", "client_version_not_checked", "catalog_publication_not_checked"},
	}
	if capabilities.ActivationMode == domain.ActivationByUser {
		result.Limitations = append(result.Limitations, "manual_activation_required")
	}
	if capabilities.ClientID == domain.ClientChatGPT {
		result.Limitations = append(result.Limitations, "chatgpt_manual_preparation_only", "remote_app_registration_not_checked")
		if (!envelope.App.Enabled && (len(envelope.MCP.Servers) > 0 || envelope.App.Present || envelope.App.Declared)) || len(missingChatGPTAppBindings(envelope)) > 0 {
			result.Limitations = append(result.Limitations, "chatgpt_app_binding_required")
		}
	}
	if capabilities.ClientID == domain.ClientWindsurf {
		result.Limitations = append(result.Limitations, "windsurf_channel_selection_required", "windsurf_skills_prepared_only")
	}
	// Reuse lifecycle component choices, then apply authoring-only uncertainty
	// bounds. No detected client or DeliveryPlan is constructed.
	decisions := componentDecisions(envelope, capabilities)
	invalid := make(map[domain.ComponentKind]map[string]bool)
	mark := func(kind domain.ComponentKind, name string) {
		if invalid[kind] == nil {
			invalid[kind] = make(map[string]bool)
		}
		invalid[kind][name] = true
	}
	for name := range envelope.MCP.InvalidServer {
		mark(domain.ComponentMCPServer, name)
	}
	for _, name := range envelope.Inventory.InvalidMCPServer {
		mark(domain.ComponentMCPServer, name)
	}
	for _, name := range envelope.Inventory.InvalidSkills {
		mark(domain.ComponentSkill, name)
	}
	for _, diagnostic := range envelope.Diagnostics {
		if diagnostic.Severity != domain.SeverityError {
			continue
		}
		result.Limitations = appendUnique(result.Limitations, "input_has_errors")
		var kind domain.ComponentKind
		switch diagnostic.Boundary {
		case domain.BoundarySkill:
			kind = domain.ComponentSkill
		case domain.BoundaryMCP, domain.BoundaryMCPServer:
			kind = domain.ComponentMCPServer
		case domain.BoundaryApp:
			kind = domain.ComponentApp
		case domain.BoundaryExtension:
			kind = domain.ComponentExtension
		}
		if kind != "" {
			mark(kind, diagnostic.Item)
		}
	}
	for kind, names := range invalid {
		for name := range names {
			if name == "" {
				continue
			}
			found := false
			for _, item := range decisions {
				if item.Kind == kind && item.Name == name {
					found = true
					break
				}
			}
			if !found {
				decisions = append(decisions, decision(kind, name, domain.SupportUnsupported))
			}
		}
	}
	sort.Slice(decisions, func(i, j int) bool {
		if decisions[i].Kind != decisions[j].Kind {
			return decisions[i].Kind < decisions[j].Kind
		}
		return decisions[i].Name < decisions[j].Name
	})
	counts := make(map[domain.ComponentKind]int)
	for _, item := range decisions {
		counts[item.Kind]++
		component := ComponentCompatibility{Kind: item.Kind, Index: counts[item.Kind], Support: item.Support}
		reject := func(code string) {
			component.Support = domain.SupportUnsupported
			component.Limitations = appendUnique(component.Limitations, code)
		}
		if item.Reason != "" {
			component.Limitations = append(component.Limitations, item.Reason)
		}
		if invalid[item.Kind][item.Name] || invalid[item.Kind][""] {
			reject("invalid_component")
		}
		switch item.Kind {
		case domain.ComponentExtension:
			// The registry has no namespace-specific interpreter contract.
			reject("extension_semantics_not_established")
		case domain.ComponentSkill:
			if envelope.Inventory.InvalidSkillsRoot {
				reject("invalid_component")
			}
		case domain.ComponentMCPServer:
			if envelope.MCP.Present && !envelope.MCP.Enabled {
				reject("component_disabled")
			}
			if capabilities.ClientID == domain.ClientChatGPT {
				server := envelope.MCP.Servers[item.Name]
				binding, mapped := envelope.App.Bindings[item.Name]
				if !envelope.App.Enabled || !mapped || binding.ID == "" {
					reject("chatgpt_app_binding_required")
				} else if server.Type != "streamable-http" && server.Type != "sse" {
					reject("chatgpt_remote_mcp_required")
				} else {
					component.Limitations = append(component.Limitations, "remote_app_registration_not_checked")
				}
			}
		case domain.ComponentApp:
			if !envelope.App.Enabled || envelope.App.Bindings[item.Name].ID == "" {
				reject("invalid_component")
			}
			if capabilities.ClientID == domain.ClientChatGPT {
				component.Limitations = append(component.Limitations, "remote_app_registration_not_checked")
			}
		}
		result.Components = append(result.Components, component)
	}
	sort.Strings(result.Limitations)
	return result
}
