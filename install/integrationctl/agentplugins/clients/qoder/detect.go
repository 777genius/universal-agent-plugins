// Package qoder integrates Qoder's shared plugin home and CLI. Qoder Desktop
// and Qoder CLI share one plugin format and one ~/.qoder home, so a single
// adapter serves both.
package qoder

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type Adapter struct{}

func New() *Adapter { return &Adapter{} }

var (
	_ clients.Adapter      = (*Adapter)(nil)
	_ clients.HostDetector = (*Adapter)(nil)
)

func (*Adapter) ID() domain.ClientID { return domain.ClientQoder }

func (*Adapter) DetectSurfaces(host clients.Host) clients.Detection {
	root, err := detectedProfileRoot(host)
	if err != nil {
		return clients.Detection{Err: err}
	}
	executable := host.LookPath("qoder")
	if executable == "" {
		executable = host.LookPath("qodercli")
	}
	return clients.Detection{
		ConfigRoot: root, ExecutablePath: executable,
		Surfaces: []domain.ClientSurface{
			host.ResolvedBinarySurface("qoder_cli", executable),
			host.DirectorySurface("qoder_config", root),
		},
	}
}
