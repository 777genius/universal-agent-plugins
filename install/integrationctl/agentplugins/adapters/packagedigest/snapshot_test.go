package packagedigest

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestDigestFramingGoldenIncludesEmptyPrefixModeAndSymlink(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a"), nil, 0o644)
	write(t, filepath.Join(root, "a-prefix"), []byte("prefix"), 0o644)
	write(t, filepath.Join(root, "bin", "x"), []byte("x"), 0o755)
	if err := os.Symlink("a", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	builder := Builder{TempRoot: t.TempDir()}
	var snapshot domain.PackageSnapshot
	var err error
	if runtime.GOOS == "windows" {
		// Windows needs explicit portable Git mode metadata. The POSIX lane
		// continues to prove executable-mode inference from the actual source.
		snapshot, err = builder.SnapshotWithExecutables(context.Background(), root, domain.SourceIdentity{}, []string{"bin/x"})
	} else {
		snapshot, err = builder.Snapshot(context.Background(), root, domain.SourceIdentity{})
	}
	if err != nil {
		t.Fatal(err)
	}
	defer Remove(snapshot)
	const want = "sha256:3f80c5a7d3a7e9a4446f2e49f6de414645643bd5d2ac63192df2686035b59f33"
	if snapshot.TreeDigest != want || snapshot.DigestAlgorithm != domain.TreeDigestAlgorithm {
		t.Fatalf("digest = %s (%s), want %s", snapshot.TreeDigest, snapshot.DigestAlgorithm, want)
	}
	if len(snapshot.ExecutableFiles) != 1 || snapshot.ExecutableFiles[0] != "bin/x" {
		t.Fatalf("executables = %v", snapshot.ExecutableFiles)
	}
	if body, err := os.ReadFile(filepath.Join(snapshot.Root, "a-prefix")); err != nil || string(body) != "prefix" {
		t.Fatalf("sealed snapshot body = %q, %v", body, err)
	}
}

func TestSnapshotIsIndependentFromSourceMutation(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "plugin.json")
	write(t, filename, []byte("first"), 0o644)
	snapshot, err := (Builder{TempRoot: t.TempDir()}).Snapshot(context.Background(), root, domain.SourceIdentity{})
	if err != nil {
		t.Fatal(err)
	}
	defer Remove(snapshot)
	write(t, filename, []byte("second"), 0o644)
	body, err := os.ReadFile(filepath.Join(snapshot.Root, "plugin.json"))
	if err != nil || string(body) != "first" {
		t.Fatalf("snapshot changed with source: %q, %v", body, err)
	}
}

func TestSnapshotRejectsPortablePathAndContentHazards(t *testing.T) {
	tests := map[string]func(*testing.T, string){
		"Git metadata case alias": func(t *testing.T, root string) {
			write(t, filepath.Join(root, ".Git", "config"), nil, 0o644)
		},
		"ownership marker case alias": func(t *testing.T, root string) {
			write(t, filepath.Join(root, ".PLUGIN-KIT-AI.LOCK"), nil, 0o644)
		},
		"lfs": func(t *testing.T, root string) {
			write(t, filepath.Join(root, "large"), []byte("version https://git-lfs.github.com/spec/v1\r\noid sha256:abc\r\nsize 1\r\n"), 0o644)
		},
		"non-normalized Unicode": func(t *testing.T, root string) {
			write(t, filepath.Join(root, "e\u0301"), nil, 0o644)
		},
	}
	tests["external symlink"] = func(t *testing.T, root string) {
		if err := os.Symlink("../outside", filepath.Join(root, "escape")); err != nil {
			t.Fatal(err)
		}
	}
	if runtime.GOOS != "windows" {
		tests["special file"] = func(t *testing.T, root string) {
			listener, err := net.Listen("unix", filepath.Join(root, "socket"))
			if err != nil {
				t.Skipf("Unix sockets unavailable: %v", err)
			}
			t.Cleanup(func() { _ = listener.Close() })
		}
	}
	for name, prepare := range tests {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			prepare(t, root)
			if _, err := (Builder{TempRoot: t.TempDir()}).Snapshot(context.Background(), root, domain.SourceIdentity{}); err == nil {
				t.Fatal("hazard accepted")
			}
		})
	}
}

