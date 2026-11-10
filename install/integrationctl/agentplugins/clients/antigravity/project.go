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
	_ clients.Projector             = (*Adapter)(nil)
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
		return nil, fmt.Errorf("antigravity native package path differs from committed delivery")
	}
	// No separate native member edits: the entire plugin directory is
	// already owned by the shared verified directory transaction.
	return nil, nil
}

// Convert only explicitly supported portable components. Preflight every
// native autoload surface before emitting any managed manifest or MCP config.
func projectPlugin(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataRoot string) error {
	if err := validateAntigravityProjection(root, envelope, plan); err != nil {
		return err
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
	manifest := map[string]any{
		"$schema":     "https://antigravity.google/schemas/v1/plugin.json",
		"name":        envelope.Manifest.Name,
		"displayName": envelope.Manifest.Name,
	}
	if envelope.Manifest.Version != "" {
		manifest["version"] = envelope.Manifest.Version
	}
	if envelope.Manifest.Description != "" {
		manifest["description"] = envelope.Manifest.Description
	}
	if err := shared.WriteJSON(filepath.Join(root, "plugin.json"), manifest); err != nil {
		return fmt.Errorf("write Antigravity native manifest: %w", err)
	}
	if len(servers) > 0 {
		return shared.WriteJSON(filepath.Join(root, "mcp_config.json"), map[string]any{"mcpServers": servers})
	}
	return nil
}

func validateAntigravityProjection(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	if !cleanAbsolute(root) || !cleanAbsolute(plan.ActivePath) {
		return fmt.Errorf("antigravity projection root must be absolute")
	}
	if !nativePluginID.MatchString(envelope.Manifest.Name) {
		return fmt.Errorf("antigravity plugin requires a lowercase kebab-case name")
	}
	// Do not silently authorize preexisting vendor hook, script, agent, or
	// raw MCP config files copied from an arbitrary source package.
	for _, name := range []string{
		"hooks.json", "hooks", "agents", "rules", "extensions",
		".antigravity-plugin", ".gemini-plugin", "mcp_config.json",
	} {
		path := filepath.Join(root, name)
		if err := pathpolicy.RequireContainedChild(root, path); err != nil {
			return err
		}
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("antigravity native %s requires a separate capability review", name)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	for _, component := range plan.Components {
		if err := validateAntigravityComponent(root, envelope, component); err != nil {
			return err
		}
	}
	// A verified source package can carry stdio MCP helper binaries under
	// scripts/, but this directory must not be activated as an unrelated
	// executable surface when no selected stdio MCP can reference it.
	if info, err := os.Lstat(filepath.Join(root, "scripts")); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("antigravity MCP helper scripts must be a real source directory")
		}
		stdio := false
		for _, component := range plan.Components {
			if component.Kind == domain.ComponentMCPServer && component.Support != domain.SupportUnsupported {
				server, ok := envelope.MCP.Servers[component.Name]
				stdio = stdio || ok && server.Type == "stdio"
			}
		}
		if !stdio {
			return fmt.Errorf("antigravity scripts directory requires an explicitly selected stdio MCP")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func validateAntigravityComponent(root string, envelope domain.PackageEnvelope, component domain.ComponentDecision) error {
	if component.Support == domain.SupportUnsupported {
		return fmt.Errorf("antigravity refuses unqualified required components")
	}
	switch component.Kind {
	case domain.ComponentMCPServer:
		if strings.TrimSpace(component.Name) == "" {
			return fmt.Errorf("antigravity MCP server name is empty")
		}
	case domain.ComponentSkill:
		if err := pathpolicy.ValidateLeafID(component.Name); err != nil {
			return fmt.Errorf("invalid Antigravity native skill name: %w", err)
		}
		skill, ok := envelope.Skills[component.Name]
		if !ok {
			return fmt.Errorf("antigravity selected skill %q is not in the verified package", component.Name)
		}
		relative := skill.RelativePath
		if relative == "" {
			relative = filepath.ToSlash(filepath.Join("skills", component.Name, "SKILL.md"))
		}
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := pathpolicy.RequireContainedChild(root, path); err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("antigravity native skill file is missing or invalid")
		}
	default:
		return fmt.Errorf("antigravity cannot silently convert unsupported native components")
	}
	return nil
}

func appendNativeServer(out map[string]map[string]any, name string, envelope domain.PackageEnvelope, staged, active, dataRoot string) error {
	server, found := envelope.MCP.Servers[name]
	if !found || server.Decoded == nil {
		return fmt.Errorf("antigravity MCP server %q is missing from verified package", name)
	}
	config := shared.CloneObject(server.Decoded)
	switch server.Type {
	case "stdio":
		if err := rejectAntigravityTransportFields(config, "url", "serverUrl", "headers"); err != nil {
			return err
		}
		delete(config, "type")
		if err := shared.ApplyStdioDataContract(config, active, dataRoot, staged); err != nil {
			return fmt.Errorf("antigravity stdio MCP %q is invalid: %w", name, err)
		}
	case "streamable-http":
		if err := rejectAntigravityTransportFields(config, "command", "args", "env", "cwd"); err != nil {
			return err
		}
		raw, ok := config["serverUrl"].(string)
		if !ok || raw == "" {
			raw, ok = config["url"].(string)
		}
		if !ok || raw == "" {
			return fmt.Errorf("antigravity remote MCP %q requires serverUrl", name)
		}
		endpoint, err := url.Parse(raw)
		if err != nil || endpoint == nil || endpoint.Host == "" ||
			(endpoint.Scheme != "https" && !(endpoint.Scheme == "http" &&
				(endpoint.Hostname() == "localhost" || endpoint.Hostname() == "127.0.0.1"))) {
			return fmt.Errorf("antigravity remote MCP %q has an unsafe endpoint", name)
		}
		delete(config, "url")
		delete(config, "type")
		config["serverUrl"] = raw
	default:
		return fmt.Errorf("antigravity MCP %q transport %q is unsupported", name, server.Type)
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("antigravity MCP selector is empty")
	}
	out[name] = config
	return nil
}

// A compatible package can reach this adapter without the portable loader's
// strict schema check. Refuse incompatible transport fields rather than
// generating an ambiguous native MCP declaration or silently dropping intent.
func rejectAntigravityTransportFields(config map[string]any, forbidden ...string) error {
	for _, field := range forbidden {
		if _, exists := config[field]; exists {
			return fmt.Errorf("antigravity MCP entry contains conflicting transport field %q", field)
		}
	}
	return nil
}
