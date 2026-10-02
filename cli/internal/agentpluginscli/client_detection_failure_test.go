package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	clientregistry "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// A failed selected client must stop planning even when its shared backend is
// visible, while unrelated targets remain usable.
func TestSelectedDetectionFailurePrecedesPhysicalFallback(t *testing.T) {
	failure := errors.New("invalid private profile")
	detected := map[domain.ClientID]domain.DetectedClient{
		domain.ClientCopilot: {ClientID: domain.ClientCopilot, Status: domain.DetectionNotDetected, DetectionError: failure},
		domain.ClientVSCode:  {ClientID: domain.ClientVSCode, Status: domain.DetectionDetected},
	}
	if _, _, err := preflightSelectedTargets(context.Background(), App{}, []domain.ClientID{domain.ClientCopilot}, detectedClientValues(detected), false); !errors.Is(err, failure) {
		t.Fatalf("explicit target did not retain detection failure: %v", err)
	}
	if _, err := preflightInstalledBindings([]domain.ClientID{domain.ClientCopilot}, detected); !errors.Is(err, failure) {
		t.Fatalf("installed target fell back through failed detection: %v", err)
	}
}

// The real detector and CLI share a disposable home. A broken Codex profile
// must be reported by doctor, reject Codex, and leave Cursor dry-run usable.
func TestCLIContinuesWhenUnselectedCodexDetectionFails(t *testing.T) {
	fixture := newCLIFixture(t, nil)
	home := fixture.app.UserHome
	if err := os.MkdirAll(filepath.Join(home, ".cursor"), 0o700); err != nil {
		t.Fatal(err)
	}
	// Editor evidence is independent of leftover config or an agent binary.
	editor := filepath.Join(home, "test-cursor-editor")
	if err := os.WriteFile(editor, []byte("synthetic editor fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	badProfile := filepath.Join(home, "codex-profile-is-a-file")
	if err := os.WriteFile(badProfile, []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	fixture.app.Detector = clientdetect.Detector{
		HomeDir: home, GOOS: "linux", Environment: map[string]string{"CODEX_HOME": badProfile},
		LookPath: func(name string) (string, error) {
			if name == "cursor" {
				return editor, nil
			}
			return "", exec.ErrNotFound
		},
		Lstat:    os.Lstat, ReadDir: os.ReadDir, EvalSymlinks: filepath.EvalSymlinks,
		Registry: clientregistry.Default(),
	}
	plugin := writeCLIPlugin(t)
	doctor, _, err := fixture.execute(false, "doctor", "--target", "cursor", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doctor, `"code":"client_detection_failed"`) || !strings.Contains(doctor, `"client_id":"codex"`) || strings.Contains(doctor, badProfile) {
		t.Fatalf("doctor diagnostic unsafe or missing: %s", doctor)
	}
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "codex", "--dry-run", "--format", "json"); err == nil || !strings.Contains(err.Error(), "detection failed") || !strings.Contains(err.Error(), "profile path is not a directory") {
		t.Fatalf("selected Codex did not fail precisely: %v", err)
	}
	output, _, err := fixture.execute(false, "add", plugin, "--target", "cursor", "--dry-run", "--format", "json")
	if err != nil {
		t.Fatalf("unrelated Cursor dry-run failed: %v", err)
	}
	if !strings.Contains(output, `"client_id":"cursor"`) && !strings.Contains(output, `"target":"cursor"`) {
		t.Fatalf("Cursor dry-run output omitted target: %s", output)
	}
}
