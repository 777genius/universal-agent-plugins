package dirswap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
)

const objectPublicationMarker = "original native root\n"

type heldPublicationFixture struct {
	root     *os.Root
	selected *os.Root
	marker   *os.File
	entries  []OriginalPublicationEntry
}

func originalPublicationFixture(t *testing.T) (Manager, Input) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(base, "stage")
	if err := os.MkdirAll(filepath.Join(stage, "plugins"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "plugins", "marker"), []byte(objectPublicationMarker), 0o600); err != nil {
		t.Fatal(err)
	}
	parent, err := directoryidentity.Capture(t.Context(), base)
	if err != nil {
		t.Fatal(err)
	}
	input := Input{
		OperationID: "objects", ClientBindingID: "binding", Sequence: 1,
		OwnedBase: base, ActivePath: filepath.Join(base, "active"), StagingPath: stage,
		RequireAbsent: true, PublicationParentAuthority: &parent,
	}
	return Manager{JournalDir: filepath.Join(base, "journal")}, input
}

func holdOriginalPublication(t *testing.T, stage string, withMarker bool) *heldPublicationFixture {
	t.Helper()
	held := &heldPublicationFixture{}
	t.Cleanup(func() { closeOriginalPublication(t, held) })
	var err error
	held.root, err = os.OpenRoot(stage)
	if err != nil {
		t.Fatal(err)
	}
	held.selected, err = held.root.OpenRoot("plugins")
	if err != nil {
		t.Fatal(err)
	}
	held.entries = []OriginalPublicationEntry{
		{RelativePath: ".", Info: originalRootInfo(t, held.root)},
		{RelativePath: "plugins", Info: originalRootInfo(t, held.selected)},
	}
	if withMarker {
		held.marker, err = held.selected.Open("marker")
		if err != nil {
			t.Fatal(err)
		}
		info, err := held.marker.Stat()
		if err != nil {
			t.Fatal(err)
		}
		held.entries = append(held.entries, OriginalPublicationEntry{RelativePath: "plugins/marker", Info: info})
	}
	return held
}

func originalRootInfo(t *testing.T, root *os.Root) os.FileInfo {
	t.Helper()
	info, err := root.Stat(".")
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func closeOriginalPublication(t *testing.T, held *heldPublicationFixture) {
	t.Helper()
	if held.marker != nil {
		if err := held.marker.Close(); err != nil {
			t.Error(err)
		}
		held.marker = nil
	}
	if held.selected != nil {
		if err := held.selected.Close(); err != nil {
			t.Error(err)
		}
		held.selected = nil
	}
	if held.root != nil {
		if err := held.root.Close(); err != nil {
			t.Error(err)
		}
		held.root = nil
	}
}

func sealOriginalPublication(t *testing.T, input *Input) {
	t.Helper()
	held := holdOriginalPublication(t, input.StagingPath, true)
	proof, err := CaptureOriginalPublication(held.root, held.entries)
	if err != nil || proof.IsZero() {
		t.Fatalf("capture original publication: %v", err)
	}
	input.ExpectedPublication = proof
	closeOriginalPublication(t, held)
}

func heldPublicationInfo(t *testing.T, name string) os.FileInfo {
	t.Helper()
	file, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	info, statErr := file.Stat()
	if err := errors.Join(statErr, file.Close()); err != nil {
		t.Fatal(err)
	}
	return info
}

func copyPublicationFile(t *testing.T, from, to string) {
	t.Helper()
	info := heldPublicationInfo(t, from)
	body, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, body, info.Mode().Perm()); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(to, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
}

func replacePublicationObject(t *testing.T, base, tree, which string) (string, string, os.FileInfo) {
	t.Helper()
	target := filepath.Join(tree, "plugins")
	if which == "marker" {
		target = filepath.Join(target, "marker")
	}
	before := heldPublicationInfo(t, target)
	saved := filepath.Join(base, "original-"+which)
	if err := os.Rename(target, saved); err != nil {
		t.Fatal(err)
	}
	if which == "child" {
		if err := os.Mkdir(target, before.Mode().Perm()); err != nil {
			t.Fatal(err)
		}
		copyPublicationFile(t, filepath.Join(saved, "marker"), filepath.Join(target, "marker"))
		if err := os.Chtimes(target, before.ModTime(), before.ModTime()); err != nil {
			t.Fatal(err)
		}
	} else {
		copyPublicationFile(t, saved, target)
	}
	after := heldPublicationInfo(t, target)
	if os.SameFile(before, after) || before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("replacement did not preserve metadata while changing physical identity")
	}
	return target, saved, after
}

