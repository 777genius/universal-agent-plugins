package pluginkitairepo_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginKitAIBundleFetchURLPythonRequirementsFlow(t *testing.T) {
	if !pythonRuntimeAvailable() {
		t.Skip("python runtime not available for bundle fetch flow")
	}

	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := runtimeProjectRoot(t)
	run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "codex-runtime", "--runtime", "python", "-o", plugRoot, "--extras")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
	}
	writeRuntimeFile(t, plugRoot, "requirements.txt", "requests==2.32.0\n")

	bootstrap := exec.Command(pluginKitAIBin, "bootstrap", plugRoot)
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap before export: %v\n%s", err, out)
	}
	exportProject(t, pluginKitAIBin, plugRoot, "codex-runtime", "plugin-kit-ai export before fetch", nil)

	bundlePath := filepath.Join(plugRoot, "genplug_codex-runtime_python_bundle.tar.gz")
	bundleBody, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(bundleBody)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bundle.tar.gz":
			_, _ = w.Write(bundleBody)
		case "/bundle.tar.gz.sha256":
			_, _ = w.Write([]byte(hex.EncodeToString(sum[:]) + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "installed")
	fetch := exec.Command(pluginKitAIBin, "bundle", "fetch", "--url", server.URL+"/bundle.tar.gz", "--dest", dest)
	fetch.Env = bundleFetchTestCAEnv(t, server)
	out, err := fetch.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai bundle fetch python requirements: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Checksum source: "+server.URL+"/bundle.tar.gz.sha256") {
		t.Fatalf("fetch output missing checksum source:\n%s", out)
	}

	doctor := exec.Command(pluginKitAIBin, "doctor", dest)
	out, err = doctor.CombinedOutput()
	if err == nil {
		t.Fatalf("expected doctor to require bootstrap after bundle fetch:\n%s", out)
	}
	if !strings.Contains(string(out), "Status: needs_bootstrap") {
		t.Fatalf("doctor output missing needs_bootstrap:\n%s", out)
	}

	bootstrap = exec.Command(pluginKitAIBin, "bootstrap", dest)
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap after bundle fetch: %v\n%s", err, out)
	}
	validateStrictProject(t, pluginKitAIBin, dest, "codex-runtime", "plugin-kit-ai validate after bundle fetch", nil)
}

func TestPluginKitAIBundleFetchURLClaudeNodeTypeScriptFlow(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not in PATH")
	}
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not in PATH")
	}

	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := runtimeProjectRoot(t)
	run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "claude", "--runtime", "node", "--typescript", "-o", plugRoot, "--extras")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai init claude node typescript: %v\n%s", err, out)
	}

	bootstrap := exec.Command(pluginKitAIBin, "bootstrap", plugRoot)
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap before claude export: %v\n%s", err, out)
	}
	exportProject(t, pluginKitAIBin, plugRoot, "claude", "plugin-kit-ai export before fetch", nil)

	bundlePath := filepath.Join(plugRoot, "genplug_claude_node_bundle.tar.gz")
	bundleBody, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(bundleBody)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bundle.tar.gz":
			_, _ = w.Write(bundleBody)
		case "/bundle.tar.gz.sha256":
			_, _ = w.Write([]byte(hex.EncodeToString(sum[:]) + "  bundle.tar.gz\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "installed")
	fetch := exec.Command(pluginKitAIBin, "bundle", "fetch", "--url", server.URL+"/bundle.tar.gz", "--dest", dest)
	fetch.Env = bundleFetchTestCAEnv(t, server)
	out, err := fetch.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai bundle fetch claude node typescript: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "platform=claude runtime=node") {
		t.Fatalf("fetch output missing claude runtime summary:\n%s", out)
	}

	doctor := exec.Command(pluginKitAIBin, "doctor", dest)
	out, err = doctor.CombinedOutput()
	if err == nil {
		t.Fatalf("expected doctor to require bootstrap after bundle fetch:\n%s", out)
	}
	if !strings.Contains(string(out), "Status: needs_bootstrap") {
		t.Fatalf("doctor output missing needs_bootstrap:\n%s", out)
	}

	bootstrap = exec.Command(pluginKitAIBin, "bootstrap", dest)
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap after bundle fetch: %v\n%s", err, out)
	}
	validateStrictProject(t, pluginKitAIBin, dest, "claude", "plugin-kit-ai validate after bundle fetch", nil)
}

