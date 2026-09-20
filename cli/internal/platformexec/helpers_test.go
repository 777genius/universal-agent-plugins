package platformexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveRelativeRefRejectsEscapingPaths(t *testing.T) {
	t.Parallel()
	for _, ref := range []string{"../outside.json", "..", "/tmp/outside.json"} {
		if _, err := resolveRelativeRef(t.TempDir(), ref); err == nil {
			t.Fatalf("resolveRelativeRef(%q) succeeded, want error", ref)
		}
	}
}

func TestResolveRelativeRefNormalizesLocalPaths(t *testing.T) {
	t.Parallel()
	got, err := resolveRelativeRef(t.TempDir(), "./nested/../sidecar.json")
	if err != nil {
		t.Fatal(err)
	}
	if got != "sidecar.json" {
		t.Fatalf("resolved ref = %q, want %q", got, "sidecar.json")
	}
}

func TestResolveOpenCodeConfigPathInDirPrefersJSONWithWarning(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{"opencode.json", "opencode.jsonc"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	path, warnings, ok, err := resolveOpenCodeConfigPathInDir(root, "imported")
	if err != nil {
		t.Fatalf("resolveOpenCodeConfigPathInDir: %v", err)
	}
	if !ok {
		t.Fatal("expected OpenCode config to be detected")
	}
	if want := filepath.Join(root, "opencode.json"); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %#v, want 1 warning", warnings)
	}
	if warnings[0].Path != "imported/opencode.jsonc" {
		t.Fatalf("warning path = %q, want %q", warnings[0].Path, "imported/opencode.jsonc")
	}
	if !strings.Contains(warnings[0].Message, "takes precedence") {
		t.Fatalf("warning message = %q", warnings[0].Message)
	}
}

func TestDecodeImportedOpenCodeConfigMergesLegacyModeWithoutOverwritingAgent(t *testing.T) {
	t.Parallel()
	body := []byte(`{
  "agent": {
    "kept": {"prompt": "primary"}
  },
  "mode": {
    "kept": {"prompt": "legacy"},
    "added": {"prompt": "secondary"}
  }
}`)

	config, err := decodeImportedOpenCodeConfig(body)
	if err != nil {
		t.Fatalf("decodeImportedOpenCodeConfig: %v", err)
	}
	if !config.AgentsProvided {
		t.Fatal("expected agents to be marked as provided")
	}
	if len(config.Agents) != 2 {
		t.Fatalf("agents len = %d, want 2: %#v", len(config.Agents), config.Agents)
	}
	kept, _ := config.Agents["kept"].(map[string]any)
	added, _ := config.Agents["added"].(map[string]any)
	if kept["prompt"] != "primary" {
		t.Fatalf("kept agent = %#v, want primary agent to win", kept)
	}
	if added["prompt"] != "secondary" {
		t.Fatalf("added agent = %#v, want legacy mode to fill missing agent", added)
	}
}

func TestDecodeImportedOpenCodeConfigRejectsInvalidTuplePluginRef(t *testing.T) {
	t.Parallel()
	body := []byte(`{
  "plugin": [
    ["@acme/demo", "invalid"]
  ]
}`)

	_, err := decodeImportedOpenCodeConfig(body)
	if err == nil {
		t.Fatal("expected invalid tuple plugin ref error")
	}
	if !strings.Contains(err.Error(), "tuple plugin ref options must be an object") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseMarkdownFrontmatterDocumentNormalizesBOMAndCRLF(t *testing.T) {
	t.Parallel()
	body := []byte("\ufeff---\r\ntitle: Demo\r\nkind: note\r\n---\r\n\r\nHello world\r\n")

	frontmatter, markdown, err := parseMarkdownFrontmatterDocument(body, "demo.md")
	if err != nil {
		t.Fatalf("parseMarkdownFrontmatterDocument: %v", err)
	}
	if frontmatter["title"] != "Demo" || frontmatter["kind"] != "note" {
		t.Fatalf("frontmatter = %#v", frontmatter)
	}
	if markdown != "Hello world" {
		t.Fatalf("markdown = %q, want %q", markdown, "Hello world")
	}
}