func TestCaptureOriginalPublicationRejectsReplacedHeldObjects(t *testing.T) {
	for _, which := range []string{"child", "marker"} {
		t.Run(which, func(t *testing.T) {
			_, input := originalPublicationFixture(t)
			if which == "child" {
				if err := os.Remove(filepath.Join(input.StagingPath, "plugins", "marker")); err != nil {
					t.Fatal(err)
				}
			}
			held := holdOriginalPublication(t, input.StagingPath, which == "marker")
			if which == "child" {
				target := filepath.Join(input.StagingPath, "plugins")
				if err := os.Rename(target, filepath.Join(input.OwnedBase, "original-child")); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(target, held.entries[1].Info.Mode().Perm()); err != nil {
					t.Fatal(err)
				}
				mtime := held.entries[1].Info.ModTime()
				if err := os.Chtimes(target, mtime, mtime); err != nil {
					t.Fatal(err)
				}
			} else {
				replacePublicationObject(t, input.OwnedBase, input.StagingPath, which)
				held.entries[1].Info = originalRootInfo(t, held.selected)
			}
			// Only the unchanged original root's temporal observation is refreshed.
			held.entries[0].Info = originalRootInfo(t, held.root)
			if proof, err := CaptureOriginalPublication(held.root, held.entries); err == nil || !proof.IsZero() {
				t.Fatalf("adopted substituted original %s: %v", which, err)
			}
		})
	}
}

func TestOriginalPublicationRejectsSameBytesBeforeFirstJournal(t *testing.T) {
	for _, which := range []string{"child", "marker"} {
		t.Run(which, func(t *testing.T) {
			manager, input := originalPublicationFixture(t)
			sealOriginalPublication(t, &input)
			legacyIdentity, legacyDigest, err := publicationProof(input.StagingPath)
			if err != nil {
				t.Fatal(err)
			}
			target, saved, foreign := replacePublicationObject(t, input.OwnedBase, input.StagingPath, which)
			if err := matchesProof(input.StagingPath, legacyIdentity, legacyDigest); err != nil {
				t.Fatalf("counterexample changed legacy content evidence: %v", err)
			}
			if receipt, err := manager.Apply(t.Context(), input); err == nil || receipt.OperationID != "" {
				t.Fatalf("adopted copied %s before intent: %v", which, err)
			}
			if _, err := os.Lstat(manager.journalPath(input.OperationID)); !os.IsNotExist(err) {
				t.Fatalf("created a journal for substituted originals: %v", err)
			}
			if _, err := os.Lstat(input.ActivePath); !os.IsNotExist(err) {
				t.Fatalf("published substituted originals: %v", err)
			}
			assertPublicationCopyRetained(t, target, saved, foreign, which)
		})
	}
}

func assertPublicationCopyRetained(t *testing.T, target, saved string, foreign os.FileInfo, which string) {
	t.Helper()
	if !os.SameFile(foreign, heldPublicationInfo(t, target)) {
		t.Fatal("refusal changed the foreign object's identity")
	}
	if which == "child" {
		target, saved = filepath.Join(target, "marker"), filepath.Join(saved, "marker")
	}
	for _, name := range []string{target, saved} {
		body, err := os.ReadFile(name)
		if err != nil || string(body) != objectPublicationMarker {
			t.Fatalf("original or foreign bytes changed: %s: %v", name, err)
		}
	}
}

func TestOriginalPublicationColdRecoveryRetainsCopiedObjects(t *testing.T) {
	for _, phase := range []string{PhaseBackupPending, FaultActivationApplied, FaultRollbackQuarantined} {
		for _, which := range []string{"child", "marker"} {
			t.Run(phase+"/"+which, func(t *testing.T) {
				assertOriginalPublicationColdRefusal(t, phase, which)
			})
		}
	}
}

