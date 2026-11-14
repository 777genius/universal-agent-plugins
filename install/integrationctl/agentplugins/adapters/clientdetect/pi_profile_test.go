package clientdetect

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/pi"
)

func TestPiNewOSCapturesNativeAgentDirectory(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home, cwd := filepath.Join(base, "home"), filepath.Join(base, "project")
	for _, root := range []string{home, cwd, filepath.Join(base, "custom profile"), filepath.Join(cwd, "relative profile"), filepath.Join(home, "tilde-profile")} {
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(cwd)
	for _, tc := range []struct{ name, value, want string }{
		{"absolute", filepath.Join(base, "custom profile"), filepath.Join(base, "custom profile")},
		{"relative", "relative profile", filepath.Join(cwd, "relative profile")},
		{"tilde", "~/tilde-profile", filepath.Join(home, "tilde-profile")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PI_CODING_AGENT_DIR", tc.value)
			detector := NewOS(home)
			detector.Registry, err = clients.NewRegistry(pi.New())
			if err != nil {
				t.Fatal(err)
			}
			got, err := detector.Detect(context.Background())
			if err != nil || len(got) != 1 || got[0].DetectionError != nil || got[0].ConfigRoot != tc.want {
				t.Fatalf("native override ignored: %+v %v, want %s", got, err, tc.want)
			}
		})
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(filepath.Join(base, "custom profile"), alias); err == nil {
		t.Setenv("PI_CODING_AGENT_DIR", alias)
		detector := NewOS(home)
		detector.Registry, _ = clients.NewRegistry(pi.New())
		got, err := detector.Detect(context.Background())
		if err != nil || len(got) != 1 || got[0].ConfigRoot != filepath.Join(base, "custom profile") {
			t.Fatalf("alias did not bind physical root: %+v %v", got, err)
		}
	}
}
