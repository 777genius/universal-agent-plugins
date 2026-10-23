package sourceacquisition

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func archiveRejectLocal(t *testing.T, payload []byte) {
	t.Helper()
	temporary := t.TempDir()
	_, err := (Acquirer{TempRoot: temporary}).AcquireArchive(context.Background(), sourcespec.Archive{URL: archiveLocalFile(t, payload)}, "", "")
	if err == nil || errors.Is(err, ErrNotArchive) {
		t.Fatalf("hostile recognized archive accepted or allowed Git fallback: %v", err)
	}
	archiveAssertEmpty(t, temporary)
}

func TestArchiveRejectsPathsAndCollisionsAcrossFormats(t *testing.T) {
	cases := map[string][]archiveFixtureEntry{
		"traversal":                   {{name: "../escape", body: "x"}},
		"absolute":                    {{name: "/escape", body: "x"}},
		"drive":                       {{name: "C:/escape", body: "x"}},
		"UNC":                         {{name: `\\host\escape`, body: "x"}},
		"backslash":                   {{name: `a\b`, body: "x"}},
		"control":                     {{name: "a\x7fb", body: "x"}},
		"decomposed Unicode":          {{name: "e\u0301", body: "x"}},
		"duplicate":                   {{name: "same", body: "x"}, {name: "same", body: "y"}},
		"case collision":              {{name: "NAME", body: "x"}, {name: "name", body: "y"}},
		"Unicode fold collision":      {{name: "Straße/a", body: "x"}, {name: "STRASSE/b", body: "y"}},
		"implicit ancestor collision": {{name: "Foo/a", body: "x"}, {name: "foo/b", body: "y"}},
		"file then directory":         {{name: "a", body: "x"}, {name: "a/b", body: "y"}},
		"directory then file":         {{name: "a/b", body: "x"}, {name: "a", body: "y"}},
		"deep":                        {{name: strings.Repeat("a/", 64) + "b", body: "x"}},
		"reserved Windows":            {{name: "CON.txt", body: "x"}},
		"Git metadata":                {{name: ".git/config", body: "x"}},
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			t.Run("ZIP", func(t *testing.T) { archiveRejectLocal(t, archiveFixtureZIP(t, entries, "")) })
			t.Run("TAR", func(t *testing.T) { archiveRejectLocal(t, archiveFixtureTAR(t, entries)) })
		})
	}
}

func TestArchiveRejectsLinksAndSpecialEntries(t *testing.T) {
	for _, kind := range []byte{tar.TypeSymlink, tar.TypeLink, tar.TypeFifo, tar.TypeChar, tar.TypeBlock, tar.TypeGNUSparse} {
		t.Run(fmt.Sprintf("TAR-%d", kind), func(t *testing.T) {
			archiveRejectLocal(t, archiveFixtureTAR(t, []archiveFixtureEntry{{name: "escape", typeflag: kind, link: "../outside"}}))
		})
	}
	for _, mode := range []os.FileMode{os.ModeSymlink, os.ModeNamedPipe, os.ModeDevice, os.ModeSocket} {
		t.Run(fmt.Sprintf("ZIP-%d", mode), func(t *testing.T) {
			archiveRejectLocal(t, archiveFixtureZIP(t, []archiveFixtureEntry{{name: "escape", body: "../outside", mode: mode | 0o777}}, ""))
		})
	}
}

func TestArchiveStrictTAREndingsAndPAX(t *testing.T) {
	valid := archiveFixtureTAR(t, archivePackageEntries(""))
	for name, payload := range map[string][]byte{
		"missing end":       valid[:len(valid)-1024],
		"one end block":     valid[:len(valid)-512],
		"truncated end":     valid[:len(valid)-1],
		"unaligned padding": append(bytes.Clone(valid), 0),
		"nonzero tail":      append(bytes.Clone(valid), bytes.Repeat([]byte{'x'}, 512)...),
		"second TAR":        append(bytes.Clone(valid), valid...),
	} {
		t.Run(name, func(t *testing.T) { archiveRejectLocal(t, payload) })
	}
	entries := append([]archiveFixtureEntry{{name: "./", typeflag: tar.TypeDir}}, archivePackageEntries("")...)
	entries[1].pax = map[string]string{"comment": "valid PAX metadata"}
	padded := append(archiveFixtureTAR(t, entries), make([]byte, 1024)...)
	snapshot, err := (Acquirer{TempRoot: t.TempDir()}).AcquireArchive(context.Background(), sourcespec.Archive{URL: archiveLocalFile(t, padded)}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	_ = packagedigest.Remove(snapshot)
	for name, pax := range map[string]map[string]string{
		"PAX traversal": {"path": "../escape"},
		"PAX sparse":    {"GNU.sparse.major": "1"},
	} {
		t.Run(name, func(t *testing.T) {
			archiveRejectLocal(t, archiveFixtureTAR(t, []archiveFixtureEntry{{name: "safe", body: "x", pax: pax}}))
		})
	}
}

func TestArchiveStrictGzipBoundaries(t *testing.T) {
	valid := archiveFixtureGzip(t, archiveFixtureTAR(t, archivePackageEntries("")))
	badCRC := bytes.Clone(valid)
	badCRC[len(badCRC)-8] ^= 1
	for name, payload := range map[string][]byte{
		"second member":     append(bytes.Clone(valid), valid...),
		"outer tail":        append(bytes.Clone(valid), 0),
		"bad CRC":           badCRC,
		"truncated trailer": valid[:len(valid)-1],
		"not TAR":           archiveFixtureGzip(t, []byte("ordinary text")),
	} {
		t.Run(name, func(t *testing.T) { archiveRejectLocal(t, payload) })
	}
}

func archiveMutateZIP(t *testing.T, source []byte, mutate func([]byte, []byte, []byte)) []byte {
	t.Helper()
	payload := bytes.Clone(source)
	central := bytes.Index(payload, []byte("PK\x01\x02"))
	end := bytes.LastIndex(payload, []byte("PK\x05\x06"))
	if central < 0 || end < 0 {
		t.Fatal("invalid fixture")
	}
	mutate(payload, payload[central:], payload[end:])
	return payload
}

func TestArchiveStrictZIPBoundariesAndMetadata(t *testing.T) {
	valid := archiveFixtureZIP(t, archivePackageEntries(""), "")
	cases := map[string][]byte{
		"prefix":        append([]byte("SFX"), valid...),
		"tail":          append(bytes.Clone(valid), 0),
		"second ZIP":    append(bytes.Clone(valid), valid...),
		"truncated end": valid[:len(valid)-1],
	}
	mutators := map[string]func([]byte, []byte, []byte){
		"multiple disks":      func(_, _, end []byte) { end[4] = 1 },
		"ZIP64":               func(_, _, end []byte) { binary.LittleEndian.PutUint16(end[10:12], 65535) },
		"encryption":          func(local, central, _ []byte) { local[6] |= 1; central[8] |= 1 },
		"unsupported method":  func(local, central, _ []byte) { local[8] = 99; central[10] = 99 },
		"declared oversized":  func(_, central, _ []byte) { binary.LittleEndian.PutUint32(central[24:28], 64<<20+1) },
		"actual size differs": func(_, central, _ []byte) { binary.LittleEndian.PutUint32(central[24:28], 1) },
		"corrupt payload":     func(local, _, _ []byte) { local[40] ^= 1 },
	}
	for name, mutate := range mutators {
		cases[name] = archiveMutateZIP(t, valid, mutate)
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) { archiveRejectLocal(t, payload) })
	}
}

