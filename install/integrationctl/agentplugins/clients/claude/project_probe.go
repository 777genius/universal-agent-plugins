package claude

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func prepareClaudeProjectActivationProbe(request domain.ActivationRequest) (ClaudeActivationProbe, error) {
	root, err := claudePlanProjectRoot(request.Client, request.Plan)
	if err != nil {
		return ClaudeActivationProbe{}, err
	}
	if request.Delivery.ActivePath != "" && request.Delivery.ActivePath != request.Plan.ActivePath {
		return ClaudeActivationProbe{}, fmt.Errorf("claude project activation path differs from selected delivery")
	}
	command, err := claudeProjectListCommand(request.BackendExecutable, request.Client, request.Plan.TargetContext, root, request.Plan.ActivePath)
	return ClaudeActivationProbe{Command: command, ConfigRoot: request.Client.ConfigRoot, ActivePath: request.Plan.ActivePath}, err
}

func claudeProjectListCommand(executable string, client domain.DetectedClient, target domain.TargetContext, registryRoot, activePath string) (legacyports.Command, error) {
	root, err := claudeProjectRoot(client, target, registryRoot)
	if err != nil {
		return legacyports.Command{}, err
	}
	if filepath.Clean(activePath) != activePath || filepath.Dir(activePath) != filepath.Join(root, "skills") {
		return legacyports.Command{}, fmt.Errorf("claude project artifact differs from exact skills-directory layout")
	}
	if err := pathpolicy.RequireContainedChild(root, activePath); err != nil {
		return legacyports.Command{}, err
	}
	return claudeListCommand(executable, client.ConfigRoot, activePath, filepath.Join(root, "skills"), target.ProjectRoot)
}
