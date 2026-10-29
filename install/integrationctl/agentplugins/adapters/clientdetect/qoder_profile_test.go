package clientdetect

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/qoder"
)

func TestQoderOSDiscoveryCapturesExplicitProfile(t *testing.T) {
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(home, "selected profile")
	t.Setenv("QODER_CONFIG_DIR", profile)
	detector := NewOS(home)
	t.Setenv("QODER_CONFIG_DIR", filepath.Join(home, "later ambient profile"))
	detector.Registry, err = clients.NewRegistry(qoder.New())
	if err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(home, "bin", "qodercli")
	detector.LookPath = func(name string) (string, error) {
		if name == "qodercli" {
			return executable, nil
		}
		return "", fmt.Errorf("TEST absent")
	}
	got, err := detector.Detect(context.Background())
	if err != nil || len(got) != 1 || got[0].DetectionError != nil || got[0].ConfigRoot != profile || got[0].ExecutablePath != executable {
		t.Fatalf("frozen Qoder discovery = %+v, %v", got, err)
	}
	detector.Environment["QODER_CONFIG_DIR"] = "relative-profile"
	got, err = detector.Detect(context.Background())
	if err != nil || len(got) != 1 || got[0].DetectionError == nil || got[0].ConfigRoot != "" || got[0].ExecutablePath != "" {
		t.Fatalf("invalid Qoder override selected a default: %+v, %v", got, err)
	}
}
