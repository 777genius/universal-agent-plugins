package copilot

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func copilotComponents(root string) (shared.ProjectComponents, error) {
	path := filepath.Join(root, ".github", "mcp.json")
	workspace := filepath.Join(root, ".mcp.json")
	if info, err := os.Lstat(workspace); err == nil {
		if !info.Mode().IsRegular() {
			return shared.ProjectComponents{}, fmt.Errorf("copilot workspace MCP config must be a regular file")
		}
		path = workspace
	} else if !os.IsNotExist(err) {
		return shared.ProjectComponents{}, err
	}
	if err := pathpolicy.RequireContainedChild(root, path); err != nil {
		return shared.ProjectComponents{}, err
	}
	return shared.ProjectComponents{ProjectRoot: root, SkillsRoot: filepath.Join(root, ".github", "skills"),
		ConfigPaths: nativeconfig.Paths{JSON: path}, Codec: nativeconfig.CodecCopilot,
		SkillKind: copilotProjectSkillKind, MCPKind: copilotProjectMCPKind}, nil
}

func validateCopilotConfig(kernel nativeconfig.Kernel, path string) error {
	file, err := kernel.ReadExactFile(path)
	if err != nil || !file.Exists {
		return err
	}
	if len(file.Body) > 1<<20 {
		return fmt.Errorf("copilot project MCP document exceeds inspection limit")
	}
	document, err := shared.DecodeStrictJSONObject(file.Body)
	if err != nil {
		return err
	}
	if len(document) == 0 {
		return nil
	}
	if _, ok := document["mcpServers"].(map[string]any); !ok {
		return fmt.Errorf("copilot project MCP requires a nested mcpServers object; existing bare maps are preserved without mutation")
	}
	return nil
}

func (*Adapter) ValidateBindingProfile(root string, binding domain.ClientBinding) error {
	if binding.Scope != string(domain.ScopeProject) {
		return nil // Preserve the existing user CLI-registry admission contract.
	}
	if binding.NativeProfileRoot != root {
		return fmt.Errorf("copilot recorded project registry differs from selected root")
	}
	if err := pathpolicy.ValidateLeafID(binding.PhysicalArtifact); err != nil {
		return err
	}
	expected := filepath.Join(root, ".uap", "managed", "clients", "copilot", binding.PhysicalArtifact)
	if binding.TargetLocator != expected {
		return fmt.Errorf("copilot recorded package differs from selected project")
	}
	if err := pathpolicy.RequireContainedChild(root, expected); err != nil {
		return err
	}
	layout, err := copilotComponents(root)
	if err != nil {
		return err
	}
	return layout.ValidateObjects(copilotNativeObjects(binding.NativeObjects))
}
