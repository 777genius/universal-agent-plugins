package environmentstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestStorePublishesExecutableInventoryAndRejectsTampering(t *testing.T) {
	ctx := context.Background()
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"store-fixture"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(source, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "bin", "helper"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	snapshot, err := (packagedigest.Builder{TempRoot: t.TempDir()}).SnapshotWithExecutables(ctx, source,
		domain.SourceIdentity{RequestedSource: "github:acme/tools@0123456789012345678901234567890123456789",
			CanonicalSource: "github:acme/tools@0123456789012345678901234567890123456789", Repository: "acme/tools",
			ResolvedRevision: "0123456789012345678901234567890123456789", SourceBindingHint: "test"},
		[]string{"bin/helper"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = packagedigest.Remove(snapshot) }()
	root := filepath.Join(t.TempDir(), "store")
	store := Store{Root: root, TempRoot: filepath.Join(root, ".tmp"), LockPath: filepath.Join(root, "store.lock")}
	published, err := store.Put(ctx, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(published.Metadata.ExecutableFiles) != 1 || published.Metadata.ExecutableFiles[0] != "bin/helper" {
		t.Fatalf("executable inventory lost: %+v", published.Metadata)
	}
	if _, err := store.Get(ctx, snapshot.DigestAlgorithm, snapshot.TreeDigest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(published.Root, "plugin.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, snapshot.DigestAlgorithm, snapshot.TreeDigest); !errors.Is(err, ErrMismatch) {
		t.Fatalf("tampered entry = %v", err)
	}
	for _, damage := range []string{"tampered", "missing-metadata"} {
		if damage == "missing-metadata" {
			if err := os.Remove(filepath.Join(filepath.Dir(published.Root), "metadata.json")); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := store.Put(ctx, snapshot); err != nil {
			t.Fatalf("repair %s: %v", damage, err)
		}
		if _, err := store.Get(ctx, snapshot.DigestAlgorithm, snapshot.TreeDigest); err != nil {
			t.Fatal(err)
		}
		leftovers, err := filepath.Glob(filepath.Join(root, ".staging", "*-corrupt"))
		if err != nil || len(leftovers) != 0 {
			t.Fatalf("successful repair retained quarantine: %v %v", leftovers, err)
		}

	}
}
