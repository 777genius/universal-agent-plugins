package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type failingRoundTripper struct{}

func (failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("publication unavailable")
}

func TestMainUsesProductionDirectoryTrustAndReleaseBootstrap(t *testing.T) {
	t.Setenv("AGENTPLUGINS_DIRECTORY_KEY_ID", "test-current")
	t.Setenv("AGENTPLUGINS_DIRECTORY_PUBLIC_KEY", "11qYAYKxCrfVS/7TyWQHOg7hcvPapiMlrwIaaPcHURo=")
	client, err := newDirectoryClient(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(client.Trust.Keys) != 1 || client.Trust.Keys[0].ID != "uap-directory-2026-01" || client.Trust.Keys[0].State != directoryv1.KeyCurrent {
		t.Fatalf("production Directory trust = %+v", client.Trust.Keys)
	}
	if encoded := base64.StdEncoding.EncodeToString(client.Trust.Keys[0].PublicKey); encoded != "HalXARjat+v3ylTPLMAnvuavRo4ZfrF+DbWwsjlp2bI=" {
		t.Fatalf("unexpected production Directory public key: %x", client.Trust.Keys[0].PublicKey)
	}
	if !client.RequireEmbeddedBootstrap {
		t.Fatal("production Directory client did not require a release-bound bootstrap")
	}
	embedded, ready, err := directoryv1.DecodeReleaseBootstrap(generatedProductionDirectoryBootstrap, client.Trust)
	if err != nil || !ready {
		t.Fatalf("generated production bootstrap readiness = %v, %v", ready, err)
	}
	bundle, err := embedded.Verify(client.Trust)
	if err != nil || bundle.Snapshot.Sequence != 2 || bundle.Digest != "sha256:fe6422853423f447d797a54c5c2af0b0eda6f89c23815f8945f5b6f48d50a460" {
		t.Fatalf("generated production bootstrap identity: sequence=%d digest=%q err=%v", bundle.Snapshot.Sequence, bundle.Digest, err)
	}
	client.HTTPClient.Transport = failingRoundTripper{}
	client.Now = func() time.Time { return time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC) }
	fallback, err := client.Load(context.Background(), 0)
	if err != nil {
		t.Fatalf("offline release bootstrap fallback: %v", err)
	}
	if fallback.Snapshot.Sequence != 2 || fallback.Digest != bundle.Digest {
		t.Fatalf("offline release bootstrap identity: sequence=%d digest=%q", fallback.Snapshot.Sequence, fallback.Digest)
	}
}

func TestHardenedHTTPClientRedirectPolicy(t *testing.T) {
	check := hardenedHTTPClient("Directory").CheckRedirect
	original, _ := http.NewRequest(http.MethodGet, "https://directory.example/registry/latest.json", nil)
	first, _ := http.NewRequest(http.MethodGet, "https://directory.example/registry/one.json", nil)
	first.Header.Set("Authorization", "secret")
	first.Header.Set("Cookie", "secret=yes")
	first.Header.Set("Proxy-Authorization", "secret")
	if err := check(first, []*http.Request{original}); err != nil {
		t.Fatalf("first same-origin redirect: %v", err)
	}
	if first.Header.Get("Authorization") != "" || first.Header.Get("Cookie") != "" || first.Header.Get("Proxy-Authorization") != "" {
		t.Fatal("credentials were forwarded on redirect")
	}
	second, _ := http.NewRequest(http.MethodGet, "https://directory.example/registry/two.json", nil)
	if err := check(second, []*http.Request{original, first}); err != nil {
		t.Fatalf("second same-origin redirect: %v", err)
	}
	third, _ := http.NewRequest(http.MethodGet, "https://directory.example/registry/three.json", nil)
	if err := check(third, []*http.Request{original, first, second}); err == nil {
		t.Fatal("third redirect was accepted")
	}
	crossOrigin, _ := http.NewRequest(http.MethodGet, "https://other.example/registry/latest.json", nil)
	if err := check(crossOrigin, []*http.Request{original}); err == nil {
		t.Fatal("cross-origin redirect was accepted")
	}
	downgrade, _ := http.NewRequest(http.MethodGet, "http://directory.example/registry/latest.json", nil)
	if err := check(downgrade, []*http.Request{original}); err == nil {
		t.Fatal("HTTPS downgrade redirect was accepted")
	}
	discoveryCheck := hardenedHTTPClient("Discovery").CheckRedirect
	if err := discoveryCheck(crossOrigin, []*http.Request{original}); err == nil || !strings.Contains(err.Error(), "Discovery redirect") {
		t.Fatalf("Discovery redirect diagnostic = %v", err)
	}
}

func TestInvalidScopeAndTargetDoNotCreateDataRoot(t *testing.T) {
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()
	t.Setenv("HOME", t.TempDir())
	for name, args := range map[string][]string{
		"project-scope":  {"agentplugins", "add", "demo", "--scope", "project"},
		"invalid-target": {"agentplugins", "add", "demo", "--target", "not-a-client"},
	} {
		t.Run(name, func(t *testing.T) {
			dataRoot := filepath.Join(t.TempDir(), "must-not-exist")
			t.Setenv("AGENTPLUGINS_HOME", dataRoot)
			os.Args = args
			err := run()
			if err == nil || (name == "project-scope" && !strings.Contains(err.Error(), "user scope")) {
				t.Fatalf("invalid invocation error = %v", err)
			}
			if _, statErr := os.Lstat(dataRoot); !os.IsNotExist(statErr) {
				t.Fatalf("invalid invocation mutated data root: %v", statErr)
			}
		})
	}
}

