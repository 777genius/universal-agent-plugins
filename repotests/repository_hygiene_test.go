package pluginkitairepo_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestRepositoryDoesNotTrackGeneratedScratchPlugins(t *testing.T) {
	root := RepoRoot(t)
	cmd := exec.Command("git", "-C", root, "ls-files")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git ls-files: %v\n%s", err, out)
	}

	for _, path := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if path == "" {
			continue
		}
		if strings.HasPrefix(path, "my-plugin/") {
			t.Fatalf("generated docs scratch plugin is tracked: %s", path)
		}
		if strings.HasPrefix(path, "cli/tmpcompat.") {
			t.Fatalf("temporary compatibility debug file is tracked: %s", path)
		}
		if isAuthoredPluginManifest(path) && !strings.HasPrefix(path, "examples/") {
			t.Fatalf("authored plugin manifest is tracked outside curated examples: %s", path)
		}
	}
}

func isAuthoredPluginManifest(path string) bool {
	return strings.HasSuffix(path, "/plugin.yaml") || strings.HasSuffix(path, "/launcher.yaml")
}
