package agentpluginscli

import legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"

// Share one finite, selected-profile observer across the composed CLI fixtures.
// This preserves explicit disablement and accurate process-closure metadata.
func fixtureOpenCodeNative(command legacyports.Command) (legacyports.CommandResult, error) {
	return dependencyOpenCodeObservation(command)
}
