package transaction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func sealedProjectRootFixture(t *testing.T, parentExists bool) (ProjectRootPublisher, *preparedNativeRoot, DirectoryMutation, statev2.Store) {
	t.Helper()
	publisher, request, spec, store := projectRootFixture(t)
	if parentExists {
		writeTransactionBody(t, filepath.Dir(spec.Path), "foreign parent bytes")
	}
	state, binding, err := publisher.projectRootRequest(t.Context(), request, spec)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareNativeRoot(t.Context(), spec)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := prepared.close(); err != nil {
			t.Error(err)
		}
	})
	parent, err := recordedPublicationParent(prepared.baseRoot.ancestry)
	if err != nil {
		t.Fatal(err)
	}
	if err := prepared.sealForPublication(); err != nil {
		t.Fatal(err)
	}
	mutation := DirectoryMutation{
		PublicationParentAuthority: parent, ExpectedPublication: prepared.expectedPublication,
		Standalone: true, RequireAbsent: true, OperationID: request.NativeAttempt.OperationID + "-root", Sequence: 1,
		InstallationID: request.NativeAttempt.InstallationID, ClientBindingID: binding.ClientBindingID,
		OwnedBase: prepared.base, ActivePath: prepared.active, StagingPath: prepared.staging,
		AfterDigest: prepared.digest, DesiredState: state,
		Verify: func(ctx context.Context, _ string) error {
			return verifyNativeRoot(ctx, spec.Path, spec.Marker, prepared.digest, parent)
		},
	}
	return publisher, prepared, mutation, store
}

func TestProjectRootHandoffRetainsOriginalParentAndPostWriteMarker(t *testing.T) {
	for _, parentExists := range []bool{false, true} {
		t.Run(fmt.Sprint(parentExists), func(t *testing.T) {
			_, prepared, _, _ := sealedProjectRootFixture(t, parentExists)
			if prepared.markerFile != nil {
				t.Fatal("sealed publication retained an open descendant marker")
			}
			if prepared.expectedPublication.IsZero() {
				t.Fatal("original publication proof was not sealed")
			}
			for _, held := range []*heldNativeRoot{prepared.baseRoot, prepared.staged} {
				if err := held.verifyNamed(); err != nil {
					t.Fatal(err)
				}
			}
			if parentExists != (prepared.selected.root != nil) {
				t.Fatal("handoff closed the stage or retained the nested descendant")
			}
			name, err := filepath.Rel(prepared.staging, filepath.Join(prepared.selected.path, prepared.markerName))
			if err != nil {
				t.Fatal(err)
			}
			current, err := prepared.staged.root.Lstat(name)
			if err != nil || prepared.markerInfo.Size() != int64(len(prepared.markerBody)) || !sameNativeMarker(prepared.markerInfo, current) {
				t.Fatalf("sealed marker info is not the original post-write stat: %v", err)
			}
			if err := prepared.cleanup(); err != nil {
				t.Fatalf("unchanged pre-journal stage could not be cleaned: %v", err)
			}
			if _, err := os.Lstat(prepared.staging); !os.IsNotExist(err) {
				t.Fatalf("unchanged staging survived cleanup: %v", err)
			}
		})
	}
}

type copiedPublicationEntry struct {
	path, marker, retainedMarker string
	info, markerInfo, original   os.FileInfo
	body                         []byte
}

func originalPublicationInfo(t *testing.T, path string) os.FileInfo {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	info, statErr := file.Stat()
	if err := errors.Join(statErr, file.Close()); err != nil {
		t.Fatal(err)
	}
	return info
}

