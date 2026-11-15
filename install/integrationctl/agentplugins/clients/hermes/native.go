package hermes

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

const (
	userSkillKind    = "hermes_user_skill"
	projectSkillKind = "hermes_project_skill"
	neverMCPKind     = "hermes_mcp_not_supported"
	userMCPKind      = "hermes_user_mcp"
)

var (
	_ clients.Projector             = (*Adapter)(nil)
	_ clients.ActiveNativeProjector = (*Adapter)(nil)
	_ clients.ReadOnlyVerifier      = (*Adapter)(nil)
)

func components(plan domain.DeliveryPlan, selectedPackage string) (shared.ProjectComponents, error) {
	authority := plan.NativeRegistryRoot
	if !cleanRoot(authority) {
		return shared.ProjectComponents{}, fmt.Errorf("hermes native root must be canonical and absolute")
	}
	layout := shared.ProjectComponents{
		SkillsRoot: filepath.Join(authority, "skills"),
		// Project Skills keep their original authority. componentLayout adds
		// the separately owned selected-session MCP path for Project requests.
		ConfigPaths:        nativeconfig.Paths{JSON: filepath.Join(authority, "config.yaml")},
		Codec:              nativeconfig.CodecHermes,
		SkillKind:          userSkillKind,
		MCPKind:            userMCPKind,
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

func (adapter *Adapter) buildSkillObjects(ctx context.Context, staged string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	layout, err := adapter.componentLayout(plan, staged)
	if err != nil {
		return nil, err
	}
	objects := make([]domain.NativeObjectOwnership, 0, len(plan.Components))
	for _, component := range plan.Components {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		object, err := adapter.projectComponent(staged, envelope, plan.Scope, layout, component)
		if err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}
	if err := layout.ValidateObjects(objects); err != nil {
		return nil, err
	}
	if adapter.hasProjectSession(plan) {
		objects = append(objects, sessionRootObject(plan.TargetContext.ProjectRoot))
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].ObjectID < objects[j].ObjectID })
	return objects, nil
}

func (adapter *Adapter) projectComponent(staged string, envelope domain.PackageEnvelope, scope domain.InstallScope, layout shared.ProjectComponents, component domain.ComponentDecision) (domain.NativeObjectOwnership, error) {
	if component.Support == domain.SupportUnsupported {
		return domain.NativeObjectOwnership{}, fmt.Errorf("hermes package contains an unsupported component")
	}
	if component.Kind == domain.ComponentMCPServer {
		server, err := readHermesMCPServer(staged, component.Name)
		if err != nil {
			return domain.NativeObjectOwnership{}, err
		}
		if scope == domain.ScopeProject {
			if err := hermesLiteralServer(server); err != nil {
				return domain.NativeObjectOwnership{}, err
			}
		}
		return layout.MCPObject(component.Name, server, layout.ConfigPaths.JSON)
	}
	if component.Kind != domain.ComponentSkill {
		return domain.NativeObjectOwnership{}, fmt.Errorf("hermes package contains an unsupported component")
	}
	skill, ok := envelope.Skills[component.Name]
	if !ok {
		return domain.NativeObjectOwnership{}, fmt.Errorf("hermes selected skill %q is missing from verified package", component.Name)
	}
	relative := filepath.ToSlash(filepath.Join("skills", component.Name))
	if skill.RelativePath != "" {
		relative = filepath.ToSlash(filepath.Dir(filepath.FromSlash(skill.RelativePath)))
	}
	return layout.SkillObject(component.Name, relative, staged)
}

func (adapter *Adapter) Project(ctx context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if adapter.hasProjectSession(in.Plan) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := shared.DeliverManagedStdio(in.DeliverLauncher(), in.StagingPath, in.Envelope, in.Plan); err != nil {
			return nil, err
		}
	}
	if in.Plan.Scope == domain.ScopeUser || adapter.hasProjectSession(in.Plan) {
		if err := shared.ProjectMCPServers(shared.MCPProjection{
			Root: in.StagingPath, Envelope: in.Envelope,
			Names:   shared.SupportedMCPNames(in.Plan),
			Dialect: shared.MCPDialectKiro, PluginRoot: in.Plan.ActivePath,
			DataPath: in.PluginDataPath,
		}); err != nil {
			return nil, err
		}
	}
	if adapter.hasProjectSession(in.Plan) {
		if err := bindHermesProjectStdio(in); err != nil {
			return nil, err
		}
	}
	return adapter.buildSkillObjects(ctx, in.StagingPath, in.Envelope, in.Plan)
}

