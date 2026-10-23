package sourceacquisition

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

type archiveFixtureEntry struct {
	name, body, link string
	mode             os.FileMode
	typeflag         byte
	pax              map[string]string
}

func archivePackageEntries(prefix string) []archiveFixtureEntry {
	return []archiveFixtureEntry{
		{name: prefix + "plugin.json", body: `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"archive-demo","version":"1.0.0"}`},
		{name: prefix + "skills/demo/SKILL.md", body: "---\nname: demo\ndescription: Archive demo\n---\nKeep package content inert.\n"},
		{name: prefix + "bin/run.sh", body: "#!/bin/sh\ntouch SHOULD_NEVER_EXIST\n", mode: 0o4755},
	}
}

func archiveFixtureZIP(t *testing.T, entries []archiveFixtureEntry, comment string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	if err := writer.SetComment(comment); err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		mode := entry.mode
		if mode == 0 {
			mode = 0o644
		}
		header.SetMode(mode)
		body, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(body, entry.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func archiveFixtureTAR(t *testing.T, entries []archiveFixtureEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	for _, entry := range entries {
		typeflag := entry.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		mode := int64(entry.mode.Perm())
		if mode == 0 {
			mode = 0o644
		}
		header := &tar.Header{Name: entry.name, Mode: mode, Typeflag: typeflag, Size: int64(len(entry.body)), Linkname: entry.link, PAXRecords: entry.pax}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(writer, entry.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func archiveFixtureGzip(t *testing.T, body []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func archiveBytesSHA(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func archiveLocalFile(t *testing.T, body []byte) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "package.bin")
	if err := os.WriteFile(filename, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return filename
}

func archiveAssertEmpty(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary archive/snapshot leaked: %v, %v", entries, err)
	}
}

// This is actual HTTPS transport plus real archive parsing, standard loading
// and sealing. It is separate from native agent/client activation evidence.
func TestArchiveHTTPSFormatsReachStandardLoader(t *testing.T) {
	entries := archivePackageEntries("release-old/")
	tarBytes := archiveFixtureTAR(t, entries)
	fixtures := map[string][]byte{
		"/package.zip":    archiveFixtureZIP(t, entries, "declared ZIP comment"),
		"/package.tar":    tarBytes,
		"/package.tar.gz": archiveFixtureGzip(t, tarBytes),
		"/package.tgz":    archiveFixtureGzip(t, tarBytes),
		"/download":       archiveFixtureZIP(t, entries, ""),
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "" || request.Header.Get("Cookie") != "" || request.Header.Get("Accept-Encoding") != "identity" {
			t.Error("archive request forwarded credentials or enabled content transformation")
		}
		writer.Header().Set("Content-Type", "text/plain") // Magic, not MIME, controls classification.
		_, _ = writer.Write(fixtures[request.URL.Path])
	}))
	defer server.Close()
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	var treeDigest string
	for endpoint, payload := range fixtures {
		t.Run(endpoint, func(t *testing.T) {
			temporary := t.TempDir()
			acquirer := Acquirer{TempRoot: temporary, HTTPClient: server.Client()}
			snapshot, err := acquirer.AcquireArchive(context.Background(), sourcespec.Archive{URL: server.URL + endpoint}, "", "")
			if err != nil {
				t.Fatal(err)
			}
			envelope, err := (loader.Loader{Registry: registry}).LoadSnapshot(context.Background(), snapshot)
			if err != nil || envelope.Manifest.Name != "archive-demo" || len(envelope.Skills) != 1 {
				t.Fatalf("standard load = %+v, %v", envelope, err)
			}
			if snapshot.Source.ArchiveSHA256 != archiveBytesSHA(payload) || snapshot.Source.ResolvedRevision != "sha256:"+archiveBytesSHA(payload) || snapshot.Source.SourceURL != server.URL+endpoint || snapshot.Source.PackageSubpath != "" {
				t.Fatalf("immutable provenance = %+v", snapshot.Source)
			}
			if treeDigest != "" && snapshot.TreeDigest != treeDigest {
				t.Fatal("archive format changed the same tree's identity")
			}
			treeDigest = snapshot.TreeDigest
			if !reflect.DeepEqual(snapshot.ExecutableFiles, []string{"bin/run.sh"}) {
				t.Fatalf("executable intent = %v", snapshot.ExecutableFiles)
			}
			if _, err := os.Stat(filepath.Join(snapshot.Root, "SHOULD_NEVER_EXIST")); !os.IsNotExist(err) {
				t.Fatal("package content executed")
			}
			if err := packagedigest.Remove(snapshot); err != nil {
				t.Fatal(err)
			}
			archiveAssertEmpty(t, temporary)
		})
	}
}

func TestArchiveLogicalSelectionAndBytePins(t *testing.T) {
	var payload []byte
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		_, _ = writer.Write(payload)
	}))
	defer server.Close()
	temporary := t.TempDir()
	acquirer := Acquirer{TempRoot: temporary, HTTPClient: server.Client()}
	source := sourcespec.Archive{URL: server.URL + "/download", Path: "plugins/demo"}
	payload = archiveFixtureZIP(t, archivePackageEntries("repo-old/plugins/demo/"), "old")
	first, err := acquirer.AcquireArchive(context.Background(), source, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if first.Source.PackageSubpath != "plugins/demo" || first.Source.CanonicalSource != sourcespec.Key(sourcespec.KindArchive, source.URL, "plugins/demo") {
		t.Fatalf("physical wrapper leaked into identity: %+v", first.Source)
	}
	if err := packagedigest.Remove(first); err != nil {
		t.Fatal(err)
	}
	payload = archiveFixtureZIP(t, archivePackageEntries("repo-new/plugins/demo/"), "repacked")
	if _, err := acquirer.AcquireArchive(context.Background(), source, first.Source.ArchiveSHA256, first.TreeDigest); err == nil || errors.Is(err, ErrNotArchive) {
		t.Fatalf("locked byte change accepted or classified as Git: %v", err)
	}
	updated, err := acquirer.AcquireArchive(context.Background(), source, "", first.TreeDigest)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Source.CanonicalSource != first.Source.CanonicalSource || updated.Source.ArchiveSHA256 == first.Source.ArchiveSHA256 || updated.TreeDigest != first.TreeDigest {
		t.Fatalf("wrapper/repack update changed stable ownership: %+v", updated.Source)
	}
	_ = packagedigest.Remove(updated)
	payload = archiveFixtureZIP(t, append(archivePackageEntries("repo/plugins/a/"), archivePackageEntries("repo/plugins/b/")...), "")
	before := requests.Load()
	_, err = acquirer.AcquireArchive(context.Background(), sourcespec.Archive{URL: source.URL}, "", "")
	if err == nil || !strings.Contains(err.Error(), "plugins/a, plugins/b") || errors.Is(err, ErrNotArchive) || requests.Load()-before != 1 {
		t.Fatalf("multi-package selection must use one stream: requests=%d, %v", requests.Load()-before, err)
	}
	archiveAssertEmpty(t, temporary)
}

func TestArchiveRootManifestNeverFallsBackToNestedPackage(t *testing.T) {
	entries := append([]archiveFixtureEntry{{name: "plugin.json", body: "malformed"}}, archivePackageEntries("nested/")...)
	snapshot, err := (Acquirer{TempRoot: t.TempDir()}).AcquireArchive(context.Background(), sourcespec.Archive{URL: archiveLocalFile(t, archiveFixtureTAR(t, entries))}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = packagedigest.Remove(snapshot) }()
	if snapshot.Source.PackageSubpath != "" {
		t.Fatal("malformed root silently selected nested package")
	}
	registry, _ := specregistry.New()
	if _, err := (loader.Loader{Registry: registry}).LoadSnapshot(context.Background(), snapshot); err == nil {
		t.Fatal("malformed root was not rejected by standard loader")
	}
}

func TestArchiveFailuresCleanOwnedFiles(t *testing.T) {
	payload := archiveFixtureZIP(t, archivePackageEntries(""), "")
	for _, test := range []struct {
		name             string
		source           sourcespec.Archive
		bytePin, treePin string
	}{
		{name: "wrong bytes", bytePin: strings.Repeat("0", 64)},
		{name: "wrong tree", treePin: "sha256:" + strings.Repeat("0", 64)},
		{name: "invalid path", source: sourcespec.Archive{Path: "../escape"}},
		{name: "conflicting byte pins", source: sourcespec.Archive{SHA256: strings.Repeat("1", 64)}, bytePin: strings.Repeat("2", 64)},
	} {
		t.Run(test.name, func(t *testing.T) {
			temporary := t.TempDir()
			test.source.URL = archiveLocalFile(t, payload)
			if _, err := (Acquirer{TempRoot: temporary}).AcquireArchive(context.Background(), test.source, test.bytePin, test.treePin); err == nil {
				t.Fatal("bad archive request succeeded")
			}
			archiveAssertEmpty(t, temporary)
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	temporary := t.TempDir()
	_, err := (Acquirer{TempRoot: temporary}).AcquireArchive(ctx, sourcespec.Archive{URL: archiveLocalFile(t, payload)}, "", "")
	if err == nil {
		t.Fatal("canceled acquisition succeeded")
	}
	archiveAssertEmpty(t, temporary)
}
