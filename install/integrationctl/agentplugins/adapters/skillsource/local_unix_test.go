//go:build unix

package skillsource

import (
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestLocalScanFIFOIsNotOpened(t *testing.T) {
	root := physicalLocalTemp(t)
	if err := unix.Mkfifo(filepath.Join(root, "SKILL.md"), 0600); err != nil {
		t.Fatal(err)
	}
	catalog, err := ScanLocal(t.Context(), root, ScanPolicy{})
	if err != nil || len(catalog.Candidates) != 1 || catalog.Candidates[0].Status != "invalid" {
		t.Fatalf("FIFO admission: %+v %v", catalog, err)
	}
	if _, err := LocalSkillPath(root, ""); err == nil {
		t.Fatal("selected FIFO document")
	}
}