func assertOriginalPublicationColdRefusal(t *testing.T, phase, which string) {
	t.Helper()
	manager, input := originalPublicationFixture(t)
	sealOriginalPublication(t, &input)
	crash := errors.New("crash after original object substitution")
	var target, saved string
	var foreign os.FileInfo
	manager.Fault = func(at string) error {
		if at != phase {
			return nil
		}
		tree := input.ActivePath
		if phase == PhaseBackupPending {
			tree = input.StagingPath
		} else if phase == FaultRollbackQuarantined {
			record, err := manager.Load(input.OperationID)
			if err != nil {
				return err
			}
			tree = record.QuarantinePath
		}
		target, saved, foreign = replacePublicationObject(t, input.OwnedBase, tree, which)
		return crash
	}
	receipt, err := manager.Apply(t.Context(), input)
	if phase == FaultRollbackQuarantined && err == nil {
		err = manager.Rollback(t.Context(), receipt)
	}
	if !errors.Is(err, crash) || target == "" {
		t.Fatalf("did not reach original crash window: %v", err)
	}
	manager.Fault = nil
	assertPublicationJournalRefusal(t, manager, receipt, input.OperationID)
	assertPublicationCopyRetained(t, target, saved, foreign, which)
}

func assertPublicationJournalRefusal(t *testing.T, manager Manager, receipt Receipt, operationID string) {
	t.Helper()
	journal := manager.journalPath(operationID)
	before, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Load(operationID); err == nil {
		t.Fatal("Load admitted changed original publication")
	}
	if _, err := manager.ListOpen(); err == nil {
		t.Fatal("ListOpen admitted changed original publication")
	}
	if err := manager.Rollback(t.Context(), receipt); err == nil {
		t.Fatal("rollback admitted changed original publication")
	}
	cold := Manager{JournalDir: manager.JournalDir}
	if err := cold.Recover(t.Context(), operationID, false); err == nil {
		t.Fatal("cold recovery admitted changed original publication")
	}
	after, err := os.ReadFile(journal)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("refusal rewrote or removed the original journal: %v", err)
	}
}

func TestOriginalPublicationProofSurvivesRenameAndColdRollback(t *testing.T) {
	for _, phase := range []string{FaultActivationApplied, FaultRollbackQuarantined, FaultRollbackRemoved} {
		t.Run(phase, func(t *testing.T) {
			manager, input := originalPublicationFixture(t)
			sealOriginalPublication(t, &input)
			crash := errors.New("original publication crash")
			manager.Fault = func(at string) error {
				if at == phase {
					return crash
				}
				return nil
			}
			receipt, err := manager.Apply(t.Context(), input)
			if phase != FaultActivationApplied && err == nil {
				err = manager.Rollback(t.Context(), receipt)
			}
			if !errors.Is(err, crash) || receipt.PublishedDigest != input.ExpectedPublication.digest {
				t.Fatalf("did not retain the sealed proof through rename: %v", err)
			}
			cold := Manager{JournalDir: manager.JournalDir}
			if err := cold.Recover(t.Context(), input.OperationID, false); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{input.ActivePath, receipt.QuarantinePath, manager.journalPath(input.OperationID)} {
				if _, err := os.Lstat(name); !os.IsNotExist(err) {
					t.Fatalf("cold rollback did not finish: %s: %v", name, err)
				}
			}
		})
	}
}

func TestOriginalPublicationCommittedAndLegacyProofsRemainReadable(t *testing.T) {
	for _, strong := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "objects"}[strong], func(t *testing.T) {
			manager, input := originalPublicationFixture(t)
			if strong {
				sealOriginalPublication(t, &input)
			}
			receipt, err := manager.Apply(t.Context(), input)
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(receipt.PublishedDigest, publicationObjectsPrefix) != strong {
				t.Fatal("changed the caller's explicit proof format")
			}
			cold := Manager{JournalDir: manager.JournalDir}
			loaded, err := cold.Load(input.OperationID)
			if err != nil || loaded.PublishedDigest != receipt.PublishedDigest {
				t.Fatalf("original receipt could not be reloaded: %v", err)
			}
			if err := cold.Recover(t.Context(), input.OperationID, true); err != nil {
				t.Fatal(err)
			}
			if err := matchesPublication(receipt, input.ActivePath); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(manager.journalPath(input.OperationID)); !os.IsNotExist(err) {
				t.Fatalf("committed journal remains: %v", err)
			}
		})
	}
}

