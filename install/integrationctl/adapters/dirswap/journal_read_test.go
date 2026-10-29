package dirswap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsRegularJournalWithinCapturedDirectory(t *testing.T) {
	manager, input := fixture(t)
	receipt, err := manager.Apply(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := manager.Load(input.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.OperationID != receipt.OperationID || loaded.Phase != receipt.Phase || loaded.ActivePath != receipt.ActivePath {
		t.Fatalf("loaded journal differs from retained operation: %#v", loaded)
	}
}

func TestLoadRejectsJournalLinksOutsideCapturedDirectory(t *testing.T) {
	for _, indirect := range []bool{false, true} {
		name := "relative-leaf"
		if indirect {
			name = "directory-indirection"
		}
		t.Run(name, func(t *testing.T) {
			manager, input := fixture(t)
			if _, err := manager.Apply(t.Context(), input); err != nil {
				t.Fatal(err)
			}
			outside := filepath.Join(filepath.Dir(manager.JournalDir), "outside-journals")
			if err := os.Mkdir(outside, 0700); err != nil {
				t.Fatal(err)
			}
			journal := manager.journalPath(input.OperationID)
			stored := filepath.Join(outside, input.OperationID+".json")
			if err := os.Rename(journal, stored); err != nil {
				t.Fatal(err)
			}
			target, err := filepath.Rel(manager.JournalDir, stored)
			if err != nil {
				t.Fatal(err)
			}
			if indirect {
				outsideRelative, err := filepath.Rel(manager.JournalDir, outside)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outsideRelative, filepath.Join(manager.JournalDir, "linked-directory")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				target = filepath.Join("linked-directory", input.OperationID+".json")
			}
			if err := os.Symlink(target, journal); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			if _, err := manager.Load(input.OperationID); err == nil {
				t.Fatal("loaded a valid journal through a link outside the captured directory")
			}
		})
	}
}

func TestLoadRejectsDirectoryAtJournalPath(t *testing.T) {
	manager := Manager{JournalDir: t.TempDir()}
	if err := os.Mkdir(manager.journalPath("operation-1"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Load("operation-1"); err == nil {
		t.Fatal("loaded a directory as a journal")
	}
}
