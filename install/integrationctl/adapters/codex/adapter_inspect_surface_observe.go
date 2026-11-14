package codex

import (
	"os"
	"os/exec"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func observeCodexSurface(scope string, paths codexSurfacePaths) codexObservedSurface {
	_, cmdErr := exec.LookPath("codex")
	catalogInfo, catalogErr := os.Stat(paths.CatalogPath)
	pluginInfo, pluginErr := os.Stat(paths.PluginRoot)
	configInfo, configErr := os.Stat(paths.ConfigPath)

	restrictions := []domain.EnvironmentRestrictionCode{}
	if cmdErr != nil {
		restrictions = append(restrictions, domain.RestrictionSourceToolMissing)
	}

	observed := []domain.NativeObjectRef{}
	if catalogErr == nil && !catalogInfo.IsDir() {
		observed = append(observed, domain.NativeObjectRef{
			Kind:            "marketplace_catalog",
			Path:            paths.CatalogPath,
			ProtectionClass: protectionForScope(scope),
		})
	}
	if pluginErr == nil && pluginInfo.IsDir() {
		observed = append(observed, domain.NativeObjectRef{
			Kind:            "plugin_root",
			Path:            paths.PluginRoot,
			ProtectionClass: protectionForScope(scope),
		})
	}
	if configErr == nil && !configInfo.IsDir() {
		observed = append(observed, domain.NativeObjectRef{
			Kind:            "config_file",
			Path:            paths.ConfigPath,
			ProtectionClass: domain.ProtectionUserMutable,
		})
	}

	return codexObservedSurface{
		Restrictions: restrictions,
		Observed:     observed,
		CatalogFound: catalogErr == nil && catalogInfo != nil && !catalogInfo.IsDir(),
		PluginFound:  pluginErr == nil && pluginInfo != nil && pluginInfo.IsDir(),
	}
}
