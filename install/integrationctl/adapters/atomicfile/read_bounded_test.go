package atomicfile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestBoundedRegularReadLimitsAndOperationalErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "authority.json")
	if _, err := ReadBoundedRegularFile(path, 4); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file: %v", err)
	}
	if err := os.WriteFile(path, []byte("1234"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ReadBoundedRegularFile(path, 4); err != nil || string(got) != "1234" {
		t.Fatalf("exact limit: %q, %v", got, err)
	}
	if _, err := ReadBoundedRegularFile(path, 3); !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("oversized file: %v", err)
	}
	permission := &os.PathError{Op: "open", Path: path, Err: os.ErrPermission}
	if _, err := readBoundedRegularFile(path, 4, func(string) (*os.File, error) { return nil, permission }); !errors.Is(err, os.ErrPermission) || errors.Is(err, ErrUnsafeFile) {
		t.Fatalf("permission must remain operational: %v", err)
	}
	after, err := os.Stat(path)
	if err != nil || !sameReadObservation(before, after) {
		t.Fatalf("reader changed the authority file: %v", err)
	}
}

func TestBoundedRegularReadDetectsInspectionToOpenReplacement(t *testing.T) {
	for _, kind := range []string{"identical-file", "directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "authority")
			if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := readBoundedRegularFile(path, 16, func(name string) (*os.File, error) {
				if err := os.Rename(name, name+".old"); err != nil {
					t.Fatal(err)
				}
				switch kind {
				case "identical-file":
					if err := os.WriteFile(name, []byte("old"), 0o600); err != nil {
						t.Fatal(err)
					}
				case "directory":
					if err := os.Mkdir(name, 0o700); err != nil {
						t.Fatal(err)
					}
				case "symlink":
					if err := os.Symlink(name+".old", name); err != nil {
						t.Skipf("symlinks unavailable: %v", err)
					}
				}
				return openRegularNoFollow(name)
			})
			want := ErrUnsafeFile
			if kind == "identical-file" {
				want = ErrFileChanged
			}
			if !errors.Is(err, want) {
				t.Fatalf("replacement %s: %v; want %v", kind, err, want)
			}
			if got, err := os.ReadFile(path + ".old"); err != nil || string(got) != "old" {
				t.Fatalf("original was altered: %q, %v", got, err)
			}
		})
	}
}

func TestAuthorityAndRootedReadHaveDifferentLinkPolicies(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "actual")
	if err := os.WriteFile(path, []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "uap.toml")
	if err := os.Symlink("actual", link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := ReadBoundedRegularFile(link, 16); !errors.Is(err, ErrUnsafeFile) {
		t.Fatalf("authority link was accepted: %v", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	file, err := OpenRootReadFile(root, "uap.toml")
	if err != nil {
		t.Fatalf("contained link refused: %v", err)
	}
	body, err := io.ReadAll(file)
	_ = file.Close()
	if err != nil || string(body) != "inside" {
		t.Fatalf("contained read: %q, %v", body, err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "escape")); err != nil {
		t.Fatal(err)
	}
	if file, err := OpenRootReadFile(root, "escape"); err == nil {
		_ = file.Close()
		t.Fatal("rooted read escaped through link")
	}
}
