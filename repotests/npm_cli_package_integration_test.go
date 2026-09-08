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

func TestNPMCLIPackageContractFiles(t *testing.T) {
	t.Parallel()
	root := RepoRoot(t)

	tracked := exec.Command("git", "ls-files", "--error-unmatch", "npm/plugin-kit-ai/bin/plugin-kit-ai.js")
	tracked.Dir = root
	if out, err := tracked.CombinedOutput(); err != nil {
		t.Fatalf("npm launcher must be tracked in git: %v\n%s", err, out)
	}

	packageBody, err := os.ReadFile(filepath.Join(root, "npm", "plugin-kit-ai", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	packageJSON := string(packageBody)
	for _, want := range []string{
		`"name": "plugin-kit-ai"`,
		`"version": "0.0.0-development"`,
		`"license": "Apache-2.0"`,
		`"postinstall": "node ./lib/install.js"`,
		`"plugin-kit-ai": "bin/plugin-kit-ai.js"`,
		`"node": ">=18"`,
		`"access": "public"`,
	} {
		if !strings.Contains(packageJSON, want) {
			t.Fatalf("package.json missing %q:\n%s", want, packageJSON)
		}
	}

	binBody, err := os.ReadFile(filepath.Join(root, "npm", "plugin-kit-ai", "bin", "plugin-kit-ai.js"))
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, string(binBody), "#!/usr/bin/env node")
	mustContain(t, string(binBody), "ensureInstalled")

	installBody, err := os.ReadFile(filepath.Join(root, "npm", "plugin-kit-ai", "lib", "install.js"))
	if err != nil {
		t.Fatal(err)
	}
	installJS := string(installBody)
	mustContain(t, installJS, "777genius/plugin-kit-ai")
	mustContain(t, installJS, "checksums.txt")
	mustContain(t, installJS, "checksum mismatch")
	mustContain(t, installJS, "brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai")

	workflowBody, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "npm-publish.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(workflowBody)
	for _, want := range []string{
		"name: NPM Publish",
		"workflow_run:",
		"workflows: [\"Release Assets\"]",
		"NPM_TOKEN",
		"npm publish --access public",
		"checksums.txt",
		"plugin-kit-ai_${version}_windows_arm64.tar.gz",
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("npm publish workflow missing %q:\n%s", want, workflow)
		}
	}
}

func TestNPMCLIPackageInstallsLatestReleaseAndRunsBinary(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	requireNodeRuntime(t)
	requireNPMSmokePlatform(t)

	packageRoot := copyNPMPackageToTemp(t)
	binaryName := npmRuntimeBinaryName()
	assetName := fmt.Sprintf("plugin-kit-ai_1.2.3_%s_%s.tar.gz", runtimeGOOSForScript(), runtimeGOARCHForScript())
	archive := mustTarGz(t, binaryName, []byte("#!/usr/bin/env sh\nprintf 'version: v1.2.3\\n'\n"))
	sum := sha256.Sum256(archive)
	checksums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), assetName)

	srv := newNPMReleaseServer(t, npmReleaseConfig{
		tag:       "v1.2.3",
		assetName: assetName,
		checksums: checksums,
		archive:   archive,
	})
	t.Cleanup(srv.Close)

	cmd := exec.Command("node", "lib/install.js")
	cmd.Dir = packageRoot
	cmd.Env = append(os.Environ(),
		"GITHUB_API_BASE="+srv.URL,
		"PLUGIN_KIT_AI_RELEASE_BASE_URL="+srv.URL,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node install.js latest: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Version: v1.2.3") {
		t.Fatalf("install output missing version:\n%s", out)
	}

	run := exec.Command("node", "bin/plugin-kit-ai.js", "version")
	run.Dir = packageRoot
	run.Env = append(os.Environ(),
		"GITHUB_API_BASE="+srv.URL,
		"PLUGIN_KIT_AI_RELEASE_BASE_URL="+srv.URL,
	)
	runOut, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("node bin/plugin-kit-ai.js version: %v\n%s", err, runOut)
	}
	if !strings.Contains(string(runOut), "version: v1.2.3") {
		t.Fatalf("wrapper output missing version:\n%s", runOut)
	}
}

func TestNPMCLIPackageUsesPinnedPackageVersionWithoutLatestLookup(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	requireNodeRuntime(t)
	requireNPMSmokePlatform(t)

	packageRoot := copyNPMPackageToTemp(t)
	rewriteNPMPackageVersion(t, packageRoot, "1.2.4")

	binaryName := npmRuntimeBinaryName()
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

	cmd := exec.Command("node", "bin/plugin-kit-ai.js", "version")
	cmd.Dir = packageRoot
	cmd.Env = append(os.Environ(),
		"GITHUB_API_BASE="+srv.URL,
		"PLUGIN_KIT_AI_RELEASE_BASE_URL="+srv.URL,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node wrapper pinned package version: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "version: v1.2.4") {
		t.Fatalf("wrapper output missing pinned version:\n%s", out)
	}
}

func TestNPMCLIPackageRejectsChecksumMismatch(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	requireNodeRuntime(t)
	requireNPMSmokePlatform(t)

	packageRoot := copyNPMPackageToTemp(t)
	binaryName := npmRuntimeBinaryName()
	assetName := fmt.Sprintf("plugin-kit-ai_1.2.3_%s_%s.tar.gz", runtimeGOOSForScript(), runtimeGOARCHForScript())
	archive := mustTarGz(t, binaryName, []byte("#!/usr/bin/env sh\nprintf 'bad\\n'\n"))
	checksums := fmt.Sprintf("%s  %s\n", strings.Repeat("0", 64), assetName)

	srv := newNPMReleaseServer(t, npmReleaseConfig{
		tag:       "v1.2.3",
		assetName: assetName,
		checksums: checksums,
		archive:   archive,
	})
	t.Cleanup(srv.Close)

	cmd := exec.Command("node", "lib/install.js")
	cmd.Dir = packageRoot
	cmd.Env = append(os.Environ(),
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

type npmReleaseConfig struct {
	tag       string
	assetName string
	checksums string
	archive   []byte
}

func newNPMReleaseServer(t *testing.T, cfg npmReleaseConfig) *httptest.Server {
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

func copyNPMPackageToTemp(t *testing.T) string {
	t.Helper()
	root := RepoRoot(t)
	dst := filepath.Join(t.TempDir(), "plugin-kit-ai-npm")
	copyTree(t, filepath.Join(root, "npm", "plugin-kit-ai"), dst)
	return dst
}

func rewriteNPMPackageVersion(t *testing.T, packageRoot, version string) {
	t.Helper()
	path := filepath.Join(packageRoot, "package.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(body), `"version": "0.0.0-development"`, fmt.Sprintf(`"version": "%s"`, version), 1)
	if updated == string(body) {
		t.Fatalf("failed to rewrite package version in %s", path)
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
}

func requireNodeRuntime(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skipf("requires node in PATH: %v", err)
	}
}

func requireNPMSmokePlatform(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("npm wrapper smoke uses a Unix shell script payload in release tarballs")
	}
}

func npmRuntimeBinaryName() string {
	if runtime.GOOS == "windows" {
		return "plugin-kit-ai.exe"
	}
	return "plugin-kit-ai"
}
