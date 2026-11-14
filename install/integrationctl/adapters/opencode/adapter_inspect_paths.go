package opencode

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
)

func (a Adapter) inspectSurfacePaths(scope string, workspaceRoot string) (string, []string) {
	if strings.EqualFold(strings.TrimSpace(scope), "project") {
		root := a.effectiveProjectRoot(workspaceRoot)
		candidates := []string{
			filepath.Join(root, "opencode.json"),
			filepath.Join(root, "opencode.jsonc"),
		}
		return pathpolicy.PreferredExistingPath(candidates...), candidates
	}
	candidates := []string{
		filepath.Join(a.userHome(), ".config", "opencode", "opencode.json"),
		filepath.Join(a.userHome(), ".config", "opencode", "opencode.jsonc"),
		filepath.Join(a.userHome(), ".local", "share", "opencode", "opencode.jsonc"),
	}
	return pathpolicy.PreferredExistingPath(candidates...), candidates
}

func (a Adapter) managedConfigPaths() []string {
	if a.managedConfigPathsFn != nil {
		return a.managedConfigPathsFn(a.userHome())
	}
	return managedConfigPathsDefault(a.userHome())
}

func managedConfigPathsDefault(userHome string) []string {
	switch runtime.GOOS {
	case "darwin":
		userName := strings.TrimSpace(filepath.Base(userHome))
		return dedupeStrings([]string{
			"/Library/Application Support/opencode/opencode.json",
			"/Library/Application Support/opencode/opencode.jsonc",
			filepath.Join("/Library/Managed Preferences", userName, "ai.opencode.managed.plist"),
			"/Library/Managed Preferences/ai.opencode.managed.plist",
		})
	case "linux":
		return []string{
			"/etc/opencode/opencode.json",
			"/etc/opencode/opencode.jsonc",
		}
	case "windows":
		base := strings.TrimSpace(os.Getenv("ProgramData"))
		if base == "" {
			base = strings.TrimSpace(os.Getenv("ALLUSERSPROFILE"))
		}
		if base == "" {
			return nil
		}
		return []string{
			filepath.Join(base, "opencode", "opencode.json"),
			filepath.Join(base, "opencode", "opencode.jsonc"),
		}
	default:
		return nil
	}
}

func preferredExistingPaths(candidates ...string) []string {
	var out []string
	for _, path := range candidates {
		if fileExists(path) {
			out = append(out, path)
		}
	}
	return out
}
