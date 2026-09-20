package codexmanifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseInterfaceDocRejectsInvalidDefaultPromptShape(t *testing.T) {
	t.Parallel()
	if _, err := ParseInterfaceDoc([]byte(`{"defaultPrompt":"Run the demo"}`)); err == nil || !strings.Contains(err.Error(), "interface.defaultPrompt must be an array of strings") {
		t.Fatalf("ParseInterfaceDoc error = %v", err)
	}
	if _, err := ParseInterfaceDoc([]byte(`{"defaultPrompt":[""]}`)); err == nil || !strings.Contains(err.Error(), "interface.defaultPrompt[0] must not be empty") {
		t.Fatalf("ParseInterfaceDoc error = %v", err)
	}
}

func TestParseInterfaceDocRejectsNonObjectJSON(t *testing.T) {
	t.Parallel()
	if _, err := ParseInterfaceDoc([]byte(`["demo"]`)); err == nil || !strings.Contains(err.Error(), "Codex interface doc must be a JSON object") {
		t.Fatalf("ParseInterfaceDoc error = %v", err)
	}
}

func TestDecodeImportedPluginManifestRejectsInvalidInterfaceDefaultPrompt(t *testing.T) {
	t.Parallel()
	if _, err := DecodeImportedPluginManifest([]byte(`{"name":"demo","version":"0.1.0","description":"demo","interface":{"defaultPrompt":"Run the demo"}}`)); err == nil || !strings.Contains(err.Error(), "interface.defaultPrompt must be an array of strings") {
		t.Fatalf("DecodeImportedPluginManifest error = %v", err)
	}
}

func TestDecodeImportedPluginManifestRejectsLegacyShapes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "author string",
			body: `{"name":"demo","version":"0.1.0","description":"demo","author":"Example Maintainer"}`,
			want: "Codex plugin author must be a JSON object",
		},
		{
			name: "apps array",
			body: `{"name":"demo","version":"0.1.0","description":"demo","apps":["./.app.json"]}`,
			want: "Codex plugin apps must be a string",
		},
		{
			name: "interface non object",
			body: `{"name":"demo","version":"0.1.0","description":"demo","interface":["prompt"]}`,
			want: "Codex plugin interface must be a JSON object",
		},
		{
			name: "keywords non string",
			body: `{"name":"demo","version":"0.1.0","description":"demo","keywords":["codex",1]}`,
			want: "Codex plugin keywords[1] must be a string",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeImportedPluginManifest([]byte(tc.body)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("DecodeImportedPluginManifest error = %v", err)
			}
		})
	}
}

func TestDecodeImportedPluginManifestNormalizesMetadataAndPreservesExtra(t *testing.T) {
	t.Parallel()
	body := []byte(`{
		"name":"demo",
		"version":"0.1.0",
		"description":"demo",
		"author":{"name":" Example Maintainer ","email":" maintainer@example.com "},
		"homepage":" https://example.com ",
		"keywords":[" codex ",""," plugin "],
		"extraConfig":{"enabled":true}
	}`)

	got, err := DecodeImportedPluginManifest(body)
	if err != nil {
		t.Fatal(err)
	}
	if got.PackageMeta.Author == nil || got.PackageMeta.Author.Name != "Example Maintainer" || got.PackageMeta.Author.Email != "maintainer@example.com" {
		t.Fatalf("author = %#v", got.PackageMeta.Author)
	}
	if got.PackageMeta.Homepage != "https://example.com" {
		t.Fatalf("homepage = %q", got.PackageMeta.Homepage)
	}
	if len(got.PackageMeta.Keywords) != 2 || got.PackageMeta.Keywords[0] != "codex" || got.PackageMeta.Keywords[1] != "plugin" {
		t.Fatalf("keywords = %#v", got.PackageMeta.Keywords)
	}
	if got.Extra["extraConfig"] == nil {
		t.Fatalf("extra = %#v", got.Extra)
	}
}

func TestParseAppManifestDocRejectsNonObjectJSON(t *testing.T) {
	t.Parallel()
	if _, err := ParseAppManifestDoc([]byte(`["demo"]`)); err == nil || !strings.Contains(err.Error(), "Codex app manifest must be a JSON object") {
		t.Fatalf("ParseAppManifestDoc error = %v", err)
	}
}

func TestReadImportedPluginManifestRejectsUnexpectedPluginDirEntries(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, PluginDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, PluginDir, PluginFileName), []byte(`{"name":"demo","version":"0.1.0","description":"demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, PluginDir, "notes.txt"), []byte("unexpected"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := ReadImportedPluginManifest(root)
	if err == nil || !strings.Contains(err.Error(), filepath.ToSlash(filepath.Join(PluginDir, "notes.txt"))) {
		t.Fatalf("ReadImportedPluginManifest error = %v", err)
	}
}

func TestUnexpectedBundleSidecars(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, AppFileName), []byte(`{"name":"demo-app"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, MCPFileName), []byte(`{"docs":{"url":"https://example.com/mcp"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	paths := UnexpectedBundleSidecars(root, ImportedPluginManifest{})
	if len(paths) != 2 || paths[0] != AppManifestPath() || paths[1] != MCPManifestPath() {
		t.Fatalf("UnexpectedBundleSidecars = %#v", paths)
	}
	if got := UnexpectedBundleSidecars(root, ImportedPluginManifest{AppsRef: AppsRef, MCPServersRef: MCPServersRef}); len(got) != 0 {
		t.Fatalf("UnexpectedBundleSidecars with refs = %#v", got)
	}
}
