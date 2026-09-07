package pluginkitairepo_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Explicit disposable boundaries are required before any native client launch.
func nativeDisposableAllowed(goos, linux, hosted, actions, environment string) bool {
	if goos == "linux" {
		return linux == "1"
	}
	return (goos == "darwin" || goos == "windows") && hosted == "1" && actions == "true" && environment == "github-hosted"
}
func nativeRequireDisposable(t *testing.T) {
	t.Helper()
	if !nativeDisposableAllowed(runtime.GOOS, os.Getenv("AGENTPLUGINS_NATIVE_DISPOSABLE_LINUX"), os.Getenv("AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED"), os.Getenv("GITHUB_ACTIONS"), os.Getenv("RUNNER_ENVIRONMENT")) {
		t.Fatal("native execution requires an opted-in disposable Linux image or explicitly opted-in GitHub-hosted macOS/Windows runner")
	}
}
func nativeExecutableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// No inherited profile, credentials, proxy, PATH or shell configuration.
func nativePlatformEnvironment(root, home string, binDirs ...string) []string {
	env := []string{"HOME=" + home, "USERPROFILE=" + home, "APPDATA=" + filepath.Join(home, "AppData", "Roaming"), "LOCALAPPDATA=" + filepath.Join(home, "AppData", "Local"), "TMPDIR=" + filepath.Join(root, "tmp"), "TEMP=" + filepath.Join(root, "tmp"), "TMP=" + filepath.Join(root, "tmp"), "XDG_CONFIG_HOME=" + filepath.Join(root, "xdg-config"), "XDG_DATA_HOME=" + filepath.Join(root, "xdg-data"), "XDG_CACHE_HOME=" + filepath.Join(root, "xdg-cache"), "XDG_STATE_HOME=" + filepath.Join(root, "xdg-state"), "LANG=en_US.UTF-8", "TERM=dumb"}
	paths := append([]string{}, binDirs...)
	if runtime.GOOS == "windows" {
		// Derive the shell from the OS directory, never inherit an arbitrary COMSPEC.
		if nativeDisposableAllowed(runtime.GOOS, "", os.Getenv("AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED"), os.Getenv("GITHUB_ACTIONS"), os.Getenv("RUNNER_ENVIRONMENT")) {
			if gitDir := os.Getenv("AGENTPLUGINS_NATIVE_GIT_BIN_DIR"); filepath.IsAbs(gitDir) {
				paths = append(paths, gitDir)
			}
			if bash := os.Getenv("AGENTPLUGINS_NATIVE_GIT_BASH_PATH"); filepath.IsAbs(bash) {
				env = append(env, "CLAUDE_CODE_GIT_BASH_PATH="+bash)
			}
		}
		systemRoot := os.Getenv("SystemRoot")
		if filepath.IsAbs(systemRoot) && !strings.HasPrefix(systemRoot, `\\`) {
			env = append(env, "SystemRoot="+systemRoot, "COMSPEC="+filepath.Join(systemRoot, "System32", "cmd.exe"))
			paths = append(paths, filepath.Join(systemRoot, "System32"), systemRoot)
		}
	} else {
		paths = append(paths, "/usr/bin", "/bin")
	}
	return append(env, "PATH="+strings.Join(paths, string(os.PathListSeparator)))
}
func TestNativeDisposableBoundary(t *testing.T) {
	for _, goos := range []string{"darwin", "windows"} {
		if !nativeDisposableAllowed(goos, "", "1", "true", "github-hosted") {
			t.Fatal(goos)
		}
		for _, args := range [][4]string{{"1", "", "true", "github-hosted"}, {"", "1", "false", "github-hosted"}, {"", "1", "true", "self-hosted"}} {
			if nativeDisposableAllowed(goos, args[0], args[1], args[2], args[3]) {
				t.Fatalf("unsafe boundary: %s %v", goos, args)
			}
		}
	}
	if !nativeDisposableAllowed("linux", "1", "", "", "") || nativeDisposableAllowed("linux", "", "1", "true", "github-hosted") {
		t.Fatal("Linux boundary changed")
	}
}

func TestNativePlatformEnvironmentIsolation(t *testing.T) {
	t.Setenv("HOME", "ambient-home")
	t.Setenv("PATH", "ambient-path")
	t.Setenv("ANTHROPIC_API_KEY", "ambient-secret")
	t.Setenv("HTTPS_PROXY", "ambient-proxy")
	root := t.TempDir()
	home := filepath.Join(root, "home")
	env := map[string]string{}
	for _, entry := range nativePlatformEnvironment(root, home, filepath.Join(root, "bin")) {
		key, value, _ := strings.Cut(entry, "=")
		if _, exists := env[key]; exists {
			t.Fatalf("duplicate environment key: %s", key)
		}
		env[key] = value
	}
	for _, key := range []string{"HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "TMPDIR", "TEMP", "TMP", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"} {
		relative, err := filepath.Rel(root, env[key])
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			t.Fatalf("%s escapes scratch: %q", key, env[key])
		}
	}
	if env["HOME"] != home || env["ANTHROPIC_API_KEY"] != "" || env["HTTPS_PROXY"] != "" || strings.Contains(env["PATH"], "ambient") {
		t.Fatal("inherited ambient environment")
	}
	if paths := filepath.SplitList(env["PATH"]); len(paths) == 0 || paths[0] != filepath.Join(root, "bin") {
		t.Fatal("scratch binary directory missing")
	}
}
