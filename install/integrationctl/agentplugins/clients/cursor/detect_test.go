package cursor

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestEditorLauncherDoesNotProveAgentCLI(t *testing.T) {
	base := testBase(t)
	for _, binaries := range []map[string]string{{"cursor": filepath.Join(base, "cursor")}, {"cursor-agent": filepath.Join(base, "cursor-agent")}, {}} {
		home := filepath.Join(base, "TEST-home")
		mustMkdir(t, filepath.Join(home, ".cursor"))
		host := clients.NewHost(clients.HostProbes{HomeDir: home, WorkingDir: base, GOOS: "linux",
			Lstat: os.Lstat, ReadDir: os.ReadDir, EvalSymlinks: filepath.EvalSymlinks,
			LookPath: func(name string) (string, error) {
				if path := binaries[name]; path != "" {
					return path, nil
				}
				return "", fmt.Errorf("TEST absent")
			}})
		userAdapter := New()
		client := userAdapter.DetectSurfaces(host)
		if client.Err != nil || client.ExecutablePath != binaries["cursor"] {
			t.Fatalf("editor selection: %+v", client)
		}
		if len(client.SelectionSurfaceIDs) != 2 || client.SelectionSurfaceIDs[0] != "cursor_editor" || client.SelectionSurfaceIDs[1] != "cursor_desktop" {
			t.Fatalf("agent or leftover config can select editor: %+v", client)
		}
		found := map[string]bool{}
		for _, surface := range client.Surfaces {
			found[surface.ID] = surface.Detected
		}
		if _, obsolete := found["cursor_cli"]; obsolete {
			t.Error("editor launcher still labeled agent CLI")
		}
		if found["cursor_editor"] != (binaries["cursor"] != "") || found["cursor_agent"] != (binaries["cursor-agent"] != "") {
			t.Errorf("separate binary presence: %+v", client.Surfaces)
		}
		wantHint := binaries["cursor-agent"] != "" && binaries["cursor"] == ""
		if gotHint := userAdapter.UndetectedTargetHint(domain.DetectedClient{Surfaces: client.Surfaces}) != ""; gotHint != wantHint {
			t.Errorf("user-scope agent-only hint = %t, want %t", gotHint, wantHint)
		}
		projectAdapter := NewProject()
		project := projectAdapter.DetectSurfaces(host)
		if hint := projectAdapter.UndetectedTargetHint(domain.DetectedClient{Surfaces: project.Surfaces}); hint != "" {
			t.Errorf("project scope received user-only hint: %q", hint)
		}
	}
}
