package pluginkitairepo_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPythonCLIPackageContractFiles(t *testing.T) {
	t.Parallel()
	root := RepoRoot(t)

	for _, trackedPath := range []string{
		"python/plugin-kit-ai/pyproject.toml",
		"python/plugin-kit-ai/src/plugin_kit_ai/cli.py",
		"python/plugin-kit-ai/src/plugin_kit_ai/install.py",
		"python/plugin-kit-ai/src/plugin_kit_ai/platform.py",
	} {
		tracked := exec.Command("git", "ls-files", "--error-unmatch", trackedPath)
		tracked.Dir = root
		if out, err := tracked.CombinedOutput(); err != nil {
			t.Fatalf("python wrapper file must be tracked in git (%s): %v\n%s", trackedPath, err, out)
		}
	}

	pyproject := readRepoFile(t, root, "python", "plugin-kit-ai", "pyproject.toml")
	for _, want := range []string{
		`name = "plugin-kit-ai"`,
		`requires-python = ">=3.9"`,
		`dynamic = ["version"]`,
		`plugin-kit-ai = "plugin_kit_ai.cli:main"`,
		`version = { attr = "plugin_kit_ai.__version__" }`,
	} {
		if !strings.Contains(pyproject, want) {
			t.Fatalf("pyproject.toml missing %q:\n%s", want, pyproject)
		}
	}

	initPy := readRepoFile(t, root, "python", "plugin-kit-ai", "src", "plugin_kit_ai", "__init__.py")
	mustContain(t, initPy, `__version__ = "0.0.0.dev0"`)

	cliPy := readRepoFile(t, root, "python", "plugin-kit-ai", "src", "plugin_kit_ai", "cli.py")
	mustContain(t, cliPy, "format_install_error")
	mustContain(t, cliPy, "run_binary")

	installPy := readRepoFile(t, root, "python", "plugin-kit-ai", "src", "plugin_kit_ai", "install.py")
	for _, want := range []string{
		"777genius/plugin-kit-ai",
		"checksums.txt",
		"checksum mismatch",
		"npm i -g plugin-kit-ai",
		"brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai",
	} {
		mustContain(t, installPy, want)
	}

	workflow := readRepoFile(t, root, "docs", "history", "plugin-kit-ai-release-workflows", "pypi-publish.yml")
	for _, want := range []string{
		"name: PyPI Publish",
		"workflow_run:",
		"workflows: [\"Release Assets\"]",
		"id-token: write",
		"environment:",
		"name: pypi",
		"checksums.txt",
		"plugin-kit-ai_${version}_windows_arm64.tar.gz",
		"pypa/gh-action-pypi-publish@release/v1",
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("PyPI publish workflow missing %q:\n%s", want, workflow)
		}
	}
}

