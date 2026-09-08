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

func TestPluginKitAIBootstrapScriptInstallsLatestRelease(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	requireShellBootstrap(t)

	binaryName := "plugin-kit-ai"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	assetName := fmt.Sprintf("plugin-kit-ai_1.2.3_%s_%s.tar.gz", runtimeGOOSForScript(), runtimeGOARCHForScript())
	archive := mustTarGz(t, binaryName, []byte("plugin-kit-ai-binary"))
	sum := sha256.Sum256(archive)
	checksums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), assetName)

	srv := newBootstrapReleaseServer(t, bootstrapReleaseConfig{
		tag:       "v1.2.3",
		assetName: assetName,
		checksums: checksums,
		archive:   archive,
	})
	t.Cleanup(srv.Close)

	binDir := filepath.Join(t.TempDir(), "bin")
	outputFile := filepath.Join(t.TempDir(), "install.outputs")
	out := runBootstrapScript(t, srv.URL, map[string]string{
		"BIN_DIR":                        binDir,
		"PLUGIN_KIT_AI_OUTPUT_FILE":      outputFile,
		"GITHUB_API_BASE":                srv.URL,
		"PLUGIN_KIT_AI_RELEASE_BASE_URL": srv.URL,
	})

	installedPath := filepath.Join(binDir, binaryName)
	body, err := os.ReadFile(installedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "plugin-kit-ai-binary" {
		t.Fatalf("installed binary = %q", body)
	}
	for _, want := range []string{
		"Installed plugin-kit-ai",
		"Version: v1.2.3",
		"Repository: 777genius/plugin-kit-ai",
		"Asset: " + assetName,
		"Installed path: " + installedPath,
		"Checksum: verified via checksums.txt",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("install output missing %q:\n%s", want, out)
		}
	}
	outputBody, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"version=v1.2.3",
		"path=" + installedPath,
		"bin_dir=" + binDir,
		"asset=" + assetName,
	} {
		if !strings.Contains(string(outputBody), want) {
			t.Fatalf("output file missing %q:\n%s", want, outputBody)
		}
	}
}

func TestPluginKitAIBootstrapScriptSupportsExplicitVersion(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	requireShellBootstrap(t)

	binaryName := "plugin-kit-ai"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	assetName := fmt.Sprintf("plugin-kit-ai_9.9.9_%s_%s.tar.gz", runtimeGOOSForScript(), runtimeGOARCHForScript())
	archive := mustTarGz(t, binaryName, []byte("explicit-version"))
	sum := sha256.Sum256(archive)
	checksums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), assetName)

	srv := newBootstrapReleaseServer(t, bootstrapReleaseConfig{
		tag:       "v9.9.9",
		assetName: assetName,
		checksums: checksums,
		archive:   archive,
	})
	t.Cleanup(srv.Close)

	binDir := filepath.Join(t.TempDir(), "bin")
	out := runBootstrapScript(t, srv.URL, map[string]string{
		"BIN_DIR":                        binDir,
		"VERSION":                        "9.9.9",
		"GITHUB_API_BASE":                srv.URL,
		"PLUGIN_KIT_AI_RELEASE_BASE_URL": srv.URL,
	})
	if !strings.Contains(out, "Version: v9.9.9") {
		t.Fatalf("expected explicit version in output:\n%s", out)
	}
}

