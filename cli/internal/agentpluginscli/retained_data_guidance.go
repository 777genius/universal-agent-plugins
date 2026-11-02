package agentpluginscli

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"

// Follow-up commands retain the already validated selection even when copied
// into a different working directory. User command wording stays unchanged.
func retainedDataPurgeCommand(app App, installationID string) string {
	command := "agentplugins remove " + installationID + " --purge-data"
	if app.scope() == domain.ScopeProject {
		command += " --scope project --project-root " + quoteResumeArgument(app.ProjectRoot)
	}
	return command
}
