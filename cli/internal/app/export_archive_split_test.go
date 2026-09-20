package app

import (
	"archive/tar"
	"bytes"
	"strings"
	"testing"
)

func TestWriteArchiveEntryRejectsParentTraversal(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	defer tw.Close()

	err := writeArchiveEntry(tw, "../escape", []byte("demo"), 0o644)
	if err == nil || !strings.Contains(err.Error(), "invalid archive path") {
		t.Fatalf("error = %v", err)
	}
}

func TestWriteExportArchiveMetadataUsesCanonicalEntryName(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := writeExportArchiveMetadata(tw, exportMetadata{PluginName: "demo"}); err != nil {
		t.Fatalf("writeExportArchiveMetadata: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}

	tr := tar.NewReader(bytes.NewReader(buf.Bytes()))
	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("read header: %v", err)
	}
	if hdr.Name != ".plugin-kit-ai-export.json" {
		t.Fatalf("header name = %q", hdr.Name)
	}
}
