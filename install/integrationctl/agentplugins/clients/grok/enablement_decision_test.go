package grok

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestPreparedRemovalDecisionPreservesNativeAuthority(t *testing.T) {
	for _, test := range []struct {
		name, attempt, entry string
		missing, owned       bool
		wantError            bool
	}{
		{name: "missing_namespace", attempt: "attempt", missing: true},
		{name: "empty_namespace", attempt: "attempt"},
		{name: "unrelated_journal", attempt: "attempt", entry: "directory-swap.json"},
		{name: "malformed_attempt", attempt: "../foreign", wantError: true},
		{name: "foreign_stage", attempt: "attempt", entry: enablementStagePrefix + "foreign", wantError: true},
		{name: "foreign_record", attempt: "attempt", entry: enablementRecordPrefix + "attempt", wantError: true},
		{name: "other_native_record", attempt: "attempt", entry: enablementRecordPrefix + "foreign", wantError: true},
		{name: "owned_missing_namespace", attempt: "attempt", missing: true, owned: true, wantError: true},
		{name: "owned_empty_namespace", attempt: "attempt", owned: true, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			recovery := EnablementRecovery{OperationsDir: filepath.Join(root, "operations-v2")}
			if !test.missing {
				if err := os.Mkdir(recovery.OperationsDir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			body := []byte("foreign journal: retain these exact bytes\n")
			var original os.FileInfo
			if test.entry != "" {
				path := filepath.Join(recovery.OperationsDir, test.entry)
				if err := os.WriteFile(path, body, 0600); err != nil {
					t.Fatal(err)
				}
				original, err = os.Lstat(path)
				if err != nil {
					t.Fatal(err)
				}
			}
			objects := []domain.NativeObjectOwnership{{Kind: "managed_package_directory"}}
			if test.owned {
				objects = append(objects, domain.NativeObjectOwnership{Kind: enabledMemberKind})
			}
			before := domain.StateFileV2{Installations: []domain.Installation{{
				InstallationID: "installation",
				Clients: map[string]domain.ClientBinding{"binding": {
					ClientBindingID: "binding", ClientID: "grok", Scope: "project",
					Activation: domain.ActivationPrepared, NativeActivationAttempt: test.attempt, NativeObjects: objects,
				}},
			}}}
			originalState, err := json.Marshal(before)
			if err != nil {
				t.Fatal(err)
			}
			handled, err := recovery.PrepareNativeRemovalDecision(t.Context(), "installation", "binding", before, domain.StateFileV2{})
			if handled != test.wantError || (err != nil) != test.wantError {
				t.Fatalf("handled=%v error=%v; want native refusal=%v", handled, err, test.wantError)
			}
			currentState, err := json.Marshal(before)
			if err != nil || !bytes.Equal(currentState, originalState) {
				t.Fatal("removal decision changed its original state", err)
			}
			entries, err := os.ReadDir(recovery.OperationsDir)
			if test.missing {
				if !os.IsNotExist(err) {
					t.Fatal("absent operation namespace changed", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if test.entry == "" {
				if len(entries) != 0 {
					t.Fatal("removal decision created a native journal")
				}
				return
			}
			if len(entries) != 1 || entries[0].Name() != test.entry {
				t.Fatal("original journal inventory changed")
			}
			path := filepath.Join(recovery.OperationsDir, test.entry)
			current, err := os.Lstat(path)
			if err != nil || !os.SameFile(original, current) || current.Mode() != original.Mode() || current.Size() != original.Size() || !current.ModTime().Equal(original.ModTime()) {
				t.Fatal("original journal identity or metadata changed", err)
			}
			currentBody, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(currentBody, body) {
				t.Fatal("original journal bytes changed", err)
			}
		})
	}
}

func TestPreparedRemovalDecisionRefusesAncestorAlias(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "existing-namespace", true: "absent-namespace"}[missing], func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			original, alias := filepath.Join(root, "original"), filepath.Join(root, "alias")
			if err := os.Mkdir(original, 0700); err != nil {
				t.Fatal(err)
			}
			if !missing {
				if err := os.Mkdir(filepath.Join(original, "operations"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(original, alias); err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(alias)
			if err != nil {
				t.Fatal(err)
			}
			state := domain.StateFileV2{Installations: []domain.Installation{{
				InstallationID: "installation", Clients: map[string]domain.ClientBinding{"binding": {
					ClientBindingID: "binding", ClientID: "grok", Scope: "project", NativeActivationAttempt: "attempt",
				}},
			}}}
			recovery := EnablementRecovery{OperationsDir: filepath.Join(alias, "operations")}
			handled, err := recovery.PrepareNativeRemovalDecision(t.Context(), "installation", "binding", state, domain.StateFileV2{})
			if !handled || err == nil || os.IsNotExist(err) {
				t.Fatalf("aliased ancestor was treated as native absence: handled=%t error=%v", handled, err)
			}
			after, err := os.Lstat(alias)
			target, readErr := os.Readlink(alias)
			if err != nil || readErr != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || target != original {
				t.Fatal("refusal changed the original alias", err, readErr)
			}
		})
	}
}
