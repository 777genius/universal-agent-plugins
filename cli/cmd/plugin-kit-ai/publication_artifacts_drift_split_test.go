package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnosePublicationStaleArtifactsFiltersNonPublicationPaths(t *testing.T) {
	t.Parallel()

	issues := diagnosePublicationStaleArtifacts([]string{
		filepath.Join(".agents", "plugins", "marketplace.json"),
		filepath.Join("notes", "readme.md"),
		"gemini-extension.json",
	})
	if len(issues) != 2 {
		t.Fatalf("issues = %+v", issues)
	}
	if issues[0].Path != filepath.ToSlash(filepath.Join(".agents", "plugins", "marketplace.json")) {
		t.Fatalf("issues = %+v", issues)
	}
	if issues[1].Path != "gemini-extension.json" {
		t.Fatalf("issues = %+v", issues)
	}
}

func TestDiagnosePublicationArtifactDriftSkipsMissingExpectedBody(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.ToSlash(filepath.Join(".codex-plugin", "plugin.json"))
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, path), []byte(`{"name":"demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if issue, ok := diagnosePublicationArtifactDrift(root, path, nil, "drifted_package_artifact"); ok {
		t.Fatalf("unexpected issue = %+v", issue)
	}
}

func TestDiagnosePublicationArtifactDriftReportsChangedBodies(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.ToSlash(filepath.Join(".codex-plugin", "plugin.json"))
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, path), []byte(`{"name":"current"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	issue, ok := diagnosePublicationArtifactDrift(root, path, []byte(`{"name":"expected"}`), "drifted_package_artifact")
	if !ok {
		t.Fatal("expected drift issue")
	}
	if !strings.Contains(issue.Message, "is out of sync with current authored inputs") {
		t.Fatalf("issue = %+v", issue)
	}
}
