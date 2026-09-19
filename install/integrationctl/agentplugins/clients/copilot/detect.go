// Package copilot is the client adapter for the GitHub Copilot CLI.
package copilot

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Adapter implements the client contract for the GitHub Copilot CLI.
type Adapter struct{}

// New builds the adapter. The composition root registers it in clients/all.
func New() *Adapter { return &Adapter{} }

var (
	_ clients.Adapter      = (*Adapter)(nil)
	_ clients.HostDetector = (*Adapter)(nil)
)

// ID reports the client this adapter serves.
func (*Adapter) ID() domain.ClientID { return domain.ClientCopilot }

// DetectSurfaces probes the Copilot CLI and its configuration directory.
func (*Adapter) DetectSurfaces(host clients.Host) clients.Detection {
	configRoot := filepath.Join(host.HomeDir(), ".copilot")
	return clients.Detection{
		ConfigRoot:     configRoot,
		ExecutablePath: host.LookPath("copilot"),
		Surfaces: []domain.ClientSurface{
			host.BinarySurface("copilot_cli", "copilot"),
			host.DirectorySurface("copilot_config", configRoot),
		},
	}
}
