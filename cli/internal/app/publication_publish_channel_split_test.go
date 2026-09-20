package app

import "testing"

func TestNewLocalPublishInputProjectsPublishOptions(t *testing.T) {
	t.Parallel()

	input, err := newLocalPublishInput(PluginPublishOptions{
		Root:        "/repo",
		Dest:        "/dest",
		PackageRoot: "plugins/demo",
		DryRun:      true,
	}, "codex-marketplace")
	if err != nil {
		t.Fatal(err)
	}
	if input.Root != "/repo" || input.Target != "codex-package" || input.Dest != "/dest" || input.PackageRoot != "plugins/demo" || !input.DryRun {
		t.Fatalf("input = %+v", input)
	}
}

func TestBuildLocalPublishResultPrependsChannelLine(t *testing.T) {
	t.Parallel()

	result := buildLocalPublishResult("codex-marketplace", PluginPublicationMaterializeResult{
		Target: "codex-package",
		Mode:   "dry-run",
		Lines:  []string{"Next: demo"},
	})
	if len(result.Lines) < 2 || result.Lines[0] != "Publish channel: codex-marketplace" {
		t.Fatalf("lines = %v", result.Lines)
	}
}
