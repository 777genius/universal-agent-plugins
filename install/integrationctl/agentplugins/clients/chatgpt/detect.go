// Package chatgpt is the client adapter for the ChatGPT desktop application.
package chatgpt

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Adapter implements the client contract for ChatGPT.
type Adapter struct{}

// New builds the adapter. The composition root registers it in clients/all.
func New() *Adapter { return &Adapter{} }

var (
	_ clients.Adapter      = (*Adapter)(nil)
	_ clients.HostDetector = (*Adapter)(nil)
)

// ID reports the client this adapter serves.
func (*Adapter) ID() domain.ClientID { return domain.ClientChatGPT }

// DetectSurfaces probes the desktop application only. ChatGPT is a remote and
// manual host: it intentionally has no executable and does not inherit the
// Codex CLI or its configuration directory.
func (*Adapter) DetectSurfaces(host clients.Host) clients.Detection {
	var surfaces []domain.ClientSurface
	switch host.GOOS() {
	case "darwin":
		surfaces = append(surfaces, host.AppSurface("chatgpt_desktop", "ChatGPT.app"))
	case "windows":
		surfaces = append(surfaces, host.WindowsAppSurface(
			"chatgpt_desktop",
			filepath.Join("Microsoft", "WindowsApps", "ChatGPT.exe"),
			filepath.Join("WindowsApps", "ChatGPT.exe"),
		))
	case "linux":
		surfaces = append(surfaces, host.LinuxDesktopSurface("chatgpt_desktop", "chatgpt.desktop"))
	}
	return clients.Detection{Surfaces: surfaces}
}
