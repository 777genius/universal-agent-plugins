package pluginkitairepo_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPluginKitAIInstall_MockGitHub(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	archName := fmt.Sprintf("plug_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	binName := "plug"
	tarGz := mustTarGz(t, binName, []byte("plugbin"))
	sum := sha256.Sum256(tarGz)
	sumLine := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), archName)

	srv := newMockGitHubServer(t, mockGitHubConfig{
		archName: archName,
		sumLine:  sumLine,
		tarGz:    tarGz,
	})
	t.Cleanup(srv.Close)

	pluginKitAIBin := buildPluginKitAI(t)
	outDir := t.TempDir()
	code, out := runInstall(t, pluginKitAIBin, "", srv.URL, "--tag", "v1", "--dir", outDir, "--force")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	assertInstallSummary(t, string(out), filepath.Join(outDir, binName), "v1", "tag", archName, runtime.GOOS, runtime.GOARCH, false)

	got, err := os.ReadFile(filepath.Join(outDir, binName))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "plugbin" {
		t.Fatalf("binary content %q", got)
	}
}

func TestPluginKitAIInstall_defaultDir(t *testing.T) {
	requireBindTests(t)
	archName := fmt.Sprintf("plug_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	binName := "plug"
	tarGz := mustTarGz(t, binName, []byte("plugbin"))
	sum := sha256.Sum256(tarGz)
	sumLine := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), archName)

	srv := newMockGitHubServer(t, mockGitHubConfig{
		archName: archName,
		sumLine:  sumLine,
		tarGz:    tarGz,
	})
	t.Cleanup(srv.Close)

	workDir := t.TempDir()
	pluginKitAIBin := buildPluginKitAI(t)
	// cmd.Dir = workDir → default --dir bin resolves to workDir/bin.
	code, out := runInstall(t, pluginKitAIBin, workDir, srv.URL, "--tag", "v1", "--force")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	assertInstallSummary(t, string(out), filepath.Join(workDir, "bin", binName), "v1", "tag", archName, runtime.GOOS, runtime.GOARCH, false)

	got, err := os.ReadFile(filepath.Join(workDir, "bin", binName))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "plugbin" {
		t.Fatalf("binary content %q", got)
	}
}

func TestPluginKitAIInstall_outputName(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	archName := fmt.Sprintf("plug_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	binName := "plug"
	tarGz := mustTarGz(t, binName, []byte("x"))
	sum := sha256.Sum256(tarGz)
	sumLine := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), archName)

	srv := newMockGitHubServer(t, mockGitHubConfig{
		archName: archName,
		sumLine:  sumLine,
		tarGz:    tarGz,
	})
	t.Cleanup(srv.Close)

	pluginKitAIBin := buildPluginKitAI(t)
	outDir := t.TempDir()
	code, out := runInstall(t, pluginKitAIBin, "", srv.URL, "--tag", "v1", "--dir", outDir, "--force", "--output-name", "renamed-bin")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	assertInstallSummary(t, string(out), filepath.Join(outDir, "renamed-bin"), "v1", "tag", archName, runtime.GOOS, runtime.GOARCH, false)
	b, err := os.ReadFile(filepath.Join(outDir, "renamed-bin"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "x" {
		t.Fatalf("got %q", b)
	}
}

func TestPluginKitAIInstall_redirectAsset(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	archName := fmt.Sprintf("plug_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	binName := "plug"
	tarGz := mustTarGz(t, binName, []byte("via-redirect"))
	sum := sha256.Sum256(tarGz)
	sumLine := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), archName)

	srv := newMockGitHubServer(t, mockGitHubConfig{
		archName:           archName,
		sumLine:            sumLine,
		tarGz:              tarGz,
		archiveViaRedirect: true,
	})
	t.Cleanup(srv.Close)

	pluginKitAIBin := buildPluginKitAI(t)
	outDir := t.TempDir()
	code, out := runInstall(t, pluginKitAIBin, "", srv.URL, "--tag", "v1", "--dir", outDir, "--force")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	assertInstallSummary(t, string(out), filepath.Join(outDir, binName), "v1", "tag", archName, runtime.GOOS, runtime.GOARCH, false)
	got, err := os.ReadFile(filepath.Join(outDir, binName))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "via-redirect" {
		t.Fatalf("binary content %q", got)
	}
}

func TestPluginKitAIInstall_checksum429ThenOK(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	archName := fmt.Sprintf("plug_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	binName := "plug"
	tarGz := mustTarGz(t, binName, []byte("after-429"))
	sum := sha256.Sum256(tarGz)
	sumLine := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), archName)

	srv := newMockGitHubServer(t, mockGitHubConfig{
		archName:        archName,
		sumLine:         sumLine,
		tarGz:           tarGz,
		checksum429Once: true,
	})
	t.Cleanup(srv.Close)

	pluginKitAIBin := buildPluginKitAI(t)
	outDir := t.TempDir()
	code, out := runInstall(t, pluginKitAIBin, "", srv.URL, "--tag", "v1", "--dir", outDir, "--force")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	assertInstallSummary(t, string(out), filepath.Join(outDir, binName), "v1", "tag", archName, runtime.GOOS, runtime.GOARCH, false)
	got, err := os.ReadFile(filepath.Join(outDir, binName))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "after-429" {
		t.Fatalf("binary content %q", got)
	}
}

