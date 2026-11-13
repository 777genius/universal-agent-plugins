package clientdetect

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestNewOSRetainsHermesProfileEnvironment(t *testing.T) {
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(home, "local-appdata")
	profile := filepath.Join(home, "selected-hermes")
	t.Setenv("LOCALAPPDATA", local)
	t.Setenv("HERMES_DATA_DIR_SUFFIX", "-canary")
	for _, tc := range []struct {
		name string
		home string
		want string
	}{
		{"explicit profile", profile, profile},
		{"native default suffix", "", filepath.Join(local, "hermes-canary")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HERMES_HOME", tc.home)
			detector := NewOS(home)
			detector.GOOS = "windows"
			detector.Registry = all.Default()
			detector.LookPath = func(string) (string, error) { return "", exec.ErrNotFound }
			found, err := detector.Detect(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			got := clientOf(found, domain.ClientHermes)
			if got.DetectionError != nil || got.ConfigRoot != tc.want {
				t.Fatalf("OS environment lost the Hermes profile: %+v; want %q", got, tc.want)
			}
		})
	}
}