func TestPluginKitAIBundleFetchGitHubClaudeNodeTypeScriptFlow(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not in PATH")
	}
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not in PATH")
	}

	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := runtimeProjectRoot(t)
	run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "claude", "--runtime", "node", "--typescript", "-o", plugRoot, "--extras")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai init claude node typescript: %v\n%s", err, out)
	}

	bootstrap := exec.Command(pluginKitAIBin, "bootstrap", plugRoot)
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap before github fetch export: %v\n%s", err, out)
	}
	exportProject(t, pluginKitAIBin, plugRoot, "claude", "plugin-kit-ai export before github fetch", nil)

	bundleName := "genplug_claude_node_bundle.tar.gz"
	bundlePath := filepath.Join(plugRoot, bundleName)
	bundleBody, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	server := newMockBundleFetchGitHubServer(t, bundleName, bundleBody)
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "installed")
	fetch := exec.Command(
		pluginKitAIBin,
		"bundle", "fetch", "o/r",
		"--tag", "v1",
		"--dest", dest,
		"--platform", "claude",
		"--runtime", "node",
		"--github-api-base", server.URL,
	)
	out, err := fetch.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai bundle fetch github claude node typescript: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Bundle source: github release o/r@v1 (tag) asset="+bundleName) {
		t.Fatalf("fetch output missing github bundle source:\n%s", out)
	}
	if !strings.Contains(string(out), "Checksum source: release asset checksums.txt") {
		t.Fatalf("fetch output missing checksums source:\n%s", out)
	}

	doctor := exec.Command(pluginKitAIBin, "doctor", dest)
	out, err = doctor.CombinedOutput()
	if err == nil {
		t.Fatalf("expected doctor to require bootstrap after github bundle fetch:\n%s", out)
	}
	if !strings.Contains(string(out), "Status: needs_bootstrap") {
		t.Fatalf("doctor output missing needs_bootstrap:\n%s", out)
	}

	bootstrap = exec.Command(pluginKitAIBin, "bootstrap", dest)
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap after github bundle fetch: %v\n%s", err, out)
	}
	validateStrictProject(t, pluginKitAIBin, dest, "claude", "plugin-kit-ai validate after github bundle fetch", nil)
}

func TestPluginKitAIBundleFetchGitHubLatestClaudeNodeTypeScriptFlow(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not in PATH")
	}
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not in PATH")
	}

	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := runtimeProjectRoot(t)
	run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "claude", "--runtime", "node", "--typescript", "-o", plugRoot, "--extras")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai init claude node typescript: %v\n%s", err, out)
	}

	bootstrap := exec.Command(pluginKitAIBin, "bootstrap", plugRoot)
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap before latest fetch export: %v\n%s", err, out)
	}
	exportProject(t, pluginKitAIBin, plugRoot, "claude", "plugin-kit-ai export before latest fetch", nil)

	bundleName := "genplug_claude_node_bundle.tar.gz"
	bundlePath := filepath.Join(plugRoot, bundleName)
	bundleBody, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	server := newMockBundleFetchGitHubServer(t, bundleName, bundleBody)
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "installed")
	fetch := exec.Command(
		pluginKitAIBin,
		"bundle", "fetch", "o/r",
		"--latest",
		"--dest", dest,
		"--platform", "claude",
		"--runtime", "node",
		"--github-api-base", server.URL,
	)
	out, err := fetch.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai bundle fetch github latest claude node typescript: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Bundle source: github release o/r@v1 (latest) asset="+bundleName) {
		t.Fatalf("fetch output missing github latest bundle source:\n%s", out)
	}

	doctor := exec.Command(pluginKitAIBin, "doctor", dest)
	out, err = doctor.CombinedOutput()
	if err == nil {
		t.Fatalf("expected doctor to require bootstrap after github latest bundle fetch:\n%s", out)
	}
	if !strings.Contains(string(out), "Status: needs_bootstrap") {
		t.Fatalf("doctor output missing needs_bootstrap:\n%s", out)
	}

	bootstrap = exec.Command(pluginKitAIBin, "bootstrap", dest)
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap after github latest bundle fetch: %v\n%s", err, out)
	}
	validateStrictProject(t, pluginKitAIBin, dest, "claude", "plugin-kit-ai validate after github latest bundle fetch", nil)
}

