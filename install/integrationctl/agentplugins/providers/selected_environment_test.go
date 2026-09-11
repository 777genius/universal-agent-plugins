package providers

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"testing"
)

func TestReservedEnvironmentValidationUsesSelectedMCPSet(t *testing.T) {
	envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{
		"unselected": {Type: "stdio", Decoded: map[string]any{"env": map[string]any{"PLUGIN_ROOT": "authored"}}},
		"remote":     {Type: "http"},
	}}}
	plan := domain.DeliveryPlan{Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "unselected", Support: domain.SupportUnsupported}, {Kind: domain.ComponentMCPServer, Name: "remote", Support: domain.SupportNative}}}
	if err := validateReservedStdioEnvironment(envelope, plan); err != nil {
		t.Fatal(err)
	}
	plan.Components[0].Support = domain.SupportNative
	if err := validateReservedStdioEnvironment(envelope, plan); err == nil {
		t.Fatal("selected reserved environment accepted")
	}
}
