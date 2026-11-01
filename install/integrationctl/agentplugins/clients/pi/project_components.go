package pi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	piProjectSkillKind = "pi_project_skill_directory"
	piProjectMCPKind   = "pi_project_mcp_server"
)

func piProjectComponents(root string) shared.ProjectComponents {
	return shared.ProjectComponents{
		ProjectRoot: filepath.Dir(root),
		SkillsRoot:  filepath.Join(root, "skills"),
		ConfigPaths: nativeconfig.Paths{JSON: filepath.Join(root, "mcp.json")},
		Codec:       nativeconfig.CodecMCPServers,
		SkillKind:   piProjectSkillKind,
		MCPKind:     piProjectMCPKind,
	}
}

// Stage only the project-selected native components. Everything outside the
// sealed active package is written later by the existing ownership kernel.
func (adapter *ProjectAdapter) Project(ctx context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if in.Plan.Scope != domain.ScopeProject {
		return adapter.Adapter.Project(ctx, in)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := projectPiRoot(domain.DetectedClient{ClientID: domain.ClientPi, ConfigRoot: in.Plan.TargetContext.UserConfigRoot}, in.Plan.TargetContext, in.Plan.NativeRegistryRoot); err != nil {
		return nil, err
	}
	if _, ok := in.Envelope.Manifest.Extensions[Namespace]; ok {
		if _, err := adapter.Adapter.Project(ctx, in); err != nil {
			return nil, err
		}
	} else {
		// An extension-less Skills/MCP package must not be discovered as a
		// runnable Pi extension merely because its files were staged there.
		for _, name := range []string{"index.ts", "index.js", DescriptorName} {
			if _, err := os.Lstat(filepath.Join(in.StagingPath, name)); err == nil {
				return nil, fmt.Errorf("project Skills/MCP package contains a native Pi entrypoint")
			} else if !os.IsNotExist(err) {
				return nil, err
			}
		}
		if err := rejectPiManifest(in.StagingPath); err != nil {
			return nil, err
		}
	}
	if err := shared.ProjectMCPServers(shared.MCPProjection{
		Root: in.StagingPath,
		Envelope: in.Envelope,
		Names: shared.SupportedMCPNames(in.Plan),
		Dialect: shared.MCPDialectKiro,
		PluginRoot: in.Plan.ActivePath,
		DataPath: in.PluginDataPath,
	}); err != nil {
		return nil, err
	}
	return piProjectObjects(in.StagingPath, in.Envelope, in.Plan)
}

// ProjectActiveNative re-reads the verified active package, not the original
// source or a process-owned ambient Pi profile.
func (adapter *ProjectAdapter) ProjectActiveNative(ctx context.Context, root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, data string) ([]domain.NativeObjectOwnership, error) {
	if plan.Scope != domain.ScopeProject {
		return adapter.Adapter.ProjectActiveNative(ctx, root, envelope, plan, data)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if root != plan.ActivePath {
		return nil, fmt.Errorf("pi active project package path differs")
	}
	if len(envelope.Manifest.Extensions) == 0 {
		body, err := os.ReadFile(filepath.Join(root, "plugin.json"))
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(body, &envelope.Manifest); err != nil {
			return nil, err
		}
	}
	if _, ok := envelope.Manifest.Extensions[Namespace]; ok {
		if _, err := adapter.Adapter.ProjectActiveNative(ctx, root, envelope, plan, data); err != nil {
			return nil, err
		}
	} else {
		for _, name := range []string{"index.ts", "index.js", DescriptorName} {
			if _, err := os.Lstat(filepath.Join(root, name)); err == nil {
				return nil, fmt.Errorf("pi project Skills/MCP package unexpectedly became an extension")
			} else if !os.IsNotExist(err) {
				return nil, err
			}
		}
	}
	return piProjectObjects(root, envelope, plan)
}

func piProjectObjects(staging string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	layout := piProjectComponents(plan.NativeRegistryRoot)
	objects := make([]domain.NativeObjectOwnership, 0)
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		var object domain.NativeObjectOwnership
		var err error
		switch component.Kind {
		case domain.ComponentExtension:
			if component.Name != Namespace {
				return nil, fmt.Errorf("pi project extension is not supported")
			}
			continue // The common sealed package-directory receipt owns it.
		case domain.ComponentSkill:
			skill, ok := envelope.Skills[component.Name]
			relative := filepath.ToSlash(filepath.Join("skills", component.Name))
			if ok && skill.RelativePath != "" {
				relative = filepath.ToSlash(filepath.Dir(filepath.FromSlash(skill.RelativePath)))
			}
			object, err = layout.SkillObject(component.Name, relative, staging)
		case domain.ComponentMCPServer:
			var server nativeconfig.Server
			server, err = piProjectServer(staging, component.Name)
			if err == nil {
				object, err = layout.MCPObject(component.Name, server, layout.ConfigPaths.JSON)
			}
		default:
			return nil, fmt.Errorf("unsupported Pi project component kind")
		}
		if err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].ObjectID < objects[j].ObjectID })
	return objects, layout.ValidateObjects(objects)
}

func piProjectServer(root, name string) (nativeconfig.Server, error) {
	path := filepath.Join(root, "mcp.json")
	if err := pathpolicy.RequireContainedChild(root, path); err != nil {
		return nativeconfig.Server{}, err
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nativeconfig.Server{}, err
	}
	document, err := shared.DecodeStrictJSONObject(body)
	if err != nil {
		return nativeconfig.Server{}, err
	}
	servers, ok := document["mcpServers"].(map[string]any)
	if !ok {
		return nativeconfig.Server{}, fmt.Errorf("pi project MCP document has no mcpServers object")
	}
	entry, ok := servers[name].(map[string]any)
	if !ok {
		return nativeconfig.Server{}, fmt.Errorf("planned Pi project MCP server is missing")
	}
	switch entry["type"] {
	case "stdio":
		// Leave the selected resolved path and environment intact.
	case "streamable-http":
		// nativeconfig's generic mcpServers codec renders remote without
		// an extra transport field, as Pi's streamable HTTP expects.
	default:
		return nativeconfig.Server{}, fmt.Errorf("unsupported Pi MCP project transport")
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return nativeconfig.Server{}, err
	}
	var server nativeconfig.Server
	if err := json.Unmarshal(raw, &server); err != nil {
		return server, err
	}
	if server.Type == "streamable-http" {
		server.Type = "remote"
	}
	server.StdioValuesResolved, server.CWDResolved = true, true
	return server, nil
}

func piProjectOnlyNative(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	selected := make([]domain.NativeObjectOwnership, 0, len(objects))
	for _, object := range objects {
		if object.Kind == piProjectSkillKind || object.Kind == piProjectMCPKind {
			selected = append(selected, object)
		}
	}
	return selected
}