func TestPluginKitAIBundlePublishFetchPythonRequirementsFlow(t *testing.T) {
	if !pythonRuntimeAvailable() {
		t.Skip("python runtime not available for bundle publish/fetch flow")
	}

	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := runtimeProjectRoot(t)
	run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "codex-runtime", "--runtime", "python", "-o", plugRoot, "--extras")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
	}
	writeRuntimeFile(t, plugRoot, "requirements.txt", "requests==2.32.0\n")

	bootstrap := exec.Command(pluginKitAIBin, "bootstrap", plugRoot)
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap before publish: %v\n%s", err, out)
	}

	server := newMockBundlePublishGitHubServer(t)
	defer server.Close()

	publish := exec.Command(
		pluginKitAIBin,
		"bundle", "publish", plugRoot,
		"--platform", "codex-runtime",
		"--repo", "o/r",
		"--tag", "v1",
		"--github-api-base", server.URL,
	)
	out, err := publish.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai bundle publish python requirements: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Release state: created published release") {
		t.Fatalf("publish output missing published release line:\n%s", out)
	}

	dest := filepath.Join(t.TempDir(), "installed")
	fetch := exec.Command(
		pluginKitAIBin,
		"bundle", "fetch", "o/r",
		"--tag", "v1",
		"--dest", dest,
		"--platform", "codex-runtime",
		"--runtime", "python",
		"--github-api-base", server.URL,
	)
	out, err = fetch.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai bundle fetch published python requirements: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Checksum source: release asset genplug_codex-runtime_python_bundle.tar.gz.sha256") {
		t.Fatalf("fetch output missing sidecar checksum source:\n%s", out)
	}

	doctor := exec.Command(pluginKitAIBin, "doctor", dest)
	out, err = doctor.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "Status: needs_bootstrap") {
		t.Fatalf("doctor output = %s, err=%v", out, err)
	}
	bootstrap = exec.Command(pluginKitAIBin, "bootstrap", dest)
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap after published fetch: %v\n%s", err, out)
	}
	validate := exec.Command(pluginKitAIBin, "validate", dest, "--platform", "codex-runtime", "--strict")
	if out, err := validate.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai validate after published fetch: %v\n%s", err, out)
	}
}

func TestPluginKitAIBundlePublishFetchClaudeNodeTypeScriptFlow(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not in PATH")
	}
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not in PATH")
	}

	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := runtimeProjectRoot(t)
	run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "claude", "--runtime", "node", "--typescript", "-o", plugRoot, "--extras")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai init claude node typescript: %v\n%s", err, out)
	}

	bootstrap := exec.Command(pluginKitAIBin, "bootstrap", plugRoot)
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap before publish: %v\n%s", err, out)
	}

	server := newMockBundlePublishGitHubServer(t)
	defer server.Close()

	publish := exec.Command(
		pluginKitAIBin,
		"bundle", "publish", plugRoot,
		"--platform", "claude",
		"--repo", "o/r",
		"--tag", "v2",
		"--github-api-base", server.URL,
	)
	out, err := publish.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai bundle publish claude node typescript: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "genplug_claude_node_bundle.tar.gz") {
		t.Fatalf("publish output missing bundle asset name:\n%s", out)
	}

	dest := filepath.Join(t.TempDir(), "installed")
	fetch := exec.Command(
		pluginKitAIBin,
		"bundle", "fetch", "o/r",
		"--tag", "v2",
		"--dest", dest,
		"--platform", "claude",
		"--runtime", "node",
		"--github-api-base", server.URL,
	)
	out, err = fetch.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai bundle fetch published claude node typescript: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Checksum source: release asset genplug_claude_node_bundle.tar.gz.sha256") {
		t.Fatalf("fetch output missing sidecar checksum source:\n%s", out)
	}

	doctor := exec.Command(pluginKitAIBin, "doctor", dest)
	out, err = doctor.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "Status: needs_bootstrap") {
		t.Fatalf("doctor output = %s, err=%v", out, err)
	}
	bootstrap = exec.Command(pluginKitAIBin, "bootstrap", dest)
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap after published fetch: %v\n%s", err, out)
	}
	validate := exec.Command(pluginKitAIBin, "validate", dest, "--platform", "claude", "--strict")
	if out, err := validate.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai validate after published fetch: %v\n%s", err, out)
	}
}

func bundleFetchTestCAEnv(t *testing.T, server *httptest.Server) []string {
	t.Helper()
	certPath := filepath.Join(t.TempDir(), "bundle-fetch-test-ca.pem")
	certDER := server.TLS.Certificates[0].Certificate[0]
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		t.Fatal(err)
	}
	return append(os.Environ(), "PLUGIN_KIT_AI_TEST_CA_FILE="+certPath)
}

