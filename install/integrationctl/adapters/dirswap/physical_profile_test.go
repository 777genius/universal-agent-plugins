package dirswap

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Regression: an unqualified TEST filesystem cannot silently authorize an opted effect.
func TestPhysicalProfileTESTCapture(t *testing.T) {
	token, err := directoryidentity.Capture(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if token.IsZero() {
		t.Fatal("capture returned no durable authority")
	}
	t.Logf("positive P1 authority: %+v", token.Facts().Ancestry[len(token.Facts().Ancestry)-1])
}

func physicalFixture(t *testing.T) (Manager, Input, string) {
	t.Helper()
	m, in := fixture(t)
	profile := filepath.Join(t.TempDir(), "TEST-profile")
	if err := os.Mkdir(profile, 0700); err != nil {
		t.Fatal(err)
	}
	token, err := directoryidentity.Capture(t.Context(), profile)
	if err != nil {
		t.Fatal(err)
	}
	m.Namespace = filepath.Dir(in.OwnedBase)
	in.ProfileOwners = []ProfileOwner{{Namespace: m.Namespace, InstallationID: "TEST-installation", ClientID: "TEST-client", ClientBindingID: in.ClientBindingID, Authority: &token}}
	return m, in, profile
}

// Regression: initial schema-4 intent has no durable owner or rename-role proof.
// This also checks a journal-only restart before a binding has ever been saved.
func TestPhysicalProfileInitialIntentDeath(t *testing.T) {
	if root := os.Getenv("UAP_TEST_PHYSICAL_CHILD"); root != "" {
		token, err := directoryidentity.Capture(t.Context(), filepath.Join(root, "TEST-profile"))
		if err != nil {
			t.Fatal(err)
		}
		m := Manager{Namespace: root, JournalDir: filepath.Join(root, "journal"), Fault: func(phase string) error {
			if phase == PhaseBackupPending {
				os.Exit(44)
			}
			return nil
		}}
		_, err = m.Apply(t.Context(), Input{OperationID: "TEST-death", ClientBindingID: "TEST-binding", Sequence: 1, OwnedBase: filepath.Join(root, "managed"), ActivePath: filepath.Join(root, "managed", "plugin"), StagingPath: filepath.Join(root, "managed", "plugin.staging"), VerifyActive: func(_ context.Context, path string) error {
			body, err := os.ReadFile(filepath.Join(path, "body"))
			if err != nil {
				return err
			}
			if string(body) != "old" {
				t.Fatal("unexpected old directory")
			}
			return nil
		}, ProfileOwners: []ProfileOwner{{Namespace: root, InstallationID: "TEST-installation", ClientID: "TEST-client", ClientBindingID: "TEST-binding", Authority: &token}}})
		t.Fatalf("death boundary not reached: %v", err)
	}
	m, in, profile := physicalFixture(t)
	root := m.Namespace
	if err := os.Rename(profile, filepath.Join(root, "TEST-profile")); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestPhysicalProfileInitialIntentDeath$")
	cmd.Env = append(os.Environ(), "UAP_TEST_PHYSICAL_CHILD="+root)
	out, err := cmd.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 44 {
		t.Fatalf("child did not die at admitted boundary: %v %s", err, out)
	}
	raw, err := os.ReadFile(m.journalPath("TEST-death"))
	if err != nil {
		t.Fatal(err)
	}
	var observed Receipt
	if err := json.Unmarshal(raw, &observed); err != nil {
		t.Fatal(err)
	}
	if observed.SchemaVersion != 5 || len(observed.ProfileOwners) != 1 || observed.OwnedBaseAuthority == nil || observed.StagingParentAuthority == nil || observed.OldObject == nil || observed.PublishedObject == nil {
		t.Fatal("first durable intent lacks complete proofs")
	}
	if err := m.Recover(t.Context(), "TEST-death", false); err != nil {
		t.Fatal(err)
	}
	assertBody(t, in.ActivePath, "old")
}

// Regression: same-byte profile or a later schema-5 role substitution authorizes cleanup.
func TestPhysicalProfileRolesAndTerminalCleanup(t *testing.T) {
	for _, role := range []string{"profile", "base", "staging-parent", "active", "backup", "quarantine"} {
		t.Run(role, func(t *testing.T) {
			m, in, profile := physicalFixture(t)
			if role == "staging-parent" {
				in.StagingPath = filepath.Join(filepath.Dir(in.OwnedBase), ".agentplugins-staging-TEST")
				writeBody(t, in.StagingPath, "new")
			}
			r, err := m.Apply(t.Context(), in)
			if err != nil {
				t.Fatal(err)
			}
			path := map[string]string{"profile": profile, "base": in.OwnedBase, "staging-parent": filepath.Dir(in.StagingPath), "active": r.ActivePath, "backup": r.BackupPath, "quarantine": r.QuarantinePath}[role]
			if role == "quarantine" {
				if err := os.Rename(r.BackupPath, r.QuarantinePath); err != nil {
					t.Fatal(err)
				}
				r.Phase = PhaseCommitPending
				if err := m.save(r); err != nil {
					t.Fatal(err)
				}
			}

			moved := path + "-TEST-old"
			if err := os.Rename(path, moved); err != nil {
				t.Fatal(err)
			}
			if role == "staging-parent" {
				t.Cleanup(func() { _ = os.RemoveAll(path); _ = os.Rename(moved, path) })
			}
			if role == "active" || role == "backup" || role == "quarantine" {
				body := "new"
				if role != "active" {
					body = "old"
				}
				writeBody(t, path, body)
			} else {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			journalPath := m.journalPath(r.OperationID)
			if role == "staging-parent" || role == "base" {
				if role == "staging-parent" {
					journalPath = filepath.Join(moved, "journal", r.OperationID+".json")
				}
			}
			raw, err := os.ReadFile(journalPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := m.Commit(t.Context(), r); err == nil {
				t.Fatal("substituted physical role authorized cleanup")
			}
			after, err := os.ReadFile(journalPath)
			if err != nil || !bytes.Equal(raw, after) {
				t.Fatal("refusal changed durable intent")
			}
		})
	}
}
