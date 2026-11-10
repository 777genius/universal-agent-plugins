package hermes

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"os"
	"encoding/json"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	userSkillKind = "hermes_user_skill"
	projectSkillKind = "hermes_project_skill"
	neverMCPKind = "hermes_mcp_not_supported"
	userMCPKind = "hermes_user_mcp"
)

var (
	_ clients.Projector = (*Adapter)(nil)
	_ clients.ActiveNativeProjector = (*Adapter)(nil)
	_ clients.ReadOnlyVerifier = (*Adapter)(nil)
)

func components(plan domain.DeliveryPlan, selectedPackage string) (shared.ProjectComponents, error) {
	authority := plan.NativeRegistryRoot
	if !cleanRoot(authority) {
		return shared.ProjectComponents{}, fmt.Errorf("hermes native root must be canonical and absolute")
	}
	layout := shared.ProjectComponents{
		SkillsRoot: filepath.Join(authority, "skills"),
		// There is no project-local Hermes MCP authority. The sentinel path
		// is never written: this layout admits Skills alone.
		ConfigPaths: nativeconfig.Paths{JSON: filepath.Join(authority, "config.yaml")},
		Codec: nativeconfig.CodecHermes,
		SkillKind: userSkillKind,
		MCPKind: userMCPKind,
		ManagedPackagePath: selectedPackage,
	}
	switch plan.Scope {
	case domain.ScopeUser:
		if plan.TargetContext.UserConfigRoot != "" && plan.TargetContext.UserConfigRoot != authority {
			return layout, fmt.Errorf("hermes user profile authority differs")
		}
		layout.ProjectRoot = authority
	case domain.ScopeProject:
		if !cleanRoot(plan.TargetContext.ProjectRoot) ||
			filepath.Join(plan.TargetContext.ProjectRoot, ".hermes") != authority {
			return layout, fmt.Errorf("hermes project Skills root must be the exact selected project")
		}
		layout.ProjectRoot = plan.TargetContext.ProjectRoot
		layout.SkillKind = projectSkillKind
		layout.ConfigPaths.JSON = filepath.Join(authority, ".uap-no-mcp.json")
		layout.Codec = nativeconfig.CodecMCPServers
	default:
		return layout, fmt.Errorf("hermes does not support this installation scope")
	}
	return layout, nil
}

func buildSkillObjects(ctx context.Context, staged string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	layout, err := components(plan, staged)
	if err != nil {
		return nil, err
	}
	objects := make([]domain.NativeObjectOwnership, 0, len(plan.Components))
	for _, component := range plan.Components {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if component.Kind == domain.ComponentMCPServer {
			if plan.Scope != domain.ScopeUser || component.Support == domain.SupportUnsupported {
				return nil, fmt.Errorf("hermes MCP has User authority only")
			}
			server, err := readHermesMCPServer(staged, component.Name)
			if err != nil { return nil, err }
			object, err := layout.MCPObject(component.Name, server, layout.ConfigPaths.JSON)
			if err != nil { return nil, err }
			objects = append(objects, object)
			continue
		}
		if component.Kind != domain.ComponentSkill || component.Support == domain.SupportUnsupported {
			return nil, fmt.Errorf("hermes package contains an unsupported component")
		}
		skill, ok := envelope.Skills[component.Name]
		if !ok {
			return nil, fmt.Errorf("hermes selected skill %q is missing from verified package", component.Name)
		}
		relative := filepath.ToSlash(filepath.Join("skills", component.Name))
		if skill.RelativePath != "" {
			relative = filepath.ToSlash(filepath.Dir(filepath.FromSlash(skill.RelativePath)))
		}
		object, err := layout.SkillObject(component.Name, relative, staged)
		if err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].ObjectID < objects[j].ObjectID })
	return objects, layout.ValidateObjects(objects)
}

func (*Adapter) Project(ctx context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if in.Plan.Scope == domain.ScopeUser {
		if err := shared.ProjectMCPServers(shared.MCPProjection{
			Root: in.StagingPath, Envelope: in.Envelope,
			Names: shared.SupportedMCPNames(in.Plan),
			Dialect: shared.MCPDialectKiro, PluginRoot: in.Plan.ActivePath,
			DataPath: in.PluginDataPath,
		}); err != nil { return nil, err }
	}
	return buildSkillObjects(ctx, in.StagingPath, in.Envelope, in.Plan)
}

func (*Adapter) ProjectActiveNative(ctx context.Context, root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, _ string) ([]domain.NativeObjectOwnership, error) {
	if root != plan.ActivePath {
		return nil, fmt.Errorf("hermes active package differs from recorded installation")
	}
	return buildSkillObjects(ctx, root, envelope, plan)
}

func (*Adapter) VerifierAvailable(_ domain.DetectedClient, plan domain.DeliveryPlan, _ string) bool {
	return plan.NativeRegistryRoot != "" && len(plan.Components) > 0
}

func ownedObjects(scope domain.InstallScope, objects []domain.NativeObjectOwnership) ([]domain.NativeObjectOwnership, error) {
	kind := userSkillKind
	if scope == domain.ScopeProject {
		kind = projectSkillKind
	}
	result := make([]domain.NativeObjectOwnership, 0, len(objects))
	for _, object := range objects {
		if object.Kind == "managed_package_directory" {
			continue // Generic package directory is owned and verified by the installer.
		}
		if object.Kind == userMCPKind && scope == domain.ScopeUser {
			result = append(result, object)
			continue
		}
		if object.Kind != kind {
			return nil, fmt.Errorf("hermes refused foreign native ownership type")
		}
		result = append(result, object)
	}
	return result, nil
}

// readHermesMCPServer decodes only a UAP-staged/verified selection. Raw source
// placeholders were expanded during ProjectMCPServers; they are never expanded
// again while creating the final ownership receipt or during activation.
func readHermesMCPServer(root, name string) (nativeconfig.Server, error) {
	path := filepath.Join(root, "mcp.json")
	body, err := os.ReadFile(path)
	if err != nil { return nativeconfig.Server{}, fmt.Errorf("read Hermes staged MCP selection: %w", err) }
	doc, err := shared.DecodeStrictJSONObject(body)
	if err != nil { return nativeconfig.Server{}, err }
	collection, ok := doc["mcpServers"].(map[string]any)
	if !ok { return nativeconfig.Server{}, fmt.Errorf("hermes staged MCP selection is missing mcpServers") }
	entry, ok := collection[name].(map[string]any)
	if !ok { return nativeconfig.Server{}, fmt.Errorf("hermes MCP server %q is absent from staged selection", name) }
	typ, _ := entry["type"].(string)
	if typ != "stdio" && typ != "streamable-http" {
		return nativeconfig.Server{}, fmt.Errorf("hermes MCP server %q transport is unsupported", name)
	}
	raw, err := json.Marshal(entry)
	if err != nil { return nativeconfig.Server{}, err }
	var server nativeconfig.Server
	if err := json.Unmarshal(raw, &server); err != nil { return server, err }
	if typ == "streamable-http" {
		server.Type = "remote"
		// Hermes supports remote HTTP through a url field, no transport tag.
		server.RemoteTransport = ""
	}
	server.StdioValuesResolved, server.CWDResolved = true, true
	return server, nil
}

func hermesDesiredServers(root string, objects []domain.NativeObjectOwnership) (map[string]nativeconfig.Server, error) {
	out := make(map[string]nativeconfig.Server)
	for _, object := range objects {
		if object.Kind != userMCPKind { continue }
		server, err := readHermesMCPServer(root, object.LogicalName)
		if err != nil { return nil, err }
		out[object.LogicalName] = server
	}
	return out, nil
}
