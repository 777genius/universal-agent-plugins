package claude

import "github.com/777genius/plugin-kit-ai/install/integrationctl/domain"

func (a Adapter) inspectRestrictions(inspect inspectContext, record *domain.InstallationRecord) ([]domain.EnvironmentRestrictionCode, []string) {
	restrictions := []domain.EnvironmentRestrictionCode{}
	settingsFiles := []string{inspect.settings}

	if !inspect.cliAvailable {
		restrictions = append(restrictions, domain.RestrictionSourceToolMissing)
	}
	if managedPath, managed, ok := a.readManagedSettings(inspect.scope, inspect.workspaceRoot); ok {
		settingsFiles = append(settingsFiles, managedPath)
		if managed.blocksAllMarketplaceAdds() {
			restrictions = append(restrictions, domain.RestrictionManagedPolicyBlock)
		} else if inspect.integrationID != "" {
			if blocked, _ := a.marketplaceAddBlocked(inspect.scope, inspect.workspaceRoot, inspect.integrationID); blocked {
				restrictions = append(restrictions, domain.RestrictionManagedPolicyBlock)
			}
		}
	}
	if inspect.integrationID != "" {
		if seedPath, ok := a.seedManagedMarketplacePath(inspect.integrationID, record); ok {
			settingsFiles = append(settingsFiles, seedPath)
			restrictions = append(restrictions, domain.RestrictionReadOnlyNativeLayer)
		}
	}
	return dedupeRestrictions(restrictions), settingsFiles
}
