// Package qoder integrates Qoder's shared plugin home and CLI. Qoder Desktop
// and Qoder CLI share one plugin format and one ~/.qoder home, so a single
// adapter serves both.
package qoder

import (
	"path/filepath"

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
	root := filepath.Join(host.HomeDir(), ".qoder")
	return clients.Detection{
		ConfigRoot: root, ExecutablePath: host.LookPath("qoder"),
		Surfaces: []domain.ClientSurface{
			host.BinarySurface("qoder_cli", "qoder"),
			host.DirectorySurface("qoder_config", root),
		},
	}
}
