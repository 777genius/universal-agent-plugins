package pluginkitairepo_test

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestPlugininstallModule runs the full plugininstall module test suite.
func TestPlugininstallModule(t *testing.T) {
	root := RepoRoot(t)
	dir := filepath.Join(root, "plugininstall")
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test in plugininstall: %v\n%s", err, out)
	}
}
