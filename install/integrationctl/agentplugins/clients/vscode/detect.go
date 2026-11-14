// Package vscode is the client adapter for Visual Studio Code.
package vscode

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Adapter implements the client contract for Visual Studio Code.
type Adapter struct{}

// New builds the adapter. The composition root registers it in clients/all.
func New() *Adapter { return &Adapter{} }

var (
	_ clients.Adapter      = (*Adapter)(nil)
	_ clients.HostDetector = (*Adapter)(nil)
)

// ID reports the client this adapter serves.
func (*Adapter) ID() domain.ClientID { return domain.ClientVSCode }

// DetectSurfaces probes the `code` CLI, the per-user configuration root and the
// desktop application.
func (*Adapter) DetectSurfaces(host clients.Host) clients.Detection {
	configRoot := host.VSCodeConfigRoot()
	surfaces := []domain.ClientSurface{
		host.BinarySurface("vscode_cli", "code"),
		host.DirectorySurface("vscode_config", configRoot),
	}
	if host.GOOS() == "darwin" {
		surfaces = append(surfaces, host.AppSurface("vscode_desktop", "Visual Studio Code.app"))
	} else if host.GOOS() == "windows" {
		surfaces = append(surfaces, host.WindowsAppSurface("vscode_desktop", filepath.Join("Programs", "Microsoft VS Code", "Code.exe"), filepath.Join("Microsoft VS Code", "Code.exe")))
	} else if host.GOOS() == "linux" {
		surfaces = append(surfaces, host.LinuxDesktopSurface("vscode_desktop", "code.desktop", "visual-studio-code.desktop"))
	}
	return clients.Detection{
		ConfigRoot:     configRoot,
		ExecutablePath: host.LookPath("code"),
		Surfaces:       surfaces,
	}
}