func TestPluginKitAIInstall_latestRawBinary(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	rawName := fmt.Sprintf("notify-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		rawName += ".exe"
	}
	body := []byte("claude-notifications-go-style")
	sum := sha256.Sum256(body)
	sumLine := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), rawName)

	srv := newMockGitHubServer(t, mockGitHubConfig{
		useLatest:  true,
		releaseTag: "v1.34.0",
		archName:   rawName,
		sumLine:    sumLine,
		tarGz:      body,
	})
	t.Cleanup(srv.Close)

	pluginKitAIBin := buildPluginKitAI(t)
	outDir := t.TempDir()
	code, out := runInstall(t, pluginKitAIBin, "", srv.URL, "--latest", "--dir", outDir, "--force")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	assertInstallSummary(t, string(out), filepath.Join(outDir, rawName), "v1.34.0", "latest", rawName, runtime.GOOS, runtime.GOARCH, false)
	got, err := os.ReadFile(filepath.Join(outDir, rawName))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Fatalf("got %q", got)
	}
}

func TestPluginKitAIInstall_API404(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	pluginKitAIBin := buildPluginKitAI(t)
	code, out := runInstall(t, pluginKitAIBin, "", srv.URL, "--tag", "v1", "--dir", t.TempDir(), "--force")
	if code != 2 {
		t.Fatalf("want exit 2, got %d: %s", code, out)
	}
}

func TestPluginKitAIInstall_API403(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	pluginKitAIBin := buildPluginKitAI(t)
	code, out := runInstall(t, pluginKitAIBin, "", srv.URL, "--tag", "v1", "--dir", t.TempDir(), "--force")
	if code != 3 {
		t.Fatalf("want exit 3, got %d: %s", code, out)
	}
}

func TestPluginKitAIInstall_forceOverwritesExistingFile(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	archName := fmt.Sprintf("plug_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	binName := "plug"
	tarGz := mustTarGz(t, binName, []byte("new-content"))
	sum := sha256.Sum256(tarGz)
	sumLine := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), archName)

	srv := newMockGitHubServer(t, mockGitHubConfig{
		archName: archName,
		sumLine:  sumLine,
		tarGz:    tarGz,
	})
	t.Cleanup(srv.Close)

	pluginKitAIBin := buildPluginKitAI(t)
	outDir := t.TempDir()
	dest := filepath.Join(outDir, binName)
	if err := os.WriteFile(dest, []byte("old-content"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out := runInstall(t, pluginKitAIBin, "", srv.URL, "--tag", "v1", "--dir", outDir, "--force")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	assertInstallSummary(t, string(out), dest, "v1", "tag", archName, runtime.GOOS, runtime.GOARCH, true)
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-content" {
		t.Fatalf("binary content %q", got)
	}
}

func TestPluginKitAIInstall_existingFileWithoutForceFails(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	archName := fmt.Sprintf("plug_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	binName := "plug"
	tarGz := mustTarGz(t, binName, []byte("new-content"))
	sum := sha256.Sum256(tarGz)
	sumLine := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), archName)

	srv := newMockGitHubServer(t, mockGitHubConfig{
		archName: archName,
		sumLine:  sumLine,
		tarGz:    tarGz,
	})
	t.Cleanup(srv.Close)

	pluginKitAIBin := buildPluginKitAI(t)
	outDir := t.TempDir()
	dest := filepath.Join(outDir, binName)
	if err := os.WriteFile(dest, []byte("old-content"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out := runInstall(t, pluginKitAIBin, "", srv.URL, "--tag", "v1", "--dir", outDir)
	if code != 5 {
		t.Fatalf("want exit 5, got %d: %s", code, out)
	}
	if !strings.Contains(string(out), "already exists") {
		t.Fatalf("want overwrite diagnostic, got %s", out)
	}
}

func TestPluginKitAIInstall_existingDestinationDirectoryFailsEvenWithForce(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	archName := fmt.Sprintf("plug_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	binName := "plug"
	tarGz := mustTarGz(t, binName, []byte("plugbin"))
	sum := sha256.Sum256(tarGz)
	sumLine := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), archName)

	srv := newMockGitHubServer(t, mockGitHubConfig{
		archName: archName,
		sumLine:  sumLine,
		tarGz:    tarGz,
	})
	t.Cleanup(srv.Close)

	pluginKitAIBin := buildPluginKitAI(t)
	outDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(outDir, binName), 0o755); err != nil {
		t.Fatal(err)
	}

	code, out := runInstall(t, pluginKitAIBin, "", srv.URL, "--tag", "v1", "--dir", outDir, "--force")
	if code != 5 {
		t.Fatalf("want exit 5, got %d: %s", code, out)
	}
	if !strings.Contains(string(out), "existing directory") {
		t.Fatalf("want directory diagnostic, got %s", out)
	}
}

