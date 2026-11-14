// Command genclientcapabilities derives landing metadata from the SDK registry.
// Run from cli: go run ./cmd/genclientcapabilities
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	clientregistry "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Project metadata comes from the selected scoped adapter, not User support.
// It predicts potential delivery only; final source, transport and host policy
// checks remain inside the published CLI.
type projectCapability struct {
	SkillSupport     domain.SupportLevel            `json:"skill_support"`
	MCPTransports    map[string]domain.SupportLevel `json:"mcp_transports"`
	ExtensionSupport domain.SupportLevel            `json:"extension_support"`
}

type clientCapability struct {
	ID               domain.ClientID                `json:"id"`
	DisplayName      string                         `json:"display_name"`
	PackageMode      domain.PackageMode             `json:"package_mode"`
	SkillSupport     domain.SupportLevel            `json:"skill_support"`
	MCPTransports    map[string]domain.SupportLevel `json:"mcp_transports"`
	AppSupport       domain.SupportLevel            `json:"app_support"`
	ExtensionSupport domain.SupportLevel            `json:"extension_support"`
	SavedDependency  bool                           `json:"saved_dependency"`
	ExcludedReason   string                         `json:"excluded_reason"`
	Project          *projectCapability             `json:"project,omitempty"`
}

func generate() ([]byte, error) {
	registry := clientregistry.Default()
	projectRegistry := clientregistry.Project()
	definitions := domain.ClientDefinitions()
	metadata := make([]clientCapability, 0, len(definitions))
	for _, definition := range definitions {
		capabilities := definition.Capabilities
		supported := installer.ClientSupported(registry, string(definition.ID))

		reason := installer.ClientSupportExclusionReason(registry, string(definition.ID))
		var project *projectCapability
		if clients.SupportsScope(projectRegistry, definition.ID, domain.ScopeProject) {
			scoped := definition.ScopedCapabilities(domain.ScopeProject)
			extensions := domain.SupportUnsupported
			if _, ok := clients.As[clients.ExtensionInterpreter](projectRegistry, definition.ID); ok {
				extensions = scoped.ExtensionSupport
			}
			project = &projectCapability{
				SkillSupport:     scoped.SkillSupport,
				MCPTransports:    scoped.MCPTransports,
				ExtensionSupport: extensions,
			}
		}
		metadata = append(metadata, clientCapability{
			ID: definition.ID, DisplayName: definition.DisplayName,
			PackageMode: capabilities.PackageMode, SkillSupport: capabilities.SkillSupport,
			MCPTransports: capabilities.MCPTransports, AppSupport: capabilities.AppSupport,
			ExtensionSupport: capabilities.ExtensionSupport,
			SavedDependency:  supported, ExcludedReason: reason, Project: project,
		})
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func main() {
	output := flag.String("output", "../landing/data/clientCapabilities.generated.json", "generated metadata path")
	flag.Parse()
	data, err := generate()
	if err == nil {
		err = os.WriteFile(*output, data, 0o644)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
