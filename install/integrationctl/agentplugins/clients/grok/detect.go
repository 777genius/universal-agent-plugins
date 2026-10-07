// Package grok integrates Grok Build's native plugin directory and CLI.
package grok

import (
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type Adapter struct{}

func New() *Adapter { return &Adapter{} }

var (
	_ clients.Adapter      = (*Adapter)(nil)
	_ clients.HostDetector = (*Adapter)(nil)
)

func (*Adapter) ID() domain.ClientID { return domain.ClientGrok }

func (*Adapter) DetectSurfaces(host clients.Host) clients.Detection {
	root := filepath.Join(host.HomeDir(), ".grok")
	if configured := strings.TrimSpace(host.Env("GROK_HOME")); configured != "" {
		root = configured
	}
	return clients.Detection{
		ConfigRoot: root, ExecutablePath: host.LookPath("grok"),
		Surfaces: []domain.ClientSurface{
			host.BinarySurface("grok_cli", "grok"),
			host.DirectorySurface("grok_config", root),
		},
	}
}
