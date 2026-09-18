// Package kiro is the client adapter for Kiro.
package kiro

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Adapter implements the client contract for Kiro.
type Adapter struct{}

// New builds the adapter. The composition root registers it in clients/all.
func New() *Adapter { return &Adapter{} }

var (
	_ clients.Adapter               = (*Adapter)(nil)
	_ clients.HostDetector          = (*Adapter)(nil)
	_ clients.Lifecycle             = (*Adapter)(nil)
	_ clients.ActivationPreflighter = (*Adapter)(nil)
	_ clients.AutomaticActivator    = (*Adapter)(nil)
	_ clients.ReadOnlyVerifier      = (*Adapter)(nil)
)

// ID reports the client this adapter serves.
func (*Adapter) ID() domain.ClientID { return domain.ClientKiro }

// DetectSurfaces probes both the current and the legacy CLI name, the
// configuration directory and the desktop application.
func (*Adapter) DetectSurfaces(host clients.Host) clients.Detection {
	configRoot := filepath.Join(host.HomeDir(), ".kiro")
	kiroCLI := host.LookPath("kiro-cli")
	legacyCLI := host.LookPath("kiro")
	// The pre-rename binary stays its own piece of evidence, distinguishable
	// from the current `kiro-cli` name.
	legacy := host.ResolvedBinarySurface("kiro_legacy_cli", legacyCLI)
	if legacy.Detected {
		legacy.Evidence = "legacy_executable_on_path"
	}
	surfaces := []domain.ClientSurface{
		host.ResolvedBinarySurface("kiro_cli", kiroCLI),
		legacy,
		host.DirectorySurface("kiro_config", configRoot),
	}
	if host.GOOS() == "darwin" {
		surfaces = append(surfaces, host.AppSurface("kiro_desktop", "Kiro.app"))
	} else if host.GOOS() == "windows" {
		surfaces = append(surfaces, host.WindowsAppSurface("kiro_desktop", filepath.Join("Programs", "Kiro", "Kiro.exe"), filepath.Join("Kiro", "Kiro.exe")))
	} else if host.GOOS() == "linux" {
		surfaces = append(surfaces, host.LinuxDesktopSurface("kiro_desktop", "kiro.desktop"))
	}
	return clients.Detection{
		ConfigRoot:     configRoot,
		ExecutablePath: shared.FirstPath(kiroCLI, legacyCLI),
		Surfaces:       surfaces,
	}
}
