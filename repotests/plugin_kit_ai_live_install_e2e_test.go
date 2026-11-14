package pluginkitairepo_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Live E2E: real GitHub API + real release assets (no httptest).
//
// Включается только явно — CI по умолчанию не ходит в сеть.
//
//	SHORT=1 go test ./...              — пропускает live-тесты
//	PLUGIN_KIT_AI_E2E_LIVE=1 go test ...    — качает с github.com
//
// Контракт для авторов плагинов (гибко, на выбор):
//   - Обязательно: checksums.txt на релизе + строка для ставимого файла.
//   - Вариант A: один *_GOOS_GOARCH.tar.gz (GoReleaser), в корне архива один бинарник.
//   - Вариант B: один сырой бинарник *-GOOS-GOARCH или .exe на Windows (как claude-notifications-go).
//   - Версия релиза: --tag T или --latest (последний стабильный, не prerelease).
//   - Имя на диске: как в релизе или --output-name.
//
// Опционально: GITHUB_TOKEN в окружении при rate limit анонимного API.
const (
	liveE2EEnvVar             = "PLUGIN_KIT_AI_E2E_LIVE"
	liveE2EPinnedTagEnv       = "PLUGIN_KIT_AI_E2E_NOTIFICATIONS_TAG" // default v1.34.0
	liveE2ETwitterballRepoEnv = "PLUGIN_KIT_AI_E2E_TARBALL_OWNER_REPO"
	liveE2ETwitterballTagEnv  = "PLUGIN_KIT_AI_E2E_TARBALL_TAG"
	liveE2ETwitterballBinEnv  = "PLUGIN_KIT_AI_E2E_TARBALL_BINARY"
	liveE2EUnsupportedRepoEnv = "PLUGIN_KIT_AI_E2E_UNSUPPORTED_OWNER_REPO"
	liveE2EUnsupportedTagEnv  = "PLUGIN_KIT_AI_E2E_UNSUPPORTED_TAG"
	liveE2EUnsupportedExitEnv = "PLUGIN_KIT_AI_E2E_UNSUPPORTED_EXPECT_EXIT"
	liveE2EUnsupportedNeedle  = "PLUGIN_KIT_AI_E2E_UNSUPPORTED_SUBSTRING"
	notificationsGoOwnerRepo  = "777genius/claude-notifications-go"
)

func skipUnlessLiveE2E(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("live E2E skipped: go test -short")
	}
	if os.Getenv(liveE2EEnvVar) != "1" {
		t.Skipf("live E2E skipped: set %s=1 (real GitHub, real download)", liveE2EEnvVar)
	}
}

func notificationsGoExpectedBinaryName() string {
	n := fmt.Sprintf("claude-notifications-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		n += ".exe"
	}
	return n
}

func assertBinaryRunnable(t *testing.T, binPath string) {
	t.Helper()
	if _, err := os.Stat(binPath); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binPath, "--version")
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s --version: %v\n%s", binPath, err, out)
	}
	lower := strings.ToLower(string(out))
	if !strings.Contains(lower, "version") && !strings.Contains(lower, "claude") {
		t.Fatalf("%s --version: unexpected output %q", binPath, strings.TrimSpace(string(out)))
	}
}

func TestLiveInstall_NotificationsGo_latest(t *testing.T) {
	skipUnlessLiveE2E(t)

	pluginKitAIBin := buildPluginKitAI(t)
	outDir := t.TempDir()

	code, out := runPluginKitAIInstall(t, pluginKitAIBin, "", notificationsGoOwnerRepo,
		"--latest", "--dir", outDir, "--force")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if !strings.Contains(string(out), "Release: ") || !strings.Contains(string(out), "Asset: ") || !strings.Contains(string(out), "Target: ") {
		t.Fatalf("missing install summary lines:\n%s", out)
	}

	binPath := filepath.Join(outDir, notificationsGoExpectedBinaryName())
	st, err := os.Stat(binPath)
	if err != nil {
		t.Fatal(err)
	}
	const minSize = 256 * 1024
	if st.Size() < minSize {
		t.Fatalf("binary too small (%d bytes); possible wrong asset or truncated download", st.Size())
	}

	assertBinaryRunnable(t, binPath)
}

