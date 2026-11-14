package dirswap

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
)

func parentPublicationFixture(t *testing.T) (Manager, Input) {
	t.Helper()
	m, input := absentFixture(t)
	base, err := filepath.EvalSymlinks(input.OwnedBase)
	if err != nil {
		t.Fatal(err)
	}
	input.OwnedBase, input.ActivePath, input.StagingPath = base, filepath.Join(base, "active"), filepath.Join(base, "stage")
	m.JournalDir = filepath.Join(filepath.Dir(base), "journal")
	parent, err := directoryidentity.Capture(t.Context(), base)
	if err != nil {
		t.Fatal(err)
	}
	input.PublicationParentAuthority = &parent
	return m, input
}

func TestPublicationParentFencesJournalPhaseChanges(t *testing.T) {
	for _, phase := range []string{PhaseBackupPending, FaultBackupRemoved} {
		t.Run(phase, func(t *testing.T) {
			manager, input := parentPublicationFixture(t)
			path := manager.journalPath(input.OperationID)
			var before []byte
			var originalInfo os.FileInfo
			calls := 0
			manager.Fault = func(current string) error {
				if current != phase {
					return nil
				}
				calls++
				var err error
				before, err = os.ReadFile(path)
				if err != nil {
					return err
				}
				file, err := os.Open(path)
				if err != nil {
					return err
				}
				originalInfo, err = file.Stat()
				if err := errors.Join(err, file.Close()); err != nil {
					return err
				}
				project := filepath.Dir(input.OwnedBase)
				original := project + "-original"
				if err := os.Rename(project, original); err != nil {
					return err
				}
				t.Cleanup(func() {
					if err := os.Remove(original); err != nil {
						t.Error(err)
					}
				})
				if err := os.Mkdir(project, 0o700); err != nil {
					return err
				}
				children, err := os.ReadDir(original)
				if err != nil {
					return err
				}
				for _, child := range children {
					if err := os.Rename(filepath.Join(original, child.Name()), filepath.Join(project, child.Name())); err != nil {
						return err
					}
				}
				return nil
			}
			receipt, err := manager.Apply(t.Context(), input)
			if phase == FaultBackupRemoved && err == nil {
				err = manager.Commit(t.Context(), receipt)
			}
			if !errors.Is(err, directoryidentity.ErrChanged) || calls != 1 {
				t.Fatalf("phase did not refuse changed parent: %v (%d faults)", err, calls)
			}
			if err := manager.removeJournal(receipt); !errors.Is(err, directoryidentity.ErrChanged) {
				t.Fatalf("terminal unlink admitted changed parent: %v", err)
			}
			after, readErr := os.ReadFile(path)
			info, statErr := os.Stat(path)
			if readErr != nil || statErr != nil || !bytes.Equal(before, after) || !os.SameFile(originalInfo, info) || !originalInfo.ModTime().Equal(info.ModTime()) || originalInfo.Mode() != info.Mode() {
				t.Fatalf("ancestry refusal rewrote or removed original journal: %v %v", readErr, statErr)
			}
			if phase == PhaseBackupPending {
				assertBody(t, input.StagingPath, "owned")
			} else {
				assertBody(t, input.ActivePath, "owned")
			}
		})
	}
}
