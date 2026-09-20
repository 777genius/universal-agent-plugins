package archive

import (
	"archive/tar"
	"strings"
	"testing"
)

func TestArchivePolicyRejectsUnsafeEntriesAndExpansion(t *testing.T) {
	t.Parallel()
	for name, header := range map[string]tar.Header{
		"traversal":    {Name: "../plugin", Typeflag: tar.TypeReg},
		"backslash":    {Name: `nested\\plugin`, Typeflag: tar.TypeReg},
		"hardlink":     {Name: "plugin-link", Typeflag: tar.TypeLink},
		"special":      {Name: "plugin-pipe", Typeflag: tar.TypeFifo},
		"oversized":    {Name: "plugin", Typeflag: tar.TypeReg, Size: maxArchiveFileBytes + 1},
		"sized-dir":    {Name: "nested/", Typeflag: tar.TypeDir, Size: 1},
		"unnormalized": {Name: "nested/../plugin", Typeflag: tar.TypeReg},
		"unicode":      {Name: "plug\u0301in", Typeflag: tar.TypeReg},
		"drive leaf":   {Name: "C:plugin", Typeflag: tar.TypeReg},
		"reserved":     {Name: "CON.exe", Typeflag: tar.TypeReg},
	} {
		name, header := name, header
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := newArchivePolicy().validate(&header); err == nil {
				t.Fatalf("unsafe archive header was accepted: %+v", header)
			}
		})
	}
}

func TestArchivePolicyRejectsExcessivePathDepth(t *testing.T) {
	t.Parallel()
	name := strings.Repeat("a/", maxArchivePathDepth) + "plugin"
	if _, err := newArchivePolicy().validate(&tar.Header{Name: name, Typeflag: tar.TypeReg}); err == nil || !strings.Contains(err.Error(), "segments") {
		t.Fatalf("depth limit error = %v", err)
	}
}

func TestArchivePolicyRejectsDuplicateAndCaseCollidingPaths(t *testing.T) {
	t.Parallel()
	policy := newArchivePolicy()
	first := tar.Header{Name: "README.md", Typeflag: tar.TypeReg}
	second := tar.Header{Name: "readme.md", Typeflag: tar.TypeReg}
	if _, err := policy.validate(&first); err != nil {
		t.Fatal(err)
	}
	if _, err := policy.validate(&second); err == nil || !strings.Contains(err.Error(), "case-colliding") {
		t.Fatalf("collision error = %v", err)
	}
}

func TestArchivePolicyEnforcesEntryAndExpandedSizeLimits(t *testing.T) {
	t.Parallel()
	policy := newArchivePolicy()
	policy.entries = maxArchiveEntries
	if _, err := policy.validate(&tar.Header{Name: "plugin", Typeflag: tar.TypeReg}); err == nil || !strings.Contains(err.Error(), "entries") {
		t.Fatalf("entry limit error = %v", err)
	}
	policy = newArchivePolicy()
	policy.expandedBytes = maxArchiveExpandedBytes
	if _, err := policy.validate(&tar.Header{Name: "plugin", Typeflag: tar.TypeReg, Size: 1}); err == nil || !strings.Contains(err.Error(), "expanded") {
		t.Fatalf("expanded size error = %v", err)
	}
}

func TestArchivePolicyRejectsAncestorCollisionsAndTypeConflicts(t *testing.T) {
	t.Parallel()
	policy := newArchivePolicy()
	if _, err := policy.validate(&tar.Header{Name: "Plugins/one", Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := policy.validate(&tar.Header{Name: "plugins/two", Typeflag: tar.TypeReg}); err == nil || !strings.Contains(err.Error(), "case-colliding") {
		t.Fatalf("ancestor collision error = %v", err)
	}

	policy = newArchivePolicy()
	if _, err := policy.validate(&tar.Header{Name: "bin", Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := policy.validate(&tar.Header{Name: "bin/run", Typeflag: tar.TypeReg}); err == nil || !strings.Contains(err.Error(), "file/directory conflict") {
		t.Fatalf("type conflict error = %v", err)
	}
}
