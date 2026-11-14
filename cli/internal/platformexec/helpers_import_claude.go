package platformexec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type claudePackageMeta struct{}

type importedClaudePluginManifest struct {
	Name               string
	Version            string
	Description        string
	SkillsRefs         []string
	SkillsOverride     bool
	CommandsRefs       []string
	CommandsOverride   bool
	AgentsRefs         []string
	AgentsOverride     bool
	HookRefs           []string
	HooksOverride      bool
	InlineHooks        map[string]any
	LSPRefs            []string
	LSPOverride        bool
	InlineLSP          map[string]any
	MCPRefs            []string
	MCPOverride        bool
	InlineMCP          map[string]any
	Settings           map[string]any
	SettingsProvided   bool
	UserConfig         map[string]any
	UserConfigProvided bool
	Extra              map[string]any
	Warnings           []string
}

func readImportedClaudePluginManifest(root string) (importedClaudePluginManifest, []byte, bool, error) {
	body, err := os.ReadFile(filepath.Join(root, ".claude-plugin", "plugin.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return importedClaudePluginManifest{}, nil, false, nil
		}
		return importedClaudePluginManifest{}, nil, false, err
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return importedClaudePluginManifest{}, nil, false, err
	}
	out := importedClaudePluginManifest{}
	if value, ok := raw["name"].(string); ok {
		out.Name = strings.TrimSpace(value)
	}
	if value, ok := raw["version"].(string); ok {
		out.Version = strings.TrimSpace(value)
	}
	if value, ok := raw["description"].(string); ok {
		out.Description = strings.TrimSpace(value)
	}
	consumeClaudePathField(raw, "skills", &out.SkillsOverride, &out.SkillsRefs, nil, &out.Warnings)
	consumeClaudePathField(raw, "commands", &out.CommandsOverride, &out.CommandsRefs, nil, &out.Warnings)
	consumeClaudePathField(raw, "agents", &out.AgentsOverride, &out.AgentsRefs, nil, &out.Warnings)
	consumeClaudePathField(raw, "hooks", &out.HooksOverride, &out.HookRefs, &out.InlineHooks, &out.Warnings)
	consumeClaudePathField(raw, "lspServers", &out.LSPOverride, &out.LSPRefs, &out.InlineLSP, &out.Warnings)
	consumeClaudePathField(raw, "mcpServers", &out.MCPOverride, &out.MCPRefs, &out.InlineMCP, &out.Warnings)
	consumeClaudeObjectField(raw, "settings", &out.SettingsProvided, &out.Settings, &out.Warnings)
	consumeClaudeObjectField(raw, "userConfig", &out.UserConfigProvided, &out.UserConfig, &out.Warnings)
	delete(raw, "name")
	delete(raw, "version")
	delete(raw, "description")
	if len(raw) > 0 {
		out.Extra = raw
	}
	return out, body, true, nil
}
