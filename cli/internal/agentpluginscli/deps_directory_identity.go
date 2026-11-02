package agentpluginscli

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"

// A reserved future installation ID is not an installed Directory receipt.
// Any already-bound authority must still be found in the live state; otherwise
// preparation is stale. Once the first target commits, its real release wins.
func unboundDependencyInstall(action dependencyAction) bool {
	authority := action.Authority
	return action.Action == environment.ActionInstall && action.BindingID == "" && authority != nil &&
		authority.ExpectedAction == string(environment.ActionInstall) && authority.InstallationID == "" &&
		authority.ClientBindingID == "" && authority.ReceiptID == ""
}