func (adapter *Adapter) ProjectActiveNative(ctx context.Context, root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, _ string) ([]domain.NativeObjectOwnership, error) {
	if root != plan.ActivePath {
		return nil, fmt.Errorf("hermes active package differs from recorded installation")
	}
	return adapter.buildSkillObjects(ctx, root, envelope, plan)
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
	if err != nil {
		return nativeconfig.Server{}, fmt.Errorf("read Hermes staged MCP selection: %w", err)
	}
	doc, err := shared.DecodeStrictJSONObject(body)
	if err != nil {
		return nativeconfig.Server{}, err
	}
	collection, ok := doc["mcpServers"].(map[string]any)
	if !ok {
		return nativeconfig.Server{}, fmt.Errorf("hermes staged MCP selection is missing mcpServers")
	}
	entry, ok := collection[name].(map[string]any)
	if !ok {
		return nativeconfig.Server{}, fmt.Errorf("hermes MCP server %q is absent from staged selection", name)
	}
	typ, _ := entry["type"].(string)
	if typ != "stdio" && typ != "streamable-http" {
		return nativeconfig.Server{}, fmt.Errorf("hermes MCP server %q transport is unsupported", name)
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return nativeconfig.Server{}, err
	}
	var server nativeconfig.Server
	if err := json.Unmarshal(raw, &server); err != nil {
		return server, err
	}
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
		if object.Kind != userMCPKind && object.Kind != projectSessionMCPKind {
			continue
		}
		server, err := readHermesMCPServer(root, object.LogicalName)
		if err != nil {
			return nil, err
		}
		out[object.LogicalName] = server
	}
	return out, nil
}

// Bind only the persisted expanded selection, preserving the authored command
// for the trusted launcher's path contract. Repair reads this final document.
func bindHermesProjectStdio(in clients.ProjectionInput) error {
	var document map[string]any
	path := filepath.Join(in.StagingPath, "mcp.json")
	for _, name := range shared.SupportedMCPNames(in.Plan) {
		source := in.Envelope.MCP.Servers[name]
		if source.Type != "stdio" {
			continue
		}
		if document == nil {
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			document, err = shared.DecodeStrictJSONObject(body)
			if err != nil {
				return err
			}
		}
		server, err := readHermesMCPServer(in.StagingPath, name)
		if err != nil {
			return err
		}
		rawCWD, _ := source.Decoded["cwd"].(string)
		cwd, err := pathcontract.ExpandCWD(rawCWD, in.Plan.ActivePath, in.PluginDataPath)
		if err != nil {
			return err
		}
		command, ok := source.Decoded["command"].(string)
		servers, collectionOK := document["mcpServers"].(map[string]any)
		entry, entryOK := servers[name].(map[string]any)
		if !ok || command == "" || !collectionOK || !entryOK {
			return fmt.Errorf("hermes selected stdio projection is incomplete")
		}
		entry["args"] = managedstdio.EncodedArguments(in.Plan.ActivePath, in.PluginDataPath, server.CWD, cwd.Anchor, command, server.Args)
		entry["command"] = filepath.Join(in.Plan.ActivePath, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)
	}
	if document == nil {
		return nil
	}
	return shared.WriteJSON(path, document)
}

// Pinned Hermes recursively interpolates every string value with this pattern.
// Encoded child command/args survive it; other Project values must be literal.
var hermesInterpolation = regexp.MustCompile(`\$\{[^}]+\}`)

func hermesLiteralServer(server nativeconfig.Server) error {
	values := append([]string{server.Command, server.CWD, server.URL}, server.Args...)
	for _, mapping := range []map[string]string{server.Env, server.Headers} {
		for _, value := range mapping {
			values = append(values, value)
		}
	}
	for _, value := range values {
		if hermesInterpolation.MatchString(value) {
			return fmt.Errorf("hermes Project MCP contains unsupported native interpolation")
		}
	}
	return nil
}
