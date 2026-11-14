package bootstrap_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/scaffold"
)

func TestAtomicCommitNeverReplacesDestination(t *testing.T) {
	root := t.TempDir()
	fromPath := filepath.Join(root, "from")
	toPath := filepath.Join(root, "to")
	if err := os.Mkdir(fromPath, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(toPath, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(fromPath, "node_modules"), 0700); err != nil {
		t.Fatal(err)
	}
	winner := filepath.Join(toPath, "node_modules")
	if err := os.WriteFile(winner, []byte("winner"), 0600); err != nil {
		t.Fatal(err)
	}
	from, err := os.Open(fromPath)
	if err != nil {
		t.Fatal(err)
	}
	defer from.Close()
	to, err := os.Open(toPath)
	if err != nil {
		t.Fatal(err)
	}
	defer to.Close()
	if err := scaffold.RenameExclusive(from, "node_modules", to, "node_modules"); err == nil {
		t.Fatal("exclusive commit replaced an existing destination")
	}
	body, err := os.ReadFile(winner)
	if err != nil || string(body) != "winner" {
		t.Fatalf("destination winner changed: %q, %v", body, err)
	}
	if info, err := os.Stat(filepath.Join(fromPath, "node_modules")); err != nil || !info.IsDir() {
		t.Fatalf("failed commit consumed source: %v", err)
	}
	if _, err := os.Lstat(winner); errors.Is(err, os.ErrNotExist) {
		t.Fatal("destination disappeared")
	}
}
