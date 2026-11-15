package transaction

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
)

func TestProjectRootR2PendingReplayRejectsSubstitutedAncestor(t *testing.T) {
	publisher, request, spec, store := projectRootFixture(t)
	parent := filepath.Dir(spec.Path)
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	publisher.Kernel.Directory.Fault = func(phase string) error {
		if phase == dirswap.PhaseActivated || phase == dirswap.PhaseRollbackPending {
			return errors.New("inert interruption before state acknowledgement")
		}
		return nil
	}
	if err := publisher.PublishProjectRoot(t.Context(), request, spec); err == nil {
		t.Fatal("injected pending publication did not stop")
	}
	open, err := publisher.Kernel.Directory.ListOpen()
	if err != nil || len(open) != 1 || open[0].SchemaVersion != 4 || open[0].PublicationParentAuthority == nil || open[0].Phase != dirswap.PhaseRollbackPending {
		t.Fatalf("fixture did not retain original schema4 pending journal: %+v %v", open, err)
	}
	project := request.Plan.TargetContext.ProjectRoot
	journal := filepath.Join(publisher.Kernel.Directory.JournalDir, open[0].OperationID+".json")
	before := map[string]os.FileInfo{}
	for _, path := range []string{project, parent, spec.Path, store.Path, journal} {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path], err = file.Stat()
		if closeErr := file.Close(); err != nil || closeErr != nil {
			t.Fatalf("capture original identity: %v %v", err, closeErr)
		}
	}
	stateBytes, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	journalBytes, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	original := project + "-original"
	if err := os.Rename(project, original); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(original); err != nil {
			t.Error(err)
		}
	})
	if err := os.Mkdir(project, before[project].Mode().Perm()); err != nil {
		t.Fatal(err)
	}
	children, err := os.ReadDir(original)
	if err != nil {
		t.Fatal(err)
	}
	for _, child := range children {
		if err := os.Rename(filepath.Join(original, child.Name()), filepath.Join(project, child.Name())); err != nil {
			t.Fatal(err)
		}
	}
	for path, previous := range before {
		current, err := os.Stat(path)
		if err != nil || os.SameFile(previous, current) != (path != project) {
			t.Fatalf("fixture did not preserve original child identity at %s: %v", path, err)
		}
	}
	for path, originalBytes := range map[string][]byte{store.Path: stateBytes, journal: journalBytes} {
		current, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(current, originalBytes) {
			t.Fatalf("fixture rewrote original authority at %s: %v", path, err)
		}
	}
	publisher.Kernel.Directory.Fault = nil
	if err := publisher.Kernel.Recover(t.Context()); err == nil {
		t.Error("pending schema4 replay accepted substituted Project ancestor")
	}
	for path, previous := range before {
		if path == project {
			continue
		}
		current, err := os.Stat(path)
		if err != nil || !os.SameFile(previous, current) || previous.Mode() != current.Mode() || !previous.ModTime().Equal(current.ModTime()) {
			t.Errorf("recovery changed original object at %s: %v", path, err)
		}
	}
	for path, originalBytes := range map[string][]byte{store.Path: stateBytes, journal: journalBytes, spec.Marker.Path: spec.MarkerBody} {
		current, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(current, originalBytes) {
			t.Errorf("recovery changed original bytes at %s: %v", path, err)
		}
	}
}