func TestPluginKitAIBootstrapScriptRunsArgumentsAfterInstall(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	requireShellBootstrap(t)
	if runtime.GOOS == "windows" {
		t.Skip("mock pass-through binary is a POSIX shell script")
	}

	binaryName := "plugin-kit-ai"
	assetName := fmt.Sprintf("plugin-kit-ai_1.2.3_%s_%s.tar.gz", runtimeGOOSForScript(), runtimeGOARCHForScript())
	archive := mustTarGz(t, binaryName, []byte("#!/usr/bin/env sh\nprintf 'fake plugin-kit-ai args:'\nfor arg in \"$@\"; do printf ' [%s]' \"$arg\"; done\nprintf '\\n'\n"))
	sum := sha256.Sum256(archive)
	checksums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), assetName)

	srv := newBootstrapReleaseServer(t, bootstrapReleaseConfig{
		tag:       "v1.2.3",
		assetName: assetName,
		checksums: checksums,
		archive:   archive,
	})
	t.Cleanup(srv.Close)

	binDir := filepath.Join(t.TempDir(), "bin")
	out := runBootstrapScript(t, srv.URL, map[string]string{
		"BIN_DIR": binDir,
	}, "install", "owner/repo", "--latest")

	for _, want := range []string{
		"Installed plugin-kit-ai",
		"Running plugin-kit-ai install owner/repo --latest",
		"fake plugin-kit-ai args: [install] [owner/repo] [--latest]",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("pass-through output missing %q:\n%s", want, out)
		}
	}
}

func TestPluginKitAIBootstrapScriptRejectsChecksumMismatch(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	requireShellBootstrap(t)

	binaryName := "plugin-kit-ai"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	assetName := fmt.Sprintf("plugin-kit-ai_1.2.3_%s_%s.tar.gz", runtimeGOOSForScript(), runtimeGOARCHForScript())
	archive := mustTarGz(t, binaryName, []byte("bad-sum"))
	checksums := fmt.Sprintf("%s  %s\n", strings.Repeat("0", 64), assetName)

	srv := newBootstrapReleaseServer(t, bootstrapReleaseConfig{
		tag:       "v1.2.3",
		assetName: assetName,
		checksums: checksums,
		archive:   archive,
	})
	t.Cleanup(srv.Close)

	root := RepoRoot(t)
	cmd := exec.Command(shellPath(t), filepath.Join(root, "scripts", "install.sh"))
	cmd.Env = append(os.Environ(),
		"BIN_DIR="+filepath.Join(t.TempDir(), "bin"),
		"GITHUB_API_BASE="+srv.URL,
		"PLUGIN_KIT_AI_RELEASE_BASE_URL="+srv.URL,
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected checksum mismatch failure:\n%s", out)
	}
	if !strings.Contains(string(out), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch output:\n%s", out)
	}
}

func TestSetupPluginKitAIActionUsesInstallScript(t *testing.T) {
	t.Parallel()
	root := RepoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "setup-plugin-kit-ai", "action.yml"))
	if err != nil {
		t.Fatal(err)
	}
	action := string(body)
	for _, want := range []string{
		"name: setup-plugin-kit-ai",
		"VERSION: ${{ inputs.version }}",
		"GITHUB_TOKEN: ${{ inputs.github-token }}",
		"GITHUB_API_BASE: ${{ inputs.github-api-base }}",
		"BIN_DIR: ${{ runner.temp }}/plugin-kit-ai-bin",
		"\"$GITHUB_ACTION_PATH/../scripts/install.sh\"",
		"cat \"$PLUGIN_KIT_AI_OUTPUT_FILE\" >> \"$GITHUB_OUTPUT\"",
		"echo \"$bin_dir\" >> \"$GITHUB_PATH\"",
	} {
		if !strings.Contains(action, want) {
			t.Fatalf("setup action missing %q:\n%s", want, action)
		}
	}
}

