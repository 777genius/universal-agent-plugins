package app

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func TestResolveRemovePublicationContextInputUsesResolvedChannelAndPackageRoot(t *testing.T) {
	t.Parallel()
	ctx := publicationContext{
		target: "claude",
		graph:  publicationGraphStub("demo"),
		inspection: pluginmanifest.Inspection{
			Publication: publicationmodel.Model{
				Packages: []publicationmodel.Package{{
					Target: "claude",
				}},
				Channels: []publicationmodel.Channel{{
					Family:         "claude-marketplace",
					PackageTargets: []string{"claude"},
				}},
			},
		},
	}
	input, err := resolveRemovePublicationContextInput(ctx, "")
	if err != nil {
		t.Fatalf("resolveRemovePublicationContextInput: %v", err)
	}
	if input.packageRoot != "plugins/demo" {
		t.Fatalf("packageRoot = %q", input.packageRoot)
	}
	if input.channel.Family != "claude-marketplace" {
		t.Fatalf("channel = %#v", input.channel)
	}
}