// Serial transport fixture: intended requests are recorded and denied in memory.
// No socket, trusted assessment, scanner executable or cache entry is supplied.
type offlineSecurityTransport struct{ urls []string }

func (r *offlineSecurityTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.urls = append(r.urls, req.URL.String())
	return nil, errors.New("offline-security-transport-sentinel")
}
func securityFixtureTree(t *testing.T, root string) map[string]string {
	t.Helper()
	entries := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		st, err := d.Info()
		if err != nil {
			return err
		}
		if !st.IsDir() && !st.Mode().IsRegular() {
			return fmt.Errorf("unexpected fixture entry %s", p)
		}
		var b []byte
		if !st.IsDir() {
			b, err = os.ReadFile(p)
			if err != nil {
				return err
			}
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		entries[rel] = fmt.Sprintf("%s/%x", st.Mode(), sha256.Sum256(b))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return entries
}
func TestOfflinePublicInstallerSecurityBoundary(t *testing.T) {
	home, source, state := t.TempDir(), t.TempDir(), t.TempDir()
	client := filepath.Join(home, ".codex")
	if err := os.Mkdir(client, 0700); err != nil {
		t.Fatal(err)
	}
	for p, body := range map[string]string{filepath.Join(client, "config.toml"): "# synthetic client\n",
		filepath.Join(source, "plugin.json"): `{"$schema":"` + domain.PluginSchemaV1 + `","name":"demo"}`} {
		if err := os.WriteFile(p, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", client)
	t.Setenv("AGENTPLUGINS_HOME", state)
	t.Setenv("PATH", home)
	t.Setenv("AGENTPLUGINS_SECURITY_ORIGIN", "")
	beforeSource, beforeHome, beforeState := securityFixtureTree(t, source), securityFixtureTree(t, home), securityFixtureTree(t, state)
	transport := &offlineSecurityTransport{}
	oldTransport, oldArgs, oldOut, oldErr := http.DefaultTransport, os.Args, os.Stdout, os.Stderr
	defer func() { http.DefaultTransport, os.Args, os.Stdout, os.Stderr = oldTransport, oldArgs, oldOut, oldErr }()
	http.DefaultTransport = transport
	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()
	os.Stdout, os.Stderr = stdout, stderr
	argv := []string{"add", source, "--target=codex", "--scope=project", "--dry-run", "--format=json"}
	os.Args = append([]string{"agentplugins"}, argv...)
	var out, errout bytes.Buffer
	err = executeRelease(context.Background(), argv, authoringcli.Streams{Out: &out, Err: &errout}, run)
	if err == nil || out.Len() != 0 || errout.String() != "agentplugins: --scope project is not supported by the current client adapters; the public CLI supports user scope only\n" || len(transport.urls) != 0 {
		t.Fatalf("preflight boundary: %v %q %q requests=%v", err, out.String(), errout.String(), transport.urls)
	}
	if !reflect.DeepEqual(beforeState, securityFixtureTree(t, state)) || !reflect.DeepEqual(beforeSource, securityFixtureTree(t, source)) || !reflect.DeepEqual(beforeHome, securityFixtureTree(t, home)) {
		t.Fatal("preflight acquired or wrote state")
	}
	os.Args = []string{"agentplugins", "add", source, "--target=codex", "--dry-run", "--format=json"}
	err = run()
	if err == nil || !strings.Contains(err.Error(), "security assessment failed before installation") || !strings.Contains(err.Error(), "offline-security-transport-sentinel") {
		t.Fatalf("security bypassed: %v", err)
	}
	if len(transport.urls) != 4 {
		t.Fatalf("expected three index attempts then scanner acquisition: %v", transport.urls)
	}
	for _, url := range transport.urls[:3] {
		if url != defaultSecurityOrigin+"latest.json" {
			t.Fatalf("unexpected index URL %s", url)
		}
	}
	if !strings.HasPrefix(transport.urls[3], "https://github.com/777genius/lintai/releases/download/v0.1.3/lintai-v0.1.3-") {
		t.Fatal(transport.urls)
	}
	b, err := os.ReadFile(stdout.Name())
	if err != nil || len(b) != 0 {
		t.Fatalf("published result: %s %v", b, err)
	}
	b, err = os.ReadFile(stderr.Name())
	if err != nil || string(b) != "Resolving and validating one Agent Plugin package for every selected target...\n" {
		t.Fatalf("progress changed: %s %v", b, err)
	}
	if !reflect.DeepEqual(beforeSource, securityFixtureTree(t, source)) || !reflect.DeepEqual(beforeHome, securityFixtureTree(t, home)) {
		t.Fatal("source/client mutation")
	}
	// Only empty scanner acquisition directories are permitted; no lifecycle state,
	// assessment cache, executable bytes or leaked private source snapshot.
	want := []string{".", "security", "security/lintai", "security/lintai/0.1.3", "security/lintai/0.1.3/" + runtime.GOOS + "-" + runtime.GOARCH}
	entries := securityFixtureTree(t, state)
	if len(entries) != len(want) {
		t.Fatalf("unexpected installer effects: %v", entries)
	}
	for _, rel := range want {
		st, err := os.Stat(filepath.Join(state, rel))
		if err != nil || !st.IsDir() {
			t.Fatalf("unexpected scanner scratch %s %v", rel, err)
		}
	}
}
