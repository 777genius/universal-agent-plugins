package pluginkitairepo_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseSurface_CurrentGuidanceAndExecutableWorkflows(t *testing.T) {
	root := RepoRoot(t)
	workflowRoot := filepath.Join(root, ".github", "workflows")
	retired := []string{
		"release-assets.yml",
		"release-preflight.yml",
		"homebrew-tap.yml",
		"npm-publish.yml",
		"pypi-publish.yml",
	}
	for _, name := range retired {
		if _, err := os.Stat(filepath.Join(workflowRoot, name)); !os.IsNotExist(err) {
			t.Fatalf("retired plugin-kit-ai publisher %s remains executable or cannot be checked: %v", name, err)
		}
	}

	entries, err := os.ReadDir(workflowRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".yml") && !strings.HasSuffix(entry.Name(), ".yaml")) {
			continue
		}
		body := readRepoFile(t, root, ".github", "workflows", entry.Name())
		for _, retiredPublisher := range []string{
			"name: Release Assets\n",
			"name: Release Preflight\n",
			"name: Homebrew Tap\n",
			"name: NPM Publish\n",
			"name: PyPI Publish\n",
			"goreleaser/goreleaser-action@",
			"./scripts/update-homebrew-tap.sh",
		} {
			if strings.Contains(body, retiredPublisher) {
				t.Fatalf("executable workflow %s restores retired CLI publisher marker %q", entry.Name(), retiredPublisher)
			}
		}
	}

	for _, name := range []string{"agentplugins-release.yml", "agentplugins-npm-publish.yml"} {
		if info, err := os.Stat(filepath.Join(workflowRoot, name)); err != nil || info.Size() < 1000 {
			t.Fatalf("current agentplugins workflow %s is missing or not substantive: %v", name, err)
		}
	}

	for _, name := range []string{"npm-runtime-publish.yml", "pypi-runtime-publish.yml"} {
		body := readRepoFile(t, root, ".github", "workflows", name)
		mustContain(t, body, "workflow_dispatch:")
		mustNotContain(t, body, "workflow_run:")
		mustNotContain(t, body, "workflows: [\"Release Assets\"]")
		mustContain(t, body, "plugin-kit-ai-runtime")
	}

	releaseDoc := readRepoFile(t, root, "docs", "RELEASE.md")
	checklist := readRepoFile(t, root, "docs", "RELEASE_CHECKLIST.md")
	runbook := readRepoFile(t, root, "docs", "agentplugins-release.md")
	for _, body := range []string{releaseDoc, checklist, runbook} {
		mustNotContain(t, body, "pipx install plugin-kit-ai")
		mustNotContain(t, body, "npm i -g plugin-kit-ai")
	}
	mustContain(t, releaseDoc, "`agentplugins` is the sole public CLI")
	mustContain(t, releaseDoc, "No new `plugin-kit-ai` CLI GitHub, npm, PyPI, Homebrew, or native release is supported")
	mustContain(t, checklist, "dispatch `agentplugins-release.yml`")
	mustContain(t, checklist, "protected `npm-agentplugins`")
	mustContain(t, runbook, "Standalone `plugin-kit-ai` npm and PyPI publishing and all paired release modes are retired")
}

func TestReleaseSurface_RetiredImplementationsAndSourceArePreserved(t *testing.T) {
	root := RepoRoot(t)
	history := filepath.Join(root, "docs", "history", "plugin-kit-ai-release-workflows")
	readme := readRepoFile(t, root, "docs", "history", "plugin-kit-ai-release-workflows", "README.md")
	mustContain(t, readme, "historical source, not GitHub Actions entrypoints")
	mustContain(t, readme, "The sole public CLI is `agentplugins`")

	expectedMarkers := map[string]string{
		"release-assets.yml": "goreleaser/goreleaser-action@v7",
		"release-preflight.yml": "Check downstream publish prerequisites",
		"homebrew-tap.yml": "./scripts/update-homebrew-tap.sh",
		"npm-publish.yml": "npm publish --access public",
		"pypi-publish.yml": "pypa/gh-action-pypi-publish@release/v1",
	}
	for name, marker := range expectedMarkers {
		info, err := os.Stat(filepath.Join(history, name))
		if err != nil || info.Size() < 500 {
			t.Fatalf("preserved workflow %s is missing or not substantive: %v", name, err)
		}
		mustContain(t, readRepoFile(t, root, "docs", "history", "plugin-kit-ai-release-workflows", name), marker)
	}
	for _, path := range []string{
		"cli/plugin-kit-ai/cmd/plugin-kit-ai/main.go",
		"npm/plugin-kit-ai/lib/install.js",
		"python/plugin-kit-ai/src/plugin_kit_ai/install.py",
	} {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil || !info.Mode().IsRegular() || info.Size() < 100 {
			t.Fatalf("preserved implementation source %s is missing or not substantive: %v", path, err)
		}
	}
}

func readRepoFile(t *testing.T, root string, parts ...string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(append([]string{root}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(body), "\r\n", "\n")
}