func TestPluginKitAIInstall_installDirIsFileFails(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	pluginKitAIBin := buildPluginKitAI(t)
	dirFile := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(dirFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := runInstall(t, pluginKitAIBin, "", "http://127.0.0.1/unused", "--tag", "v1", "--dir", dirFile, "--force")
	if code != 5 {
		t.Fatalf("want exit 5, got %d: %s", code, out)
	}
	if !strings.Contains(string(out), "install dir is an existing file") {
		t.Fatalf("want install-dir diagnostic, got %s", out)
	}
}

func TestPluginKitAIInstall_targetOverrideInSummary(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	goos, goarch := "linux", "amd64"
	archName := fmt.Sprintf("plug_1.0.0_%s_%s.tar.gz", goos, goarch)
	binName := "plug"
	tarGz := mustTarGz(t, binName, []byte("linux-amd64"))
	sum := sha256.Sum256(tarGz)
	sumLine := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), archName)

	srv := newMockGitHubServer(t, mockGitHubConfig{
		archName: archName,
		sumLine:  sumLine,
		tarGz:    tarGz,
	})
	t.Cleanup(srv.Close)

	pluginKitAIBin := buildPluginKitAI(t)
	outDir := t.TempDir()
	code, out := runInstall(t, pluginKitAIBin, "", srv.URL, "--tag", "v1", "--dir", outDir, "--force", "--goos", goos, "--goarch", goarch)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	assertInstallSummary(t, string(out), filepath.Join(outDir, binName), "v1", "tag", archName, goos, goarch, false)
}

func assertInstallSummary(t *testing.T, output, path, releaseRef, releaseSource, asset, goos, goarch string, overwritten bool) {
	t.Helper()
	installedLines := []string{"Installed " + path}
	if resolved, err := filepath.EvalSymlinks(path); err == nil && resolved != path {
		installedLines = append(installedLines, "Installed "+resolved)
	}
	foundInstalled := false
	for _, want := range installedLines {
		if strings.Contains(output, want) {
			foundInstalled = true
			break
		}
	}
	if !foundInstalled {
		t.Fatalf("missing installed path line in output:\n%s", output)
	}
	for _, want := range []string{
		"Release: " + releaseRef + " (" + releaseSource + ")",
		"Asset: " + asset,
		"Target: " + goos + "/" + goarch,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing summary line %q in output:\n%s", want, output)
		}
	}
	if overwritten {
		if !strings.Contains(output, "Overwrote existing file: yes") {
			t.Fatalf("missing overwrite line in output:\n%s", output)
		}
		return
	}
	if strings.Contains(output, "Overwrote existing file: yes") {
		t.Fatalf("unexpected overwrite line in output:\n%s", output)
	}
}

type mockGitHubConfig struct {
	archName           string
	sumLine            string
	tarGz              []byte
	archiveViaRedirect bool
	checksum429Once    bool
	useLatest          bool
	releaseTag         string // JSON tag_name; default v1
}

func newMockGitHubServer(t *testing.T, cfg mockGitHubConfig) *httptest.Server {
	t.Helper()
	type ghAsset struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	}
	release := struct {
		TagName    string    `json:"tag_name"`
		Draft      bool      `json:"draft"`
		Prerelease bool      `json:"prerelease"`
		Assets     []ghAsset `json:"assets"`
	}{
		Draft:      false,
		Prerelease: false,
		Assets:     nil,
	}

	var checksumHits int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		tagName := cfg.releaseTag
		if tagName == "" {
			tagName = "v1"
		}
		writeReleaseJSON := func() {
			archiveURL := base + "/a"
			if cfg.archiveViaRedirect {
				archiveURL = base + "/via"
			}
			release.TagName = tagName
			release.Assets = []ghAsset{
				{Name: "checksums.txt", BrowserDownloadURL: base + "/c", Size: int64(len(cfg.sumLine))},
				{Name: cfg.archName, BrowserDownloadURL: archiveURL, Size: int64(len(cfg.tarGz))},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(release)
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/o/r/releases/tags/v1" && !cfg.useLatest:
			writeReleaseJSON()
		case r.Method == http.MethodGet && r.URL.Path == "/repos/o/r/releases/latest" && cfg.useLatest:
			writeReleaseJSON()

		case r.Method == http.MethodGet && r.URL.Path == "/c":
			if cfg.checksum429Once {
				if atomic.AddInt32(&checksumHits, 1) == 1 {
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(cfg.sumLine))

		case r.Method == http.MethodGet && r.URL.Path == "/via":
			http.Redirect(w, r, base+"/blob", http.StatusFound)

		case r.Method == http.MethodGet && r.URL.Path == "/blob":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(cfg.tarGz)

		case r.Method == http.MethodGet && r.URL.Path == "/a":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(cfg.tarGz)

		default:
			http.NotFound(w, r)
		}
	}))
	return srv
}

func mustTarGz(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
