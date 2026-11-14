package dirswap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestReplacementRejectsForeignFileAddedAfterPreflight(t *testing.T) {
	m, in := fixture(t)
	m.Fault = func(phase string) error {
		if phase == PhaseBackupPending {
			return os.WriteFile(filepath.Join(in.ActivePath, "foreign"), []byte("keep these bytes"), 0600)
		}
		return nil
	}
	r, err := m.Apply(context.Background(), in)
	if err == nil {
		t.Fatal("replacement accepted a foreign file added after preflight; commit would delete it")
	}
	m.Fault = nil
	if err := m.Rollback(context.Background(), r); err == nil {
		t.Fatal("rollback accepted a changed backup")
	}
	body, err := os.ReadFile(filepath.Join(r.BackupPath, "foreign"))
	if err != nil || string(body) != "keep these bytes" {
		t.Fatalf("foreign bytes lost: %q, %v", body, err)
	}
	assertBody(t, in.StagingPath, "new")
}

func TestReplacementAndRemovalRetainChangedBackup(t *testing.T) {
	for _, remove := range []bool{false, true} {
		for _, point := range []string{FaultBackupRenamed, PhaseActivationPending, PhaseCommitPending, FaultCommitQuarantined} {
			t.Run(fmt.Sprintf("remove=%v/%s", remove, point), func(t *testing.T) {
				m, in := fixture(t)
				in.Remove = remove
				if remove {
					in.StagingPath = ""
				}
				m.Fault = func(at string) error {
					if at != point {
						return nil
					}
					r, err := m.Load(in.OperationID)
					if err != nil {
						return err
					}
					path := r.BackupPath
					if at == FaultCommitQuarantined {
						path = r.QuarantinePath
					}
					return os.WriteFile(filepath.Join(path, "foreign"), []byte("never discard"), 0600)
				}
				r, applyErr := m.Apply(context.Background(), in)
				if point == PhaseCommitPending || point == FaultCommitQuarantined {
					if applyErr != nil {
						t.Fatal(applyErr)
					}
					if err := m.Commit(context.Background(), r); err == nil {
						t.Fatal("commit accepted changed backup")
					}
				} else if applyErr == nil {
					t.Fatal("published over a changed backup")
				}
				m.Fault = nil
				if err := m.Recover(context.Background(), in.OperationID, point == PhaseCommitPending || point == FaultCommitQuarantined); err == nil {
					t.Fatal("recovery discarded changed backup")
				}
				path := r.BackupPath
				if point == FaultCommitQuarantined {
					path = r.QuarantinePath
				}
				assertFileBytes(t, filepath.Join(path, "foreign"), "never discard")
				assertBody(t, path, "old")
			})
		}
	}
}