// Retain the original outside the staged tree so the closed entry set and every
// byte/mode/time remain identical. Only the copied object's identity differs.
func copyPublicationEntry(t *testing.T, selected, markerName, retained, kind string) copiedPublicationEntry {
	t.Helper()
	marker := filepath.Join(selected, markerName)
	original := originalPublicationInfo(t, marker)
	body, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	path, retainedMarker := marker, retained
	if kind == "child" {
		path, retainedMarker = selected, filepath.Join(retained, markerName)
	}
	info := originalPublicationInfo(t, path)
	if err := os.Rename(path, retained); err != nil {
		t.Fatal(err)
	}
	if kind == "child" {
		if err := os.Mkdir(selected, info.Mode().Perm()); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(marker, body, original.Mode().Perm()); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(marker, original.ModTime(), original.ModTime()); err != nil {
		t.Fatal(err)
	}
	copied := originalPublicationInfo(t, path)
	if os.SameFile(info, copied) {
		t.Fatal("test did not substitute the original object")
	}
	return copiedPublicationEntry{path: path, marker: marker, retainedMarker: retainedMarker,
		info: copied, markerInfo: originalPublicationInfo(t, marker), original: original, body: body}
}

func assertCopiedPublicationRetained(t *testing.T, copied copiedPublicationEntry) {
	t.Helper()
	current := originalPublicationInfo(t, copied.path)
	if !os.SameFile(copied.info, current) || copied.info.Mode() != current.Mode() {
		t.Fatal("refusal changed the copied foreign object")
	}
	for _, marker := range []struct {
		path string
		info os.FileInfo
	}{{copied.marker, copied.markerInfo}, {copied.retainedMarker, copied.original}} {
		info := originalPublicationInfo(t, marker.path)
		body, err := os.ReadFile(marker.path)
		if err != nil || !sameNativeMarker(marker.info, info) || string(body) != string(copied.body) {
			t.Fatalf("refusal changed a foreign or retained original marker: %s %v", marker.path, err)
		}
	}
}

func assertProjectRootState(t *testing.T, store statev2.Store, before domain.StateFileV2) {
	t.Helper()
	after, err := store.Load()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("publication refusal changed state: %v", err)
	}
}

func TestProjectRootHandoffRejectsCopiesBeforeFirstJournal(t *testing.T) {
	for _, kind := range []string{"child", "marker"} {
		t.Run(kind, func(t *testing.T) {
			publisher, prepared, mutation, store := sealedProjectRootFixture(t, false)
			retained := filepath.Join(prepared.base, "retained-original-"+kind)
			copied := copyPublicationEntry(t, prepared.selected.path, prepared.markerName, retained, kind)
			if _, err := publisher.Kernel.ApplyDirectory(t.Context(), mutation); err == nil {
				t.Fatal("kernel adopted the new copy instead of the sealed original")
			}
			if open, err := publisher.Kernel.Directory.ListOpen(); err != nil || len(open) != 0 {
				t.Fatalf("pre-journal substitution wrote intent: %v %v", open, err)
			}
			assertProjectRootState(t, store, mutation.DesiredState)
			assertCopiedPublicationRetained(t, copied)
			if err := prepared.cleanup(); err == nil {
				t.Fatal("pre-journal cleanup adopted a same-byte copy")
			}
			assertCopiedPublicationRetained(t, copied)
		})
	}
}

func TestProjectRootHandoffCannotBeDiscardedByPackageMutation(t *testing.T) {
	for _, kind := range []string{"ordinary", "group", "replace"} {
		t.Run(kind, func(t *testing.T) {
			publisher, prepared, mutation, store := sealedProjectRootFixture(t, false)
			if kind == "replace" {
				mutation.RequireAbsent = false
			} else {
				mutation.Standalone = false
				mutation.PublicationParentAuthority = nil
			}
			var err error
			if kind == "group" {
				_, err = publisher.Kernel.ApplyDirectoryGroup(t.Context(), DirectoryGroup{
					OperationGroupID: "discarded-proof", Mutations: []DirectoryMutation{mutation}, DesiredState: mutation.DesiredState,
				})
			} else {
				_, err = publisher.Kernel.ApplyDirectory(t.Context(), mutation)
			}
			if err == nil || !strings.Contains(err.Error(), "original publication proof") {
				t.Fatalf("package operation discarded sealed authority: %v", err)
			}
			assertProjectRootState(t, store, mutation.DesiredState)
			if open, err := publisher.Kernel.Directory.ListOpen(); err != nil || len(open) != 0 {
				t.Fatalf("misplaced proof wrote a journal: %v %v", open, err)
			}
			if err := prepared.cleanup(); err != nil {
				t.Fatalf("invalid request prevented exact owned cleanup: %v", err)
			}
		})
	}
}

