package agentpluginscli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/pem"
	"io/fs"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type directTransportFixture struct {
	server       *httptest.Server
	work, remote string
	mu           sync.Mutex
	payload      []byte
	downloads    int
}

func newDirectTransportFixture(t *testing.T, format string, private bool) *directTransportFixture {
	t.Helper()
	fixture := &directTransportFixture{work: dependencyRepo(t)}
	project := t.TempDir()
	fixture.remote = filepath.Join(project, "repo.git")
	dependencyGit(t, fixture.work, "clone", "--quiet", "--bare", fixture.work, fixture.remote)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	backend := &cgi.Handler{Path: git, Args: []string{"http-backend"}, Env: []string{"GIT_PROJECT_ROOT=" + project, "GIT_HTTP_EXPORT_ALL=1"}}
	fixture.payload = directArchiveBytes(t, fixture.work, format, "provider-v1/")
	fixture.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/repo.git/") {
			if private {
				user, password, ok := r.BasicAuth()
				if !ok || user != "fixture" || password != "disposable-fixture-token" {
					w.Header().Set("WWW-Authenticate", `Basic realm="fixture"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
			}
			backend.ServeHTTP(w, r)
			return
		}
		fixture.mu.Lock()
		defer fixture.mu.Unlock()
		fixture.downloads++
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(fixture.payload)
	}))
	t.Cleanup(fixture.server.Close)
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	dependencyWrite(t, caPath, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: fixture.server.Certificate().Raw})))
	t.Setenv("GIT_SSL_CAINFO", caPath)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	if private {
		dependencyWrite(t, filepath.Join(home, ".gitconfig"), "[credential \""+fixture.server.URL+"\"]\n helper = \"!f() { printf 'username=fixture\\npassword=disposable-fixture-token\\n'; }; f\"\n")
	}
	return fixture
}

func directArchiveBytes(t *testing.T, root, format, wrapper string) []byte {
	t.Helper()
	var result bytes.Buffer
	if format == "git" || format == "private-git" {
		return nil
	}
	if format == "zip" || format == "extensionless" {
		writer := zip.NewWriter(&result)
		directWalkPackage(t, root, func(name string, body []byte) error {
			file, err := writer.Create(wrapper + name)
			if err != nil {
				return err
			}
			_, err = file.Write(body)
			return err
		})
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		return result.Bytes()
	}
	var target interface{ Write([]byte) (int, error) } = &result
	var compressed *gzip.Writer
	if format != "tar" {
		compressed = gzip.NewWriter(&result)
		target = compressed
	}
	writer := tar.NewWriter(target)
	directWalkPackage(t, root, func(name string, body []byte) error {
		if err := writer.WriteHeader(&tar.Header{Name: wrapper + name, Mode: 0600, Size: int64(len(body))}); err != nil {
			return err
		}
		_, err := writer.Write(body)
		return err
	})
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if compressed != nil {
		if err := compressed.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return result.Bytes()
}

func directWalkPackage(t *testing.T, root string, visit func(string, []byte) error) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return visit(filepath.ToSlash(name), body)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func directCLIEnvironment(t *testing.T, transport *directTransportFixture) cliFixture {
	t.Helper()
	fixture, _, _ := dependencySpecHarness(t, transport.work, dependencyClientSpec{id: domain.ClientGemini, activation: "active"})
	fixture.app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: t.TempDir(), HTTPClient: transport.server.Client()}
	return fixture
}

func directLockedPin(t *testing.T, fixture cliFixture) environment.LockedPackage {
	t.Helper()
	manifestPath, lockPath := fixture.app.dependencyPaths()
	requirements, err := readDependencyManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	locked, err := readDependencyLock(lockPath, requirements)
	if err != nil {
		t.Fatal(err)
	}
	if len(locked.Packages) != 1 {
		t.Fatalf("unexpected pins: %+v", locked.Packages)
	}
	return locked.Packages[0]
}

// Git transports and archive downloads are native/real TLS. Agent process
// responses are inert fixtures; the standard loader, store, ownership and CLI
// transactions are production implementations.
func TestDirectSourceSavedLifecycleRealHTTPS(t *testing.T) {
	for _, format := range []string{"git", "private-git", "zip", "tar", "tar.gz", "tgz", "extensionless"} {
		t.Run(format, func(t *testing.T) { directSavedLifecycleCase(t, format) })
	}
}

func directSavedLifecycleCase(t *testing.T, format string) {
	t.Helper()
	transport := newDirectTransportFixture(t, format, format == "private-git")
	fixture := directCLIEnvironment(t, transport)
	endpoint := transport.server.URL + "/package." + format
	if format == "git" || format == "private-git" {
		endpoint = transport.server.URL + "/repo.git"
	}
	if format == "extensionless" {
		endpoint = transport.server.URL + "/download"
	}
	directSavedLifecycleEndpoint(t, fixture, transport, format, endpoint)
}

func directSavedLifecycleEndpoint(t *testing.T, fixture cliFixture, transport *directTransportFixture, format, endpoint string) {
	t.Helper()
	dependencyRun(t, fixture, "add", endpoint, "--target", "gemini", "--format", "json")
	first := directLockedPin(t, fixture)
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	installationID, sourceID := state.Installations[0].InstallationID, state.Installations[0].Source.SourceBindingID
	directAssertSourceFacts(t, first, state.Installations[0], endpoint)
	directAdvanceTransport(t, transport, format)
	dependencyRun(t, fixture, "update", first.Alias, "--target", "gemini", "--format", "json")
	second := directLockedPin(t, fixture)
	if second.TreeDigest == first.TreeDigest || second.SourceKey != first.SourceKey {
		t.Fatal("moving source did not update within its stable ownership")
	}
	state, err = fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Installations[0].InstallationID != installationID || state.Installations[0].Source.SourceBindingID != sourceID {
		t.Fatal("update changed binding identity")
	}
	dependencyRun(t, fixture, "repair", installationID, "--target", "gemini", "--format", "json")
	directColdRestore(t, fixture, transport, second)
}

func directAdvanceTransport(t *testing.T, transport *directTransportFixture, format string) {
	t.Helper()
	dependencyWrite(t, filepath.Join(transport.work, "revision.txt"), "second immutable package")
	dependencyGit(t, transport.work, "add", ".")
	dependencyGit(t, transport.work, "commit", "-m", "test: advance source")
	dependencyGit(t, transport.work, "push", "--quiet", transport.remote, "main")
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.payload = directArchiveBytes(t, transport.work, format, "provider-v2/")
}

func directAssertSourceFacts(t *testing.T, pin environment.LockedPackage, installation domain.Installation, endpoint string) {
	t.Helper()
	kind := "git"
	if pin.Source.Archive != nil {
		kind = "archive"
		if pin.Source.Archive.SHA256 == "" || installation.Source.ArchiveSHA256 != pin.Source.Archive.SHA256 {
			t.Fatal("archive byte checksum was lost")
		}
	}
	if installation.Source.SourceKind != kind || installation.Source.SourceURL != endpoint || installation.Source.PackageSubpath != "" {
		t.Fatalf("transport provenance lost: %+v", installation.Source)
	}
}

func directColdRestore(t *testing.T, original cliFixture, transport *directTransportFixture, pin environment.LockedPackage) {
	t.Helper()
	fresh := directCLIEnvironment(t, transport)
	oldManifest, oldLock := original.app.dependencyPaths()
	newManifest, newLock := fresh.app.dependencyPaths()
	dependencyWrite(t, newManifest, dependencyRead(t, oldManifest))
	dependencyWrite(t, newLock, dependencyRead(t, oldLock))
	dependencyRun(t, fresh, "deps", "sync", "--locked", "--yes", "--format", "json")
	state, err := fresh.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || state.Installations[0].Source.TreeDigest != pin.TreeDigest || state.Installations[0].Source.ResolvedRevision != pin.Revision() {
		t.Fatalf("cold restore changed pinned identity: %+v", state.Installations)
	}
	dependencyRun(t, fresh, "deps", "sync", "--locked", "--yes", "--format", "json")
	view, err := fresh.app.Installer.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if view.Installations[0].SourceKey != pin.SourceKey {
		t.Fatalf("inspection lost source key: %+v", view.Installations)
	}
}
