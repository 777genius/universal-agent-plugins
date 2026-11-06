package environmentstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestStoreReadsLargeInventoryFromPreviousWriter(t *testing.T) {
	source := t.TempDir()
	parent := filepath.Join(source, strings.Repeat("a", 190), strings.Repeat("b", 190), strings.Repeat("c", 190))
	if err := os.MkdirAll(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	executables := make([]string, 0, 8_000)
	for index := range 8_000 {
		name := filepath.Join(parent, fmt.Sprintf("script-%04d", index))
		if err := os.WriteFile(name, []byte("#!/bin/sh\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		relative, err := filepath.Rel(source, name)
		if err != nil {
			t.Fatal(err)
		}
		executables = append(executables, filepath.ToSlash(relative))
	}
	snapshot, err := (packagedigest.Builder{TempRoot: t.TempDir()}).SnapshotWithExecutables(t.Context(), source, domain.SourceIdentity{}, executables)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = packagedigest.Remove(snapshot) }()
	store := Store{Root: filepath.Join(t.TempDir(), "store"), TempRoot: t.TempDir()}
	entry, err := store.entryPath(snapshot.DigestAlgorithm, snapshot.TreeDigest)
	if err != nil {
		t.Fatal(err)
	}
	if err := copyTree(snapshot.Root, filepath.Join(entry, "package"), snapshot.ExecutableFiles); err != nil {
		t.Fatal(err)
	}
	// Reproduce the old writer directly, without the current encoder/admission.
	metadata := Metadata{SchemaVersion: 1, DigestAlgorithm: snapshot.DigestAlgorithm, TreeDigest: snapshot.TreeDigest, ExecutableFiles: snapshot.ExecutableFiles, Source: snapshot.Source}
	body, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	body = append(body, '\n')
	if len(body) <= 4<<20 || int64(len(body)) >= MaxMetadataBytes {
		t.Fatalf("fixture must cross the old 4 MiB admission ceiling: %d bytes", len(body))
	}
	if err := os.WriteFile(filepath.Join(entry, "metadata.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(t.Context(), snapshot.DigestAlgorithm, snapshot.TreeDigest); err != nil {
		t.Fatalf("previously published legal inventory was rejected: %v", err)
	}
	if _, err := store.Put(t.Context(), snapshot); err != nil {
		t.Fatalf("legal inventory could not be republished: %v", err)
	}
	assertSnapshotScratchEmpty(t, store)
}
