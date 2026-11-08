package securityscan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func pinRetryTestRelease(t *testing.T, archive []byte) {
	t.Helper()
	key := "test/test"
	original, exists := releaseAssets[key]
	digest := sha256.Sum256(archive)
	releaseAssets[key] = releaseAsset{Name: "lintai.tar.gz", Digest: hex.EncodeToString(digest[:])}
	t.Cleanup(func() {
		if exists {
			releaseAssets[key] = original
		} else {
			delete(releaseAssets, key)
		}
	})
}

func TestReleaseScannerRetriesTransientGatewayFailuresAndCachesVerifiedBytes(t *testing.T) {
	archive := tarArchive(t, "release/lintai", []byte("pinned-scanner"))
	pinRetryTestRelease(t, archive)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) < 3 {
			w.WriteHeader(http.StatusGatewayTimeout)
			return
		}
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	scanner := ReleaseScanner{
		Root: t.TempDir(), HTTPClient: server.Client(), Origin: server.URL + "/",
		GOOS: "test", GOARCH: "test",
	}
	got, err := scanner.resolve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(got)
	if err != nil || string(body) != "pinned-scanner" {
		t.Fatalf("scanner bytes: got=%q err=%v", body, err)
	}
	if requests.Load() != 3 {
		t.Fatalf("expected two gateway refusals followed by one exact asset, requests=%d", requests.Load())
	}
	reused, err := scanner.resolve(context.Background())
	if err != nil || reused != got || requests.Load() != 3 {
		t.Fatalf("verified cache must be reused unchanged: path=%q err=%v requests=%d", reused, err, requests.Load())
	}
}

func TestReleaseScannerExhausts504WithoutInstallingOrBypassingScanner(t *testing.T) {
	archive := tarArchive(t, "lintai", []byte("pinned-scanner"))
	pinRetryTestRelease(t, archive)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer server.Close()

	root := t.TempDir()
	_, err := (ReleaseScanner{
		Root: root, HTTPClient: server.Client(), Origin: server.URL + "/",
		GOOS: "test", GOARCH: "test",
	}).resolve(context.Background())
	if err == nil || !strings.Contains(err.Error(), "HTTP 504") || requests.Load() != 3 {
		t.Fatalf("persistent upstream gateway failure must fail closed: err=%v requests=%d", err, requests.Load())
	}
	if _, statErr := os.Lstat(filepath.Join(root, "0.1.3", "test-test", "lintai")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("failed download installed scanner executable: %v", statErr)
	}
}

func TestReleaseScannerDoesNotRetry403OrBadChecksum(t *testing.T) {
	archive := tarArchive(t, "lintai", []byte("pinned-scanner"))
	pinRetryTestRelease(t, archive)
	for _, tc := range []struct {
		name string
		status int
		body []byte
		reason string
	}{
		{name: "forbidden", status: http.StatusForbidden, reason: "HTTP 403"},
		{name: "bad checksum", status: http.StatusOK, body: []byte("wrong bytes"), reason: "checksum"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				w.WriteHeader(tc.status)
				_, _ = w.Write(tc.body)
			}))
			defer server.Close()
			root := t.TempDir()
			_, err := (ReleaseScanner{
				Root: root, HTTPClient: server.Client(), Origin: server.URL + "/",
				GOOS: "test", GOARCH: "test",
			}).resolve(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.reason) || requests.Load() != 1 {
				t.Fatalf("untrusted or invalid asset must not be retried: err=%v requests=%d", err, requests.Load())
			}
		})
	}
}

func TestReleaseScannerCancellationStopsRetryImmediately(t *testing.T) {
	archive := tarArchive(t, "lintai", []byte("pinned-scanner"))
	pinRetryTestRelease(t, archive)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		cancel()
	}))
	defer server.Close()
	_, err := (ReleaseScanner{
		Root: t.TempDir(), HTTPClient: server.Client(), Origin: server.URL + "/",
		GOOS: "test", GOARCH: "test",
	}).resolve(ctx)
	if !errors.Is(err, context.Canceled) || requests.Load() != 1 {
		t.Fatalf("cancellation must stop retry without fallback: err=%v requests=%d", err, requests.Load())
	}
}
