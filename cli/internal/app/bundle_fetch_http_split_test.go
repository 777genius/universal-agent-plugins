package app

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestBundleFetchRedirectPolicyRejectsHTTPRedirect(t *testing.T) {
	t.Parallel()

	req := &http.Request{URL: &url.URL{Scheme: "http"}}
	via := []*http.Request{{URL: &url.URL{Scheme: "https"}}}
	if err := bundleFetchRedirectPolicy(req, via); err != http.ErrUseLastResponse {
		t.Fatalf("error = %v", err)
	}
}

func TestNewBundleHTTPTransportRejectsInvalidAdditionalRoots(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/roots.pem"
	if err := os.WriteFile(path, []byte("not pem"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := newBundleHTTPTransport(bundleHTTPClientConfig{AdditionalRootsFile: path})
	if err == nil || !strings.Contains(err.Error(), "valid PEM certificates") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateBundleHTTPResponseRejectsOversizedContentLength(t *testing.T) {
	t.Parallel()

	resp := &http.Response{StatusCode: http.StatusOK, ContentLength: 33}
	err := validateBundleHTTPResponse(resp, 32)
	if err == nil || !strings.Contains(err.Error(), "content-length") {
		t.Fatalf("error = %v", err)
	}
}
