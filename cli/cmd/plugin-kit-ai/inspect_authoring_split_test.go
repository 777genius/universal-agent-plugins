package main

import (
	"slices"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestInspectAuthoringPath_MixedPortableMCP(t *testing.T) {
	t.Parallel()

	report := pluginmanifest.Inspection{
		Portable: pluginmodel.PortableComponents{
			MCP: &pluginmodel.PortableMCP{
				File: &pluginmodel.PortableMCPFile{
					Servers: map[string]pluginmodel.PortableMCPServer{
						"local":  {Type: "stdio"},
						"remote": {Type: "remote"},
					},
				},
			},
		},
	}
	if got := inspectAuthoringPath(report); got != "connect online services and local tools" {
		t.Fatalf("inspectAuthoringPath() = %q", got)
	}
}

func TestAuthoredGeneratedOutputs_OrdersGuidesBeforeArtifacts(t *testing.T) {
	t.Parallel()

	report := pluginmanifest.Inspection{
		Layout: pluginmanifest.InspectLayout{
			GeneratedOutputs: []string{"AGENTS.md", ".mcp.json", "README.md", "AGENTS.md"},
			BoundaryDocs:     []string{"CLAUDE.md"},
			GeneratedGuide:   "GENERATED.md",
		},
	}
	guides, outputs := authoredGeneratedOutputs(report)
	if !slices.Equal(guides, []string{"README.md", "CLAUDE.md", "AGENTS.md", "GENERATED.md"}) {
		t.Fatalf("guides = %v", guides)
	}
	if !slices.Equal(outputs, []string{".mcp.json"}) {
		t.Fatalf("outputs = %v", outputs)
	}
}