func TestOriginalPublicationRequiresFreshOriginalParent(t *testing.T) {
	for _, variant := range []string{"missing-parent", "replacement", "remove"} {
		t.Run(variant, func(t *testing.T) {
			manager, input := originalPublicationFixture(t)
			sealOriginalPublication(t, &input)
			switch variant {
			case "missing-parent":
				input.PublicationParentAuthority = nil
			case "replacement":
				input.RequireAbsent = false
			case "remove":
				input.Remove = true
			}
			calls := 0
			input.VerifyActive = func(_ context.Context, _ string) error {
				calls++
				return nil
			}
			if _, err := manager.Apply(t.Context(), input); err == nil || calls != 0 {
				t.Fatalf("invalid original publication role reached verification: %v, %d", err, calls)
			}
			if _, err := os.Lstat(manager.journalPath(input.OperationID)); !os.IsNotExist(err) {
				t.Fatalf("invalid role created a journal: %v", err)
			}
		})
	}
}

func TestOriginalPublicationRejectsUnknownProofBeforeTerminalCleanup(t *testing.T) {
	for _, digest := range []string{"objects-v2:" + strings.Repeat("a", 64), "objects-v1:" + strings.Repeat("A", 64), "objects-v1:", "objects-v1", "objects-v1:" + strings.Repeat("a", 65)} {
		t.Run(digest, func(t *testing.T) {
			manager, input := originalPublicationFixture(t)
			sealOriginalPublication(t, &input)
			receipt, err := manager.Apply(t.Context(), input)
			if err != nil {
				t.Fatal(err)
			}
			receipt.PublishedDigest, receipt.Phase = digest, PhaseCommitted
			if err := manager.save(receipt); err != nil {
				t.Fatal(err)
			}
			journal := manager.journalPath(input.OperationID)
			before, err := os.ReadFile(journal)
			if err != nil {
				t.Fatal(err)
			}
			if err := (Manager{JournalDir: manager.JournalDir}).Recover(t.Context(), input.OperationID, true); err == nil {
				t.Fatal("unknown or malformed object proof reached terminal cleanup")
			}
			after, err := os.ReadFile(journal)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("unknown proof changed its journal: %v", err)
			}
			body, err := os.ReadFile(filepath.Join(input.ActivePath, "plugins", "marker"))
			if err != nil || string(body) != objectPublicationMarker {
				t.Fatalf("unknown proof changed publication bytes: %v", err)
			}
		})
	}
}

func TestOriginalPublicationManifestAndByteBounds(t *testing.T) {
	for _, variant := range []string{"missing", "duplicate", "root", "backslash", "alias", "absolute", "depth", "utf8", "members", "census", "bytes", "aggregate", "hardlink", "unknown"} {
		t.Run(variant, func(t *testing.T) {
			assertOriginalPublicationBound(t, variant)
		})
	}
}