func TestPluginKitAIInitExtrasPythonEmitsBundleReleaseWorkflow(t *testing.T) {
	t.Parallel()
	pluginKitAIBin := buildPluginKitAI(t)
	outDir := filepath.Join(t.TempDir(), "pyplug")
	cmd := exec.Command(pluginKitAIBin, "init", "pyplug", "--platform", "codex-runtime", "--runtime", "python", "--extras", "--output", outDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("init python extras: %v\n%s", err, out)
	}
	body, err := os.ReadFile(filepath.Join(outDir, ".github", "workflows", "bundle-release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(body)
	for _, want := range []string{
		"actions/setup-python@v5",
		"777genius/universal-agent-plugins/setup-plugin-kit-ai@v1",
		"plugin-kit-ai doctor .",
		"plugin-kit-ai bootstrap .",
		"plugin-kit-ai validate . --platform codex-runtime --strict",
		"plugin-kit-ai bundle publish . --platform codex-runtime --repo ${{ github.repository }} --tag ${{ github.ref_name }}",
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("python workflow missing %q:\n%s", want, workflow)
		}
	}
	if strings.Contains(workflow, "plugin-kit-ai install") {
		t.Fatalf("workflow should not use binary install:\n%s", workflow)
	}
}

func TestPluginKitAIInitExtrasNodeTypeScriptEmitsBundleReleaseWorkflow(t *testing.T) {
	t.Parallel()
	pluginKitAIBin := buildPluginKitAI(t)
	outDir := filepath.Join(t.TempDir(), "tsplug")
	cmd := exec.Command(pluginKitAIBin, "init", "tsplug", "--platform", "claude", "--runtime", "node", "--typescript", "--extras", "--output", outDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("init node extras: %v\n%s", err, out)
	}
	body, err := os.ReadFile(filepath.Join(outDir, ".github", "workflows", "bundle-release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(body)
	for _, want := range []string{
		"actions/setup-node@v6",
		"777genius/universal-agent-plugins/setup-plugin-kit-ai@v1",
		"plugin-kit-ai doctor .",
		"plugin-kit-ai bootstrap .",
		"plugin-kit-ai validate . --platform claude --strict",
		"plugin-kit-ai bundle publish . --platform claude --repo ${{ github.repository }} --tag ${{ github.ref_name }}",
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("node workflow missing %q:\n%s", want, workflow)
		}
	}
	if strings.Contains(workflow, "plugin-kit-ai install") {
		t.Fatalf("workflow should not use binary install:\n%s", workflow)
	}
}

type bootstrapReleaseConfig struct {
	tag       string
	assetName string
	checksums string
	archive   []byte
}

func newBootstrapReleaseServer(t *testing.T, cfg bootstrapReleaseConfig) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/777genius/plugin-kit-ai/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name": cfg.tag,
			})
		case fmt.Sprintf("/777genius/plugin-kit-ai/releases/download/%s/checksums.txt", cfg.tag):
			_, _ = w.Write([]byte(cfg.checksums))
		case fmt.Sprintf("/777genius/plugin-kit-ai/releases/download/%s/%s", cfg.tag, cfg.assetName):
			_, _ = w.Write(cfg.archive)
		default:
			http.NotFound(w, r)
		}
	}))
	return srv
}

func runBootstrapScript(t *testing.T, serverURL string, env map[string]string, passthroughArgs ...string) string {
	t.Helper()
	root := RepoRoot(t)
	args := []string{shellPath(t), filepath.Join(root, "scripts", "install.sh")}
	args = append(args, passthroughArgs...)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = append(os.Environ(),
		"PLUGIN_KIT_AI_REPOSITORY=777genius/plugin-kit-ai",
		"GITHUB_API_BASE="+serverURL,
		"PLUGIN_KIT_AI_RELEASE_BASE_URL="+serverURL,
	)
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bootstrap script: %v\n%s", err, out)
	}
	return string(out)
}

func requireShellBootstrap(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath(shellPath(t)); err != nil {
		t.Skipf("requires %s in PATH: %v", shellPath(t), err)
	}
}

func shellPath(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return "bash"
	}
	return "sh"
}

func runtimeGOOSForScript() string {
	if runtime.GOOS == "windows" {
		return "windows"
	}
	return runtime.GOOS
}

func runtimeGOARCHForScript() string {
	switch runtime.GOARCH {
	case "amd64":
		return "amd64"
	case "arm64":
		return "arm64"
	default:
		return runtime.GOARCH
	}
}
