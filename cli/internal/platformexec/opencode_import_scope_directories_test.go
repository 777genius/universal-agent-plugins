package platformexec

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestApplyOpenCodeToolArtifactsImportMarksInput(t *testing.T) {
	t.Parallel()
	state := newOpenCodeImportedState()
	applyOpenCodeToolArtifactsImport(&state, openCodeToolArtifactsImport{
		artifacts: []pluginmodel.Artifact{{RelPath: "plugin/targets/opencode/tools/demo.txt"}},
	})
	if !state.hasInput {
		t.Fatal("expected tool artifacts to mark input")
	}
}
