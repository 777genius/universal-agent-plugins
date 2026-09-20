package scaffold

import "testing"

func TestFilesFor_GeminiGoExtrasStayPlatformScoped(t *testing.T) {
	t.Parallel()

	files := filesFor("gemini", RuntimeGo, true, false, false)
	if !containsTemplate(files, "targets.gemini.hooks.json.tmpl") {
		t.Fatalf("missing gemini hooks scaffold: %v", files)
	}
	if !containsTemplate(files, "goreleaser.yml.tmpl") {
		t.Fatalf("missing go extras scaffold: %v", files)
	}
	if containsTemplate(files, "bundle-release.workflow.yml.tmpl") {
		t.Fatalf("unexpected runtime bundle workflow in gemini go scaffold: %v", files)
	}
}

func TestFilesFor_CodexRuntimeSharedPackageSkipsVendoredHelper(t *testing.T) {
	t.Parallel()

	files := filesFor("codex-runtime", RuntimeNode, true, true, true)
	if containsTemplate(files, "node.plugin-runtime.ts.tmpl") {
		t.Fatalf("unexpected vendored runtime helper in shared package mode: %v", files)
	}
	if !containsTemplate(files, "bundle-release.workflow.yml.tmpl") {
		t.Fatalf("missing codex runtime release workflow: %v", files)
	}
}

func containsTemplate(files []TemplateFile, template string) bool {
	for _, file := range files {
		if file.Template == template {
			return true
		}
	}
	return false
}
