package managementstate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

func baselineObservation(t *testing.T) ([]byte, management.Observation) {
	t.Helper()
	// Captured from actual main 999c25fc before extraction through the production
	// CLI + SDK journey. Do not regenerate it with the implementation under test.
	body, err := os.ReadFile("testdata/observation-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var record management.Observation
	if err := json.Unmarshal(body, &record); err != nil {
		t.Fatal(err)
	}
	return body, record
}

func TestBaselineObservationRemainsByteCompatible(t *testing.T) {
	body, original := baselineObservation(t)
	store, err := NewStore(filepath.Join(t.TempDir(), "state"), original.EnvironmentID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.stateRoot); !os.IsNotExist(err) {
		t.Fatal("construction changed the filesystem", err)
	}
	if err := store.ensureDirectory(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.path(original.Operation.RequestId), body, 0o600); err != nil {
		t.Fatal(err)
	}
	byRequest, err := store.ByRequest(original.Operation.RequestId)
	if err != nil || byRequest == nil || !reflect.DeepEqual(*byRequest, original) {
		t.Fatal("baseline request changed", byRequest, err)
	}
	byOperation, err := store.ByOperation(original.Operation.OperationId)
	if err != nil || byOperation == nil || !reflect.DeepEqual(*byOperation, original) {
		t.Fatal("baseline operation changed", byOperation, err)
	}
	byRequest.Operation.LocalSkill.SkillName = "caller-edited"
	if !reflect.DeepEqual(*byOperation, original) {
		t.Fatal("independent reads share mutable state")
	}
	if err := store.Save(*byOperation); err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile(store.path(original.Operation.RequestId))
	if err != nil || !bytes.Equal(encoded, body) {
		t.Fatal("version-1 encoding changed", err)
	}
	if runtime.GOOS != "windows" {
		for path, want := range map[string]os.FileMode{
			store.stateRoot: 0o700, filepath.Dir(store.root): 0o700, store.root: 0o700,
			store.path(original.Operation.RequestId): 0o600,
		} {
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != want {
				t.Fatalf("private observation permissions: %s: %v", path, err)
			}
		}
	}
	if err := store.Delete(original.Operation.RequestId); err != nil {
		t.Fatal(err)
	}
	if absent, err := store.ByRequest(original.Operation.RequestId); err != nil || absent != nil {
		t.Fatal("missing request is not absence", absent, err)
	}
}

func TestObservationReaderPreservesRefusals(t *testing.T) {
	body, original := baselineObservation(t)
	for _, tc := range []struct {
		name string
		body []byte
	}{
		{"corrupt", []byte("not-json")},
		{"extra document", append(append([]byte{}, body...), []byte("{}")...)},
		{"oversized", bytes.Repeat([]byte(" "), management.RecordBytesLimit+1)},
		{"unknown version", bytes.Replace(body, []byte(`"version":1`), []byte(`"version":2`), 1)},
		{"foreign environment", bytes.Replace(body, []byte(original.EnvironmentID), []byte("foreign-environment"), 1)},
		{"foreign context", bytes.Replace(body, []byte(original.ContextID), []byte("foreign-context"), 1)},
		{"wrong request filename", bytes.ReplaceAll(body, []byte(original.Operation.RequestId), []byte("a-different-request-123456"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, err := NewStore(t.TempDir(), original.EnvironmentID)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.ensureDirectory(); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(store.path(original.Operation.RequestId), tc.body, 0o600); err != nil {
				t.Fatal(err)
			}
			if record, err := store.ByRequest(original.Operation.RequestId); err == nil || record != nil {
				t.Fatal("invalid observation was accepted", record, err)
			}
			if _, err := store.List(); err == nil {
				t.Fatal("corrupt history was silently ignored")
			}
		})
	}
}

func TestMutationLockRemainsNonblockingAcrossOwners(t *testing.T) {
	root := t.TempDir()
	first, err := NewLock(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewLock(root)
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := first.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unlock() }()
	result := make(chan error, 1)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	go func() {
		release, err := second.Acquire(ctx)
		if release != nil {
			_ = release()
		}
		result <- err
	}()
	select {
	case err := <-result:
		if !second.IsActive(fmt.Errorf("wrapped contention: %w", err)) {
			t.Fatal("contention classification changed", err)
		}
	case <-ctx.Done():
		t.Fatal("mutation lock waited behind another owner")
	}
}
