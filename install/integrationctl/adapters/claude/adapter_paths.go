package claude

import (
	"fmt"
	"path/filepath"
	"strings"

	fsadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/fs"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) runner() ports.ProcessRunner {
	if a.Runner != nil {
		return a.Runner
	}
	return process.OS{}
}

func (a Adapter) fs() ports.FileSystem {
	if a.FS != nil {
		return a.FS
	}
	return fsadapter.OS{}
}

func (a Adapter) userHome() string {
	return pathpolicy.UserHome(a.UserHome)
}

func (a Adapter) settingsPath(scope string, workspaceRoot string) string {
	scope = strings.ToLower(strings.TrimSpace(scope))
	switch scope {
	case "project":
		root := effectiveWorkspaceRoot(workspaceRoot, a.ProjectRoot)
		return filepath.Join(root, ".claude", "settings.json")
	default:
		return filepath.Join(a.userHome(), ".claude", "settings.json")
	}
}

func (a Adapter) ownedObjects(integrationID, scope, workspaceRoot, materializedRoot string) []domain.NativeObjectRef {
	return []domain.NativeObjectRef{
		{
			Kind:            "managed_marketplace_root",
			Path:            materializedRoot,
			ProtectionClass: domain.ProtectionUserMutable,
		},
		{
			Kind:            "settings_file",
			Path:            a.settingsPath(scope, workspaceRoot),
			ProtectionClass: protectionForScope(scope),
		},
	}
}

func protectionForScope(scope string) domain.ProtectionClass {
	return pathpolicy.ProtectionForScope(scope)
}

func managedMarketplaceRoot(home, integrationID string) string {
	return filepath.Join(home, ".plugin-kit-ai", "materialized", "claude", integrationID)
}

func (a Adapter) validateManagedMarketplaceRoot(integrationID, candidate string) error {
	if err := pathpolicy.ValidateLeafID(integrationID); err != nil {
		return fmt.Errorf("invalid Claude integration id: %w", err)
	}
	if err := pathpolicy.RequireExactPath(managedMarketplaceRoot(a.userHome(), integrationID), candidate); err != nil {
		return fmt.Errorf("unsafe Claude managed marketplace root: %w", err)
	}
	return nil
}

func managedMarketplaceName(integrationID string) string {
	return "integrationctl-" + strings.ToLower(strings.TrimSpace(integrationID))
}

func marketplaceNameFromRecord(record domain.InstallationRecord) string {
	target, ok := record.Targets[domain.TargetClaude]
	if !ok {
		return ""
	}
	if value, ok := target.AdapterMetadata["marketplace_name"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return ""
}

func pluginRefFromRecord(record domain.InstallationRecord) string {
	target, ok := record.Targets[domain.TargetClaude]
	if !ok {
		return ""
	}
	if value, ok := target.AdapterMetadata["plugin_ref"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return ""
}

func materializedRootFromRecord(record domain.InstallationRecord) string {
	target, ok := record.Targets[domain.TargetClaude]
	if !ok {
		return ""
	}
	if value, ok := target.AdapterMetadata["materialized_source_root"].(string); ok && strings.TrimSpace(value) != "" {
		return filepath.Clean(value)
	}
	for _, obj := range target.OwnedNativeObjects {
		if obj.Kind == "managed_marketplace_root" && strings.TrimSpace(obj.Path) != "" {
			return filepath.Clean(obj.Path)
		}
	}
	return ""
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

func effectiveWorkspaceRoot(workspaceRoot string, projectRoot string) string {
	return pathpolicy.ProjectRoot(workspaceRoot, projectRoot)
}

func (a Adapter) commandDirForScope(scope string, workspaceRoot string) string {
	if strings.EqualFold(strings.TrimSpace(scope), "project") {
		return effectiveWorkspaceRoot(workspaceRoot, a.ProjectRoot)
	}
	return ""
}
