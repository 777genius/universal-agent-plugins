package dirswap

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func absentFixture(t *testing.T) (Manager, Input) {
	t.Helper()
	root := t.TempDir()
	base := filepath.Join(root, "managed")
	stage := filepath.Join(base, "stage")
	writeBody(t, stage, "owned")
	return Manager{JournalDir: filepath.Join(root, "journal")}, Input{OperationID: "publication", ClientBindingID: "binding", Sequence: 1, OwnedBase: base, ActivePath: filepath.Join(base, "active"), StagingPath: stage, RequireAbsent: true}
}

func TestAbsentExclusivePublicationPreservesEmptyForeignDirectory(t *testing.T) {
	m, in := absentFixture(t)
	m.Fault = func(phase string) error {
		if phase == PhaseActivationPending {
			return os.Mkdir(in.ActivePath, 0700)
		}
		return nil
	}
	r, err := m.Apply(context.Background(), in)
	if err == nil {
		t.Fatal("overwrote foreign empty directory")
	}
	m.Fault = nil
	if err = m.Rollback(context.Background(), r); err == nil {
		t.Fatal("unknown ownership must retain recovery journal")
	}
	if err = m.Recover(context.Background(), in.OperationID, false); err == nil {
		t.Fatal("recovery accepted foreign directory")
	}
	if _, err = os.Stat(in.ActivePath); err != nil {
		t.Fatal(err)
	}
	assertBody(t, in.StagingPath, "owned")
}

func TestAbsentRollbackAndRecoveryPreserveDrift(t *testing.T) {
	for _, variant := range []string{"bytes", "mode", "replacement", "legacy"} {
		t.Run(variant, func(t *testing.T) {
			m, in := absentFixture(t)
			r, err := m.Apply(context.Background(), in)
			if err != nil {
				t.Fatal(err)
			}
			switch variant {
			case "bytes":
				writeBody(t, in.ActivePath, "foreign")
			case "mode":
				if err = os.Chmod(in.ActivePath, 0700); err != nil {
					t.Fatal(err)
				}
			case "replacement":
				if err = os.Rename(in.ActivePath, in.ActivePath+".saved"); err != nil {
					t.Fatal(err)
				}
				writeBody(t, in.ActivePath, "owned")
			case "legacy":
				r.PublishedIdentity = ""
				r.PublishedDigest = ""
			}
			if err = m.Rollback(context.Background(), r); err == nil {
				t.Fatal("rollback accepted drift")
			}
			if err = m.Recover(context.Background(), in.OperationID, false); err == nil {
				t.Fatal("recovery accepted drift")
			}
			if _, err = os.Stat(in.ActivePath); err != nil {
				t.Fatal(err)
			}
			if _, err = m.Load(in.OperationID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAbsentCrashRecoveryAfterPublicationAndQuarantine(t *testing.T) {
	for _, phase := range []string{FaultActivationApplied, FaultRollbackQuarantined, FaultRollbackRemoved} {
		t.Run(phase, func(t *testing.T) {
			m, in := absentFixture(t)
			m.Fault = func(at string) error {
				if at == phase {
					return errors.New("crash")
				}
				return nil
			}
			r, err := m.Apply(context.Background(), in)
			if phase == FaultActivationApplied {
				if err == nil {
					t.Fatal("missing crash")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if err = m.Rollback(context.Background(), r); err == nil {
					t.Fatal("missing rollback crash")
				}
			}
			m.Fault = nil
			if err = m.Recover(context.Background(), in.OperationID, false); err != nil {
				t.Fatal(err)
			}
			if _, err = os.Lstat(in.ActivePath); !os.IsNotExist(err) {
				t.Fatalf("active persists: %v", err)
			}
			if _, err = m.Load(in.OperationID); !os.IsNotExist(err) {
				t.Fatalf("journal persists: %v", err)
			}
		})
	}
}

func TestAbsentRecoveryPreservesChangedQuarantine(t *testing.T) {
	m, in := absentFixture(t)
	r, err := m.Apply(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	m.Fault = func(at string) error {
		if at == FaultRollbackQuarantined {
			return errors.New("crash")
		}
		return nil
	}
	if err = m.Rollback(context.Background(), r); err == nil {
		t.Fatal("missing crash")
	}
	writeBody(t, r.BackupPath, "foreign")
	m.Fault = nil
	if err = m.Recover(context.Background(), in.OperationID, false); err == nil {
		t.Fatal("recovery accepted changed quarantine")
	}
	assertBody(t, r.BackupPath, "foreign")
	if _, err = m.Load(in.OperationID); err != nil {
		t.Fatal(err)
	}
}

func TestAbsentRollbackPreservesReplacementDuringQuarantine(t *testing.T) {
	m, in := absentFixture(t)
	r, err := m.Apply(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	m.Fault = func(at string) error {
		if at == FaultRollbackQuarantined {
			if err := os.Rename(r.BackupPath, r.BackupPath+".saved"); err != nil {
				return err
			}
			writeBody(t, r.BackupPath, "foreign")
		}
		return nil
	}
	if err = m.Rollback(context.Background(), r); err == nil {
		t.Fatal("rollback accepted quarantine replacement")
	}
	assertBody(t, r.ActivePath, "foreign")
	m.Fault = nil
	if err = m.Recover(context.Background(), in.OperationID, false); err == nil {
		t.Fatal("recovery accepted replacement")
	}
	assertBody(t, r.ActivePath, "foreign")
}
