//go:build linux

package process

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func readonlyRuntimeFixture(t *testing.T) (string, *os.File, readonlyRuntimeCensus, [4]readonlyRuntimeRoot) {
	t.Helper()
	base := t.TempDir()
	for _, name := range []string{"bin", "lib", "lib64"} {
		if err := os.Mkdir(filepath.Join(base, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	roots := [4]readonlyRuntimeRoot{}
	for i, visible := range readonlyRuntimePaths() {
		path, logical := base, "/usr"
		if i != 0 {
			path, logical = filepath.Join(base, filepath.Base(visible)), filepath.Join("/usr", filepath.Base(visible))
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = file.Close() })
		info, err := file.Stat()
		if err != nil {
			t.Fatal(err)
		}
		facts, err := readonlyFacts(info)
		if err != nil {
			t.Fatal(err)
		}
		roots[i] = readonlyRuntimeRoot{visible: visible, object: readonlyOriginalObject{path: logical, file: file, facts: facts, directory: true}}
	}
	census := readonlyRuntimeCensus{rows: make(map[string]readonlyRuntimeRow), aliases: make(map[[2]uint64]readonlyRuntimeAlias), uid: uint32(os.Getuid()), gid: uint32(os.Getgid())}
	return base, roots[0].object.file, census, roots
}

func observeReadonlyRuntimeFixture(t *testing.T, census *readonlyRuntimeCensus, roots *[4]readonlyRuntimeRoot) error {
	t.Helper()
	for i := range roots {
		info, err := roots[i].object.file.Stat()
		if err != nil {
			t.Fatal(err)
		}
		roots[i].object.facts, err = readonlyFacts(info)
		if err != nil {
			t.Fatal(err)
		}
	}
	iterator, err := openReadonlyDirectory(roots[0].object.file, ".")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = iterator.Close() })
	census.mount, err = readonlyMountID(iterator, "", unix.AT_EMPTY_PATH)
	if err != nil {
		t.Fatal(err)
	}
	return census.directory(t.Context(), iterator, "/usr", roots[0].object.facts, 0)
}

func TestReadonlyRuntimeCensusJoinsRealInternalAliasesWithoutConsumingBorrow(t *testing.T) {
	base, original, census, roots := readonlyRuntimeFixture(t)
	if err := os.WriteFile(filepath.Join(base, "bin", "native"), []byte("native"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("native", filepath.Join(base, "bin", "alias")); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(base, "bin", "native"), filepath.Join(base, "lib", "native")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/lib/native", filepath.Join(base, "lib64", "alias")); err != nil {
		t.Fatal(err)
	}
	if first, err := original.ReadDir(1); err != nil || len(first) != 1 {
		t.Fatal("original iterator setup differs", err)
	}
	if err := observeReadonlyRuntimeFixture(t, &census, &roots); err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(census.requireAliases(t.Context()), census.requireRoots(t.Context(), roots)); err != nil {
		t.Fatal("complete original runtime aliases were refused", err)
	}
	proof, err := digestReadonlyRuntime(t.Context(), census.rows)
	if err != nil || proof.Count != 8 || len(proof.SHA256) != 64 || census.rows["/usr/lib64/alias"].link != "/lib/native" {
		t.Fatal("runtime census omitted original alias facts", proof, err)
	}
	if rest, err := original.ReadDir(-1); err != nil || len(rest) != 2 {
		t.Fatal("runtime census consumed the borrowed iterator", err)
	}
}

func TestReadonlyRuntimeClosureRefusesExternalHardlinkAndSpecialMember(t *testing.T) {
	for _, fault := range []string{"external-hardlink", "special"} {
		t.Run(fault, func(t *testing.T) {
			base, original, census, roots := readonlyRuntimeFixture(t)
			leaf := filepath.Join(base, "bin", "native")
			if err := os.WriteFile(leaf, []byte("native"), 0700); err != nil {
				t.Fatal(err)
			}
			var err error
			switch fault {
			case "external-hardlink":
				err = os.Link(leaf, filepath.Join(t.TempDir(), "outside"))
			case "special":
				err = unix.Mkfifo(filepath.Join(base, "lib", "fifo"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			err = observeReadonlyRuntimeFixture(t, &census, &roots)
			if err == nil {
				err = errors.Join(census.requireAliases(t.Context()), census.requireRoots(t.Context(), roots))
			}
			if err == nil {
				t.Fatal("incomplete or special runtime closure was admitted", fault)
			}
			if _, err := original.Stat(); err != nil {
				t.Fatal("runtime refusal closed a borrowed root", err)
			}
		})
	}
}

func TestReadonlyRuntimeAliasesAreCapturedDataAndChangesRemainVisible(t *testing.T) {
	for _, target := range []string{"/etc/alternatives/awk", "missing", "alias"} {
		t.Run(target, func(t *testing.T) {
			base, _, census, roots := readonlyRuntimeFixture(t)
			path := filepath.Join(base, "bin", "alias")
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
			if err := observeReadonlyRuntimeFixture(t, &census, &roots); err != nil {
				t.Fatal(err)
			}
			if err := errors.Join(census.requireAliases(t.Context()), census.requireRoots(t.Context(), roots)); err != nil {
				t.Fatal("runtime alias target was followed", err)
			}
			before, err := digestReadonlyRuntime(t.Context(), census.rows)
			if err != nil || census.rows["/usr/bin/alias"].link != target {
				t.Fatal("original runtime alias text was omitted", err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("changed", path); err != nil {
				t.Fatal(err)
			}
			census.rows = make(map[string]readonlyRuntimeRow)
			if err := observeReadonlyRuntimeFixture(t, &census, &roots); err != nil {
				t.Fatal(err)
			}
			after, err := digestReadonlyRuntime(t.Context(), census.rows)
			if err != nil || before == after {
				t.Fatal("changed original runtime alias was ignored", err)
			}
		})
	}
}

func TestReadonlyRuntimeOwnerTranslationAndCancellationAreExplicit(t *testing.T) {
	before := readonlyObjectFacts{Device: 1, Inode: 2, Links: 1, Mode: unix.S_IFREG | 04755, UID: 65534, GID: 65534, Size: 4}
	normalized, err := readonlyRuntimeFacts(before, 65534, 65534)
	if err != nil || normalized.UID != 0 || normalized.GID != 0 || normalized.Mode != before.Mode {
		t.Fatal("known original runtime mapping differs", normalized, err)
	}
	if _, err := readonlyRuntimeFacts(before, 1000, 1000); err == nil {
		t.Fatal("overflow was treated as an arbitrary owner")
	}
	if err := admitReadonlyFacts(normalized, false, 0, 0); err == nil {
		t.Fatal("runtime metadata rules relaxed selected executable admission")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := censusReadonlyRuntime(ctx, [4]readonlyRuntimeRoot{}, 0, 0); !errors.Is(err, context.Canceled) {
		t.Fatal("runtime census reached descriptors after cancellation", err)
	}
}
