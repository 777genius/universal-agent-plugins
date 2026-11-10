package environmentstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func strictStoreFixture(t *testing.T) (Store, domain.PackageSnapshot, Package) {
	t.Helper()
	source, temp := t.TempDir(), t.TempDir()
	for _, name := range []string{"one", "two"} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(name+" original bytes"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := (packagedigest.Builder{TempRoot: temp}).SnapshotWithExecutables(t.Context(), source,
		domain.SourceIdentity{RequestedSource: "first-origin"}, []string{"one", "two"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = packagedigest.Remove(snapshot) })
	store := Store{Root: filepath.Join(t.TempDir(), "store"), TempRoot: t.TempDir()}
	pkg, err := store.Put(t.Context(), snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return store, snapshot, pkg
}

func TestStoreRejectsAmbiguousMetadataAndRepairsFromVerifiedSource(t *testing.T) {
	store, snapshot, pkg := strictStoreFixture(t)
	path := filepath.Join(filepath.Dir(pkg.Root), "metadata.json")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"duplicate":        bytes.Replace(original, []byte(`"schema_version": 1`), []byte(`"schema_version": 1, "schema_version": 1`), 1),
		"folded":           bytes.Replace(original, []byte(`"schema_version": 1`), []byte(`"schema_version": 1, "SCHEMA_VERSION": 1`), 1),
		"escaped":          bytes.Replace(original, []byte(`"schema_version": 1`), []byte(`"schema_version": 1, "schema\u005fversion": 1`), 1),
		"nested-duplicate": bytes.Replace(original, []byte(`"requested_source": "first-origin"`), []byte(`"requested_source": "first-origin", "REQUESTED_SOURCE": "other"`), 1),
		"unknown":          bytes.Replace(original, []byte(`"schema_version": 1`), []byte(`"unexpected": true, "schema_version": 1`), 1),
		"unknown-nested":   bytes.Replace(original, []byte(`"requested_source": "first-origin"`), []byte(`"requested_source": "first-origin", "unexpected": true`), 1),
		"trailing":         append(append([]byte{}, original...), []byte("{}")...),
		"invalid-utf8":     bytes.Replace(original, []byte("first-origin"), []byte{'x', 0xff}, 1),
		"wrong-schema":     bytes.Replace(original, []byte(`"schema_version": 1`), []byte(`"schema_version": 2`), 1),
		"null":             []byte("null"),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if bytes.Equal(body, original) {
				t.Fatal("fixture failed to change metadata")
			}
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Get(t.Context(), snapshot.DigestAlgorithm, snapshot.TreeDigest); !errors.Is(err, ErrMismatch) {
				t.Fatalf("corrupt metadata accepted: %v", err)
			}
			if _, err := store.Snapshot(t.Context(), snapshot.DigestAlgorithm, snapshot.TreeDigest); !errors.Is(err, ErrMismatch) {
				t.Fatalf("snapshot admitted corrupt metadata: %v", err)
			}
			if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, body) {
				t.Fatalf("inspection changed evidence: %v", err)
			}
			assertSnapshotScratchEmpty(t, store)
			if _, err := store.Put(t.Context(), snapshot); err != nil {
				t.Fatalf("verified repair failed: %v", err)
			}
			if got, err := store.Get(t.Context(), snapshot.DigestAlgorithm, snapshot.TreeDigest); err != nil || got.Metadata.TreeDigest != snapshot.TreeDigest {
				t.Fatalf("repaired entry invalid: %+v, %v", got, err)
			}
		})
	}
}