func TestLiveInstall_NotificationsGo_pinnedTag(t *testing.T) {
	skipUnlessLiveE2E(t)

	tag := strings.TrimSpace(os.Getenv(liveE2EPinnedTagEnv))
	if tag == "" {
		tag = "v1.34.0"
	}

	pluginKitAIBin := buildPluginKitAI(t)
	outDir := t.TempDir()

	code, out := runPluginKitAIInstall(t, pluginKitAIBin, "", notificationsGoOwnerRepo,
		"--tag", tag, "--dir", outDir, "--force")
	if code != 0 {
		t.Fatalf("exit %d (tag=%q)\n%s", code, tag, out)
	}
	if !strings.Contains(string(out), "Release: "+tag+" (tag)") {
		t.Fatalf("missing release summary:\n%s", out)
	}

	binPath := filepath.Join(outDir, notificationsGoExpectedBinaryName())
	if _, err := os.Stat(binPath); err != nil {
		t.Fatal(err)
	}
	assertBinaryRunnable(t, binPath)
}

func TestLiveInstall_NotificationsGo_customOutputName(t *testing.T) {
	skipUnlessLiveE2E(t)

	wantName := "notify-e2e-bin"
	if runtime.GOOS == "windows" {
		wantName = "notify-e2e-bin.exe"
	}
	pluginKitAIBin := buildPluginKitAI(t)
	outDir := t.TempDir()

	code, out := runPluginKitAIInstall(t, pluginKitAIBin, "", notificationsGoOwnerRepo,
		"--latest", "--dir", outDir, "--force", "--output-name", wantName)
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if !strings.Contains(string(out), "Installed "+filepath.Join(outDir, wantName)) {
		t.Fatalf("missing installed-path summary:\n%s", out)
	}

	binPath := filepath.Join(outDir, wantName)
	if _, err := os.Stat(binPath); err != nil {
		t.Fatal(err)
	}

	assertBinaryRunnable(t, binPath)
}

func TestLiveInstall_ConfiguredTarballRelease(t *testing.T) {
	skipUnlessLiveE2E(t)

	ownerRepo := strings.TrimSpace(os.Getenv(liveE2ETwitterballRepoEnv))
	tag := strings.TrimSpace(os.Getenv(liveE2ETwitterballTagEnv))
	binaryName := strings.TrimSpace(os.Getenv(liveE2ETwitterballBinEnv))
	if ownerRepo == "" || tag == "" || binaryName == "" {
		t.Skipf("set %s, %s, and %s to exercise live tarball compatibility", liveE2ETwitterballRepoEnv, liveE2ETwitterballTagEnv, liveE2ETwitterballBinEnv)
	}

	pluginKitAIBin := buildPluginKitAI(t)
	outDir := t.TempDir()

	code, out := runPluginKitAIInstall(t, pluginKitAIBin, "", ownerRepo,
		"--tag", tag, "--dir", outDir, "--force")
	if code != 0 {
		t.Fatalf("exit %d (repo=%q tag=%q)\n%s", code, ownerRepo, tag, out)
	}
	if !strings.Contains(string(out), "Release: "+tag+" (tag)") || !strings.Contains(string(out), "Asset: ") {
		t.Fatalf("missing install summary:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(outDir, binaryName)); err != nil {
		t.Fatal(err)
	}
}

func TestLiveInstall_ConfiguredUnsupportedReleaseFailsCleanly(t *testing.T) {
	skipUnlessLiveE2E(t)

	ownerRepo := strings.TrimSpace(os.Getenv(liveE2EUnsupportedRepoEnv))
	tag := strings.TrimSpace(os.Getenv(liveE2EUnsupportedTagEnv))
	if ownerRepo == "" || tag == "" {
		t.Skipf("set %s and %s to exercise live unsupported-release compatibility", liveE2EUnsupportedRepoEnv, liveE2EUnsupportedTagEnv)
	}

	wantExit := "2"
	if v := strings.TrimSpace(os.Getenv(liveE2EUnsupportedExitEnv)); v != "" {
		wantExit = v
	}
	wantNeedle := strings.TrimSpace(os.Getenv(liveE2EUnsupportedNeedle))

	pluginKitAIBin := buildPluginKitAI(t)
	outDir := t.TempDir()

	code, out := runPluginKitAIInstall(t, pluginKitAIBin, "", ownerRepo,
		"--tag", tag, "--dir", outDir, "--force")
	if fmt.Sprint(code) != wantExit {
		t.Fatalf("want exit %s, got %d (repo=%q tag=%q)\n%s", wantExit, code, ownerRepo, tag, out)
	}
	if wantNeedle != "" && !strings.Contains(string(out), wantNeedle) {
		t.Fatalf("want diagnostic %q in output:\n%s", wantNeedle, out)
	}
}