func assertOriginalPublicationBound(t *testing.T, variant string) {
	t.Helper()
	_, input := originalPublicationFixture(t)
	marker := filepath.Join(input.StagingPath, "plugins", "marker")
	switch variant {
	case "bytes", "aggregate":
		size := 4097
		if variant == "aggregate" {
			size = 2049
		}
		if err := os.WriteFile(marker, bytes.Repeat([]byte("a"), size), 0o600); err != nil {
			t.Fatal(err)
		}
	case "hardlink":
		if err := os.Link(marker, filepath.Join(input.OwnedBase, "second-link")); err != nil {
			t.Fatal(err)
		}
	case "unknown":
		if err := os.WriteFile(filepath.Join(input.StagingPath, "plugins", "foreign"), []byte("foreign"), 0o600); err != nil {
			t.Fatal(err)
		}
	case "census":
		for index := 0; index < 9; index++ {
			if err := os.WriteFile(filepath.Join(input.StagingPath, fmt.Sprintf("extra-%d", index)), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	held := holdOriginalPublication(t, input.StagingPath, true)
	entries := publicationManifestVariant(t, held, variant)
	proof, err := CaptureOriginalPublication(held.root, entries)
	if err == nil || !proof.IsZero() {
		t.Fatalf("admitted invalid original publication %s: %v", variant, err)
	}
	if slices.Contains([]string{"members", "census", "bytes", "aggregate"}, variant) && !strings.Contains(err.Error(), "limit") {
		t.Fatalf("expected exact finite-bound refusal for %s: %v", variant, err)
	}
}

func publicationManifestVariant(t *testing.T, held *heldPublicationFixture, variant string) []OriginalPublicationEntry {
	t.Helper()
	entries := append([]OriginalPublicationEntry(nil), held.entries...)
	switch variant {
	case "missing":
		entries = entries[:2]
	case "duplicate":
		entries = append(entries, entries[2])
	case "root":
		entries = entries[1:]
	case "backslash":
		entries[2].RelativePath = "plugins\\marker"
	case "alias":
		entries[2].RelativePath = "plugins/./marker"
	case "absolute":
		entries[2].RelativePath = "/plugins/marker"
	case "depth":
		entries[2].RelativePath = "plugins/nested/marker"
	case "utf8":
		entries[2].RelativePath = string([]byte{0xff})
	case "members":
		entries = append(entries, make([]OriginalPublicationEntry, 6)...)
	case "aggregate":
		entries = appendOriginalPublicationSecondFile(t, held, entries)
	}
	return entries
}

func appendOriginalPublicationSecondFile(t *testing.T, held *heldPublicationFixture, entries []OriginalPublicationEntry) []OriginalPublicationEntry {
	t.Helper()
	file, err := held.selected.OpenFile("second", os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := file.Write(bytes.Repeat([]byte("b"), 2048)); err != nil {
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		t.Fatal(err)
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	entries[1].Info = originalRootInfo(t, held.selected)
	return append(entries, OriginalPublicationEntry{RelativePath: "plugins/second", Info: info})
}

func TestOriginalPublicationMissingRecoveryObjectRetainsJournal(t *testing.T) {
	for _, beforeActivation := range []bool{true, false} {
		t.Run(fmt.Sprint(beforeActivation), func(t *testing.T) {
			manager, input := originalPublicationFixture(t)
			sealOriginalPublication(t, &input)
			if beforeActivation {
				manager.Fault = func(phase string) error {
					if phase == PhaseBackupPending {
						return errors.New("intent crash")
					}
					return nil
				}
			}
			receipt, err := manager.Apply(t.Context(), input)
			if (err != nil) != beforeActivation {
				t.Fatalf("unexpected original phase: %v", err)
			}
			tree := input.ActivePath
			if beforeActivation {
				tree = input.StagingPath
			}
			if err := os.RemoveAll(tree); err != nil {
				t.Fatal(err)
			}
			manager.Fault = nil
			assertPublicationJournalRefusal(t, manager, receipt, input.OperationID)
		})
	}
}

func TestOriginalPublicationRefusesReplacedParentBeforeJournal(t *testing.T) {
	manager, input := originalPublicationFixture(t)
	sealOriginalPublication(t, &input)
	original := input.OwnedBase + "-original"
	if err := os.Rename(input.OwnedBase, original); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(original); err != nil {
			t.Error(err)
		}
	})
	if err := os.Mkdir(input.OwnedBase, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(original, "stage"), input.StagingPath); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Apply(t.Context(), input); !errors.Is(err, directoryidentity.ErrChanged) {
		t.Fatalf("original parent was not physically revalidated first: %v", err)
	}
	if _, err := os.Lstat(manager.journalPath(input.OperationID)); !os.IsNotExist(err) {
		t.Fatalf("replaced parent acquired a journal: %v", err)
	}
	if err := matchesPublicationObjects(input.StagingPath, input.ExpectedPublication.identity, input.ExpectedPublication.digest); err != nil {
		t.Fatalf("parent-refusal fixture changed the original publication itself: %v", err)
	}
}
