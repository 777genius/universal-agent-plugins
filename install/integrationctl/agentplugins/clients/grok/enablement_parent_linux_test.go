//go:build linux

package grok

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

func enablementFixtureParent(t *testing.T, root string) directoryidentity.Authority {
	t.Helper()
	parent, err := captureEnablementParent(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	return parent
}

func replaceEnablementParent(t *testing.T, fixture enablementFixture) {
	t.Helper()
	root := fixture.record.UserRoot
	before, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	originals := capturePrototypeFiles(t, fixture.config, filepath.Join(root, ".config-init.lock"))
	if err := os.Rename(root, root+"-retained"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"config.toml", ".config-init.lock"} {
		if err := os.Rename(filepath.Join(root+"-retained", name), filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	after, err := os.Stat(root)
	if err != nil || os.SameFile(before, after) {
		t.Fatal("fixture did not replace the User parent", err)
	}
	assertPrototypeFilesUnchanged(t, originals)
}

func coldEnablementRecovery(fixture enablementFixture) EnablementRecovery {
	return EnablementRecovery{ProjectRoot: fixture.record.ProjectRoot, UserRoot: fixture.record.UserRoot,
		OperationsDir: fixture.recovery.OperationsDir, Kernel: nativeconfig.New(),
		State:           transaction.Kernel{StateStore: statev2.Store{Path: fixture.state.Path, ProjectRoot: fixture.record.ProjectRoot}},
		PackageVerifier: enablementTestPackage{path: fixture.record.ArtifactPath, digest: fixture.record.PackageDigest}}
}

func coldParentFixture(t *testing.T, stage string) enablementFixture {
	t.Helper()
	if stage == "remove-native" || stage == "remove-decision" || stage == "remove-state" {
		fixture, desired := removalEnablementFixture(t)
		applyRemovalFixture(t, fixture)
		if stage != "remove-native" {
			if _, err := fixture.recovery.PrepareNativeRemovalDecision(t.Context(), fixture.record.InstallationID, fixture.record.BindingID, fixture.loaded(t), desired); err != nil {
				t.Fatal(err)
			}
		}
		if stage == "remove-state" {
			if err := fixture.state.Save(desired); err != nil {
				t.Fatal(err)
			}
		}
		return fixture
	}
	fixture := newEnablementFixture(t)
	if stage != "journal" {
		fixture.attempt(t, stage != "attempt", stage == "state-commit")
	}
	return fixture
}

func TestEnablementColdParentRejectsOriginalLeavesInReplacement(t *testing.T) {
	for _, stage := range []string{"journal", "attempt", "native-write", "state-commit", "remove-native", "remove-decision", "remove-state"} {
		t.Run(stage, func(t *testing.T) {
			fixture := coldParentFixture(t, stage)
			journal, err := fixture.recovery.recordRoot(fixture.record.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			originals := capturePrototypeFiles(t, fixture.config, fixture.state.Path, journal, filepath.Join(fixture.record.UserRoot, ".config-init.lock"))
			replaceEnablementParent(t, fixture)
			cold := coldEnablementRecovery(fixture)
			if err := cold.recoverRecord(t.Context(), fixture.record.OperationID); !errors.Is(err, directoryidentity.ErrChanged) {
				t.Fatal("private cold recovery adopted a replacement User parent", err)
			}
			assertPrototypeFilesUnchanged(t, originals)
		})
	}
}

func TestEnablementColdParentRecoversOriginalIncarnation(t *testing.T) {
	fixture := newEnablementFixture(t)
	fixture.attempt(t, true, false)
	cold := coldEnablementRecovery(fixture)
	if err := cold.recoverRecord(t.Context(), fixture.record.OperationID); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(fixture.config)
	if err != nil || !bytes.Equal(current, fixture.original) || enablementStateDigest(fixture.loaded(t)) != fixture.record.BeforeStateDigest {
		t.Fatal("cold recovery changed the original predecessor", err)
	}
	if err := directoryidentity.Revalidate(t.Context(), fixture.record.UserParentAuthority); err != nil {
		t.Fatal("recovery changed original parent authority", err)
	}
}

func missingParentRecord(t *testing.T, fixture enablementFixture, mode string) []byte {
	t.Helper()
	record, err := fixture.recovery.readRecord(fixture.record.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	record.Hash = ""
	unsigned, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := json.Marshal(record.UserParentAuthority)
	if err != nil {
		t.Fatal(err)
	}
	field := append([]byte(",\"user_parent_authority\":"), parent...)
	if bytes.Count(unsigned, field) != 1 {
		t.Fatal("parent fixture field differs")
	}
	switch mode {
	case "v1":
		unsigned = bytes.Replace(unsigned, []byte("\"version\":2,"), []byte("\"version\":1,"), 1)
		fallthrough
	case "missing":
		unsigned = bytes.Replace(unsigned, field, nil, 1)
	case "null":
		unsigned = bytes.Replace(unsigned, field, []byte(",\"user_parent_authority\":null"), 1)
	case "foreign-root":
		root := record.UserRoot + "-foreign"
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
		foreign, err := json.Marshal(enablementFixtureParent(t, root))
		if err != nil {
			t.Fatal(err)
		}
		unsigned = bytes.Replace(unsigned, field, append([]byte(",\"user_parent_authority\":"), foreign...), 1)
	default:
		t.Fatal("unknown parent fixture mutation")
	}
	return bytes.Replace(unsigned, []byte("\"hash\":\"\""), []byte("\"hash\":\""+enablementDigest(unsigned)+"\""), 1)
}

func TestEnablementColdParentRefusesLegacyAndAbsentAuthority(t *testing.T) {
	for _, mode := range []string{"v1", "missing", "null", "foreign-root"} {
		t.Run(mode, func(t *testing.T) {
			fixture := newEnablementFixture(t)
			fixture.attempt(t, true, false)
			journal, err := fixture.recovery.recordRoot(fixture.record.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(journal, missingParentRecord(t, fixture, mode), 0600); err != nil {
				t.Fatal(err)
			}
			originals := capturePrototypeFiles(t, fixture.config, fixture.state.Path, journal)
			cold := coldEnablementRecovery(fixture)
			if err := cold.recoverRecord(t.Context(), fixture.record.OperationID); err == nil {
				t.Fatal("cold recovery adopted absent original-parent authority")
			}
			assertPrototypeFilesUnchanged(t, originals)
		})
	}
}

type replacingParentVerifier struct {
	enablementTestPackage
	replace func()
}

func (verifier replacingParentVerifier) Verify(ctx context.Context, path, digest string) error {
	if err := verifier.enablementTestPackage.Verify(ctx, path, digest); err != nil {
		return err
	}
	verifier.replace()
	return nil
}

func TestEnablementColdParentRejoinsBeforeConfigAndStateWrites(t *testing.T) {
	for _, stage := range []string{"native-write", "state-commit"} {
		t.Run(stage, func(t *testing.T) {
			fixture := coldParentFixture(t, stage)
			journal, _ := fixture.recovery.recordRoot(fixture.record.OperationID)
			originals := capturePrototypeFiles(t, fixture.config, fixture.state.Path, journal)
			cold := coldEnablementRecovery(fixture)
			cold.PackageVerifier = replacingParentVerifier{enablementTestPackage: enablementTestPackage{path: fixture.record.ArtifactPath, digest: fixture.record.PackageDigest},
				replace: func() { replaceEnablementParent(t, fixture) }}
			if err := cold.recoverRecord(t.Context(), fixture.record.OperationID); !errors.Is(err, directoryidentity.ErrChanged) {
				t.Fatal("late parent replacement was admitted", err)
			}
			assertPrototypeFilesUnchanged(t, originals)
		})
	}
}

type parentRefusalFileIO struct {
	allow                  bool
	reads, writes, removes int
}

func (files *parentRefusalFileIO) ReadNoFollow(path string) ([]byte, os.FileMode, bool, error) {
	if files.allow {
		snapshot, err := nativeconfig.New().ReadExactFile(path)
		return snapshot.Body, snapshot.Mode, snapshot.Exists, err
	}
	files.reads++
	return nil, 0, false, errors.New("unexpected config read after parent replacement")
}

func (files *parentRefusalFileIO) WriteAtomic(path string, body []byte, mode os.FileMode) error {
	if files.allow {
		return atomicfile.Write(path, body, mode)
	}
	files.writes++
	return errors.New("unexpected config write after parent replacement")
}

func (files *parentRefusalFileIO) RemoveNoFollow(string) error {
	files.removes++
	return errors.New("unexpected config removal after parent replacement")
}

func TestEnablementColdParentRefusesPredecessorBeforeConfigRead(t *testing.T) {
	fixture := coldParentFixture(t, "journal")
	journal, err := fixture.recovery.recordRoot(fixture.record.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	originals := capturePrototypeFiles(t, fixture.config, fixture.state.Path, journal,
		filepath.Join(fixture.record.UserRoot, ".config-init.lock"))
	cold := coldEnablementRecovery(fixture)
	files := &parentRefusalFileIO{}
	cold.Kernel = nativeconfig.NewWithFileIO(files)
	replaced := false
	cold.PackageVerifier = replacingParentVerifier{
		enablementTestPackage: enablementTestPackage{path: fixture.record.ArtifactPath, digest: fixture.record.PackageDigest},
		replace: func() {
			replaceEnablementParent(t, fixture)
			replaced = true
		},
	}
	if err := cold.recoverRecord(t.Context(), fixture.record.OperationID); !errors.Is(err, directoryidentity.ErrChanged) ||
		!replaced || files.reads != 0 || files.writes != 0 || files.removes != 0 {
		t.Fatalf("lost parent reached config IO: error=%v replaced=%t calls=%+v", err, replaced, files)
	}
	assertPrototypeFilesUnchanged(t, originals)
}
