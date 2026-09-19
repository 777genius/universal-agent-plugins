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

// DetectSurfaces probes the Cursor CLI, its configuration directory and the
// desktop application.
func (*Adapter) DetectSurfaces(host clients.Host) clients.Detection {
	configRoot := filepath.Join(host.HomeDir(), ".cursor")
	surfaces := []domain.ClientSurface{
		host.BinarySurface("cursor_cli", "cursor"),
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
		ConfigRoot:     configRoot,
		ExecutablePath: host.LookPath("cursor"),
		Surfaces:       surfaces,
	}
}
