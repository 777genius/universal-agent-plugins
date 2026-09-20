package platformexec

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestApplyOpenCodeScopeConfigImportMarksInputWhenConfigPresent(t *testing.T) {
	t.Parallel()
	state := newOpenCodeImportedState()
	err := applyOpenCodeScopeConfigImport(&state, openCodeScopeConfigImport{
		importedConfig: importedOpenCodeConfig{},
		ok:             true,
	})
	if err != nil {
		t.Fatalf("applyOpenCodeScopeConfigImport: %v", err)
	}
	if !state.hasInput {
		t.Fatal("expected config import to mark input present")
	}
}

func TestApplyOpenCodeScopeConfigImportAppendsWarnings(t *testing.T) {
	t.Parallel()
	state := newOpenCodeImportedState()
	err := applyOpenCodeScopeConfigImport(&state, openCodeScopeConfigImport{
		warnings: []pluginmodel.Warning{{
			Path: "opencode.jsonc",
		}},
	})
	if err != nil {
		t.Fatalf("applyOpenCodeScopeConfigImport: %v", err)
	}
	if len(state.warnings) != 1 || state.warnings[0].Path != "opencode.jsonc" {
		t.Fatalf("warnings = %#v", state.warnings)
	}
}
