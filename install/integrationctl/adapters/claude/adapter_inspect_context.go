package claude

import (
	"os"
	"os/exec"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type inspectContext struct {
	scope         string
	workspaceRoot string
	settings      string
	integrationID string
	cliAvailable  bool
	installed     bool
}

func (a Adapter) newInspectContext(in ports.InspectInput) inspectContext {
	scope := scopeForInspect(in)
	workspaceRoot := workspaceRootFromInspectInput(in)
	settings := a.settingsPath(scope, workspaceRoot)
	_, cmdErr := exec.LookPath("claude")
	_, statErr := os.Stat(settings)
	return inspectContext{
		scope:         scope,
		workspaceRoot: workspaceRoot,
		settings:      settings,
		integrationID: resolveInspectIntegrationID(in),
		cliAvailable:  cmdErr == nil,
		installed:     statErr == nil,
	}
}

func resolveInspectIntegrationID(in ports.InspectInput) string {
	if value := strings.TrimSpace(in.IntegrationID); value != "" {
		return value
	}
	if in.Record != nil {
		return strings.TrimSpace(in.Record.IntegrationID)
	}
	return ""
}
