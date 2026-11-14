package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func TestDetectPublicationMaterializeActionsReturnsReplaceAndMerge(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ctx := publicationContext{dest: root, packageRoot: "plugins/demo"}
	if err := os.MkdirAll(ctx.destPackageRoot(), 0o755); err != nil {
		t.Fatal(err)
	}
	artifact := pluginmanifest.Artifact{RelPath: ".agents/plugins/marketplace.json"}
	full := filepath.Join(root, filepath.FromSlash(artifact.RelPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	packageAction, catalogAction, err := detectPublicationMaterializeActions(ctx, artifact)
	if err != nil {
		t.Fatalf("detectPublicationMaterializeActions: %v", err)
	}
	if packageAction != "replace" || catalogAction != "merge" {
		t.Fatalf("actions = %q/%q", packageAction, catalogAction)
	}
}
