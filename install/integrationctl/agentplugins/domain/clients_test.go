package domain

import (
	"reflect"
	"testing"
)

func TestClientRegistryHasStableOrderAndSharedCopilotBackend(t *testing.T) {
	want := []ClientID{
		ClientCodex, ClientChatGPT, ClientCursor, ClientCopilot, ClientVSCode, ClientKiro,
		ClientClaude, ClientGemini, ClientOpenCode, ClientCline, ClientWindsurf,
	}
	if got := SupportedClientIDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("supported clients = %v, want %v", got, want)
	}
	if !SameClientBackend(ClientCopilot, ClientVSCode) || SameClientBackend(ClientCursor, ClientVSCode) {
		t.Fatal("Copilot / VS Code backend family contract changed")
	}
}

func TestClientRegistryReturnsDefensiveCapabilityCopies(t *testing.T) {
	definitions := ClientDefinitions()
	definitions[0].Capabilities.Scopes[0] = ScopeProject
	definitions[0].Capabilities.MCPTransports["stdio"] = SupportUnsupported
	definition, ok := ClientDefinitionFor(ClientCodex)
	if !ok || definition.Capabilities.Scopes[0] != ScopeUser || definition.Capabilities.MCPTransports["stdio"] != SupportProjected {
		t.Fatalf("registry was mutated through returned copy: %+v", definition)
	}
}

func TestNewClientsUsePreparedReadOnlyFoundation(t *testing.T) {
	for _, id := range []ClientID{ClientWindsurf} {
		definition, ok := ClientDefinitionFor(id)
		if !ok || definition.Capabilities.PackageMode != PackagePrepared || definition.DirectoryDelivery != "prepared" || definition.LegacyCatalogRequired {
			t.Fatalf("new client definition %s = %+v", id, definition)
		}
	}
	cline, ok := ClientDefinitionFor(ClientCline)
	if !ok || cline.Capabilities.PackageMode != PackageNative || cline.Capabilities.ActivationMode != ActivationAutomatic || cline.DirectoryDelivery != "managed" || cline.LegacyCatalogRequired {
		t.Fatalf("Cline native lifecycle definition = %+v", cline)
	}
	claude, ok := ClientDefinitionFor(ClientClaude)
	if !ok || claude.Capabilities.PackageMode != PackageProjection || claude.Capabilities.ActivationMode != ActivationAutomatic || claude.DirectoryDelivery != "managed" || claude.LegacyCatalogRequired {
		t.Fatalf("Claude native lifecycle definition = %+v", claude)
	}
	openCode, ok := ClientDefinitionFor(ClientOpenCode)
	if !ok || openCode.Capabilities.PackageMode != PackagePrepared || openCode.Capabilities.ActivationMode != ActivationAutomatic || openCode.DirectoryDelivery != "managed" || openCode.LegacyCatalogRequired {
		t.Fatalf("OpenCode native lifecycle definition = %+v", openCode)
	}
	gemini, ok := ClientDefinitionFor(ClientGemini)
	if !ok || gemini.Capabilities.PackageMode != PackageNative || gemini.DirectoryDelivery != "managed" || gemini.CatalogPackage != "native" || gemini.LegacyCatalogRequired {
		t.Fatalf("Gemini client definition = %+v", gemini)
	}
}
