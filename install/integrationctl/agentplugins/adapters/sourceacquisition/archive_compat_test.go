package sourceacquisition

import (
	"archive/tar"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func TestArchiveConventionalDotMembers(t *testing.T) {
	entries := append([]archiveFixtureEntry{{name: "./", typeflag: tar.TypeDir}}, archivePackageEntries("./")...)
	for name, payload := range map[string][]byte{
		"TAR": archiveFixtureTAR(t, entries),
		"TGZ": archiveFixtureGzip(t, archiveFixtureTAR(t, entries)),
		"ZIP": archiveFixtureZIP(t, archivePackageEntries("./"), ""),
	} {
		t.Run(name, func(t *testing.T) {
			snapshot, err := (Acquirer{TempRoot: t.TempDir()}).AcquireArchive(context.Background(), sourcespec.Archive{URL: archiveLocalFile(t, payload)}, "", "")
			if err != nil {
				t.Fatal(err)
			}
			if snapshot.Source.PackageSubpath != "" || len(snapshot.ExecutableFiles) != 1 || snapshot.ExecutableFiles[0] != "bin/run.sh" {
				t.Fatalf("effective root/member inventory = %+v", snapshot)
			}
			_ = packagedigest.Remove(snapshot)
		})
	}
	for _, unsafe := range []string{"././x", "./../x", ".//x", "./C:/x", "./a//b"} {
		t.Run(unsafe, func(t *testing.T) {
			archiveRejectLocal(t, archiveFixtureTAR(t, []archiveFixtureEntry{{name: unsafe, body: "x"}}))
		})
	}
	// TAR's writer refuses a slash-terminated regular filename, so build that
	// hostile header directly and retain its valid checksum.
	rootFile := archiveFixtureTAR(t, []archiveFixtureEntry{{name: "xx", body: "x"}})
	clear(rootFile[:100])
	copy(rootFile[:100], "./")
	for index := 148; index < 156; index++ {
		rootFile[index] = ' '
	}
	checksum := 0
	for _, value := range rootFile[:512] {
		checksum += int(value)
	}
	copy(rootFile[148:156], fmt.Sprintf("%06o\x00 ", checksum))
	archiveRejectLocal(t, rootFile)
	for _, entries := range [][]archiveFixtureEntry{
		{{name: "./x", body: "a"}, {name: "x", body: "b"}},
		{{name: "./X/a", body: "a"}, {name: "x/b", body: "b"}},
		{{name: "./x/a", body: "a"}, {name: "x", body: "b"}},
	} {
		archiveRejectLocal(t, archiveFixtureTAR(t, entries))
		archiveRejectLocal(t, archiveFixtureZIP(t, entries, ""))
	}
	archiveRejectLocal(t, archiveFixtureTAR(t, []archiveFixtureEntry{{name: "./link", typeflag: tar.TypeSymlink, link: "x"}}))
}

func TestArchiveSignedRedirectIsTransientAndSanitized(t *testing.T) {
	const signedQuery = "sig=fixture-token%2Fnever-persist%2B&part=a%2Fb"
	payload := archiveFixtureZIP(t, archivePackageEntries(""), "")
	var targetCalls atomic.Int32
	cdn := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		targetCalls.Add(1)
		if request.URL.RawQuery != signedQuery {
			t.Errorf("signed query encoding changed: %q", request.URL.RawQuery)
		}
		for _, name := range []string{"Referer", "Authorization", "Cookie", "X-Secret"} {
			if request.Header.Get(name) != "" {
				t.Errorf("redirect retained %s", name)
			}
		}
		if request.URL.Path == "/denied" {
			writer.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(writer, signedQuery)
			return
		}
		_, _ = writer.Write(payload)
	}))
	defer cdn.Close()
	location := cdn.URL + "/asset?" + signedQuery
	stable := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, location, http.StatusFound)
	}))
	defer stable.Close()
	temporary := t.TempDir()
	acquirer := Acquirer{TempRoot: temporary, HTTPClient: stable.Client()}
	source := sourcespec.Archive{URL: stable.URL + "/package.zip"}
	snapshot, err := acquirer.AcquireArchive(context.Background(), source, "", "")
	if err != nil {
		t.Fatal(err)
	}
	metadata, _ := json.Marshal(snapshot)
	if strings.Contains(string(metadata), "fixture-token") || strings.Contains(string(metadata), cdn.URL) || snapshot.Source.SourceURL != source.URL || snapshot.Source.RequestedSource != source.URL {
		t.Fatalf("transient redirect leaked into metadata: %s", metadata)
	}
	_ = packagedigest.Remove(snapshot)
	for name, target := range map[string]string{
		"denied":    cdn.URL + "/denied?" + signedQuery,
		"downgrade": "http://127.0.0.1:1/asset?" + signedQuery,
		"userinfo":  strings.Replace(cdn.URL, "https://", "https://secret@", 1) + "/asset",
		"fragment":  cdn.URL + "/asset#secret",
		"length":    cdn.URL + "/asset?sig=" + strings.Repeat("x", 16<<10),
	} {
		t.Run(name, func(t *testing.T) {
			location = target
			_, err := acquirer.AcquireArchive(context.Background(), source, "", "")
			if err == nil || strings.Contains(err.Error(), "fixture-token") || strings.Contains(err.Error(), target) {
				t.Fatalf("unsafe redirect or unsanitized error: %v", err)
			}
			archiveAssertEmpty(t, temporary)
		})
	}
	if targetCalls.Load() != 2 {
		t.Fatalf("rejected redirect reached CDN: calls=%d", targetCalls.Load())
	}
	if _, err := acquirer.AcquireArchive(context.Background(), sourcespec.Archive{URL: source.URL + "?" + signedQuery}, "", ""); err == nil {
		t.Fatal("initial signed URL admitted to persistent intent")
	}
}

func TestArchiveCancellationAndDeclaredDownloadLimit(t *testing.T) {
	payload := archiveFixtureZIP(t, archivePackageEntries(""), "")
	started := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/large" {
			writer.Header().Set("Content-Length", fmt.Sprint(archiveMaxBytes+1))
			_, _ = writer.Write(payload)
			return
		}
		_, _ = writer.Write(payload[:512])
		writer.(http.Flusher).Flush()
		close(started)
		<-request.Context().Done()
	}))
	defer server.Close()
	temporary := t.TempDir()
	acquirer := Acquirer{TempRoot: temporary, HTTPClient: server.Client()}
	_, err := acquirer.AcquireArchive(context.Background(), sourcespec.Archive{URL: server.URL + "/large"}, "", "")
	if err == nil || errors.Is(err, ErrNotArchive) || !strings.Contains(err.Error(), "256 MiB") {
		t.Fatalf("declared download limit not enforced: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { <-started; cancel() }()
	_, err = acquirer.AcquireArchive(ctx, sourcespec.Archive{URL: server.URL + "/cancel"}, "", "")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("active download cancellation = %v", err)
	}
	archiveAssertEmpty(t, temporary)
}

func TestArchiveNeverAcceptsTLSBypass(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write(archiveFixtureZIP(t, archivePackageEntries(""), ""))
	}))
	defer server.Close()
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	_, err := (Acquirer{TempRoot: t.TempDir(), HTTPClient: client}).AcquireArchive(context.Background(), sourcespec.Archive{URL: server.URL + "/package.zip"}, "", "")
	if err == nil {
		t.Fatal("host-supplied TLS bypass was inherited")
	}
}
