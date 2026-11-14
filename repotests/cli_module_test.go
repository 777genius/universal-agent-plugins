package pluginkitairepo_test

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestCLIModule runs the full CLI module test suite (cli).
func TestCLIModule(t *testing.T) {
	root := RepoRoot(t)
	cliDir := filepath.Join(root, "cli")
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = cliDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test in cli: %v\n%s", err, out)
	}
}
