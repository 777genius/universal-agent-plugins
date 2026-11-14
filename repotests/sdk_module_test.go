package pluginkitairepo_test

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestSDKModule runs the full SDK test suite (module sdk).
// Repository root is a workspace-only module so `go test ./...` from the plugin-kit-ai
// repo root satisfies the iter1 DoD while keeping sdk/go.mod as in the plan.
func TestSDKModule(t *testing.T) {
	root := RepoRoot(t)
	sdkDir := filepath.Join(root, "sdk")
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = sdkDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test in sdk: %v\n%s", err, out)
	}
}