func newMockBundleFetchGitHubServer(t *testing.T, bundleName string, bundleBody []byte) *httptest.Server {
	t.Helper()
	sum := sha256.Sum256(bundleBody)
	checksums := hex.EncodeToString(sum[:]) + "  " + bundleName + "\n"
	type ghAsset struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	}
	release := struct {
		TagName string    `json:"tag_name"`
		Assets  []ghAsset `json:"assets"`
	}{}

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := srv.URL
		switch r.URL.Path {
		case "/repos/o/r/releases/tags/v1":
			release.TagName = "v1"
			release.Assets = []ghAsset{
				{Name: "checksums.txt", BrowserDownloadURL: base + "/checksums.txt", Size: int64(len(checksums))},
				{Name: bundleName, BrowserDownloadURL: base + "/bundle.tar.gz", Size: int64(len(bundleBody))},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(release)
		case "/repos/o/r/releases/latest":
			release.TagName = "v1"
			release.Assets = []ghAsset{
				{Name: "checksums.txt", BrowserDownloadURL: base + "/checksums.txt", Size: int64(len(checksums))},
				{Name: bundleName, BrowserDownloadURL: base + "/bundle.tar.gz", Size: int64(len(bundleBody))},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(release)
		case "/checksums.txt":
			_, _ = w.Write([]byte(checksums))
		case "/bundle.tar.gz":
			_, _ = w.Write(bundleBody)
		default:
			http.NotFound(w, r)
		}
	}))
	return srv
}

func newMockBundlePublishGitHubServer(t *testing.T) *httptest.Server {
	t.Helper()
	type ghAsset struct {
		ID                 int64  `json:"id"`
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	}
	type ghRelease struct {
		ID         int64     `json:"id"`
		TagName    string    `json:"tag_name"`
		Draft      bool      `json:"draft"`
		Prerelease bool      `json:"prerelease"`
		UploadURL  string    `json:"upload_url"`
		Assets     []ghAsset `json:"assets"`
	}
	releases := map[string]*ghRelease{}
	bodies := map[string][]byte{}
	var nextReleaseID int64 = 100
	var nextAssetID int64 = 1000

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := srv.URL
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/repos/o/r/releases/tags/"):
			tag := strings.TrimPrefix(r.URL.Path, "/repos/o/r/releases/tags/")
			release, ok := releases[tag]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(release)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/o/r/releases/latest":
			var latest *ghRelease
			for _, candidate := range releases {
				if candidate.Draft || candidate.Prerelease {
					continue
				}
				if latest == nil || candidate.ID > latest.ID {
					latest = candidate
				}
			}
			if latest == nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(latest)
		case r.Method == http.MethodPost && r.URL.Path == "/repos/o/r/releases":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			tag, _ := payload["tag_name"].(string)
			draft, _ := payload["draft"].(bool)
			release := &ghRelease{
				ID:         nextReleaseID,
				TagName:    tag,
				Draft:      draft,
				Prerelease: false,
				UploadURL:  base + "/upload/" + tag + "/assets{?name,label}",
				Assets:     []ghAsset{},
			}
			nextReleaseID++
			releases[tag] = release
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(release)
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/repos/o/r/releases/"):
			idText := strings.TrimPrefix(r.URL.Path, "/repos/o/r/releases/")
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			draft, _ := payload["draft"].(bool)
			var release *ghRelease
			for _, candidate := range releases {
				if fmt.Sprintf("%d", candidate.ID) == idText {
					release = candidate
					break
				}
			}
			if release == nil {
				http.NotFound(w, r)
				return
			}
			release.Draft = draft
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(release)
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/upload/") && strings.HasSuffix(r.URL.Path, "/assets"):
			trimmed := strings.TrimPrefix(r.URL.Path, "/upload/")
			tag := strings.TrimSuffix(trimmed, "/assets")
			release, ok := releases[tag]
			if !ok {
				http.NotFound(w, r)
				return
			}
			name := r.URL.Query().Get("name")
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			bodies[name] = body
			asset := ghAsset{
				ID:                 nextAssetID,
				Name:               name,
				BrowserDownloadURL: base + "/download/" + name,
				Size:               int64(len(body)),
			}
			nextAssetID++
			release.Assets = append(release.Assets, asset)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(asset)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/repos/o/r/releases/assets/"):
			assetIDText := strings.TrimPrefix(r.URL.Path, "/repos/o/r/releases/assets/")
			for _, release := range releases {
				filtered := release.Assets[:0]
				for _, asset := range release.Assets {
					if fmt.Sprintf("%d", asset.ID) == assetIDText {
						delete(bodies, asset.Name)
						continue
					}
					filtered = append(filtered, asset)
				}
				release.Assets = filtered
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/download/"):
			name := strings.TrimPrefix(r.URL.Path, "/download/")
			body, ok := bodies[name]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	return srv
}