func TestGitExecutableOverridesIgnoreHostCheckoutModes(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	write(t, filepath.Join(left, "bin", "server"), []byte("same"), 0o644)
	write(t, filepath.Join(right, "bin", "server"), []byte("same"), 0o755)
	builder := Builder{TempRoot: t.TempDir()}
	leftSnapshot, err := builder.SnapshotWithExecutables(context.Background(), left, domain.SourceIdentity{}, []string{"bin/server"})
	if err != nil {
		t.Fatal(err)
	}
	defer Remove(leftSnapshot)
	rightSnapshot, err := builder.SnapshotWithExecutables(context.Background(), right, domain.SourceIdentity{}, []string{"bin/server"})
	if err != nil {
		t.Fatal(err)
	}
	defer Remove(rightSnapshot)
	if leftSnapshot.TreeDigest != rightSnapshot.TreeDigest || len(leftSnapshot.ExecutableFiles) != 1 || len(rightSnapshot.ExecutableFiles) != 1 {
		t.Fatalf("Git-mode digest parity failed: left=%+v right=%+v", leftSnapshot, rightSnapshot)
	}
}

func TestDigestChangesForExecutableMode(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "server"), []byte("same"), 0o644)
	builder := Builder{TempRoot: t.TempDir()}
	regular, err := builder.SnapshotWithExecutables(context.Background(), root, domain.SourceIdentity{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer Remove(regular)
	executable, err := builder.SnapshotWithExecutables(context.Background(), root, domain.SourceIdentity{}, []string{"server"})
	if err != nil {
		t.Fatal(err)
	}
	defer Remove(executable)
	if regular.TreeDigest == executable.TreeDigest {
		t.Fatalf("executable mode did not change digest: %s", regular.TreeDigest)
	}
}

func TestDigestChangesForExactSymlinkTarget(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a"), nil, 0o644)
	write(t, filepath.Join(root, "b"), nil, 0o644)
	link := filepath.Join(root, "link")
	if err := os.Symlink("a", link); err != nil {
		t.Fatal(err)
	}
	builder := Builder{TempRoot: t.TempDir()}
	first, err := builder.Snapshot(context.Background(), root, domain.SourceIdentity{})
	if err != nil {
		t.Fatal(err)
	}
	defer Remove(first)
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("b", link); err != nil {
		t.Fatal(err)
	}
	second, err := builder.Snapshot(context.Background(), root, domain.SourceIdentity{})
	if err != nil {
		t.Fatal(err)
	}
	defer Remove(second)
	if first.TreeDigest == second.TreeDigest {
		t.Fatalf("exact symlink target did not change digest: %s", first.TreeDigest)
	}
}

func TestSnapshotReportsConfiguredSizeLimits(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "too-large"), []byte("12345"), 0o644)
	_, err := (Builder{TempRoot: t.TempDir(), Limits: domain.PackageLimits{MaxFileBytes: 4}}).Snapshot(context.Background(), root, domain.SourceIdentity{})
	if err == nil || !strings.Contains(err.Error(), "4 bytes") {
		t.Fatalf("limit error = %v", err)
	}
}

func TestSnapshotEnforcesTreeCountAndDepthLimits(t *testing.T) {
	tests := map[string]struct {
		limits  domain.PackageLimits
		prepare func(*testing.T, string)
		want    string
	}{
		"tree bytes": {
			limits: domain.PackageLimits{MaxTreeBytes: 4}, want: "4 bytes",
			prepare: func(t *testing.T, root string) {
				write(t, filepath.Join(root, "one"), []byte("123"), 0o644)
				write(t, filepath.Join(root, "two"), []byte("456"), 0o644)
			},
		},
		"path count": {
			limits: domain.PackageLimits{MaxFiles: 1}, want: "path count 1",
			prepare: func(t *testing.T, root string) {
				write(t, filepath.Join(root, "directory", "file"), nil, 0o644)
			},
		},
		"depth": {
			limits: domain.PackageLimits{MaxDepth: 1}, want: "depth 1",
			prepare: func(t *testing.T, root string) {
				write(t, filepath.Join(root, "directory", "file"), nil, 0o644)
			},
		},
	}
	for name, test := range tests {
		name, test := name, test
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			test.prepare(t, root)
			_, err := (Builder{TempRoot: t.TempDir(), Limits: test.limits}).Snapshot(context.Background(), root, domain.SourceIdentity{})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("limit error = %v, want %q", err, test.want)
			}
		})
	}
}

func write(t *testing.T, filename string, body []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, body, mode); err != nil {
		t.Fatal(err)
	}
}
