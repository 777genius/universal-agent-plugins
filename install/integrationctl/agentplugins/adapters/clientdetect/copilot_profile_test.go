package clientdetect

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/copilot"
)

func TestCopilotOSSnapshotSelectsProfileWithoutMutation(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home, selected := filepath.Join(base, "home"), filepath.Join(base, "selected")
	defaults := filepath.Join(home, ".copilot")
	profiles := map[string]string{
		defaults: `{"trustedFolders":["foreign-default"],"defaultSentinel":true}`,
		selected: `{"trustedFolders":["foreign-selected"],"selectedSentinel":true}`,
	}
	for root, body := range profiles {
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "config.json"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct{ name, value, want string }{
		{"selected", selected, selected},
		{"default", "", defaults},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("COPILOT_HOME", tc.value)
			d := NewOS(home)
			if d.Environment["COPILOT_HOME"] != tc.value {
				t.Fatal("OS snapshot lost selected Copilot profile")
			}
			d.Registry, err = clients.NewRegistry(copilot.New())
			if err != nil {
				t.Fatal(err)
			}
			d.LookPath = func(string) (string, error) { return "", errors.New("absent") }
			d.ProbeVersionWithEnvironment = func(context.Context, string, []string) (string, error) {
				t.Fatal("read-only detection executed a native client")
				return "", nil
			}
			t.Setenv("COPILOT_HOME", filepath.Join(base, "changed-after-snapshot"))
			got, err := d.Detect(context.Background())
			if err != nil || len(got) != 1 || got[0].ConfigRoot != tc.want {
				t.Fatalf("detection = %+v, %v; want %s", got, err, tc.want)
			}
			for root, want := range profiles {
				body, err := os.ReadFile(filepath.Join(root, "config.json"))
				if err != nil || string(body) != want {
					t.Fatalf("profile %s changed: %q, %v", root, body, err)
				}
				entries, err := os.ReadDir(root)
				if err != nil || len(entries) != 1 || entries[0].Name() != "config.json" {
					t.Fatalf("profile %s gained files: %+v, %v", root, entries, err)
				}
			}
			if _, err := os.Stat(filepath.Join(base, "changed-after-snapshot")); !os.IsNotExist(err) {
				t.Fatal("ambient profile was accessed or created")
			}
		})
	}
}
