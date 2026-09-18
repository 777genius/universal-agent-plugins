// Package cline is the client adapter for Cline.
package cline

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// storageName is how both editors name Cline's extension directory.
const storageName = "saoudrizwan.claude-dev"

// Adapter implements the client contract for Cline.
type Adapter struct{}

// New builds the adapter. The composition root registers it in clients/all.
func New() *Adapter { return &Adapter{} }

var (
	_ clients.Adapter      = (*Adapter)(nil)
	_ clients.HostDetector = (*Adapter)(nil)
)

// ID reports the client this adapter serves.
func (*Adapter) ID() domain.ClientID { return domain.ClientCline }

// DetectSurfaces probes the extension in both host editors. Editor
// globalStorage is only a detection surface: Cline's shared native
// configuration and skill roots live under its product-owned home.
func (*Adapter) DetectSurfaces(host clients.Host) clients.Detection {
	vscodeConfig := filepath.Join(host.VSCodeConfigRoot(), "globalStorage", storageName)
	cursorConfig := filepath.Join(host.EditorChannelConfigRoot("Cursor"), "globalStorage", storageName)
	return clients.Detection{
		ConfigRoot: filepath.Join(host.HomeDir(), ".cline"),
		Surfaces: []domain.ClientSurface{
			host.ExtensionSurface("cline_vscode_extension", filepath.Join(host.HomeDir(), ".vscode", "extensions"), storageName, "cline.cline"),
			host.DirectorySurface("cline_vscode_config", vscodeConfig),
			host.ExtensionSurface("cline_cursor_extension", filepath.Join(host.HomeDir(), ".cursor", "extensions"), storageName, "cline.cline"),
			host.DirectorySurface("cline_cursor_config", cursorConfig),
		},
	}
}
