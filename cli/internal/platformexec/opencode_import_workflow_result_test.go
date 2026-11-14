package platformexec

import (
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestAppendOpenCodeImportedArtifactsUsesSortedKeys(t *testing.T) {
	t.Parallel()

	artifacts := appendOpenCodeImportedArtifacts(nil, opencodeImportedState{
		artifacts: map[string]pluginmodel.Artifact{
			"z": {RelPath: "z"},
			"a": {RelPath: "a"},
		},
	})
	if len(artifacts) != 2 {
		t.Fatalf("artifacts = %#v", artifacts)
	}
	if filepath.ToSlash(artifacts[0].RelPath) != "a" || filepath.ToSlash(artifacts[1].RelPath) != "z" {
		t.Fatalf("artifacts = %#v", artifacts)
	}
}
