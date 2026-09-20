package app

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeBundleArchiveMetadataRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := decodeBundleArchiveMetadata([]byte("{"))
	if err == nil || !strings.Contains(err.Error(), "valid .plugin-kit-ai-export.json") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateBundleHeaderTypeRejectsSymlink(t *testing.T) {
	t.Parallel()

	err := validateBundleHeaderType(&tar.Header{Name: "link", Typeflag: tar.TypeSymlink})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("error = %v", err)
	}
}

func TestBundleArchivePolicyRejectsUnsafePathsDuplicatesAndLimits(t *testing.T) {
	t.Parallel()
	for name, header := range map[string]tar.Header{
		"parent traversal": {Name: "../escape", Typeflag: tar.TypeReg},
		"Windows drive":    {Name: "C:/escape", Typeflag: tar.TypeReg},
		"backslash":        {Name: `nested\\escape`, Typeflag: tar.TypeReg},
		"special file":     {Name: "pipe", Typeflag: tar.TypeFifo},
		"hardlink":         {Name: "linked", Typeflag: tar.TypeLink},
		"oversized file":   {Name: "large", Typeflag: tar.TypeReg, Size: maxBundleArchiveFileBytes + 1},
		"non-NFC Unicode":  {Name: "cafe\u0301", Typeflag: tar.TypeReg},
	} {
		name, header := name, header
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := newBundleArchivePolicy().validate(&header); err == nil {
				t.Fatalf("unsafe header was accepted: %+v", header)
			}
		})
	}
	policy := newBundleArchivePolicy()
	if _, err := policy.validate(&tar.Header{Name: "README.md", Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := policy.validate(&tar.Header{Name: "readme.md", Typeflag: tar.TypeReg}); err == nil || !strings.Contains(err.Error(), "case-colliding") {
		t.Fatalf("duplicate collision error = %v", err)
	}
	policy = newBundleArchivePolicy()
	policy.expandedBytes = maxBundleArchiveExpandedBytes
	if _, err := policy.validate(&tar.Header{Name: "one-more", Typeflag: tar.TypeReg, Size: 1}); err == nil || !strings.Contains(err.Error(), "expanded") {
		t.Fatalf("expanded limit error = %v", err)
	}
}

func TestExtractBundleArchiveStripsSpecialModeBits(t *testing.T) {
	t.Parallel()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	body := []byte("run")
	if err := writer.WriteHeader(&tar.Header{Name: "bin/run", Typeflag: tar.TypeReg, Mode: 0o6777, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	if err := extractBundleArchive(tar.NewReader(bytes.NewReader(buffer.Bytes())), destination); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(destination, "bin", "run"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("extracted mode = %o, want 755", info.Mode().Perm())
	}
}

func TestBundleArchivePolicyRejectsAncestorCollisionsAndTypeConflicts(t *testing.T) {
	t.Parallel()
	policy := newBundleArchivePolicy()
	if _, err := policy.validate(&tar.Header{Name: "Plugins/one", Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := policy.validate(&tar.Header{Name: "plugins/two", Typeflag: tar.TypeReg}); err == nil || !strings.Contains(err.Error(), "case-colliding") {
		t.Fatalf("ancestor collision error = %v", err)
	}

	policy = newBundleArchivePolicy()
	if _, err := policy.validate(&tar.Header{Name: "bin", Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := policy.validate(&tar.Header{Name: "bin/run", Typeflag: tar.TypeReg}); err == nil || !strings.Contains(err.Error(), "file/directory conflict") {
		t.Fatalf("type conflict error = %v", err)
	}
}
