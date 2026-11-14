package cursor

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	fsadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/fs"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/safemutate"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) fs() ports.FileSystem {
	if a.FS != nil {
		return a.FS
	}
	return fsadapter.OS{}
}

func (a Adapter) mutator() ports.SafeFileMutator {
	if a.SafeMutator != nil {
		return a.SafeMutator
	}
	return safemutate.OS{}
}

func (a Adapter) targetConfigPath(scope string, workspaceRoot string) string {
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope == "project" {
		return filepath.Join(a.projectRoot(workspaceRoot), ".cursor", "mcp.json")
	}
	return filepath.Join(a.userHome(), ".cursor", "mcp.json")
}

func (a Adapter) validateConfigPath(scope, workspaceRoot, candidate string) error {
	expected := a.targetConfigPath(scope, workspaceRoot)
	if err := pathpolicy.RequireExactPath(expected, candidate); err != nil {
		return fmt.Errorf("unsafe Cursor MCP config path: %w", err)
	}
	return nil
}

func (a Adapter) projectRoot(workspaceRoot string) string {
	return pathpolicy.ProjectRoot(workspaceRoot, a.ProjectRoot)
}

func workspaceRootFromInspectInput(in ports.InspectInput) string {
	return pathpolicy.WorkspaceRootFromInspect(in)
}

func workspaceRootFromApplyInput(in ports.ApplyInput) string {
	return pathpolicy.WorkspaceRootFromApply(in)
}

func workspaceRootFromRecord(record domain.InstallationRecord) string {
	return pathpolicy.WorkspaceRootFromRecord(record)
}

func (a Adapter) userHome() string {
	return pathpolicy.UserHome(a.UserHome)
}

func protectionForScope(scope string) domain.ProtectionClass {
	return pathpolicy.ProtectionForScope(scope)
}

func ownedObjectsForConfig(path string, aliases []string, protection domain.ProtectionClass) []domain.NativeObjectRef {
	out := make([]domain.NativeObjectRef, 0, 1+len(aliases))
	out = append(out, domain.NativeObjectRef{
		Kind:            "file",
		Path:            path,
		ProtectionClass: protection,
	})
	for _, alias := range aliases {
		out = append(out, domain.NativeObjectRef{
			Kind:            "cursor_mcp_server",
			Name:            alias,
			Path:            path,
			ProtectionClass: protection,
		})
	}
	return out
}

func ownedAliases(items []domain.NativeObjectRef) []string {
	var out []string
	for _, item := range items {
		if item.Kind == "cursor_mcp_server" && strings.TrimSpace(item.Name) != "" {
			out = append(out, item.Name)
		}
	}
	sort.Strings(out)
	return out
}

func ownedPluginRoot(items []domain.NativeObjectRef) string {
	for _, item := range items {
		if item.Kind == cursorPluginRootKind && strings.TrimSpace(item.Path) != "" {
			return item.Path
		}
	}
	return ""
}

func ownedObjectsForPluginRoot(path, name string) []domain.NativeObjectRef {
	return []domain.NativeObjectRef{{
		Kind:            cursorPluginRootKind,
		Name:            name,
		Path:            path,
		ProtectionClass: domain.ProtectionUserMutable,
	}}
}

func pluginRootFromTarget(target domain.TargetInstallation, fallback string) string {
	if root := ownedPluginRoot(target.OwnedNativeObjects); root != "" {
		return root
	}
	if metadataPath, ok := target.AdapterMetadata["plugin_root"].(string); ok && strings.TrimSpace(metadataPath) != "" {
		return metadataPath
	}
	return fallback
}

func configPathFromTarget(target domain.TargetInstallation, fallback string) string {
	for _, item := range target.OwnedNativeObjects {
		if item.Kind == "file" && strings.TrimSpace(item.Path) != "" {
			return item.Path
		}
	}
	if metadataPath, ok := target.AdapterMetadata["config_path"].(string); ok && strings.TrimSpace(metadataPath) != "" {
		return metadataPath
	}
	return fallback
}
