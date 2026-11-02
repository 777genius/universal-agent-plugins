package windsurf

import (
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestProjectVersionRequirementPreservesPrimaryDevinSelection(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"devin", "devin.exe", "DEVIN", "windsurf", "devin-other", "relative", "unclean", "wrong-client"} {
		t.Run(name, func(t *testing.T) {
			client := domain.DetectedClient{ClientID: domain.ClientWindsurf, ExecutablePath: filepath.Join(root, name), Surfaces: []domain.ClientSurface{{ID: "devin_cli", Detected: true}}}
			want := name == "devin" || name == "devin.exe" || name == "DEVIN"
			switch name {
			case "relative":
				client.ExecutablePath = "devin"
			case "unclean":
				client.ExecutablePath = filepath.Join(root, "devin") + string(filepath.Separator)
			case "wrong-client":
				client.ClientID, client.ExecutablePath = domain.ClientKimi, filepath.Join(root, "devin")
			}
			if got := New().RequiresProjectVersion(client); got != want {
				t.Fatalf("primary %q project version requirement = %v, want %v", client.ExecutablePath, got, want)
			}
			if !want {
				// A legacy/foreign primary never asks for original-host facts,
				// even when a secondary Devin surface is present.
				env, err := New().VersionProbeEnvironmentForHost(nil, client)
				if err != nil || len(env) != 0 {
					t.Fatalf("ineligible primary changed the legacy environment: %v, %v", env, err)
				}
			}
		})
	}
}
