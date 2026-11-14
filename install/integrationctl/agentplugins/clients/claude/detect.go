// Package claude is the client adapter for Claude Code.
package claude

import (
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Adapter implements the client contract for Claude Code.
type Adapter struct{}

// New builds the adapter. The composition root registers it in clients/all.
func New() *Adapter { return &Adapter{} }

var (
	_ clients.Adapter      = (*Adapter)(nil)
	_ clients.HostDetector = (*Adapter)(nil)
)

// ID reports the client this adapter serves.
func (*Adapter) ID() domain.ClientID { return domain.ClientClaude }

// DetectSurfaces probes the Claude CLI and its configuration directory. Only
// the CLI selects the client: a configuration directory is kept as evidence but
// never makes Claude an automatic lifecycle target on its own.
func (*Adapter) DetectSurfaces(host clients.Host) clients.Detection {
	configRoot := strings.TrimSpace(host.Env("CLAUDE_CONFIG_DIR"))
	if configRoot == "" {
		configRoot = filepath.Join(host.HomeDir(), ".claude")
	}
	return clients.Detection{
		ConfigRoot:     configRoot,
		ExecutablePath: host.LookPath("claude"),
		Surfaces: []domain.ClientSurface{
			host.BinarySurface("claude_cli", "claude"),
			host.DirectorySurface("claude_config", configRoot),
		},
		SelectionSurfaceIDs: []string{"claude_cli"},
	}
}
