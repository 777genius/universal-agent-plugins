package domain

import (
	"reflect"
	"testing"
)

func TestParseClientIDAliasesAndLenientPassThrough(t *testing.T) {
	canonical := map[string]ClientID{
		"codex": ClientCodex, "chatgpt": ClientChatGPT, "cursor": ClientCursor,
		"copilot": ClientCopilot, "vscode": ClientVSCode, "kiro": ClientKiro,
		"claude": ClientClaude, "gemini": ClientGemini, "opencode": ClientOpenCode,
		"cline": ClientCline, "windsurf": ClientWindsurf,
		"grok": ClientGrok, "kimi": ClientKimi,
	}
	for input, want := range canonical {
		got, ok := ParseClientID(input)
		if !ok || got != want {
			t.Errorf("ParseClientID(%q) = %q, %v, want %q, true", input, got, ok, want)
		}
	}
	aliases := map[string]ClientID{
		"github-copilot": ClientCopilot, "vs-code": ClientVSCode, "claude-code": ClientClaude,
		"gemini-cli": ClientGemini, "open-code": ClientOpenCode, "devin": ClientWindsurf,
		"  CURSOR  ": ClientCursor, "Claude-Code": ClientClaude,
		"grok-build": ClientGrok, "kimi-code": ClientKimi,
	}
	for input, want := range aliases {
		got, ok := ParseClientID(input)
		if !ok || got != want {
			t.Errorf("ParseClientID(%q) = %q, %v, want %q, true", input, got, ok, want)
		}
	}
	got, ok := ParseClientID("Not-A-Client")
	if ok || got != ClientID("not-a-client") {
		t.Fatalf("unknown name = %q, %v, want not-a-client, false", got, ok)
	}
	got, ok = ParseClientID("  Zed  ")
	if ok || got != ClientID("zed") {
		t.Fatalf("unknown name = %q, %v, want zed, false", got, ok)
	}
}

func TestClientRegistryHasStableOrderAndSharedCopilotBackend(t *testing.T) {
	want := []ClientID{
		ClientCodex, ClientChatGPT, ClientCursor, ClientCopilot, ClientVSCode, ClientKiro,
		ClientClaude, ClientGemini, ClientOpenCode, ClientCline, ClientWindsurf, ClientGrok, ClientKimi,
	}
	if got := SupportedClientIDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("supported clients = %v, want %v", got, want)
	}
	if !SameClientBackend(ClientCopilot, ClientVSCode) || SameClientBackend(ClientCursor, ClientVSCode) {
		t.Fatal("Copilot / VS Code backend family contract changed")
	}
	if !SharesBackend(ClientCopilot, ClientVSCode) || SharesBackend(ClientCursor, ClientVSCode) {
		t.Fatal("SharesBackend must match SameClientBackend")
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
	definitions[0].Traits.InstallIntents[0] = InstallIntentPrepare
	definition, ok = ClientDefinitionFor(ClientCodex)
	if !ok || definition.Traits.Allows(InstallIntentPrepare) {
		t.Fatal("traits install intents were mutated through returned copy")
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

// TestDirectoryPreparationPurposesAreDeclared keeps the table's bounded resolve
// purposes joined to the constants the Directory eligibility rules compare
// against. A typo here would silently declare a purpose nobody honors, and the
// client would simply stop being eligible for preparation.
func TestDirectoryPreparationPurposesAreDeclared(t *testing.T) {
	known := map[DirectoryResolvePurpose]bool{DirectoryResolveContext7ChatGPTPreparation: true}
	declared := 0
	for _, definition := range ClientDefinitions() {
		if definition.DirectoryPreparationPurpose == "" {
			continue
		}
		declared++
		if !known[definition.DirectoryPreparationPurpose] {
			t.Errorf("client %q declares unknown resolve purpose %q", definition.ID, definition.DirectoryPreparationPurpose)
		}
	}
	if declared != len(known) {
		t.Fatalf("%d clients declare a preparation purpose, but %d purposes exist", declared, len(known))
	}
}
