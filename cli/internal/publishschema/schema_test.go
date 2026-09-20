package publishschema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscover_ParsesPublicationSchemas(t *testing.T) {
	root := t.TempDir()
	mustWritePublishFile(t, root, CodexMarketplaceRel, "api_version: v1\nmarketplace_name: local-repo\ncategory: Productivity\n")
	mustWritePublishFile(t, root, ClaudeMarketplaceRel, "api_version: v1\nmarketplace_name: acme-tools\nowner_name: ACME Team\nsource_root: ./\n")
	mustWritePublishFile(t, root, GeminiGalleryRel, "api_version: v1\ndistribution: github_release\nrepository_visibility: public\ngithub_topic: gemini-cli-extension\nmanifest_root: release_archive_root\n")

	state, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if state.Codex == nil || state.Codex.InstallationPolicy != "AVAILABLE" || state.Codex.AuthenticationPolicy != "ON_INSTALL" {
		t.Fatalf("codex = %+v", state.Codex)
	}
	if state.Claude == nil || state.Claude.SourceRoot != "./" {
		t.Fatalf("claude = %+v", state.Claude)
	}
	if state.Gemini == nil || state.Gemini.Distribution != "github_release" {
		t.Fatalf("gemini = %+v", state.Gemini)
	}
	if paths := state.Paths(); len(paths) != 3 {
		t.Fatalf("paths = %+v", paths)
	}
}

func TestDiscover_RejectsInvalidPublicationSchema(t *testing.T) {
	root := t.TempDir()
	mustWritePublishFile(t, root, CodexMarketplaceRel, "api_version: v1\nmarketplace_name: local repo\ncategory: Productivity\n")
	_, err := Discover(root)
	if err == nil || !strings.Contains(err.Error(), "invalid marketplace_name") {
		t.Fatalf("Discover error = %v", err)
	}

	root = t.TempDir()
	mustWritePublishFile(t, root, GeminiGalleryRel, "api_version: v1\nrepository_visibility: private\n")
	_, err = Discover(root)
	if err == nil || !strings.Contains(err.Error(), "repository_visibility must be") {
		t.Fatalf("Discover error = %v", err)
	}

	root = t.TempDir()
	mustWritePublishFile(t, root, GeminiGalleryRel, "api_version: v1\ngithub_topic: custom-topic\n")
	_, err = Discover(root)
	if err == nil || !strings.Contains(err.Error(), `github_topic must be "gemini-cli-extension"`) {
		t.Fatalf("Discover error = %v", err)
	}

	root = t.TempDir()
	mustWritePublishFile(t, root, GeminiGalleryRel, "api_version: v1\ndistribution: git_repository\nmanifest_root: release_archive_root\n")
	_, err = Discover(root)
	if err == nil || !strings.Contains(err.Error(), `manifest_root must be "repository_root" when distribution is "git_repository"`) {
		t.Fatalf("Discover error = %v", err)
	}
}

func TestValidateTargets_RejectsMissingPublicationTarget(t *testing.T) {
	root := t.TempDir()
	mustWritePublishFile(t, root, CodexMarketplaceRel, "api_version: v1\nmarketplace_name: local-repo\ncategory: Productivity\n")
	state, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.ValidateTargets([]string{"gemini"}); err == nil || !strings.Contains(err.Error(), `requires target "codex-package"`) {
		t.Fatalf("ValidateTargets error = %v", err)
	}
}

func TestDiscoverInLayoutPrefixesPublicationPaths(t *testing.T) {
	root := t.TempDir()
	authoredRoot := "plugin"
	mustWritePublishFile(t, root, filepath.Join(authoredRoot, CodexMarketplaceRel), "api_version: v1\nmarketplace_name: local-repo\ncategory: Productivity\n")
	mustWritePublishFile(t, root, filepath.Join(authoredRoot, ClaudeMarketplaceRel), "api_version: v1\nmarketplace_name: acme-tools\nowner_name: ACME Team\n")

	state, err := DiscoverInLayout(root, authoredRoot)
	if err != nil {
		t.Fatal(err)
	}
	if state.Codex == nil || state.Codex.Path != filepath.ToSlash(filepath.Join(authoredRoot, CodexMarketplaceRel)) {
		t.Fatalf("codex path = %+v", state.Codex)
	}
	if state.Claude == nil || state.Claude.Path != filepath.ToSlash(filepath.Join(authoredRoot, ClaudeMarketplaceRel)) {
		t.Fatalf("claude path = %+v", state.Claude)
	}
}

func mustWritePublishFile(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
