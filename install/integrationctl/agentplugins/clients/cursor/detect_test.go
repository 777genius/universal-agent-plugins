package cursor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		client := New().DetectSurfaces(host)
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
	}
}

// cursor-agent alone selects Cursor for project scope only, so the user-scope
// adapter explains the other scope and the project adapter stays silent.
func TestAgentOnlyHintIsUserScopeOnly(t *testing.T) {
	t.Parallel()
	agentOnly := []domain.ClientSurface{{ID: domain.CursorAgentSurfaceID, Detected: true}}
	if hint := New().agentOnlyHint(agentOnly); !strings.Contains(hint, "--scope project") {
		t.Fatalf("user scope hint = %q", hint)
	}
	if hint := NewProject().agentOnlyHint(agentOnly); hint != "" {
		t.Fatalf("project scope hint = %q", hint)
	}
	if hint := New().agentOnlyHint([]domain.ClientSurface{{ID: "cursor_editor", Detected: true}}); hint != "" {
		t.Fatalf("editor hint = %q", hint)
	}
}
