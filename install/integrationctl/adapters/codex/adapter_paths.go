package codex

import (
	"strings"

	fsadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/fs"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type codexSurfacePaths struct {
	Scope         string
	WorkspaceRoot string
	CatalogPath   string
	PluginRoot    string
	ConfigPath    string
}

func (a Adapter) fs() ports.FileSystem {
	if a.FS != nil {
		return a.FS
	}
	return fsadapter.OS{}
}

func pluginRootFromRecord(record *domain.InstallationRecord) string {
	if record == nil {
		return ""
	}
	target, ok := record.Targets[domain.TargetCodex]
	if !ok {
		return ""
	}
	return pluginRootFromTarget(target, "")
}

func pluginRootFromTarget(target domain.TargetInstallation, fallback string) string {
	for _, item := range target.OwnedNativeObjects {
		if item.Kind == "plugin_root" && strings.TrimSpace(item.Path) != "" {
			return item.Path
		}
	}
	return fallback
}

func catalogPathFromTarget(target domain.TargetInstallation, fallback string) string {
	for _, item := range target.OwnedNativeObjects {
		if item.Kind == "marketplace_catalog" && strings.TrimSpace(item.Path) != "" {
			return item.Path
		}
	}
	return fallback
}

func marketplaceNameFromRecord(record *domain.InstallationRecord) string {
	if record == nil {
		return ""
	}
	target, ok := record.Targets[domain.TargetCodex]
	if !ok || target.AdapterMetadata == nil {
		return ""
	}
	value, _ := target.AdapterMetadata["catalog_name"].(string)
	return strings.TrimSpace(value)
}
