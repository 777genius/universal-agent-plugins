package domain

import "testing"

func TestCodexTransportCapabilities(t *testing.T) {
	d, ok := ClientDefinitionFor(ClientCodex)
	if !ok {
		t.Fatal("missing Codex definition")
	}
	for transport, want := range map[string]SupportLevel{"sse": SupportUnsupported, "stdio": SupportProjected, "streamable-http": SupportProjected} {
		if got := d.Capabilities.MCPTransports[transport]; got != want {
			t.Fatalf("%s: %s, want %s", transport, got, want)
		}
	}
	d.Capabilities.MCPTransports["sse"] = SupportNative
	fresh, _ := ClientDefinitionFor(ClientCodex)
	if fresh.Capabilities.MCPTransports["sse"] != SupportUnsupported {
		t.Fatal("caller mutated registry")
	}
	for _, id := range []ClientID{ClientClaude, ClientCursor, ClientCopilot, ClientGemini, ClientCline, ClientWindsurf} {
		other, _ := ClientDefinitionFor(id)
		if other.Capabilities.MCPTransports["sse"] == SupportUnsupported {
			t.Fatalf("unrelated SSE capability narrowed: %s", id)
		}
	}
}
