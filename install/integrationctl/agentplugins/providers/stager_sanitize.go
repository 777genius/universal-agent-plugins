package providers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func sanitizePackage(paths ports.PathPolicy, root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	if err := removeUnsupportedPortableHooks(paths, root); err != nil {
		return err
	}
	if err := removeInvalidAndUnsupportedSkills(paths, root, envelope, plan); err != nil {
		return err
	}
	if err := writeSanitizedMCP(root, envelope, plan); err != nil {
		return err
	}
	if err := removeStagedApp(root); err != nil {
		return err
	}
	return writeSanitizedExtensions(root, envelope, plan)
}

func removeUnsupportedPortableHooks(paths ports.PathPolicy, root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("inspect staged package root: %w", err)
	}
	for _, entry := range entries {
		if !strings.EqualFold(entry.Name(), "hooks") {
			continue
		}
		candidate := filepath.Join(root, entry.Name())
		if err := paths.RequireContainedChild(root, candidate); err != nil {
			return fmt.Errorf("unsafe staged hooks path: %w", err)
		}
		if err := os.RemoveAll(candidate); err != nil {
			return fmt.Errorf("remove unsupported staged hooks: %w", err)
		}
	}
	return nil
}

func removeStagedApp(root string) error {
	path := filepath.Join(root, ".app.json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unsupported .app.json: %w", err)
	}
	return nil
}

func removeInvalidAndUnsupportedSkills(paths ports.PathPolicy, root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	names := append([]string(nil), envelope.Inventory.InvalidSkills...)
	for _, component := range plan.Components {
		if component.Kind == domain.ComponentSkill && component.Support == domain.SupportUnsupported {
			names = append(names, component.Name)
		}
	}
	skillsRoot := filepath.Join(root, "skills")
	if envelope.Inventory.InvalidSkillsRoot {
		if err := paths.RequireExactPath(filepath.Join(root, "skills"), skillsRoot); err != nil {
			return err
		}
		if err := os.RemoveAll(skillsRoot); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove invalid skills root: %w", err)
		}
		return nil
	}
	for _, name := range names {
		candidate := filepath.Join(skillsRoot, name)
		if filepath.Dir(filepath.Clean(candidate)) != filepath.Clean(skillsRoot) {
			return fmt.Errorf("unsafe invalid skill path %q", name)
		}
		if err := paths.RequireContainedChild(skillsRoot, candidate); err != nil {
			return fmt.Errorf("unsafe invalid skill path %q: %w", name, err)
		}
		if err := os.RemoveAll(candidate); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove skipped skill %q: %w", name, err)
		}
	}
	return nil
}

func writeSanitizedMCP(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	path := filepath.Join(root, "mcp.json")
	if !envelope.MCP.Present || !envelope.MCP.Enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove disabled mcp.json: %w", err)
		}
		return nil
	}
	supported := shared.SupportedMCPNames(plan)
	servers := make(map[string]json.RawMessage, len(supported))
	for _, name := range supported {
		server, ok := envelope.MCP.Servers[name]
		if ok {
			servers[name] = append(json.RawMessage(nil), server.Raw...)
		}
	}
	if len(servers) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove unsupported mcp.json: %w", err)
		}
		return nil
	}
	document := struct {
		Schema     string                     `json:"$schema"`
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}{Schema: domain.MCPSchemaV1, MCPServers: servers}
	return shared.WriteJSON(path, document)
}

func writeSanitizedExtensions(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	unsupported, ignoredInvalid := unsupportedExtensionNames(envelope, plan)
	if len(unsupported) == 0 && !ignoredInvalid {
		return nil
	}
	path := filepath.Join(root, "plugin.json")
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read staged plugin.json: %w", err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(body, &document); err != nil {
		return fmt.Errorf("decode staged plugin.json: %w", err)
	}
	if ignoredInvalid {
		delete(document, "extensions")
		return shared.WriteJSON(path, document)
	}
	return rewriteStagedExtensions(path, document, unsupported)
}

func unsupportedExtensionNames(envelope domain.PackageEnvelope, plan domain.DeliveryPlan) ([]string, bool) {
	var unsupported []string
	for _, component := range plan.Components {
		if component.Kind == domain.ComponentExtension && component.Support == domain.SupportUnsupported {
			unsupported = append(unsupported, component.Name)
		}
	}
	for _, diagnostic := range envelope.Diagnostics {
		if diagnostic.Code == "plugin_extensions_ignored" {
			return unsupported, true
		}
	}
	return unsupported, false
}

func rewriteStagedExtensions(path string, document map[string]json.RawMessage, unsupported []string) error {
	var extensions map[string]json.RawMessage
	if raw := document["extensions"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &extensions); err != nil {
			return fmt.Errorf("decode staged plugin extensions: %w", err)
		}
	}
	for _, name := range unsupported {
		delete(extensions, name)
	}
	if len(extensions) == 0 {
		delete(document, "extensions")
	} else {
		raw, err := json.Marshal(extensions)
		if err != nil {
			return err
		}
		document["extensions"] = raw
	}
	return shared.WriteJSON(path, document)
}
