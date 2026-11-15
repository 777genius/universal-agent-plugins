//go:build !linux

package grok

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func preparedRemovalState(owned bool) domain.StateFileV2 {
	binding := domain.ClientBinding{ClientBindingID: "binding", ClientID: "grok", Scope: "project", NativeActivationAttempt: "remove"}
	if owned {
		binding.NativeObjects = []domain.NativeObjectOwnership{{Kind: enabledMemberKind}}
	}
	return domain.StateFileV2{Installations: []domain.Installation{{InstallationID: "installation", Clients: map[string]domain.ClientBinding{"binding": binding}}}}
}

func TestUnqualifiedJournalAllowsOnlyAbsentPreparedRemoval(t *testing.T) {
	for _, owned := range []bool{false, true} {
		t.Run(map[bool]string{false: "prepared", true: "enabled-ownership"}[owned], func(t *testing.T) {
			recovery := EnablementRecovery{OperationsDir: projectTestRoot(t)}
			before := preparedRemovalState(owned)
			original, err := json.Marshal(before)
			if err != nil {
				t.Fatal(err)
			}
			handled, err := recovery.PrepareNativeRemovalDecision(t.Context(), "installation", "binding", before, domain.StateFileV2{})
			if handled != owned || owned && !errors.Is(err, os.ErrNotExist) || !owned && err != nil {
				t.Fatalf("absent journal changed ownership authority: handled=%t error=%v", handled, err)
			}
			after, err := json.Marshal(before)
			if err != nil || !bytes.Equal(original, after) {
				t.Fatal("read-only decision changed its input state", err)
			}
			entries, err := os.ReadDir(recovery.OperationsDir)
			if err != nil || len(entries) != 0 {
				t.Fatal("absence probe created an operation", err)
			}
		})
	}
}

func TestUnqualifiedJournalRetainsExistingRecords(t *testing.T) {
	for _, directory := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary-record", true: "directory"}[directory], func(t *testing.T) {
			recovery := EnablementRecovery{OperationsDir: projectTestRoot(t)}
			path, err := recovery.recordRoot("remove")
			if err != nil {
				t.Fatal(err)
			}
			if directory {
				err = os.Mkdir(path, 0700)
			} else {
				err = os.WriteFile(path, []byte("original unsupported record"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			original, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			handled, err := recovery.PrepareNativeRemovalDecision(t.Context(), "installation", "binding", preparedRemovalState(false), domain.StateFileV2{})
			if !handled || err == nil || errors.Is(err, os.ErrNotExist) {
				t.Fatalf("existing record was treated as absent: handled=%t error=%v", handled, err)
			}
			after, err := os.Lstat(path)
			if err != nil || !os.SameFile(original, after) || original.Mode() != after.Mode() || original.Size() != after.Size() || !original.ModTime().Equal(after.ModTime()) {
				t.Fatal("existing operation changed", err)
			}
			if !directory {
				body, err := os.ReadFile(path)
				if err != nil || string(body) != "original unsupported record" {
					t.Fatal("unsupported journal bytes changed", err)
				}
			}
		})
	}
}

func TestUnqualifiedJournalDoesNotTreatAliasesAsAbsence(t *testing.T) {
	for _, ancestor := range []bool{false, true} {
		t.Run(map[bool]string{false: "dangling-leaf", true: "missing-leaf-under-alias"}[ancestor], func(t *testing.T) {
			root, target := projectTestRoot(t), projectTestRoot(t)
			recovery := EnablementRecovery{OperationsDir: root}
			alias := filepath.Join(root, enablementRecordPrefix+"remove")
			if ancestor {
				alias = filepath.Join(root, "operations")
				recovery.OperationsDir = alias
			} else {
				target = filepath.Join(target, "missing")
			}
			if err := os.Symlink(target, alias); err != nil {
				t.Fatalf("required alias fixture unavailable: %v", err)
			}
			original, err := os.Lstat(alias)
			if err != nil {
				t.Fatal(err)
			}
			body, err := recovery.readRecordBytes("remove")
			if body != nil || err == nil || errors.Is(err, os.ErrNotExist) {
				t.Fatal("alias was treated as proven absence", err)
			}
			after, err := os.Lstat(alias)
			link, readErr := os.Readlink(alias)
			if err != nil || readErr != nil || !os.SameFile(original, after) || link != target {
				t.Fatal("refusal changed the alias", err, readErr)
			}
		})
	}
}

func TestUnqualifiedJournalValidatesOperationBeforeAbsence(t *testing.T) {
	recovery := EnablementRecovery{OperationsDir: projectTestRoot(t)}
	for _, operationID := range []string{"", "../remove", "a/b"} {
		body, err := recovery.readRecordBytes(operationID)
		if body != nil || err == nil || errors.Is(err, os.ErrNotExist) {
			t.Fatalf("invalid operation %q reached the absence path: %v", operationID, err)
		}
	}
}
