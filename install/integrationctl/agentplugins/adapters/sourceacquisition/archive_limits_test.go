package sourceacquisition

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

type archiveZeroReader struct{}

func (archiveZeroReader) Read(body []byte) (int, error) {
	clear(body)
	return len(body), nil
}

type archiveRoundTripper func(*http.Request) (*http.Response, error)

func (roundTrip archiveRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func TestArchiveStreamingDownloadLimit(t *testing.T) {
	// This component fixture deliberately omits Content-Length. The retained
	// stream must be bounded by actual bytes, independently of server metadata.
	client := &http.Client{Transport: archiveRoundTripper(func(*http.Request) (*http.Response, error) {
		body := io.MultiReader(bytes.NewReader([]byte("PK\x03\x04")), io.LimitReader(archiveZeroReader{}, archiveMaxBytes))
		return &http.Response{StatusCode: http.StatusOK, ContentLength: -1, Header: make(http.Header), Body: io.NopCloser(body)}, nil
	})}
	temporary := t.TempDir()
	_, err := (Acquirer{TempRoot: temporary, HTTPClient: client}).AcquireArchive(context.Background(), sourcespec.Archive{URL: "https://fixture.invalid/download"}, "", "")
	if err == nil || errors.Is(err, ErrNotArchive) || !strings.Contains(err.Error(), "256 MiB") {
		t.Fatalf("actual download bytes escaped the bound: %v", err)
	}
	archiveAssertEmpty(t, temporary)
}

func TestArchiveStreamingGzipExpansionLimit(t *testing.T) {
	var compressed bytes.Buffer
	writer, err := gzip.NewWriterLevel(&compressed, gzip.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.CopyN(writer, archiveZeroReader{}, archiveMaxBytes+1); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	temporary := t.TempDir()
	_, err = (Acquirer{TempRoot: temporary}).AcquireArchive(context.Background(), sourcespec.Archive{URL: archiveLocalFile(t, compressed.Bytes())}, "", "")
	if err == nil || errors.Is(err, ErrNotArchive) || !strings.Contains(err.Error(), "expanded stream limit") {
		t.Fatalf("actual gzip expansion escaped the bound: %v", err)
	}
	archiveAssertEmpty(t, temporary)
}

func TestArchiveCandidateBoundAllowsExplicitSelection(t *testing.T) {
	entries := make([]archiveFixtureEntry, 0, 17*3)
	for index := range 17 {
		entries = append(entries, archivePackageEntries(fmt.Sprintf("wrapper/plugins/p%02d/", index))...)
	}
	temporary := t.TempDir()
	acquirer := Acquirer{TempRoot: temporary}
	source := sourcespec.Archive{URL: archiveLocalFile(t, archiveFixtureZIP(t, entries, ""))}
	if _, err := acquirer.AcquireArchive(context.Background(), source, "", ""); err == nil || !strings.Contains(err.Error(), "16 package candidates") {
		t.Fatalf("unbounded package discovery: %v", err)
	}
	source.Path = "plugins/p16"
	snapshot, err := acquirer.AcquireArchive(context.Background(), source, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Source.PackageSubpath != source.Path {
		t.Fatalf("explicit logical path = %q", snapshot.Source.PackageSubpath)
	}
	_ = packagedigest.Remove(snapshot)
	archiveAssertEmpty(t, temporary)
}

func TestArchiveInvalidRootManifestDirectoryNeverSelectsNested(t *testing.T) {
	entries := append([]archiveFixtureEntry{{name: "plugin.json", typeflag: tar.TypeDir}}, archivePackageEntries("nested/")...)
	snapshot, err := (Acquirer{TempRoot: t.TempDir()}).AcquireArchive(context.Background(), sourcespec.Archive{URL: archiveLocalFile(t, archiveFixtureTAR(t, entries))}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = packagedigest.Remove(snapshot) }()
	if snapshot.Source.PackageSubpath != "" {
		t.Fatal("invalid root manifest object fell back to a nested package")
	}
	registry, _ := specregistry.New()
	if _, err := (loader.Loader{Registry: registry}).LoadSnapshot(context.Background(), snapshot); err == nil {
		t.Fatal("invalid root manifest directory bypassed standard validation")
	}
}
