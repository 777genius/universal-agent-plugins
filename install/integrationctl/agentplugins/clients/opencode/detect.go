// Package opencode is the client adapter for OpenCode.
package opencode

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Adapter implements the client contract for OpenCode.
type Adapter struct{}

// New builds the adapter. The composition root registers it in clients/all.
func New() *Adapter { return &Adapter{} }

var (
	_ clients.Adapter      = (*Adapter)(nil)
	_ clients.HostDetector = (*Adapter)(nil)
)

// ID reports the client this adapter serves.
func (*Adapter) ID() domain.ClientID { return domain.ClientOpenCode }

// DetectSurfaces probes the OpenCode CLI, its XDG configuration directory and
// the desktop application.
func (*Adapter) DetectSurfaces(host clients.Host) clients.Detection {
	configRoot := host.XDGConfigRoot("opencode")
	surfaces := []domain.ClientSurface{
		host.BinarySurface("opencode_cli", "opencode"),
		host.DirectorySurface("opencode_config", configRoot),
	}
	switch host.GOOS() {
	case "darwin":
		surfaces = append(surfaces, host.AppSurface("opencode_desktop", "OpenCode.app"))
	case "windows":
		surfaces = append(surfaces, host.WindowsAppSurface("opencode_desktop", filepath.Join("Programs", "OpenCode", "OpenCode.exe"), filepath.Join("OpenCode", "OpenCode.exe")))
	case "linux":
		surfaces = append(surfaces, host.LinuxDesktopSurface("opencode_desktop", "opencode.desktop"))
	}
	return clients.Detection{
		ConfigRoot:     configRoot,
		ExecutablePath: host.LookPath("opencode"),
		Surfaces:       surfaces,
	}
}