func TestArchivePreflightLimits(t *testing.T) {
	largeTAR := archiveFixtureTAR(t, []archiveFixtureEntry{{name: "huge"}})
	copy(largeTAR[124:136], []byte("00400000001\x00")) // 512 MiB + 1: rejected before allocation/write.
	archiveRejectLocal(t, largeTAR)
	policy := newArchivePolicy()
	for index := range 4 {
		if _, err := policy.entry(fmt.Sprintf("file-%d", index), false, 64<<20); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := policy.entry("too-much", false, 1); err == nil {
		t.Fatal("expanded byte limit not enforced before writes")
	}
	policy = newArchivePolicy()
	for index := range 10000 {
		if _, err := policy.entry(fmt.Sprintf("file-%d", index), false, 0); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := policy.entry("too-many", false, 0); err == nil {
		t.Fatal("entry limit not enforced")
	}
}

func TestArchiveAutoClassificationAndRedirects(t *testing.T) {
	valid := archiveFixtureZIP(t, archivePackageEntries(""), "")
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/html", "/explicit.zip":
			_, _ = writer.Write([]byte("<html>not a package</html>"))
		case "/private":
			writer.WriteHeader(http.StatusUnauthorized)
		case "/corrupt":
			_, _ = writer.Write(valid[:len(valid)-1])
		case "/loop":
			http.Redirect(writer, request, "/loop", http.StatusFound)
		case "/downgrade":
			http.Redirect(writer, request, "http://127.0.0.1:1/no", http.StatusFound)
		case "/redirect":
			http.SetCookie(writer, &http.Cookie{Name: "secret", Value: "do-not-forward"})
			http.Redirect(writer, request, "/valid", http.StatusFound)
		case "/valid":
			if request.Header.Get("Cookie") != "" || request.Header.Get("Authorization") != "" {
				t.Error("redirect forwarded credentials")
			}
			_, _ = writer.Write(valid)
		}
	}))
	defer server.Close()
	client := server.Client()
	client.Jar, _ = cookiejar.New(nil)
	temporary := t.TempDir()
	acquirer := Acquirer{TempRoot: temporary, HTTPClient: client}
	for _, endpoint := range []string{"/html", "/private", "/corrupt", "/explicit.zip", "/loop", "/downgrade"} {
		_, err := acquirer.AcquireArchive(context.Background(), sourcespec.Archive{URL: server.URL + endpoint}, "", "")
		wantFallback := endpoint == "/html" || endpoint == "/private"
		if err == nil || errors.Is(err, ErrNotArchive) != wantFallback {
			t.Errorf("%s fallback=%v: %v", endpoint, errors.Is(err, ErrNotArchive), err)
		}
		archiveAssertEmpty(t, temporary)
	}
	snapshot, err := acquirer.AcquireArchive(context.Background(), sourcespec.Archive{URL: server.URL + "/redirect"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	_ = packagedigest.Remove(snapshot)
	archiveAssertEmpty(t, temporary)
}

func TestArchiveStoreZIPAndRootDirectoryMarker(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, entry := range append([]archiveFixtureEntry{{name: "./", mode: os.ModeDir | 0o755}}, archivePackageEntries("")...) {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Store}
		header.SetMode(entry.mode)
		body, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = body.Write([]byte(entry.body))
	}
	_ = writer.Close()
	snapshot, err := (Acquirer{TempRoot: t.TempDir()}).AcquireArchive(context.Background(), sourcespec.Archive{URL: archiveLocalFile(t, buffer.Bytes())}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	_ = packagedigest.Remove(snapshot)
}