func TestPythonCLIPackageInstallsLatestReleaseAndRunsBinary(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	requirePythonRuntime(t)
	requirePythonWrapperSmokePlatform(t)

	packageRoot := copyPythonPackageToTemp(t)
	rewritePythonPackageVersion(t, packageRoot, "1.2.3")
	binaryName := pythonRuntimeBinaryName()
	assetName := fmt.Sprintf("plugin-kit-ai_1.2.3_%s_%s.tar.gz", runtimeGOOSForScript(), runtimeGOARCHForScript())
	archive := mustTarGz(t, binaryName, []byte("#!/usr/bin/env sh\nprintf 'version: v1.2.3\\n'\n"))
	sum := sha256.Sum256(archive)
	checksums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), assetName)

	srv := newPythonReleaseServer(t, pythonReleaseConfig{
		tag:       "v1.2.3",
		assetName: assetName,
		checksums: checksums,
		archive:   archive,
	})
	t.Cleanup(srv.Close)

	cacheDir := filepath.Join(t.TempDir(), "cache")
	cmd := exec.Command("python3", "-m", "plugin_kit_ai.cli", "version")
	cmd.Dir = packageRoot
	cmd.Env = append(os.Environ(),
		"PYTHONPATH="+filepath.Join(packageRoot, "src"),
		"PLUGIN_KIT_AI_CACHE_DIR="+cacheDir,
		"GITHUB_API_BASE="+srv.URL,
		"PLUGIN_KIT_AI_RELEASE_BASE_URL="+srv.URL,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("python wrapper latest version: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "version: v1.2.3") {
		t.Fatalf("wrapper output missing version:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "v1.2.3", binaryName)); err != nil {
		t.Fatalf("installed binary missing from cache: %v", err)
	}
}

func TestPythonCLIPackageUsesPinnedPackageVersionWithoutLatestLookup(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	requirePythonRuntime(t)
	requirePythonWrapperSmokePlatform(t)

	packageRoot := copyPythonPackageToTemp(t)
	rewritePythonPackageVersion(t, packageRoot, "1.2.4")

	binaryName := pythonRuntimeBinaryName()
	assetName := fmt.Sprintf("plugin-kit-ai_1.2.4_%s_%s.tar.gz", runtimeGOOSForScript(), runtimeGOARCHForScript())
	archive := mustTarGz(t, binaryName, []byte("#!/usr/bin/env sh\nprintf 'version: v1.2.4\\n'\n"))
	sum := sha256.Sum256(archive)
	checksums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), assetName)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/777genius/plugin-kit-ai/releases/latest":
			http.NotFound(w, r)
		case "/777genius/plugin-kit-ai/releases/download/v1.2.4/checksums.txt":
			_, _ = w.Write([]byte(checksums))
		case fmt.Sprintf("/777genius/plugin-kit-ai/releases/download/v1.2.4/%s", assetName):
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	cacheDir := filepath.Join(t.TempDir(), "cache")
	cmd := exec.Command("python3", "-m", "plugin_kit_ai.cli", "version")
	cmd.Dir = packageRoot
	cmd.Env = append(os.Environ(),
		"PYTHONPATH="+filepath.Join(packageRoot, "src"),
		"PLUGIN_KIT_AI_CACHE_DIR="+cacheDir,
		"GITHUB_API_BASE="+srv.URL,
		"PLUGIN_KIT_AI_RELEASE_BASE_URL="+srv.URL,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("python wrapper pinned package version: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "version: v1.2.4") {
		t.Fatalf("wrapper output missing pinned version:\n%s", out)
	}
}

func TestPythonCLIPackageReusesCachedBinary(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	requirePythonRuntime(t)
	requirePythonWrapperSmokePlatform(t)

	packageRoot := copyPythonPackageToTemp(t)
	rewritePythonPackageVersion(t, packageRoot, "1.2.3")
	binaryName := pythonRuntimeBinaryName()
	assetName := fmt.Sprintf("plugin-kit-ai_1.2.3_%s_%s.tar.gz", runtimeGOOSForScript(), runtimeGOARCHForScript())
	archive := mustTarGz(t, binaryName, []byte("#!/usr/bin/env sh\nprintf 'version: v1.2.3\\n'\n"))
	sum := sha256.Sum256(archive)
	checksums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), assetName)

	srv := newPythonReleaseServer(t, pythonReleaseConfig{
		tag:       "v1.2.3",
		assetName: assetName,
		checksums: checksums,
		archive:   archive,
	})

	cacheDir := filepath.Join(t.TempDir(), "cache")
	run := func(base string) []byte {
		cmd := exec.Command("python3", "-m", "plugin_kit_ai.cli", "version")
		cmd.Dir = packageRoot
		cmd.Env = append(os.Environ(),
			"PYTHONPATH="+filepath.Join(packageRoot, "src"),
			"PLUGIN_KIT_AI_CACHE_DIR="+cacheDir,
			"GITHUB_API_BASE="+base,
			"PLUGIN_KIT_AI_RELEASE_BASE_URL="+base,
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("python wrapper cached run: %v\n%s", err, out)
		}
		return out
	}

	first := run(srv.URL)
	if !strings.Contains(string(first), "version: v1.2.3") {
		t.Fatalf("first run output missing version:\n%s", first)
	}
	srv.Close()
	second := run("http://127.0.0.1:1")
	if !strings.Contains(string(second), "version: v1.2.3") {
		t.Fatalf("cached run output missing version:\n%s", second)
	}
}

