package agentpluginscli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type dependencyCheckFileObservation struct {
	info os.FileInfo
	body []byte
}

func observeDependencyCheckFiles(t *testing.T, root string) map[string]dependencyCheckFileObservation {
	t.Helper()
	files := map[string]dependencyCheckFileObservation{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		cached, err := entry.Info()
		if err != nil {
			return err
		}
		observation, err := observeDependencyCheckFile(path)
		if err != nil {
			return err
		}
		fresh := observation.info
		if cached.Mode() != fresh.Mode() || cached.Size() != fresh.Size() || !cached.ModTime().Equal(fresh.ModTime()) {
			t.Logf("directory enumeration differs from fresh observation at %s: cached_mode=%s fresh_mode=%s cached_mtime=%s fresh_mtime=%s cached_size=%d fresh_size=%d",
				path, cached.Mode(), fresh.Mode(), cached.ModTime().Format(time.RFC3339Nano), fresh.ModTime().Format(time.RFC3339Nano), cached.Size(), fresh.Size())
		}
		files[path] = observation
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func observeDependencyCheckFile(path string) (dependencyCheckFileObservation, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return dependencyCheckFileObservation{}, err
	}
	if !info.Mode().IsRegular() && !info.IsDir() {
		// Do not follow links or open special files. Windows Lstat defers its
		// identity query until SameFile; freeze that identity before the command.
		if !os.SameFile(info, info) {
			return dependencyCheckFileObservation{}, fmt.Errorf("cannot capture file identity at %s", path)
		}
		return dependencyCheckFileObservation{info: info}, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return dependencyCheckFileObservation{}, err
	}
	// Windows directory enumeration can contain stale NTFS shadow metadata.
	// Handle Stat reads current metadata and captures identity immediately.
	info, err = file.Stat()
	observation := dependencyCheckFileObservation{info: info}
	if err == nil && info.Mode().IsRegular() {
		observation.body, err = io.ReadAll(file)
	}
	return observation, errors.Join(err, file.Close())
}

func dependencyCheckFilesDifference(before, after map[string]dependencyCheckFileObservation) error {
	if len(before) != len(after) {
		return fmt.Errorf("offline check changed persistent directory entries: before=%d after=%d", len(before), len(after))
	}
	for path, expected := range before {
		actual, ok := after[path]
		if !ok {
			return fmt.Errorf("offline check removed persistent path %s", path)
		}
		identity, mode := os.SameFile(expected.info, actual.info), expected.info.Mode() == actual.info.Mode()
		mtime, body := expected.info.ModTime().Equal(actual.info.ModTime()), bytes.Equal(expected.body, actual.body)
		if !identity || !mode || !mtime || !body {
			return fmt.Errorf("offline check changed persistent file %s: identity_equal=%t mode_equal=%t (%s -> %s) mtime_equal=%t (%s -> %s) body_equal=%t (bytes %d -> %d)",
				path, identity, mode, expected.info.Mode(), actual.info.Mode(), mtime, expected.info.ModTime().Format(time.RFC3339Nano), actual.info.ModTime().Format(time.RFC3339Nano), body, len(expected.body), len(actual.body))
		}
	}
	return nil
}

func assertDependencyCheckFilesUnchanged(t *testing.T, root string, before map[string]dependencyCheckFileObservation) {
	t.Helper()
	if err := dependencyCheckFilesDifference(before, observeDependencyCheckFiles(t, root)); err != nil {
		t.Fatal(err)
	}
}

func TestDependencyCheckObservationWithoutCommand(t *testing.T) {
	f := newDependencyCheckFixture(t)
	pin := seedDependencyCheckPackage(t, f, "demo", "")
	writeDependencyCheckAgreement(t, f, []environment.LockedPackage{pin}, "demo")
	// Fixture publication has just renamed a child out of .staging. The observer
	// must be stable even before a command has had any opportunity to read it.
	before := observeDependencyCheckFiles(t, f.root)
	assertDependencyCheckFilesUnchanged(t, f.root, before)
}

func TestDependencyCheckObservationDetectsMutations(t *testing.T) {
	for _, tc := range []struct{ name, member, predicate string }{
		{"body", "file", "body_equal=false"},
		{"file-mode", "file", "mode_equal=false"},
		{"directory-mode", "directory", "mode_equal=false"},
		{"file-mtime", "file", "mtime_equal=false"},
		{"directory-mtime", "directory", "mtime_equal=false"},
		{"file-replacement", "file", "identity_equal=false"},
		{"directory-replacement", "directory", "identity_equal=false"},
		{"entry-add", "extra", ""},
		{"entry-remove", "file", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dependencyWrite(t, filepath.Join(root, "file"), "before")
			if err := os.Mkdir(filepath.Join(root, "directory"), 0o700); err != nil {
				t.Fatal(err)
			}
			before := observeDependencyCheckFiles(t, root)
			path := filepath.Join(root, tc.member)
			if err := mutateDependencyCheckObservation(tc.name, path, before[path], t.TempDir()); err != nil {
				t.Fatal(err)
			}
			after := observeDependencyCheckFiles(t, root)
			if err := dependencyCheckFilesDifference(before, after); err == nil {
				t.Fatal("observer accepted a filesystem mutation")
			}
			if tc.predicate != "" {
				// Isolate the member so parent-directory changes cannot conceal a
				// missed identity, permission, timestamp or content comparison.
				err := dependencyCheckFilesDifference(map[string]dependencyCheckFileObservation{path: before[path]}, map[string]dependencyCheckFileObservation{path: after[path]})
				if err == nil || !strings.Contains(err.Error(), tc.predicate) {
					t.Fatalf("mutation did not fail its own predicate %s: %v", tc.predicate, err)
				}
			}
			if strings.HasSuffix(tc.name, "-replacement") && (before[path].info.Mode() != after[path].info.Mode() || !before[path].info.ModTime().Equal(after[path].info.ModTime()) || !bytes.Equal(before[path].body, after[path].body)) {
				t.Fatal("replacement control failed to preserve original mode, mtime and bytes")
			}
		})
	}
}

func mutateDependencyCheckObservation(kind, path string, before dependencyCheckFileObservation, retained string) error {
	switch kind {
	case "body":
		if err := os.WriteFile(path, []byte("after!"), before.info.Mode().Perm()); err != nil {
			return err
		}
		return os.Chtimes(path, before.info.ModTime(), before.info.ModTime())
	case "file-mode", "directory-mode":
		return os.Chmod(path, before.info.Mode().Perm()&^0o222)
	case "file-mtime", "directory-mtime":
		changed := before.info.ModTime().Add(-time.Hour)
		return os.Chtimes(path, changed, changed)
	case "entry-add":
		return os.WriteFile(path, []byte("new entry"), 0o600)
	case "entry-remove":
		return os.Remove(path)
	default:
		// Retain the old object outside the observed tree so the filesystem cannot
		// reuse its identity for the replacement.
		if err := os.Rename(path, filepath.Join(retained, "original")); err != nil {
			return err
		}
		if before.info.IsDir() {
			if err := os.Mkdir(path, before.info.Mode().Perm()); err != nil {
				return err
			}
		} else if err := os.WriteFile(path, before.body, before.info.Mode().Perm()); err != nil {
			return err
		}
		if err := os.Chmod(path, before.info.Mode().Perm()); err != nil {
			return err
		}
		return os.Chtimes(path, before.info.ModTime(), before.info.ModTime())
	}
}
