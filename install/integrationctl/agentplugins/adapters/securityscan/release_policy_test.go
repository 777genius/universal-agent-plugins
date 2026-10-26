package securityscan

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type releaseNetworkDenied struct{ calls int }

func (transport *releaseNetworkDenied) RoundTrip(*http.Request) (*http.Response, error) {
	transport.calls++
	return nil, errors.New("inert resolver contract prohibits network")
}

func seedReleaseCache(t *testing.T, root, platform, name, body string) string {
	t.Helper()
	path := filepath.Join(root, domain.SecurityScannerVersion, platform, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReleaseScannerLinuxAMD64RejectsGNUSubstitution(t *testing.T) {
	root := t.TempDir()
	gnu := seedReleaseCache(t, root, "linux-amd64", "lintai", "inert historical GNU")
	var lock sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		lock.Lock()
		requests = append(requests, request.URL.Path)
		lock.Unlock()
		_, _ = writer.Write([]byte("inert bytes that cannot match the pinned portable archive"))
	}))
	defer server.Close()
	scanner := ReleaseScanner{Root: root, HTTPClient: server.Client(), Origin: server.URL + "/", GOOS: "linux", GOARCH: "amd64"}
	resolved, err := scanner.resolve(context.Background())
	if err == nil || !strings.Contains(err.Error(), "checksum verification") || resolved != "" {
		t.Fatalf("GNU-only cache bypassed portable archive admission: resolved=%q err=%v", resolved, err)
	}
	lock.Lock()
	requested := append([]string(nil), requests...)
	lock.Unlock()
	if len(requested) != 1 || requested[0] != "/lintai-v0.1.3-x86_64-unknown-linux-musl.tar.gz" {
		t.Fatalf("portable acquisition differs: paths=%q", requested)
	}
	body, err := os.ReadFile(gnu)
	if err != nil || string(body) != "inert historical GNU" {
		t.Fatalf("historical GNU cache changed: body=%q err=%v", body, err)
	}
	portable := filepath.Join(root, domain.SecurityScannerVersion, "linux-amd64-musl", "lintai")
	if _, err := os.Lstat(portable); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unverified portable binary was installed: %v", err)
	}
}

func TestReleaseScannerPortableCacheDoesNotUseNetwork(t *testing.T) {
	root := t.TempDir()
	gnu := seedReleaseCache(t, root, "linux-amd64", "lintai", "inert historical GNU")
	portable := seedReleaseCache(t, root, "linux-amd64-musl", "lintai", "inert portable")
	transport := &releaseNetworkDenied{}
	scanner := ReleaseScanner{Root: root, HTTPClient: &http.Client{Transport: transport}, GOOS: "linux", GOARCH: "amd64"}
	resolved, err := scanner.resolve(context.Background())
	if err != nil || resolved != portable || transport.calls != 0 {
		t.Fatalf("portable cache route differs: resolved=%q calls=%d err=%v", resolved, transport.calls, err)
	}
	for path, expected := range map[string]string{gnu: "inert historical GNU", portable: "inert portable"} {
		body, err := os.ReadFile(path)
		if err != nil || string(body) != expected {
			t.Fatalf("cache bytes changed: path=%q body=%q err=%v", path, body, err)
		}
	}
}

func TestReleaseScannerOtherTargetCacheIdentity(t *testing.T) {
	for _, target := range []struct{ goos, goarch, binary string }{
		{"linux", "arm64", "lintai"},
		{"darwin", "amd64", "lintai"},
		{"darwin", "arm64", "lintai"},
		{"windows", "amd64", "lintai.exe"},
		{"windows", "arm64", "lintai.exe"},
	} {
		t.Run(target.goos+"/"+target.goarch, func(t *testing.T) {
			root := t.TempDir()
			expected := seedReleaseCache(t, root, target.goos+"-"+target.goarch, target.binary, "inert unchanged target")
			transport := &releaseNetworkDenied{}
			scanner := ReleaseScanner{Root: root, HTTPClient: &http.Client{Transport: transport}, GOOS: target.goos, GOARCH: target.goarch}
			resolved, err := scanner.resolve(context.Background())
			if err != nil || resolved != expected || transport.calls != 0 {
				t.Fatalf("existing target cache changed: resolved=%q calls=%d err=%v", resolved, transport.calls, err)
			}
		})
	}
}