func TestStoreInventoryMustNameActualRegularMembers(t *testing.T) {
	store, snapshot, pkg := strictStoreFixture(t)
	path := filepath.Join(filepath.Dir(pkg.Root), "metadata.json")
	for _, inventory := range [][]string{{"one", "two", "ghost"}, {"one", "two", "one"}, {"one", "two", "ONE"}, {"one", "two", "../escape"}, {"one"}, nil} {
		t.Run(fmt.Sprint(inventory), func(t *testing.T) {
			metadata := pkg.Metadata
			metadata.ExecutableFiles = inventory
			body, err := json.Marshal(metadata)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Get(t.Context(), snapshot.DigestAlgorithm, snapshot.TreeDigest); !errors.Is(err, ErrMismatch) {
				t.Fatalf("unproven inventory accepted: %v", err)
			}
			assertSnapshotScratchEmpty(t, store)
		})
	}
	metadata := pkg.Metadata
	metadata.ExecutableFiles = []string{"two", "one"}
	body, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(t.Context(), snapshot.DigestAlgorithm, snapshot.TreeDigest)
	if err != nil || !reflect.DeepEqual(got.Metadata.ExecutableFiles, []string{"one", "two"}) {
		t.Fatalf("historical order should compare canonically: %+v, %v", got, err)
	}
	if after, err := os.ReadFile(path); err != nil || !bytes.Equal(after, body) {
		t.Fatalf("valid inspection rewrote historical order: %v", err)
	}
}

