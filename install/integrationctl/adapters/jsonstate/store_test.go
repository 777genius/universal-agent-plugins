package jsonstate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fsadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/fs"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestStoreLoadSaveRoundTrip(t *testing.T) {
	t.Parallel()
	store := Store{
		FS:   fsadapter.OS{},
		Path: filepath.Join(t.TempDir(), "state.json"),
	}
	state := ports.StateFile{SchemaVersion: 1}
	if err := store.Save(context.Background(), state); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d, want 1", got.SchemaVersion)
	}
}

func TestStoreRejectsNewerSchemaWithoutRewritingIt(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "state.json")
	original := []byte("{\"schema_version\":2,\"future_field\":true}\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("write future state: %v", err)
	}
	store := Store{FS: fsadapter.OS{}, Path: path}
	if _, err := store.Load(context.Background()); err == nil || !strings.Contains(err.Error(), "unsupported state schema_version 2") {
		t.Fatalf("load error = %v, want unsupported schema", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read future state: %v", err)
	}
	if string(after) != string(original) {
		t.Fatalf("future state was rewritten: got %q want %q", after, original)
	}
}

func TestStoreRejectsSavingUnsupportedSchema(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "state.json")
	store := Store{FS: fsadapter.OS{}, Path: path}
	if err := store.Save(context.Background(), ports.StateFile{SchemaVersion: 2}); err == nil || !strings.Contains(err.Error(), "refusing to write unsupported") {
		t.Fatalf("save error = %v, want unsupported schema rejection", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("unsupported state file exists or stat failed: %v", err)
	}
}

func TestStoreRejectsUnknownTopLevelFields(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{\"schema_version\":1,\"future_field\":true}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := Store{FS: fsadapter.OS{}, Path: path}
	if _, err := store.Load(context.Background()); err == nil || !strings.Contains(err.Error(), "decode integration state") {
		t.Fatalf("load error = %v, want unknown field rejection", err)
	}
}

func TestStoreRejectsEmptySavePath(t *testing.T) {
	t.Parallel()
	store := Store{FS: fsadapter.OS{}}
	if err := store.Save(context.Background(), ports.StateFile{SchemaVersion: 1}); err == nil || !strings.Contains(err.Error(), "state path required") {
		t.Fatalf("save error = %v, want empty path rejection", err)
	}
}
