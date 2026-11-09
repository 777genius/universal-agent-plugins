package antigravity

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.Projector = (*Adapter)(nil)
	_ clients.ActiveNativeProjector = (*Adapter)(nil)
)

var nativePluginID = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Validating and writing a native manifest into the *staging tree* is not
// equivalent to an enabled plugin. The generic transaction owns its directory.
func (*Adapter) Project(ctx context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, projectPlugin(in.StagingPath, in.Envelope, in.Plan, in.PluginDataPath)
}

func (*Adapter) ProjectActiveNative(ctx context.Context, root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, _ string) ([]domain.NativeObjectOwnership, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if root != plan.ActivePath || !cleanAbsolute(root) {
		return nil, fmt.Errorf("Antigravity native package path differs from committed delivery")
	}
	// No separate native member edits: the entire plugin directory is
	// already owned by the shared verified directory transaction.
	return nil, nil
}

func projectPlugin(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataRoot string) error {
	if !cleanAbsolute(root) || !cleanAbsolute(plan.ActivePath) {
		return fmt.Errorf("Antigravity projection root must be absolute")
	}
	// Never auto-enable undisclosed native hooks/subagents/rules. A UAP
	// component declaration is not evidence of authorization for these
	// Antigravity-specific executable or prompt surfaces.
	for _, relative := range []string{"hooks.json", "hooks", "agents", "rules", "scripts", "extensions", ".antigravity-plugin", ".gemini-plugin"} {
		candidate := filepath.Join(root, relative)
		if err := pathpolicy.RequireContainedChild(root, candidate); err != nil {
			return err
		}
		if _, err := os.Lstat(candidate); err == nil {
			return fmt.Errorf("Antigravity native %s requires a separate, explicit capability review", relative)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if !nativePluginID.MatchString(envelope.Manifest.Name) {
		return fmt.Errorf("Antigravity plugin requires a lowercase kebab-case name")
	}
	for _, component := range plan.Components {
		if component.Kind == domain.ComponentSkill {
			if err := pathpolicy.ValidateLeafID(component.Name); err != nil {
				return fmt.Errorf("invalid Antigravity native skill name: %w", err)
			}
			skill, ok := envelope.Skills[component.Name]
			if !ok {
				return fmt.Errorf("Antigravity selected skill %q is not in the verified package", component.Name)
			}
			path := skill.RelativePath
			if path == "" { path = filepath.ToSlash(filepath.Join("skills", component.Name, "SKILL.md")) }
			file := filepath.Join(root, filepath.FromSlash(path))
			if err := pathpolicy.RequireContainedChild(root, file); err != nil { return err }
			info, err := os.Lstat(file)
			if err != nil || !info.Mode().IsRegular() { return fmt.Errorf("Antigravity native skill file is missing or invalid") }
		}
		if component.Kind != domain.ComponentSkill && component.Kind != domain.ComponentMCPServer {
			return fmt.Errorf("Antigravity cannot silently convert unsupported native components")
		}
		if component.Support == domain.SupportUnsupported {
			return fmt.Errorf("Antigravity refuses unqualified required components")
		}
	}
	manifest := map[string]any{
		"$schema": "https://antigravity.google/schemas/v1/plugin.json",
		"name": envelope.Manifest.Name,
	}
	if envelope.Manifest.Version != "" {
		manifest["version"] = envelope.Manifest.Version
	}
	if envelope.Manifest.Description != "" {
		manifest["description"] = envelope.Manifest.Description
	}
	if err := shared.WriteJSON(filepath.Join(root, ".antigravity-plugin", "plugin.json"), manifest); err != nil {
		return fmt.Errorf("write Antigravity native manifest: %w", err)
	}
	servers := map[string]map[string]any{}
	for _, component := range plan.Components {
		if component.Kind != domain.ComponentMCPServer {
			continue
		}
		if err := appendNativeServer(servers, component.Name, envelope, root, plan.ActivePath, dataRoot); err != nil {
			return err
		}
	}
	if len(servers) == 0 {
		// A source may already contain an Antigravity-native file. It is not
		// selected through UAP's verified MCP manifest, so never install it.
		path := filepath.Join(root, "mcp_config.json")
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("unselected Antigravity MCP configuration in package is not authorized")
		} else if !os.IsNotExist(err) { return err }
		return nil
	}
	return shared.WriteJSON(filepath.Join(root, "mcp_config.json"), map[string]any{"mcpServers": servers})
}

func appendNativeServer(out map[string]map[string]any, name string, envelope domain.PackageEnvelope, staged, active, dataRoot string) error {
	server, found := envelope.MCP.Servers[name]
	if !found || server.Decoded == nil {
		return fmt.Errorf("Antigravity MCP server %q is missing from verified package", name)
	}
	config := shared.CloneObject(server.Decoded)
	switch server.Type {
	case "stdio":
		delete(config, "type")
		if err := shared.ApplyStdioDataContract(config, active, dataRoot, staged); err != nil {
			return fmt.Errorf("Antigravity stdio MCP %q is invalid: %w", name, err)
		}
	case "streamable-http":
		raw, ok := config["serverUrl"].(string)
		if !ok || raw == "" {
			raw, ok = config["url"].(string)
		}
		if !ok || raw == "" {
			return fmt.Errorf("Antigravity remote MCP %q requires serverUrl", name)
		}
		endpoint, err := url.Parse(raw)
		if err != nil || endpoint == nil || endpoint.Host == "" ||
			(endpoint.Scheme != "https" && !(endpoint.Scheme == "http" &&
				(endpoint.Hostname() == "localhost" || endpoint.Hostname() == "127.0.0.1"))) {
			return fmt.Errorf("Antigravity remote MCP %q has an unsafe endpoint", name)
		}
		delete(config, "url")
		delete(config, "type")
		config["serverUrl"] = raw
	default:
		return fmt.Errorf("Antigravity MCP %q transport %q is unsupported", name, server.Type)
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("Antigravity MCP selector is empty")
	}
	out[name] = config
	return nil
}
