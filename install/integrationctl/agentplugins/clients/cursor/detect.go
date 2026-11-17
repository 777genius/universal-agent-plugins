// Package cursor is the client adapter for the Cursor editor.
package cursor

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Adapter implements the client contract for Cursor.
type Adapter struct{ projectComponents bool }

// New builds the adapter. The composition root registers it in clients/all.
func New() *Adapter { return &Adapter{} }

// NewProject selects project components using either the editor or Agent
// surface. It does not qualify native discovery or replace editor identity.
func NewProject() *Adapter { return &Adapter{projectComponents: true} }

var (
	_ clients.Adapter      = (*Adapter)(nil)
	_ clients.HostDetector = (*Adapter)(nil)
)

// ID reports the client this adapter serves.
func (*Adapter) ID() domain.ClientID { return domain.ClientCursor }

// DetectSurfaces reports editor and agent binary presence separately. The
// retained package preparation contract is for the editor; neither presence
// nor a leftover directory qualifies native agent plugins or Stop events.
func (adapter *Adapter) DetectSurfaces(host clients.Host) clients.Detection {
	configRoot, err := detectedProfileRoot(host)
	if err != nil {
		return clients.Detection{Err: err}
	}
	editor := host.LookPath("cursor")
	editorSurface := host.ResolvedBinarySurface("cursor_editor", editor)
	agentSurface := host.BinarySurface(domain.CursorAgentSurfaceID, "cursor-agent")
	surfaces := []domain.ClientSurface{
		editorSurface,
		agentSurface,
		host.DirectorySurface("cursor_config", configRoot),
	}
	if host.GOOS() == "darwin" {
		surfaces = append(surfaces, host.AppSurface("cursor_desktop", "Cursor.app"))
	} else if host.GOOS() == "windows" {
		surfaces = append(surfaces, host.WindowsAppSurface("cursor_desktop", filepath.Join("Programs", "cursor", "Cursor.exe"), filepath.Join("Cursor", "Cursor.exe")))
	} else if host.GOOS() == "linux" {
		surfaces = append(surfaces, host.LinuxDesktopSurface("cursor_desktop", "cursor.desktop"))
	}
	selection := []string{"cursor_editor", "cursor_desktop"}
	if adapter.projectComponents {
		selection = append(selection, domain.CursorAgentSurfaceID)
	}
	return clients.Detection{
		ConfigRoot:          configRoot,
		ExecutablePath:      editor,
		Surfaces:            surfaces,
		SelectionSurfaceIDs: selection,
	}
}

// UndetectedTargetHint explains an agent-only user-scope host without
// changing the detector's frozen raw evidence or project selection.
func (adapter *Adapter) UndetectedTargetHint(client domain.DetectedClient) string {
	if adapter.projectComponents {
		return ""
	}
	var editor, agent bool
	for _, surface := range client.Surfaces {
		switch surface.ID {
		case "cursor_editor":
			editor = surface.Detected
		case domain.CursorAgentSurfaceID:
			agent = surface.Detected
		}
	}
	if agent && !editor {
		return "; Cursor user scope installs for the Cursor editor, but only cursor-agent was found. Use --scope project to install project skills and MCP servers for cursor-agent"
	}
	return ""
}
