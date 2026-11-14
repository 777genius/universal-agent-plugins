package gemini

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type inspectPaths struct {
	settings          string
	workspaceSettings string
	enablement        string
	trusted           string
	extensionDir      string
}

func (a Adapter) inspectPaths(in ports.InspectInput) inspectPaths {
	home := a.userHome()
	workspaceRoot := workspaceRootFromInspectInput(in)
	paths := inspectPaths{
		settings:          a.settingsPath(scopeFromInspectInput(in), workspaceRoot),
		workspaceSettings: workspaceSettingsPath(workspaceRoot),
		enablement:        a.enablementPath(),
		trusted:           filepath.Join(home, ".gemini", "trustedFolders.json"),
	}
	if in.Record != nil {
		paths.extensionDir = filepath.Join(home, ".gemini", "extensions", in.Record.IntegrationID)
	}
	return paths
}