func TestReplacementRollbackPreservesForeignPublicationChanges(t *testing.T) {
	for _, variant := range []string{"added-file", "bytes", "mode", "empty-dir", "symlink", "replacement"} {
		t.Run(variant, func(t *testing.T) {
			m, in := fixture(t)
			r, err := m.Apply(context.Background(), in)
			if err != nil {
				t.Fatal(err)
			}
			switch variant {
			case "added-file":
				err = os.WriteFile(filepath.Join(in.ActivePath, "foreign"), []byte("foreign"), 0600)
			case "bytes":
				writeBody(t, in.ActivePath, "foreign")
			case "mode":
				changeModeForDrift(t, filepath.Join(in.ActivePath, "body"), 0600)
			case "empty-dir":
				err = os.Mkdir(filepath.Join(in.ActivePath, "foreign-empty"), 0700)
			case "symlink":
				err = os.Symlink("foreign-target", filepath.Join(in.ActivePath, "foreign-link"))
			case "replacement":
				err = os.Rename(in.ActivePath, in.ActivePath+".saved")
				if err == nil {
					writeBody(t, in.ActivePath, "new")
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			identity, digest, err := publicationProof(in.ActivePath)
			if err != nil {
				t.Fatal(err)
			}
			for _, action := range []func() error{
				func() error { return m.Rollback(context.Background(), r) },
				func() error { return m.Recover(context.Background(), in.OperationID, false) },
			} {
				assertRecoveryPaths(t, m, r, action())
				if err := matchesProof(in.ActivePath, identity, digest); err != nil {
					t.Fatal(err)
				}
				assertBody(t, r.BackupPath, "old")
			}
		})
	}
}

func TestReplacementExclusivePublicationAndRestore(t *testing.T) {
	for _, point := range []string{PhaseActivationPending, FaultRollbackRemoved} {
		for _, kind := range []string{"empty", "file", "symlink"} {
			t.Run(point+"/"+kind, func(t *testing.T) {
				m, in := fixture(t)
				foreign := filepath.Join(t.TempDir(), "foreign")
				writeBody(t, foreign, "foreign target")
				m.Fault = func(at string) error {
					if at != point {
						return nil
					}
					switch kind {
					case "empty":
						return os.Mkdir(in.ActivePath, 0700)
					case "file":
						return os.WriteFile(in.ActivePath, []byte("foreign file"), 0600)
					default:
						return os.Symlink(foreign, in.ActivePath)
					}
				}
				r, err := m.Apply(context.Background(), in)
				if point == PhaseActivationPending && err == nil {
					t.Fatal("replaced a foreign entry")
				}
				if point == FaultRollbackRemoved && err != nil {
					t.Fatal(err)
				}
				if point == PhaseActivationPending {
					m.Fault = nil
				}
				if err := m.Rollback(context.Background(), r); err == nil {
					t.Fatal("overwrote foreign entry on restore")
				}
				switch kind {
				case "empty":
					entries, err := os.ReadDir(in.ActivePath)
					if err != nil || len(entries) != 0 {
						t.Fatalf("foreign empty directory changed: %v %v", entries, err)
					}
				case "file":
					assertFileBytes(t, in.ActivePath, "foreign file")
				default:
					got, err := os.Readlink(in.ActivePath)
					if err != nil || got != foreign {
						t.Fatalf("foreign link changed: %q %v", got, err)
					}
				}
				assertBody(t, foreign, "foreign target")
				assertBody(t, r.BackupPath, "old")
			})
		}
	}
}

func TestReplacementRollbackRechecksQuarantinedPublication(t *testing.T) {
	m, in := fixture(t)
	r, err := m.Apply(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	m.Fault = func(at string) error {
		if at != FaultRollbackQuarantined {
			return nil
		}
		if err := os.Rename(r.QuarantinePath, r.QuarantinePath+".saved"); err != nil {
			return err
		}
		writeBody(t, r.QuarantinePath, "foreign quarantine")
		writeBody(t, in.ActivePath, "late foreign active")
		return nil
	}
	err = m.Rollback(context.Background(), r)
	assertRecoveryPaths(t, m, r, err)
	assertBody(t, in.ActivePath, "late foreign active")
	assertBody(t, r.QuarantinePath, "foreign quarantine")
	assertBody(t, r.QuarantinePath+".saved", "new")
	assertBody(t, r.BackupPath, "old")
}

func assertRecoveryPaths(t *testing.T, m Manager, r Receipt, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("missing recovery error")
	}
	for label, path := range map[string]string{
		"active": r.ActivePath, "backup": r.BackupPath,
		"quarantine": r.QuarantinePath, "staging": r.StagingPath,
		"journal": m.journalPath(r.OperationID),
	} {
		want := label + "=" + strconv.Quote(path)
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("missing recovery field %s: %v", want, err)
		}
	}
}

func TestOldOrIncompleteJournalFailsClosed(t *testing.T) {
	for _, variant := range []string{"old-schema", "backup-proof", "publication-proof"} {
		t.Run(variant, func(t *testing.T) {
			m, in := fixture(t)
			r, err := m.Apply(context.Background(), in)
			if err != nil {
				t.Fatal(err)
			}
			switch variant {
			case "old-schema":
				r.SchemaVersion = 3
			case "backup-proof":
				r.BackupDigest = ""
			case "publication-proof":
				r.PublishedDigest = ""
			}
			if err := m.save(r); err != nil {
				t.Fatal(err)
			}
			if err := m.Recover(context.Background(), in.OperationID, false); err == nil {
				t.Fatal("unsafe old journal accepted")
			}
			if err := m.Recover(context.Background(), in.OperationID, true); err == nil {
				t.Fatal("unsafe old journal committed")
			}
			assertBody(t, r.BackupPath, "old")
			assertBody(t, r.ActivePath, "new")
		})
	}
}

func TestExclusiveRenameLongPathRoundTrip(t *testing.T) {
	root := t.TempDir()
	short := filepath.Join(root, "short")
	long := filepath.Join(root, strings.Repeat("long-directory-", 12), strings.Repeat("long-directory-", 12), "publication")
	writeBody(t, short, "owned long-path bytes")
	if err := os.MkdirAll(filepath.Dir(long), 0755); err != nil {
		t.Fatal(err)
	}
	for _, paths := range [][2]string{{short, long}, {long, short}} {
		if err := renameDirectoryExclusive(paths[0], paths[1]); err != nil {
			t.Fatal(err)
		}
		assertBody(t, paths[1], "owned long-path bytes")
		if _, err := os.Lstat(paths[0]); !os.IsNotExist(err) {
			t.Fatalf("rename left source at %q: %v", paths[0], err)
		}
	}
}

func TestReplacementRequiresCallerOwnership(t *testing.T) {
	m, in := fixture(t)
	in.VerifyActive = nil
	if _, err := m.Apply(context.Background(), in); err == nil {
		t.Fatal("unproved replacement accepted")
	}
	in.Remove = true
	if _, err := m.Apply(context.Background(), in); err == nil {
		t.Fatal("unproved removal accepted")
	}
	assertBody(t, in.ActivePath, "old")
	assertBody(t, in.StagingPath, "new")
	if _, err := os.Stat(m.JournalDir); !os.IsNotExist(err) {
		t.Fatalf("rejection left a journal: %v", err)
	}
}

func TestPathReplacementBeforeBackupFailsClosed(t *testing.T) {
	m, in := fixture(t)
	foreign := filepath.Join(t.TempDir(), "foreign")
	writeBody(t, filepath.Join(foreign, "plugin"), "foreign")
	m.Fault = func(at string) error {
		if at != PhaseBackupPending {
			return nil
		}
		if err := os.Rename(in.OwnedBase, in.OwnedBase+".saved"); err != nil {
			return err
		}
		return os.Symlink(foreign, in.OwnedBase)
	}
	r, err := m.Apply(context.Background(), in)
	if err == nil {
		t.Fatal("accepted substituted parent")
	}
	m.Fault = nil
	if err := m.Rollback(context.Background(), r); err == nil {
		t.Fatal("rollback accepted substituted parent")
	}
	assertBody(t, filepath.Join(foreign, "plugin"), "foreign")
	assertBody(t, filepath.Join(in.OwnedBase+".saved", "plugin"), "old")
	assertBody(t, filepath.Join(in.OwnedBase+".saved", "plugin.staging"), "new")
}

func assertFileBytes(t *testing.T, path, want string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil || string(body) != want {
		t.Fatalf("bytes at %s = %q, err=%v; want %q", path, body, err, want)
	}
}

func TestOwnedSymlinksNeverDeleteTheirTargets(t *testing.T) {
	m, in := fixture(t)
	target := filepath.Join(t.TempDir(), "target")
	writeBody(t, target, "outside bytes")
	if err := os.Symlink(target, filepath.Join(in.ActivePath, "owned-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(in.ActivePath, "owned-empty"), 0700); err != nil {
		t.Fatal(err)
	}
	in.VerifyActive = verifyFixture(t, in.ActivePath)
	in.Remove = true
	r, err := m.Apply(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Commit(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	assertBody(t, target, "outside bytes")
}

func TestBackupCollisionPreservesBothTrees(t *testing.T) {
	m, in := fixture(t)
	var backup string
	m.Fault = func(at string) error {
		if at != PhaseBackupPending {
			return nil
		}
		r, err := m.Load(in.OperationID)
		if err != nil {
			return err
		}
		backup = r.BackupPath
		writeBody(t, backup, "foreign backup")
		return nil
	}
	r, err := m.Apply(context.Background(), in)
	if err == nil {
		t.Fatal("overwrote backup collision")
	}
	m.Fault = nil
	if err := m.Rollback(context.Background(), r); err == nil {
		t.Fatal("accepted foreign backup")
	}
	assertBody(t, backup, "foreign backup")
	assertBody(t, in.ActivePath, "old")
	assertBody(t, in.StagingPath, "new")
}

func TestRollbackRejectsReplacedRealParent(t *testing.T) {
	m, in := fixture(t)
	r, err := m.Apply(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	saved := in.OwnedBase + ".saved"
	if err := os.Rename(in.OwnedBase, saved); err != nil {
		t.Fatal(err)
	}
	writeBody(t, in.ActivePath, "foreign")
	if err := m.Rollback(context.Background(), r); err == nil {
		t.Fatal("accepted replacement parent")
	}
	assertBody(t, in.ActivePath, "foreign")
	assertBody(t, filepath.Join(saved, filepath.Base(r.BackupPath)), "old")
	assertBody(t, filepath.Join(saved, filepath.Base(r.ActivePath)), "new")
}

func TestCommitRetainsBackupWhenActiveIsReplaced(t *testing.T) {
	m, in := fixture(t)
	r, err := m.Apply(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(in.ActivePath, in.ActivePath+".saved"); err != nil {
		t.Fatal(err)
	}
	writeBody(t, in.ActivePath, "late foreign active")
	if err := m.Commit(context.Background(), r); err == nil {
		t.Fatal("committed a replaced active tree")
	}
	assertBody(t, in.ActivePath, "late foreign active")
	assertBody(t, in.ActivePath+".saved", "new")
	assertBody(t, r.BackupPath, "old")
}

func TestRollbackRetainsBothCopiesWhenBackupChangesAfterQuarantine(t *testing.T) {
	m, in := fixture(t)
	r, err := m.Apply(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	m.Fault = func(at string) error {
		if at == FaultRollbackQuarantined {
			return os.WriteFile(filepath.Join(r.BackupPath, "foreign"), []byte("late backup bytes"), 0600)
		}
		return nil
	}
	if err := m.Rollback(context.Background(), r); err == nil {
		t.Fatal("ignored backup mutation during quarantine")
	}
	assertBody(t, r.QuarantinePath, "new")
	assertBody(t, r.BackupPath, "old")
	assertFileBytes(t, filepath.Join(r.BackupPath, "foreign"), "late backup bytes")
}

func TestCommitRetainsOriginalWhenActiveChangesAfterBackupQuarantine(t *testing.T) {
	m, in := fixture(t)
	r, err := m.Apply(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	m.Fault = func(at string) error {
		if at == FaultCommitQuarantined {
			return os.WriteFile(filepath.Join(r.ActivePath, "foreign"), []byte("late active bytes"), 0600)
		}
		return nil
	}
	if err := m.Commit(context.Background(), r); err == nil {
		t.Fatal("ignored active mutation during backup quarantine")
	}
	assertBody(t, r.QuarantinePath, "old")
	assertBody(t, r.ActivePath, "new")
	assertFileBytes(t, filepath.Join(r.ActivePath, "foreign"), "late active bytes")
}
