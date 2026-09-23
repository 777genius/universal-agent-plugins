package kimi

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (*Adapter) Project(ctx context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Kimi discovers agents/ implicitly even without a manifest declaration.
	// This adapter only qualifies portable skills and MCP.
	if _, err := os.Lstat(filepath.Join(in.StagingPath, "agents")); err == nil {
		return nil, fmt.Errorf("portable Kimi projection cannot include an implicitly discovered agents directory")
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	manifest := shared.ManifestFromEnvelope(in.Envelope, shared.WithAuthorNameEmail())
	if shared.ComponentKindPresent(in.Plan.Components, domain.ComponentSkill) {
		manifest["skills"] = "./skills/"
	}
	servers := map[string]any{}
	for _, name := range shared.SupportedMCPNames(in.Plan) {
		server, ok := in.Envelope.MCP.Servers[name]
		if !ok {
			return nil, fmt.Errorf("missing Kimi MCP server %q", name)
		}
		config := shared.CloneObject(server.Decoded)
		switch server.Type {
		case "stdio":
			delete(config, "type")
			if err := shared.ApplyStdioDataContract(config, in.Plan.ActivePath, in.PluginDataPath, in.StagingPath); err != nil {
				return nil, fmt.Errorf("Kimi MCP %s: %w", name, err)
			}
			command, _ := config["command"].(string)
			if filepath.IsAbs(command) {
				relative, err := pluginRelative(in.Plan.ActivePath, command)
				if err != nil {
					return nil, err
				}
				config["command"] = relative
			} else if command == "" || strings.ContainsAny(command, `/\:`) {
				return nil, fmt.Errorf("Kimi MCP command must be on PATH or within plugin root")
			}
			cwd, _ := config["cwd"].(string)
			relative, err := pluginRelative(in.Plan.ActivePath, cwd)
			if err != nil {
				return nil, err
			}
			config["cwd"] = relative
		case "streamable-http":
			config["type"] = "http"
		case "sse":
			config["type"] = "sse"
		default:
			return nil, fmt.Errorf("unsupported Kimi MCP transport %q", server.Type)
		}
		servers[name] = config
	}
	if len(servers) > 0 {
		manifest["mcpServers"] = servers
	}
	// The top-level Kimi manifest takes precedence over .kimi-plugin/plugin.json.
	// Reject it instead of silently enabling unselected native capabilities.
	if _, err := os.Lstat(filepath.Join(in.StagingPath, "kimi.plugin.json")); err == nil {
		return nil, fmt.Errorf("portable package contains conflicting kimi.plugin.json")
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	target := filepath.Join(in.StagingPath, ".kimi-plugin", "plugin.json")
	if err := pathpolicy.RequireContainedChild(in.StagingPath, target); err != nil {
		return nil, err
	}
	if err := shared.WriteJSON(target, manifest); err != nil {
		return nil, err
	}
	for _, name := range []string{"mcp.json", ".mcp.json"} {
		path := filepath.Join(in.StagingPath, name)
		if err := pathpolicy.RequireContainedChild(in.StagingPath, path); err != nil {
			return nil, err
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	return nil, nil
}
func pluginRelative(root, path string) (string, error) {
	if !filepath.IsAbs(root) || !filepath.IsAbs(path) {
		return "", fmt.Errorf("Kimi MCP requires an absolute resolved plugin path")
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("Kimi MCP command/cwd must remain inside the plugin root")
	}
	if rel == "." {
		return "./", nil
	}
	return "./" + filepath.ToSlash(rel), nil
}
