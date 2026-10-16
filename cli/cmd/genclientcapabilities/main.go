// Command genclientcapabilities derives landing metadata from the SDK registry.
// Run from cli: go run ./cmd/genclientcapabilities
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	clientregistry "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

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
}

func generate() ([]byte, error) {
	registry := clientregistry.Default()
	definitions := domain.ClientDefinitions()
	metadata := make([]clientCapability, 0, len(definitions))
	for _, definition := range definitions {
		capabilities := definition.Capabilities
		supported := installer.ClientSupported(registry, string(definition.ID))
		reason := installer.ClientSupportExclusionReason(registry, string(definition.ID))
		metadata = append(metadata, clientCapability{
			ID: definition.ID, DisplayName: definition.DisplayName,
			PackageMode: capabilities.PackageMode, SkillSupport: capabilities.SkillSupport,
			MCPTransports: capabilities.MCPTransports, AppSupport: capabilities.AppSupport,
			ExtensionSupport: capabilities.ExtensionSupport,
			SavedDependency:  supported, ExcludedReason: reason,
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
