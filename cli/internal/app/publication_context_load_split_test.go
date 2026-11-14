package app

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func TestRequireMaterializePublicationChannelDefaultsAuthoredRoot(t *testing.T) {
	t.Parallel()
	_, err := requireMaterializePublicationChannel(publicationContext{target: "claude"}, publicationmodel.Model{})
	if err == nil || !strings.Contains(err.Error(), "plugin/publish/...") {
		t.Fatalf("error = %v", err)
	}
}

func TestResolvePublicationContextPackageRootUsesManifestName(t *testing.T) {
	t.Parallel()
	ctx := publicationContext{
		graph: publicationGraphStub("demo"),
	}
	got, err := resolvePublicationContextPackageRoot(ctx, "")
	if err != nil {
		t.Fatalf("resolvePublicationContextPackageRoot: %v", err)
	}
	if got != "plugins/demo" {
		t.Fatalf("packageRoot = %q", got)
	}
}
