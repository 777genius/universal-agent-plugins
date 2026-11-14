package opencode

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/authoredpath"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"gopkg.in/yaml.v3"
)

func (m *sourceMaterial) loadFirstClassDocs(sourceRoot string) error {
	defaultAgentPath := authoredpath.Join(sourceRoot, "targets", "opencode", "default_agent.txt")
	if fileExists(defaultAgentPath) {
		body, err := os.ReadFile(defaultAgentPath)
		if err != nil {
			return domain.NewError(domain.ErrManifestLoad, "read OpenCode default agent", err)
		}
		text := strings.TrimSpace(string(body))
		if text == "" {
			return domain.NewError(domain.ErrManifestLoad, "OpenCode default agent must be a non-empty string", nil)
		}
		m.WholeFields["default_agent"] = text
	}

	instructionsPath := authoredpath.Join(sourceRoot, "targets", "opencode", "instructions.yaml")
	if fileExists(instructionsPath) {
		body, err := os.ReadFile(instructionsPath)
		if err != nil {
			return domain.NewError(domain.ErrManifestLoad, "read OpenCode instructions", err)
		}
		var instructions []string
		if err := yaml.Unmarshal(body, &instructions); err != nil {
			return domain.NewError(domain.ErrManifestLoad, "parse OpenCode instructions", err)
		}
		if len(instructions) == 0 {
			return domain.NewError(domain.ErrManifestLoad, "OpenCode instructions must contain at least one path", nil)
		}
		for i, item := range instructions {
			instructions[i] = strings.TrimSpace(item)
			if instructions[i] == "" {
				return domain.NewError(domain.ErrManifestLoad, "OpenCode instructions must contain only non-empty paths", nil)
			}
		}
		m.WholeFields["instructions"] = instructions
	}

	permissionPath := authoredpath.Join(sourceRoot, "targets", "opencode", "permission.json")
	if fileExists(permissionPath) {
		body, err := os.ReadFile(permissionPath)
		if err != nil {
			return domain.NewError(domain.ErrManifestLoad, "read OpenCode permission config", err)
		}
		var permission any
		if err := json.Unmarshal(body, &permission); err != nil {
			return domain.NewError(domain.ErrManifestLoad, "parse OpenCode permission config", err)
		}
		if !isPermissionValue(permission) {
			return domain.NewError(domain.ErrManifestLoad, "OpenCode permission must be a string or object", nil)
		}
		m.WholeFields["permission"] = permission
	}
	return nil
}

func (m *sourceMaterial) mergeExtra(extra map[string]any) error {
	for key, value := range extra {
		if _, exists := m.WholeFields[key]; exists || key == "plugin" || key == "mcp" || key == "mode" {
			return domain.NewError(domain.ErrManifestLoad, "OpenCode config.extra.json conflicts with managed key "+key, nil)
		}
		m.WholeFields[key] = value
	}
	return nil
}

func readConfigExtra(path string) (map[string]any, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, domain.NewError(domain.ErrManifestLoad, "read OpenCode config.extra.json", err)
	}
	var extra map[string]any
	if err := json.Unmarshal(body, &extra); err != nil {
		return nil, domain.NewError(domain.ErrManifestLoad, "parse OpenCode config.extra.json", err)
	}
	return extra, nil
}

func isPermissionValue(value any) bool {
	if _, ok := value.(string); ok {
		return true
	}
	_, ok := value.(map[string]any)
	return ok
}
