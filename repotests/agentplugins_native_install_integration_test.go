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

const nativeInstallerTestVersion = "9.8.7"

type nativeInstallerRelease struct {
	tag        string
	assetName  string
	asset      []byte
	checksums  string
	repository string
}

func TestAgentpluginsNativeInstallerInstallsLatestAndRunsCommand(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	requireShellBootstrap(t)
	if runtime.GOOS == "windows" {
		t.Skip("the POSIX installer is covered by the cross-platform workflow on Windows")
	}

	assetName := nativeInstallerAssetName(nativeInstallerTestVersion)
	asset := []byte(`#!/usr/bin/env sh
if [ "${1:-}" = version ]; then
  echo "agentplugins 9.8.7"
  exit 0
fi
printf 'native test args:'
for arg in "$@"; do printf ' [%s]' "$arg"; done
printf '\n'
`)
	digest := sha256.Sum256(asset)
	release := nativeInstallerRelease{
		tag:        "agentplugins-v" + nativeInstallerTestVersion,
		assetName:  assetName,
		asset:      asset,
		checksums:  fmt.Sprintf("%s  %s\n", hex.EncodeToString(digest[:]), assetName),
		repository: "example/agentplugins",
	}
	server := newNativeInstallerReleaseServer(t, release)
	t.Cleanup(server.Close)

	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	outputFile := filepath.Join(root, "installer.outputs")
	output := runNativeInstaller(t, server.URL, map[string]string{
		"AGENTPLUGINS_BIN_DIR":     binDir,
		"AGENTPLUGINS_OUTPUT_FILE": outputFile,
	}, "add", "context7", "--dry-run")

	installedPath := filepath.Join(binDir, "agentplugins")
	installed, err := os.ReadFile(installedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(installed) != string(asset) {
		t.Fatal("installed binary bytes differ from the verified release asset")
	}
	for _, expected := range []string{
		"Installed agentplugins " + nativeInstallerTestVersion,
		"Path: " + installedPath,
		"SHA-256: " + hex.EncodeToString(digest[:]),
		"native test args: [add] [context7] [--dry-run]",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("installer output missing %q:\n%s", expected, output)
		}
	}

	outputs, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"version=" + nativeInstallerTestVersion,
		"tag=agentplugins-v" + nativeInstallerTestVersion,
		"path=" + installedPath,
		"asset=" + assetName,
		"sha256=" + hex.EncodeToString(digest[:]),
	} {
		if !strings.Contains(string(outputs), expected) {
			t.Fatalf("installer output file missing %q:\n%s", expected, outputs)
		}
	}
}

func TestAgentpluginsNativeInstallerAcceptsExplicitVersion(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	requireShellBootstrap(t)
	if runtime.GOOS == "windows" {
		t.Skip("the POSIX installer is covered by the cross-platform workflow on Windows")
	}

	release := validNativeInstallerRelease(t)
	server := newNativeInstallerReleaseServer(t, release)
	t.Cleanup(server.Close)
	output := runNativeInstaller(t, server.URL, map[string]string{
		"AGENTPLUGINS_VERSION": release.tag,
		"AGENTPLUGINS_BIN_DIR": filepath.Join(t.TempDir(), "bin"),
	})
	if !strings.Contains(output, "Installed agentplugins "+nativeInstallerTestVersion) {
		t.Fatalf("explicit version was not installed:\n%s", output)
	}
}

