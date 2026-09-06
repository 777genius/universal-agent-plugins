package domain

import "testing"

func TestOpenCodeDeclaredSSEUnsupportedAndCapabilitiesIsolated(t *testing.T) {
	for _, definition := range ClientDefinitions() {
		got := definition.Capabilities.MCPTransports
		if definition.ID == ClientOpenCode {
			if got["sse"] != SupportUnsupported || got["stdio"] == SupportUnsupported || got["streamable-http"] == SupportUnsupported {
				t.Fatalf("OpenCode transports: %v", got)
			}
		} else if (definition.ID == ClientCline || definition.ID == ClientWindsurf || definition.ID == ClientGemini) && got["sse"] == SupportUnsupported {
			t.Fatalf("SSE accidentally disabled for %s", definition.ID)
		}
	}
	d, _ := ClientDefinitionFor(ClientOpenCode)
	d.Capabilities.MCPTransports["sse"] = SupportNative
	fresh, _ := ClientDefinitionFor(ClientOpenCode)
	if fresh.Capabilities.MCPTransports["sse"] != SupportUnsupported {
		t.Fatal("capability map is shared")
	}
}
