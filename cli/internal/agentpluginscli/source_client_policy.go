package agentpluginscli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/catalog"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func prepareLoadedPackageForClient(loaded *loadedPackage, clientID domain.ClientID) error {
	if loaded == nil || !requiresPersonalMapping(clientID) {
		return nil
	}
	if loaded.chatGPTPreparation {
		if loaded.localChatGPTMapping == nil {
			return fmt.Errorf("action_required: %s", domain.ChatGPTRegistrationAction)
		}
		mapping := *loaded.localChatGPTMapping
		if err := mapping.ValidatePackage(loaded.envelope); err != nil {
			return err
		}
		if loaded.envelope.App.Present || loaded.envelope.App.Declared {
			return fmt.Errorf("refuse to replace publisher app mapping with personal registration")
		}
		raw, err := json.Marshal(map[string]any{"apps": map[string]any{mapping.Server: map[string]string{"id": mapping.AppID}}})
		if err != nil {
			return err
		}
		entryRaw, _ := json.Marshal(map[string]string{"id": mapping.AppID})
		loaded.envelope.LocalChatGPTMapping = &mapping
		loaded.envelope.App = domain.AppComponent{Present: true, Declared: true, Enabled: true, Raw: raw, Bindings: map[string]domain.AppBinding{mapping.Server: {Alias: mapping.Server, ID: mapping.AppID, Raw: entryRaw}}}
		loaded.envelope.Inventory.AppPresent = true
		loaded.envelope.Inventory.AppBindings = []string{mapping.Server}
		return nil
	}
	compatibility, ok := loaded.hints.Compatibility[string(clientID)]
	if !ok || compatibility.AppBinding == nil {
		return nil
	}
	binding := *compatibility.AppBinding
	validateBinding := catalog.ValidateAppBinding
	if loaded.origin == domain.OriginModeDirectory {
		validateBinding = catalog.ValidateAppBindingReference
	}
	if err := validateBinding(binding); err != nil {
		return fmt.Errorf("Directory ChatGPT app binding is invalid: %w", err)
	}
	server, ok := loaded.envelope.MCP.Servers[binding.MCPServer]
	if !ok || !loaded.envelope.MCP.Enabled {
		return fmt.Errorf("Directory ChatGPT app binding references missing MCP server %q", binding.MCPServer)
	}
	serverURL, ok := server.Decoded["url"].(string)
	if !ok || serverURL != binding.MCPURL {
		return fmt.Errorf("Directory ChatGPT app binding URL does not match MCP server %q", binding.MCPServer)
	}
	if loaded.envelope.App.Present {
		existing, matches := loaded.envelope.App.Bindings[binding.AppKey]
		if !loaded.envelope.App.Enabled || !matches || len(loaded.envelope.App.Bindings) != 1 || existing.ID != binding.ID {
			return fmt.Errorf("package .app.json does not exactly match the Directory ChatGPT app binding")
		}
		return nil
	}
	entry := struct {
		ID string `json:"id"`
	}{ID: binding.ID}
	document := struct {
		Apps map[string]any `json:"apps"`
	}{Apps: map[string]any{binding.AppKey: entry}}
	raw, err := json.Marshal(document)
	if err != nil {
		return err
	}
	entryRaw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	loaded.envelope.App = domain.AppComponent{Present: true, Declared: true, Enabled: true, Raw: raw, Bindings: map[string]domain.AppBinding{binding.AppKey: {Alias: binding.AppKey, ID: binding.ID, Raw: entryRaw}}}
	loaded.envelope.Inventory.AppPresent = true
	loaded.envelope.Inventory.AppBindings = []string{binding.AppKey}
	return nil
}

func restoreCatalogEvidence(loaded *loadedPackage, binding domain.ClientBinding) {
	if loaded == nil || binding.PackageRevision == nil {
		return
	}
	// Directory acquisition has already composed compatibility evidence for the
	// currently detected installer, client, platform, and dependency tuple. The
	// recorded copy identifies what was applicable when this binding was last
	// written and must not replace that freshly filtered evidence during repair.
	if loaded.origin == domain.OriginModeDirectory && loaded.envelope.CatalogEvidence != nil {
		return
	}
	if binding.PackageRevision.CatalogEvidence == nil {
		loaded.envelope.CatalogEvidence = nil
		loaded.hints.Compatibility = nil
		return
	}
	evidence := *binding.PackageRevision.CatalogEvidence
	evidence.Compatibility = cloneCatalogCompatibility(evidence.Compatibility)
	loaded.envelope.CatalogEvidence = &evidence
	loaded.hints.Compatibility = cloneCatalogCompatibility(evidence.Compatibility)
}

func isDirectorySelector(value string) bool {
	if isShortName(value) {
		return true
	}
	parts := strings.Split(value, "/")
	return len(parts) == 2 && isShortName(parts[0]) && isShortName(parts[1])
}

func isShortName(value string) bool {
	return shortNamePattern.MatchString(value) && !strings.ContainsAny(value, `/\\`)
}

func explicitLocalPath(value string) bool {
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") || strings.HasPrefix(value, `.\`) || strings.HasPrefix(value, `..\`) {
		return true
	}
	// Classify syntax independently of the host OS, including Unix roots and
	// only genuinely absolute Windows spellings, so copied sources are stable;
	// drive-relative forms such as C:plugin intentionally remain selectors.
	if len(value) >= 3 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':' && (value[2] == '\\' || value[2] == '/') {
		return true
	}
	if !strings.HasPrefix(value, `\\`) {
		return false
	}
	unc := strings.TrimPrefix(value, `\\`)
	server, share, ok := strings.Cut(unc, `\`)
	return ok && server != "" && strings.Trim(share, `\`) != ""
}