func TestPythonCLIPackageRejectsChecksumMismatch(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	requirePythonRuntime(t)
	requirePythonWrapperSmokePlatform(t)

	packageRoot := copyPythonPackageToTemp(t)
	binaryName := pythonRuntimeBinaryName()
	assetName := fmt.Sprintf("plugin-kit-ai_1.2.3_%s_%s.tar.gz", runtimeGOOSForScript(), runtimeGOARCHForScript())
	archive := mustTarGz(t, binaryName, []byte("#!/usr/bin/env sh\nprintf 'bad\\n'\n"))
	checksums := fmt.Sprintf("%s  %s\n", strings.Repeat("0", 64), assetName)

	srv := newPythonReleaseServer(t, pythonReleaseConfig{
		tag:       "v1.2.3",
		assetName: assetName,
		checksums: checksums,
		archive:   archive,
	})
	t.Cleanup(srv.Close)

	cmd := exec.Command("python3", "-m", "plugin_kit_ai.cli", "version")
	cmd.Dir = packageRoot
	cmd.Env = append(os.Environ(),
		"PYTHONPATH="+filepath.Join(packageRoot, "src"),
		"PLUGIN_KIT_AI_CACHE_DIR="+filepath.Join(t.TempDir(), "cache"),
		"GITHUB_API_BASE="+srv.URL,
		"PLUGIN_KIT_AI_RELEASE_BASE_URL="+srv.URL,
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected checksum mismatch failure:\n%s", out)
	}
	if !strings.Contains(string(out), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch in output:\n%s", out)
	}
}

type pythonReleaseConfig struct {
	tag       string
	assetName string
	checksums string
	archive   []byte
}

func newPythonReleaseServer(t *testing.T, cfg pythonReleaseConfig) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/777genius/plugin-kit-ai/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": cfg.tag})
		case fmt.Sprintf("/777genius/plugin-kit-ai/releases/download/%s/checksums.txt", cfg.tag):
			_, _ = w.Write([]byte(cfg.checksums))
		case fmt.Sprintf("/777genius/plugin-kit-ai/releases/download/%s/%s", cfg.tag, cfg.assetName):
			_, _ = w.Write(cfg.archive)
		default:
			http.NotFound(w, r)
		}
	}))
}

func copyPythonPackageToTemp(t *testing.T) string {
	t.Helper()
	root := RepoRoot(t)
	dst := filepath.Join(t.TempDir(), "plugin-kit-ai-pypi")
	copyTree(t, filepath.Join(root, "python", "plugin-kit-ai"), dst)
	return dst
}

func rewritePythonPackageVersion(t *testing.T, packageRoot, version string) {
	t.Helper()
	path := filepath.Join(packageRoot, "src", "plugin_kit_ai", "__init__.py")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(body), `__version__ = "0.0.0.dev0"`, fmt.Sprintf(`__version__ = "%s"`, version), 1)
	if updated == string(body) {
		t.Fatalf("failed to rewrite package version in %s", path)
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
}

func requirePythonRuntime(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skipf("requires python3 in PATH: %v", err)
	}
}

func requirePythonWrapperSmokePlatform(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("python wrapper smoke uses a Unix shell script payload in release tarballs")
	}
}

func pythonRuntimeBinaryName() string {
	if runtime.GOOS == "windows" {
		return "plugin-kit-ai.exe"
	}
	return "plugin-kit-ai"
}

func TestPythonCLIReleaseTagRegressions(t *testing.T) {
	t.Parallel()
	requirePythonRuntime(t)
	root := RepoRoot(t)
	cmd := exec.Command("python3", "-m", "unittest", "discover", "-s", filepath.Join(root, "python", "plugin-kit-ai", "tests"), "-v")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "PYTHONPATH="+filepath.Join(root, "python", "plugin-kit-ai", "src"), "PYTHONDONTWRITEBYTECODE=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Python release tag and cold-cache regressions: %v\n%s", err, out)
	}
}