func rawProjectRootJournal(t *testing.T, journalDir, operation string) (dirswap.Receipt, []byte) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(journalDir, operation+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var receipt dirswap.Receipt
	if err := json.Unmarshal(body, &receipt); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(receipt.PublishedDigest, "objects-v1:") {
		t.Fatal("pending original-object proof was not persisted")
	}
	return receipt, body
}

func interruptProjectRootWithCopy(t *testing.T, publisher *ProjectRootPublisher, request domain.ActivationRequest, spec ports.NativeProjectRoot, phase, kind string) *copiedPublicationEntry {
	t.Helper()
	copied := new(copiedPublicationEntry)
	publisher.Kernel.Directory.Fault = func(current string) error {
		if current != phase || copied.path != "" {
			return nil
		}
		receipt, _ := rawProjectRootJournal(t, publisher.Kernel.Directory.JournalDir, request.NativeAttempt.OperationID+"-root")
		role := receipt.StagingPath
		if phase == dirswap.PhaseActivated {
			role = receipt.ActivePath
		}
		*copied = copyPublicationEntry(t, filepath.Join(role, filepath.Base(spec.Path)), filepath.Base(spec.Marker.Path),
			filepath.Join(receipt.OwnedBase, "retained-original-"+kind), kind)
		return errors.New("interrupted publication after copying original entry")
	}
	return copied
}

func TestProjectRootHandoffColdRecoveryRetainsCopiedEntries(t *testing.T) {
	for _, phase := range []string{dirswap.PhaseActivationPending, dirswap.PhaseActivated} {
		for _, kind := range []string{"child", "marker"} {
			t.Run(phase+"/"+kind, func(t *testing.T) {
				publisher, request, spec, store := projectRootFixture(t)
				before, err := store.Load()
				if err != nil {
					t.Fatal(err)
				}
				copied := interruptProjectRootWithCopy(t, &publisher, request, spec, phase, kind)
				if err := publisher.PublishProjectRoot(t.Context(), request, spec); err == nil || copied.path == "" {
					t.Fatalf("publication did not retain copied-object failure: %v", err)
				}
				journalDir := publisher.Kernel.Directory.JournalDir
				operation := request.NativeAttempt.OperationID + "-root"
				_, originalJournal := rawProjectRootJournal(t, journalDir, operation)
				cold := Kernel{StateStore: store, Directory: dirswap.Manager{JournalDir: journalDir}}
				for range 2 {
					if err := cold.Recover(t.Context()); err == nil {
						t.Fatal("cold recovery accepted the copied object")
					}
					_, retainedJournal := rawProjectRootJournal(t, journalDir, operation)
					if string(originalJournal) != string(retainedJournal) {
						t.Fatal("cold refusal changed or removed the pending journal")
					}
					assertProjectRootState(t, store, before)
					assertCopiedPublicationRetained(t, *copied)
				}
			})
		}
	}
}

func TestProjectRootHandoffKeepsOriginalMarkerHandleUntilSeal(t *testing.T) {
	_, _, spec, _ := projectRootFixture(t)
	prepared, err := prepareNativeRoot(t.Context(), spec)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := prepared.close(); err != nil {
			t.Error(err)
		}
	}()
	if prepared.markerFile == nil {
		t.Fatal("preparation closed the original marker before sealing")
	}
	original := prepared.markerFile
	info, err := original.Stat()
	if err != nil || !sameNativeMarker(prepared.markerInfo, info) {
		t.Fatalf("preparation did not retain the original completed marker: %v", err)
	}
	if err := prepared.sealForPublication(); err != nil {
		t.Fatal(err)
	}
	if prepared.markerFile != nil {
		t.Fatal("seal retained the descendant marker handle")
	}
	if _, err := original.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("seal did not close the original marker before publication: %v", err)
	}
	if err := prepared.cleanup(); err != nil {
		t.Fatal(err)
	}
}
