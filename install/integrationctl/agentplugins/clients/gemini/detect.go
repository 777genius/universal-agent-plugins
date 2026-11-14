// Package gemini is the client adapter for the Gemini CLI.
package gemini

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Adapter implements the client contract for the Gemini CLI.
type Adapter struct{}

// New builds the adapter. The composition root registers it in clients/all.
func New() *Adapter { return &Adapter{} }

var (
	_ clients.Adapter            = (*Adapter)(nil)
	_ clients.HostDetector       = (*Adapter)(nil)
	_ clients.Lifecycle          = (*Adapter)(nil)
	_ clients.AutomaticActivator = (*Adapter)(nil)
	_ clients.ReadOnlyVerifier   = (*Adapter)(nil)
)

// ID reports the client this adapter serves.
func (*Adapter) ID() domain.ClientID { return domain.ClientGemini }

// DetectSurfaces probes the Gemini CLI and its configuration directory, which
// GEMINI_CLI_HOME may relocate away from the user home.
func (*Adapter) DetectSurfaces(host clients.Host) clients.Detection {
	configRoot, err := detectedProfileRoot(host)
	if err != nil {
		return clients.Detection{Err: err}
	}
	executable := host.LookPath("gemini")
	return clients.Detection{
		ConfigRoot:     configRoot,
		ExecutablePath: executable,
		Surfaces: []domain.ClientSurface{
			host.ResolvedBinarySurface("gemini_cli", executable),
			host.DirectorySurface("gemini_config", configRoot),
		},
	}
}
