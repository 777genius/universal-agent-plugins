// Package cursor is the client adapter for the Cursor editor.
package cursor

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Adapter implements the client contract for Cursor.
type Adapter struct{}

// New builds the adapter. The composition root registers it in clients/all.
func New() *Adapter { return &Adapter{} }

var (
	_ clients.Adapter      = (*Adapter)(nil)
	_ clients.HostDetector = (*Adapter)(nil)
)

// ID reports the client this adapter serves.
func (*Adapter) ID() domain.ClientID { return domain.ClientCursor }

// DetectSurfaces reports editor and agent binary presence separately. The
// retained package preparation contract is for the editor; neither presence
// nor a leftover directory qualifies native agent plugins or Stop events.
func (*Adapter) DetectSurfaces(host clients.Host) clients.Detection {
	configRoot, err := detectedProfileRoot(host)
	if err != nil {
		return clients.Detection{Err: err}
	}
	editor := host.LookPath("cursor")
	surfaces := []domain.ClientSurface{
		host.ResolvedBinarySurface("cursor_editor", editor),
		host.BinarySurface("cursor_agent", "cursor-agent"),
		host.DirectorySurface("cursor_config", configRoot),
	}
	if host.GOOS() == "darwin" {
		surfaces = append(surfaces, host.AppSurface("cursor_desktop", "Cursor.app"))
	} else if host.GOOS() == "windows" {
		surfaces = append(surfaces, host.WindowsAppSurface("cursor_desktop", filepath.Join("Programs", "cursor", "Cursor.exe"), filepath.Join("Cursor", "Cursor.exe")))
	} else if host.GOOS() == "linux" {
		surfaces = append(surfaces, host.LinuxDesktopSurface("cursor_desktop", "cursor.desktop"))
	}
	return clients.Detection{
		ConfigRoot:          configRoot,
		ExecutablePath:      editor,
		Surfaces:            surfaces,
		SelectionSurfaceIDs: []string{"cursor_editor", "cursor_desktop"},
	}
}
