package sourceacquisition

import (
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func TestNativeGitRejectsConfiguredFileRewrite(t *testing.T) {
	root, _, commit := nativeFixtureRepository(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	// Scoped protocol config is stronger than protocol.allow. GIT_ALLOW_PROTOCOL
	// must independently forbid this rewrite of a syntactically HTTPS endpoint.
	config := fmt.Sprintf("[protocol \"file\"]\n allow = always\n[url \"file://%s/\"]\n insteadOf = https://fixture.invalid/\n", filepath.ToSlash(root))
	sshFixtureWrite(t, filepath.Join(home, ".gitconfig"), config)
	source := sourcespec.Git{URL: "https://fixture.invalid/repo.git"}
	if _, err := (Acquirer{TempRoot: t.TempDir()}).AcquireGit(t.Context(), source, commit, ""); err == nil {
		t.Fatal("HTTPS endpoint was rewritten to forbidden local file transport")
	}
}

func TestNativeGitRequiresTLSDespiteScopedUserConfig(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { requests.Add(1); w.WriteHeader(http.StatusNotFound) }))
	defer server.Close()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("GIT_SSL_CAINFO", "")
	config := "[http \"" + server.URL + "\"]\n sslVerify = false\n"
	sshFixtureWrite(t, filepath.Join(home, ".gitconfig"), config)
	_, err := (Acquirer{TempRoot: t.TempDir()}).ResolveGit(t.Context(), sourcespec.Git{URL: server.URL + "/repo.git"}, "default", false)
	if err == nil || requests.Load() != 0 {
		t.Fatalf("untrusted TLS source reached HTTP despite authoritative verification: requests=%d err=%v", requests.Load(), err)
	}
}

func TestNativeGitRequiresTLSAfterConfiguredHTTPSRewrite(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { requests.Add(1); w.WriteHeader(http.StatusNotFound) }))
	defer server.Close()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("GIT_SSL_CAINFO", "")
	config := "[http \"" + server.URL + "\"]\n sslVerify = false\n[url \"" + server.URL + "/\"]\n insteadOf = https://fixture.invalid/\n"
	sshFixtureWrite(t, filepath.Join(home, ".gitconfig"), config)
	_, err := (Acquirer{TempRoot: t.TempDir()}).ResolveGit(t.Context(), sourcespec.Git{URL: "https://fixture.invalid/repo.git"}, "default", false)
	if err == nil || requests.Load() != 0 {
		t.Fatalf("rewritten endpoint bypassed TLS policy: requests=%d err=%v", requests.Load(), err)
	}
}

func TestNativeGitRejectsHTTPSDowngradeRedirect(t *testing.T) {
	var downgraded atomic.Int32
	insecure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { downgraded.Add(1); w.WriteHeader(http.StatusNotFound) }))
	defer insecure.Close()
	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, insecure.URL+"/repo.git", http.StatusFound)
	}))
	defer secure.Close()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	ca := filepath.Join(home, "ca.pem")
	// Reuse the TLS fixture's actual trust anchor without disabling verification.
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: secure.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_SSL_CAINFO", ca)
	_, err := (Acquirer{TempRoot: t.TempDir()}).ResolveGit(t.Context(), sourcespec.Git{URL: secure.URL + "/repo.git"}, "default", false)
	if err == nil || downgraded.Load() != 0 {
		t.Fatalf("Git followed an HTTPS downgrade: requests=%d err=%v", downgraded.Load(), err)
	}
}
