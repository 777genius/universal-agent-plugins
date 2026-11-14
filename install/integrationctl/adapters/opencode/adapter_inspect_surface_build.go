package opencode

import "github.com/777genius/plugin-kit-ai/install/integrationctl/domain"

func buildInspectSurface(configPath string, settings []string, restrictions []domain.EnvironmentRestrictionCode, sourceAccess string) inspectSurface {
	return inspectSurface{
		ConfigPath:              configPath,
		SettingsFiles:           dedupeStrings(settings),
		ConfigPrecedenceContext: []string{"remote", "global", "project", ".opencode", "managed"},
		EnvironmentRestrictions: restrictions,
		VolatileOverride:        false,
		SourceAccessState:       sourceAccess,
	}
}
