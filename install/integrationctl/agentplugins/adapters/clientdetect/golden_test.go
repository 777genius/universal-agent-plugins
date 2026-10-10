package clientdetect

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/internal/goldentest"
)

// TestDetectGoldenAcrossPlatforms freezes surface identity, surface order and
// the selected ConfigRoot/ExecutablePath of every client on the three supported
// operating systems. Detection moves behind the client registry in Part 3 and
// must produce the same evidence afterwards.
func TestDetectGoldenAcrossPlatforms(t *testing.T) {
	t.Parallel()
	for _, goos := range []string{"darwin", "linux", "windows"} {
		t.Run(goos, func(t *testing.T) {
			t.Parallel()
			home := t.TempDir()
			detector := goldenDetector(t, goos, home)
			clients, err := detector.Detect(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			golden := goldentest.Golden{Replace: []goldentest.Replacement{{From: home, To: "<home>"}}}
			golden.Assert(t, "detect_"+goos, clients)
		})
	}
}

// goldenDetector populates one synthetic home with an installation of every
// client, so each platform's golden file covers detected as well as absent
// surfaces rather than only the empty case.
func goldenDetector(t *testing.T, goos, home string) Detector {
	t.Helper()
	binaries := map[string]string{}
	for _, binary := range []string{"codex", "cursor", "copilot", "code", "kiro-cli", "kiro", "claude", "gemini", "opencode", "windsurf", "devin", "grok", "kimi"} {
		binaries[binary] = filepath.Join(home, "bin", binary)
	}
	detector := testDetector(home, binaries)
	detector.GOOS = goos
	detector.Environment = map[string]string{"XDG_CONFIG_HOME": filepath.Join(home, ".config")}

	makeDirs(t, home,
		".codex", ".cursor", ".copilot", ".kiro", ".claude", ".gemini", ".grok", ".kimi-code",
		filepath.Join(".config", "opencode"),
		filepath.Join(".codeium", "windsurf"),
		filepath.Join(".vscode", "extensions", "saoudrizwan.claude-dev"),
		filepath.Join(".cursor", "extensions", "saoudrizwan.claude-dev"),
	)
	editorRoot := platformEditorRoot(goos)
	makeDirs(t, home,
		filepath.Join(editorRoot, "Code", "User", "globalStorage", "saoudrizwan.claude-dev"),
		filepath.Join(editorRoot, "Cursor", "User", "globalStorage", "saoudrizwan.claude-dev"),
		filepath.Join(editorRoot, "Windsurf", "User"),
	)

	switch goos {
	case "darwin":
		makeDirs(t, home, applicationBundles("system-applications")...)
	case "windows":
		detector.Environment["APPDATA"] = filepath.Join(home, editorRoot)
		detector.Environment["LOCALAPPDATA"] = filepath.Join(home, "AppData", "Local")
		detector.WindowsProgramFiles = []string{filepath.Join(home, "Program Files")}
		makeFiles(t, home, windowsInstallations()...)
	case "linux":
		applications := filepath.Join(home, "share", "applications")
		detector.LinuxApplicationDirs = []string{applications}
		makeFiles(t, home, desktopEntries(filepath.Join("share", "applications"))...)
	}
	return detector
}

// platformEditorRoot is where VS Code family editors keep per-user settings.
func platformEditorRoot(goos string) string {
	switch goos {
	case "darwin":
		return filepath.Join("Library", "Application Support")
	case "windows":
		return filepath.Join("AppData", "Roaming")
	default:
		return ".config"
	}
}

func applicationBundles(root string) []string {
	names := []string{"Codex.app", "ChatGPT.app", "Cursor.app", "Visual Studio Code.app", "Kiro.app", "OpenCode.app", "Windsurf.app"}
	bundles := make([]string, 0, len(names))
	for _, name := range names {
		bundles = append(bundles, filepath.Join(root, name))
	}
	return bundles
}

// windowsInstallations deliberately splits across the per-user and the system
// location so both branches of windowsAppSurface are covered.
func windowsInstallations() []string {
	local := filepath.Join("AppData", "Local")
	return []string{
		filepath.Join(local, "Microsoft", "WindowsApps", "ChatGPT.exe"),
		filepath.Join(local, "Programs", "cursor", "Cursor.exe"),
		filepath.Join(local, "Programs", "Microsoft VS Code", "Code.exe"),
		filepath.Join(local, "Programs", "OpenCode", "OpenCode.exe"),
		filepath.Join(local, "Programs", "Windsurf", "Windsurf.exe"),
		filepath.Join("Program Files", "Kiro", "Kiro.exe"),
	}
}

func desktopEntries(root string) []string {
	names := []string{"codex.desktop", "chatgpt.desktop", "cursor.desktop", "code.desktop", "kiro.desktop", "opencode.desktop", "windsurf.desktop"}
	entries := make([]string, 0, len(names))
	for _, name := range names {
		entries = append(entries, filepath.Join(root, name))
	}
	return entries
}

func makeDirs(t *testing.T, home string, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Join(home, path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func makeFiles(t *testing.T, home string, paths ...string) {
	t.Helper()
	for _, path := range paths {
		full := filepath.Join(home, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("synthetic"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
