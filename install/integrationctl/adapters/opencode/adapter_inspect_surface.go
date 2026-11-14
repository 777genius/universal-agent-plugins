package opencode

import (
	"os/exec"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

var execLookPath = exec.LookPath

func (a Adapter) inspectSurface(scope string, workspaceRoot string) inspectSurface {
	configPath, settings := a.inspectSurfaceSettings(scope, workspaceRoot)
	restrictions, sourceAccess, managedPaths := a.inspectManagedSurfaceLayer()
	return buildInspectSurface(configPath, append(settings, managedPaths...), restrictions, sourceAccess)
}

func (a Adapter) inspectSurfaceSettings(scope string, workspaceRoot string) (string, []string) {
	configPath, candidates := a.inspectSurfacePaths(scope, workspaceRoot)
	return configPath, dedupeStrings(preferredExistingPaths(candidates...))
}

func (a Adapter) inspectManagedSurfaceLayer() ([]domain.EnvironmentRestrictionCode, string, []string) {
	var restrictions []domain.EnvironmentRestrictionCode
	sourceAccess := ""
	var managedPaths []string
	for _, path := range a.managedConfigPaths() {
		if !fileExists(path) {
			continue
		}
		managedPaths = append(managedPaths, path)
		restrictions = append(restrictions, domain.RestrictionReadOnlyNativeLayer)
		if sourceAccess == "" {
			sourceAccess = "managed_config_layer"
		}
	}
	return dedupeRestrictionCodes(restrictions), sourceAccess, managedPaths
}
