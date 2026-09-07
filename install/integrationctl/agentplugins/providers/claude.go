package providers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

const (
	claudeRuntimeDirectory = ".agentplugins-runtime"
	claudeRuntimeSource    = ".agentplugins-runtime-source"
)

// projectClaude renders the portable envelope into Claude Code's official
// skills-directory plugin layout. Only the exact planned Claude surfaces are
// exposed at the plugin root. The original package remains available to stdio
// MCP servers below a manager-owned, non-auto-discovery runtime directory.
func projectClaude(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataPath string) error {
	runtimeRoot, err := isolateClaudeRuntime(root)
	if err != nil {
		return err
	}
	if err := pruneClaudeRuntimeSkills(runtimeRoot, plan); err != nil {
		return err
	}

	manifest := map[string]any{"name": envelope.Manifest.Name}
	copyString := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			manifest[key] = value
		}
	}
	copyString("version", envelope.Manifest.Version)
	copyString("description", envelope.Manifest.Description)
	copyString("homepage", envelope.Manifest.Homepage)
	copyString("repository", envelope.Manifest.Repository)
	copyString("license", envelope.Manifest.License)
	if envelope.Manifest.Author != nil {
		manifest["author"] = envelope.Manifest.Author
	}
	if len(envelope.Manifest.Keywords) > 0 {
		manifest["keywords"] = envelope.Manifest.Keywords
	}
	if hasSupported(plan, domain.ComponentSkill) {
		manifest["skills"] = "./" + claudeRuntimeDirectory + "/skills/"
	}

	serverNames := supportedMCPNames(plan)
	if len(serverNames) > 0 {
		manifest["mcpServers"] = "./.mcp.json"
	}
	if err := writeJSON(filepath.Join(root, ".claude-plugin", "plugin.json"), manifest); err != nil {
		return fmt.Errorf("write Claude Code plugin manifest: %w", err)
	}
	activeRuntimeRoot := filepath.Join(plan.ActivePath, claudeRuntimeDirectory)
	if err := projectClaudeMCP(root, envelope, serverNames, activeRuntimeRoot, dataPath, runtimeRoot); err != nil {
		return err
	}
	return validateClaudeProjectionRoot(root, len(serverNames) > 0)
}

func isolateClaudeRuntime(root string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", fmt.Errorf("read staged Claude package: %w", err)
	}
	for _, entry := range entries {
		if entry.Name() == claudeRuntimeDirectory || entry.Name() == claudeRuntimeSource {
			return "", fmt.Errorf("portable package uses reserved Claude runtime path %q", entry.Name())
		}
	}
	sourceRoot := filepath.Join(root, claudeRuntimeSource)
	if err := os.Mkdir(sourceRoot, 0o700); err != nil {
		return "", fmt.Errorf("create Claude runtime isolation source: %w", err)
	}
	for _, entry := range entries {
		if err := os.Rename(filepath.Join(root, entry.Name()), filepath.Join(sourceRoot, entry.Name())); err != nil {
			return "", fmt.Errorf("isolate Claude runtime path %s: %w", entry.Name(), err)
		}
	}
	runtimeRoot := filepath.Join(root, claudeRuntimeDirectory)
	if err := os.Rename(sourceRoot, runtimeRoot); err != nil {
		return "", fmt.Errorf("activate isolated Claude runtime: %w", err)
	}
	return runtimeRoot, nil
}

func pruneClaudeRuntimeSkills(runtimeRoot string, plan domain.DeliveryPlan) error {
	planned := map[string]bool{}
	for _, component := range plan.Components {
		if component.Kind == domain.ComponentSkill && component.Support != domain.SupportUnsupported {
			if err := pathpolicy.ValidateLeafID(component.Name); err != nil {
				return fmt.Errorf("invalid planned Claude skill %q: %w", component.Name, err)
			}
			planned[component.Name] = true
		}
	}
	skillsRoot := filepath.Join(runtimeRoot, "skills")
	entries, err := os.ReadDir(skillsRoot)
	if os.IsNotExist(err) && len(planned) == 0 {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read Claude runtime skills: %w", err)
	}
	for _, entry := range entries {
		candidate := filepath.Join(skillsRoot, entry.Name())
		if !planned[entry.Name()] {
			if err := pathpolicy.RequireContainedChild(skillsRoot, candidate); err != nil {
				return err
			}
			if err := os.RemoveAll(candidate); err != nil {
				return fmt.Errorf("remove unplanned Claude skill %s: %w", entry.Name(), err)
			}
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() {
			return fmt.Errorf("planned Claude skill %s is not a real directory", entry.Name())
		}
		delete(planned, entry.Name())
	}
	for name := range planned {
		return fmt.Errorf("planned Claude skill %s is missing from runtime", name)
	}
	return nil
}

func validateClaudeProjectionRoot(root string, wantMCP bool) error {
	allowed := map[string]bool{claudeRuntimeDirectory: true, ".claude-plugin": true}
	if wantMCP {
		allowed[".mcp.json"] = true
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !allowed[entry.Name()] {
			return fmt.Errorf("unplanned Claude auto-discovery surface %q survived projection", entry.Name())
		}
	}
	return nil
}

func projectClaudeMCP(root string, envelope domain.PackageEnvelope, serverNames []string, pluginRoot, dataPath string, observationRoot ...string) error {
	observedRuntimeRoot := root
	if len(observationRoot) > 0 {
		observedRuntimeRoot = observationRoot[0]
	}
	path := filepath.Join(root, ".mcp.json")
	if len(serverNames) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove empty Claude MCP projection: %w", err)
		}
		return nil
	}
	servers := make(map[string]map[string]any, len(serverNames))
	for _, name := range serverNames {
		server := envelope.MCP.Servers[name]
		config := cloneObject(server.Decoded)
		switch server.Type {
		case "stdio":
			delete(config, "type")
			rawCommand, _ := config["command"].(string)
			rawCWD, _ := config["cwd"].(string)
			if err := applyStdioDataContract(config, pluginRoot, dataPath, observedRuntimeRoot); err != nil {
				return fmt.Errorf("project Claude stdio MCP server %s: %w", name, err)
			}
			// Claude does not honor cwd on stdio entries. The trusted process
			// adapter makes both default and explicit cwd part of its argv,
			// preserving otherwise identical servers as distinct native entries.
			cwd, err := pathcontract.ParseCWD(rawCWD)
			if err != nil {
				return err
			}
			var args []string
			switch values := config["args"].(type) {
			case []string:
				args = values
			case []any:
				for _, value := range values {
					text, ok := value.(string)
					if !ok {
						return fmt.Errorf("Claude stdio args must be strings")
					}
					args = append(args, text)
				}
			}
			config["args"] = managedstdio.Arguments(pluginRoot, dataPath, config["cwd"].(string), cwd.Anchor, rawCommand, args)
			config["command"] = filepath.Join(pluginRoot, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)
			delete(config, "cwd")
		case "streamable-http":
			config["type"] = "http"
		case "sse":
			config["type"] = "sse"
		default:
			continue
		}
		servers[name] = config
	}
	return writeJSON(path, servers)
}
