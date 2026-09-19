// Package windsurf is the client adapter for Windsurf and Devin, which share
// one product identity across several installable channels.
package windsurf

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Adapter implements the client contract for Windsurf and Devin.
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
func (*Adapter) ID() domain.ClientID { return domain.ClientWindsurf }

// DetectSurfaces probes both CLIs, every editor channel and the legacy Cascade
// configuration roots.
func (*Adapter) DetectSurfaces(host clients.Host) clients.Detection {
	windsurfCLI := host.LookPath("windsurf")
	devinCLI := host.LookPath("devin")
	legacy := legacyChannelRoots(host)
	surfaces := channelSurfaces(host, windsurfCLI, devinCLI, legacy)
	surfaces = append(surfaces, platformSurfaces(host)...)
	return clients.Detection{
		ConfigRoot:     selectedLegacyRoot(surfaces, legacy),
		ExecutablePath: shared.FirstPath(windsurfCLI, devinCLI),
		Surfaces:       surfaces,
	}
}

// legacyRoots are the per-channel Cascade configuration directories.
type legacyRoots struct{ stable, next, insiders string }

func legacyChannelRoots(host clients.Host) legacyRoots {
	codeium := filepath.Join(host.HomeDir(), ".codeium")
	return legacyRoots{
		stable:   filepath.Join(codeium, "windsurf"),
		next:     filepath.Join(codeium, "windsurf-next"),
		insiders: filepath.Join(codeium, "windsurf-insiders"),
	}
}

func channelSurfaces(host clients.Host, windsurfCLI, devinCLI string, legacy legacyRoots) []domain.ClientSurface {
	return []domain.ClientSurface{
		host.ResolvedBinarySurface("windsurf_cli", windsurfCLI),
		host.ResolvedBinarySurface("devin_cli", devinCLI),
		host.DirectorySurface("windsurf_config", host.EditorChannelConfigRoot("Windsurf")),
		host.DirectorySurface("windsurf_next_config", host.EditorChannelConfigRoot("Windsurf - Next")),
		host.DirectorySurface("devin_config", host.EditorChannelConfigRoot("Devin")),
		host.DirectorySurface("devin_local_config", host.XDGConfigRoot("devin")),
		host.DirectorySurface("windsurf_legacy_mcp", legacy.stable),
		host.DirectorySurface("windsurf_next_legacy_mcp", legacy.next),
		host.DirectorySurface("windsurf_insiders_legacy_mcp", legacy.insiders),
	}
}

func platformSurfaces(host clients.Host) []domain.ClientSurface {
	switch host.GOOS() {
	case "darwin":
		return []domain.ClientSurface{
			host.AppSurface("windsurf_desktop", "Windsurf.app"),
			host.AppSurface("windsurf_next_desktop", "Windsurf - Next.app"),
			host.AppSurface("windsurf_insiders_desktop", "Windsurf - Insiders.app"),
			host.AppSurface("devin_desktop", "Devin.app"),
		}
	case "windows":
		return []domain.ClientSurface{
			host.WindowsAppSurface("windsurf_desktop", filepath.Join("Programs", "Windsurf", "Windsurf.exe"), filepath.Join("Windsurf", "Windsurf.exe")),
			host.WindowsAppSurface("devin_desktop", filepath.Join("Programs", "Devin", "Devin.exe"), filepath.Join("Devin", "Devin.exe")),
		}
	case "linux":
		return []domain.ClientSurface{
			host.LinuxDesktopSurface("windsurf_desktop", "windsurf.desktop"),
			host.LinuxDesktopSurface("devin_desktop", "devin.desktop"),
		}
	}
	return nil
}

// selectedLegacyRoot picks the single Cascade channel that may be mutated. A
// ConfigRoot is a mutation authority, not merely a detection hint, so several
// installed channels deliberately resolve to nothing and require the user to
// narrow the environment before automatic activation is allowed. A cloud-synced
// Devin location is never guessed.
func selectedLegacyRoot(surfaces []domain.ClientSurface, legacy legacyRoots) string {
	channels := []struct {
		root       string
		surfaceIDs []string
	}{
		{root: legacy.stable, surfaceIDs: []string{"windsurf_legacy_mcp", "windsurf_config", "windsurf_desktop"}},
		{root: legacy.next, surfaceIDs: []string{"windsurf_next_legacy_mcp", "windsurf_next_config", "windsurf_next_desktop"}},
		{root: legacy.insiders, surfaceIDs: []string{"windsurf_insiders_legacy_mcp", "windsurf_insiders_desktop"}},
	}
	selected := make([]string, 0, len(channels))
	for _, candidate := range channels {
		for _, id := range candidate.surfaceIDs {
			if detectedSurface(surfaces, id) {
				selected = append(selected, candidate.root)
				break
			}
		}
	}
	if len(selected) == 1 {
		return selected[0]
	}
	return ""
}

func detectedSurface(surfaces []domain.ClientSurface, id string) bool {
	for _, surface := range surfaces {
		if surface.ID == id {
			return surface.Detected
		}
	}
	return false
}
