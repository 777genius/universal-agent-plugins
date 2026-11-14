package app

import (
	"os"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func TestDetectMaterializePackageRootActionReportsReplace(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ctx := publicationContext{dest: root, packageRoot: "plugins/demo"}
	if err := os.MkdirAll(ctx.destPackageRoot(), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := detectMaterializePackageRootAction(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != "replace" {
		t.Fatalf("action = %q", got)
	}
}

func TestBuildPublicationMaterializeResultReportsDrift(t *testing.T) {
	t.Parallel()

	result := buildPublicationMaterializeResult(publicationContext{
		root:        ".",
		target:      "codex-package",
		dest:        "/tmp/out",
		packageRoot: "plugins/demo",
		channel:     publicationChannelStub("codex-marketplace"),
	}, publicationMaterializePlan{
		packageFiles:       []pluginmanifest.Artifact{{RelPath: "plugins/demo/.codex-plugin/plugin.json"}},
		generated:          pluginmanifest.RenderResult{StalePaths: []string{"old"}},
		catalogArtifact:    pluginmanifest.Artifact{RelPath: ".agents/plugins/marketplace.json"},
		packageRootAction:  "create",
		catalogArtifactAct: "merge",
	}, true)
	if !strings.Contains(strings.Join(result.Lines, "\n"), "Source generate drift observed") {
		t.Fatalf("lines = %v", result.Lines)
	}
}