func TestStoreRejectsDirectoryAndLinkExecutableClaims(t *testing.T) {
	for _, kind := range []string{"directory", "link"} {
		t.Run(kind, func(t *testing.T) {
			source := t.TempDir()
			if err := os.WriteFile(filepath.Join(source, "file"), []byte("payload"), 0o600); err != nil {
				t.Fatal(err)
			}
			if kind == "directory" {
				if err := os.Mkdir(filepath.Join(source, "claim"), 0o700); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Symlink("file", filepath.Join(source, "claim")); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			snapshot, err := (packagedigest.Builder{}).SnapshotWithExecutables(t.Context(), source, domain.SourceIdentity{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = packagedigest.Remove(snapshot) }()
			store := Store{Root: filepath.Join(t.TempDir(), "store"), TempRoot: t.TempDir()}
			pkg, err := store.Put(t.Context(), snapshot)
			if err != nil {
				t.Fatal(err)
			}
			pkg.Metadata.ExecutableFiles = []string{"claim"}
			body, err := json.Marshal(pkg.Metadata)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(filepath.Dir(pkg.Root), "metadata.json"), body, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Snapshot(t.Context(), snapshot.DigestAlgorithm, snapshot.TreeDigest); !errors.Is(err, ErrMismatch) {
				t.Fatalf("nonregular executable claim accepted: %v", err)
			}
			assertSnapshotScratchEmpty(t, store)
		})
	}
}

func TestStoreMetadataEnvelopeBoundariesAndLargeInventory(t *testing.T) {
	metadata := Metadata{SchemaVersion: 1, DigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: "sha256:" + strings.Repeat("a", 64)}
	for index := range domain.DefaultMaxFiles {
		metadata.ExecutableFiles = append(metadata.ExecutableFiles, strings.Repeat("deep&path/", 8)+fmt.Sprintf("file-%05d", index))
	}
	body, err := encodeMetadata(metadata)
	if err != nil {
		t.Fatalf("supported large inventory could not be serialized: %v", err)
	}
	got, err := decodeMetadata(body)
	if err != nil || !reflect.DeepEqual(metadata.ExecutableFiles, got.ExecutableFiles) {
		t.Fatalf("supported large inventory did not roundtrip: %v", err)
	}
	padding := bytes.Repeat([]byte(" "), int(MaxMetadataBytes)-len(body))
	atLimit := append(append([]byte{}, body...), padding...)
	if _, err := decodeMetadata(atLimit); err != nil {
		t.Fatalf("exact-limit valid metadata refused: %v", err)
	}
	if _, err := decodeMetadata(append(atLimit, ' ')); !errors.Is(err, ErrMismatch) {
		t.Fatalf("metadata over limit accepted: %v", err)
	}
	metadata.ExecutableFiles = nil
	metadata.Source.RequestedSource = "x"
	base, err := encodeMetadata(metadata)
	if err != nil {
		t.Fatal(err)
	}
	metadata.Source.RequestedSource = strings.Repeat("x", int(MaxMetadataBytes)-len(base)+1)
	if body, err := encodeMetadata(metadata); err != nil || int64(len(body)) != MaxMetadataBytes {
		t.Fatalf("writer exact boundary: length %d, %v", len(body), err)
	}
	metadata.Source.RequestedSource += "x"
	if _, err := encodeMetadata(metadata); !errors.Is(err, ErrMismatch) {
		t.Fatalf("writer over limit accepted: %v", err)
	}
}

func TestStoreSnapshotOwnsVerifiedObservationAndCleansUp(t *testing.T) {
	store, original, pkg := strictStoreFixture(t)
	snapshot, err := store.Snapshot(t.Context(), original.DigestAlgorithm, original.TreeDigest)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Root == pkg.Root || snapshot.TreeDigest != original.TreeDigest || !reflect.DeepEqual(snapshot.ExecutableFiles, original.ExecutableFiles) {
		t.Fatalf("snapshot is not separate verified content: %+v", snapshot)
	}
	if err := os.WriteFile(filepath.Join(pkg.Root, "one"), []byte("changed store bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(snapshot.Root, "one")); err != nil || string(got) != "one original bytes" {
		t.Fatalf("store mutation changed verified observation: %q, %v", got, err)
	}
	if err := packagedigest.Remove(snapshot); err != nil {
		t.Fatal(err)
	}
	assertSnapshotScratchEmpty(t, store)
	if _, err := store.Snapshot(t.Context(), original.DigestAlgorithm, original.TreeDigest); !errors.Is(err, ErrMismatch) {
		t.Fatalf("subsequent observation missed corruption: %v", err)
	}
	assertSnapshotScratchEmpty(t, store)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := store.Snapshot(ctx, original.DigestAlgorithm, original.TreeDigest); !errors.Is(err, context.Canceled) || errors.Is(err, ErrMismatch) {
		t.Fatalf("cancellation was not operational: %v", err)
	}
	assertSnapshotScratchEmpty(t, store)
}

type cancelAfterSnapshotCreated struct {
	context.Context
	cancel    context.CancelFunc
	directory string
	triggered bool
}

func (ctx *cancelAfterSnapshotCreated) Err() error {
	if entries, err := os.ReadDir(ctx.directory); err == nil && len(entries) != 0 {
		ctx.triggered = true
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func TestStoreCancellationDuringSnapshotCleansDisposableBytes(t *testing.T) {
	store, snapshot, _ := strictStoreFixture(t)
	base, cancel := context.WithCancel(t.Context())
	defer cancel()
	ctx := &cancelAfterSnapshotCreated{Context: base, cancel: cancel, directory: store.TempRoot}
	if _, err := store.Snapshot(ctx, snapshot.DigestAlgorithm, snapshot.TreeDigest); !errors.Is(err, context.Canceled) || errors.Is(err, ErrMismatch) {
		t.Fatalf("mid-verification cancellation was not operational: %v", err)
	}
	if !ctx.triggered {
		t.Fatal("fixture did not cancel after disposable snapshot creation")
	}
	assertSnapshotScratchEmpty(t, store)
	if _, err := store.Get(t.Context(), snapshot.DigestAlgorithm, snapshot.TreeDigest); err != nil {
		t.Fatalf("canceled observation damaged stored bytes: %v", err)
	}
}

func TestStoreOversizedPublicationAndFailedRepairPreserveEvidence(t *testing.T) {
	store, snapshot, pkg := strictStoreFixture(t)
	path := filepath.Join(filepath.Dir(pkg.Root), "metadata.json")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	oversized := snapshot
	oversized.Source.RequestedSource = strings.Repeat("x", int(MaxMetadataBytes))
	if _, err := store.Put(t.Context(), oversized); !errors.Is(err, ErrMismatch) {
		t.Fatalf("oversized publication accepted: %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("oversized publication changed healthy metadata: %v", err)
	}
	corrupt := []byte("invalid old metadata retained for recovery")
	if err := os.WriteFile(path, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}
	bad := snapshot
	bad.Root = filepath.Join(t.TempDir(), "missing-source")
	if _, err := store.Put(t.Context(), bad); err == nil {
		t.Fatal("repair with missing source succeeded")
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, corrupt) {
		t.Fatalf("failed repair discarded prior evidence: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(pkg.Root, "one")); err != nil || string(got) != "one original bytes" {
		t.Fatalf("failed repair discarded payload: %q, %v", got, err)
	}
}

func TestStoreIdenticalContentRetainsAdvisoryFirstOrigin(t *testing.T) {
	store, snapshot, _ := strictStoreFixture(t)
	snapshot.Source = domain.SourceIdentity{RequestedSource: "different-origin", CanonicalSource: "another-source"}
	if _, err := store.Put(t.Context(), snapshot); err != nil {
		t.Fatalf("identical content across origins refused: %v", err)
	}
	verified, err := store.Snapshot(t.Context(), snapshot.DigestAlgorithm, snapshot.TreeDigest)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = packagedigest.Remove(verified) }()
	if verified.Source.RequestedSource != "first-origin" {
		t.Fatalf("shared cache provenance unexpectedly rewritten: %+v", verified.Source)
	}
}

func TestStoreMetadataFileAdmissionAndMissingClassification(t *testing.T) {
	for _, kind := range []string{"symlink", "directory", "oversized", "missing-metadata", "missing-entry"} {
		t.Run(kind, func(t *testing.T) {
			store, snapshot, pkg := strictStoreFixture(t)
			path := filepath.Join(filepath.Dir(pkg.Root), "metadata.json")
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			want := ErrMismatch
			switch kind {
			case "symlink":
				target := filepath.Join(t.TempDir(), "source.json")
				if err := os.WriteFile(target, original, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			case "directory":
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			case "oversized":
				file, err := os.Create(path)
				if err != nil {
					t.Fatal(err)
				}
				err = file.Truncate(MaxMetadataBytes + 1)
				_ = file.Close()
				if err != nil {
					t.Fatal(err)
				}
			case "missing-entry":
				if err := os.RemoveAll(filepath.Dir(pkg.Root)); err != nil {
					t.Fatal(err)
				}
				want = ErrMissing
			}
			if _, err := store.Get(t.Context(), snapshot.DigestAlgorithm, snapshot.TreeDigest); !errors.Is(err, want) {
				t.Fatalf("%s: %v; want %v", kind, err, want)
			}
			assertSnapshotScratchEmpty(t, store)
		})
	}
}

func TestStoreOperationalReadFailureDoesNotAuthorizeQuarantine(t *testing.T) {
	_, snapshot, _ := strictStoreFixture(t)
	root := t.TempDir()
	obstruction := filepath.Join(root, "packages")
	const previous = "existing unrelated file"
	if err := os.WriteFile(obstruction, []byte(previous), 0o600); err != nil {
		t.Fatal(err)
	}
	store := Store{Root: root, TempRoot: t.TempDir()}
	for _, operation := range []string{"get", "put"} {
		var err error
		if operation == "get" {
			_, err = store.Get(t.Context(), snapshot.DigestAlgorithm, snapshot.TreeDigest)
		} else {
			_, err = store.Put(t.Context(), snapshot)
		}
		var pathError *os.PathError
		if !errors.As(err, &pathError) || errors.Is(err, ErrMismatch) || errors.Is(err, ErrMissing) {
			t.Fatalf("%s classified operational error as replaceable: %v", operation, err)
		}
		if got, err := os.ReadFile(obstruction); err != nil || string(got) != previous {
			t.Fatalf("%s changed unrelated obstruction: %q, %v", operation, got, err)
		}
		if _, err := os.Stat(filepath.Join(root, ".staging")); !os.IsNotExist(err) {
			t.Fatalf("%s started replacement after operational error: %v", operation, err)
		}
	}
}

func assertSnapshotScratchEmpty(t *testing.T, store Store) {
	t.Helper()
	entries, err := os.ReadDir(store.TempRoot)
	if err != nil || len(entries) != 0 {
		t.Fatalf("verification leaked disposable snapshots: %v, %v", entries, err)
	}
}
