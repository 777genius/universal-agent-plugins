// Package codex is the client adapter for the OpenAI Codex CLI.
package codex

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Adapter implements the client contract for OpenAI Codex.
type Adapter struct{}

// New builds the adapter. The composition root registers it in clients/all.
func New() *Adapter { return &Adapter{} }

var (
	_ clients.Adapter      = (*Adapter)(nil)
	_ clients.HostDetector = (*Adapter)(nil)
)

// ID reports the client this adapter serves.
func (*Adapter) ID() domain.ClientID { return domain.ClientCodex }

// DetectSurfaces probes the Codex CLI, its configuration directory and the
// desktop application.
func (*Adapter) DetectSurfaces(host clients.Host) clients.Detection {
	configRoot := filepath.Join(host.HomeDir(), ".codex")
	surfaces := []domain.ClientSurface{
		host.BinarySurface("codex_cli", "codex"),
		host.DirectorySurface("codex_config", configRoot),
	}
	if host.GOOS() == "darwin" {
		surfaces = append(surfaces, host.AppSurface("codex_desktop", "Codex.app"))
	} else if host.GOOS() == "linux" {
		surfaces = append(surfaces, host.LinuxDesktopSurface("codex_desktop", "codex.desktop"))
	}
	return clients.Detection{
		ConfigRoot:     configRoot,
		ExecutablePath: host.LookPath("codex"),
		Surfaces:       surfaces,
	}
}