func TestAgentpluginsNativeInstallerRejectsChecksumMismatchWithoutMutation(t *testing.T) {
	t.Parallel()
	requireBindTests(t)
	requireShellBootstrap(t)
	if runtime.GOOS == "windows" {
		t.Skip("the POSIX installer is covered by the cross-platform workflow on Windows")
	}

	release := validNativeInstallerRelease(t)
	release.checksums = strings.Repeat("0", 64) + "  " + release.assetName + "\n"
	server := newNativeInstallerReleaseServer(t, release)
	t.Cleanup(server.Close)
	binDir := filepath.Join(t.TempDir(), "bin")

	root := RepoRoot(t)
	cmd := exec.Command(shellPath(t), filepath.Join(root, "install.sh"))
	cmd.Env = nativeInstallerEnvironment(server.URL, map[string]string{
		"AGENTPLUGINS_VERSION": release.tag,
		"AGENTPLUGINS_BIN_DIR": binDir,
	})
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected checksum mismatch failure:\n%s", output)
	}
	if !strings.Contains(string(output), "checksum mismatch") {
		t.Fatalf("unexpected checksum mismatch output:\n%s", output)
	}
	if _, statErr := os.Stat(filepath.Join(binDir, "agentplugins")); !os.IsNotExist(statErr) {
		t.Fatalf("failed verification must not create the destination, stat error: %v", statErr)
	}
}

func TestAgentpluginsNativeInstallerRejectsInvalidVersionBeforeNetwork(t *testing.T) {
	t.Parallel()
	requireShellBootstrap(t)
	if runtime.GOOS == "windows" {
		t.Skip("the POSIX installer is covered by the cross-platform workflow on Windows")
	}

	root := RepoRoot(t)
	cmd := exec.Command(shellPath(t), filepath.Join(root, "install.sh"))
	cmd.Env = append(os.Environ(),
		"AGENTPLUGINS_VERSION=../../main",
		"AGENTPLUGINS_RELEASE_BASE_URL=http://127.0.0.1:1",
		"AGENTPLUGINS_BIN_DIR="+filepath.Join(t.TempDir(), "bin"),
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected invalid version failure:\n%s", output)
	}
	if !strings.Contains(string(output), "invalid stable version or tag") {
		t.Fatalf("invalid version was not rejected before download:\n%s", output)
	}
}

func validNativeInstallerRelease(t *testing.T) nativeInstallerRelease {
	t.Helper()
	assetName := nativeInstallerAssetName(nativeInstallerTestVersion)
	asset := []byte("#!/usr/bin/env sh\necho 'agentplugins 9.8.7'\n")
	digest := sha256.Sum256(asset)
	return nativeInstallerRelease{
		tag:        "agentplugins-v" + nativeInstallerTestVersion,
		assetName:  assetName,
		asset:      asset,
		checksums:  fmt.Sprintf("%s  %s\n", hex.EncodeToString(digest[:]), assetName),
		repository: "example/agentplugins",
	}
}

func nativeInstallerAssetName(version string) string {
	return fmt.Sprintf("agentplugins_%s_%s_%s", version, runtimeGOOSForScript(), runtimeGOARCHForScript())
}

func newNativeInstallerReleaseServer(t *testing.T, release nativeInstallerRelease) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case fmt.Sprintf("/repos/%s/releases/latest", release.repository):
			writer.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(writer).Encode(map[string]string{"tag_name": release.tag})
		case fmt.Sprintf("/%s/releases/download/%s/checksums.txt", release.repository, release.tag):
			_, _ = writer.Write([]byte(release.checksums))
		case fmt.Sprintf("/%s/releases/download/%s/%s", release.repository, release.tag, release.assetName):
			_, _ = writer.Write(release.asset)
		default:
			http.NotFound(writer, request)
		}
	}))
}

func runNativeInstaller(t *testing.T, serverURL string, environment map[string]string, command ...string) string {
	t.Helper()
	root := RepoRoot(t)
	arguments := []string{filepath.Join(root, "install.sh")}
	arguments = append(arguments, command...)
	cmd := exec.Command(shellPath(t), arguments...)
	cmd.Env = nativeInstallerEnvironment(serverURL, environment)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("native installer failed: %v\n%s", err, output)
	}
	return string(output)
}

func nativeInstallerEnvironment(serverURL string, overrides map[string]string) []string {
	environment := append(os.Environ(),
		"AGENTPLUGINS_REPOSITORY=example/agentplugins",
		"AGENTPLUGINS_API_BASE="+serverURL,
		"AGENTPLUGINS_RELEASE_BASE_URL="+serverURL,
	)
	for key, value := range overrides {
		environment = append(environment, key+"="+value)
	}
	return environment
}
