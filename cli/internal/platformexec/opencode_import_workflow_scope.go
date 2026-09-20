package platformexec

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func newOpenCodeImportedState() opencodeImportedState {
	return opencodeImportedState{
		artifacts: map[string]pluginmodel.Artifact{},
	}
}

func importOpenCodeUserScope(state *opencodeImportedState, seed ImportSeed) error {
	cfg, ok, err := resolveOpenCodeUserScopeConfig(seed)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return importOpenCodeScope(state, cfg)
}

func importOpenCodeProjectScope(state *opencodeImportedState, root string) error {
	cfg, err := resolveOpenCodeProjectScopeConfig(root)
	if err != nil {
		return err
	}
	return importOpenCodeScope(state, cfg)
}

func requireOpenCodeImportedInput(state opencodeImportedState) error {
	if state.hasInput {
		return nil
	}
	return fmt.Errorf("OpenCode import requires opencode.json, opencode.jsonc, supported workspace directories, or --include-user-scope with OpenCode native sources")
}

type openCodeCompatSkillRoot struct {
	full    string
	display string
}
