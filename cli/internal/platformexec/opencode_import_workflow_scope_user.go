package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
)

func resolveOpenCodeUserScopeConfig(seed ImportSeed) (opencodeScopeConfig, bool, error) {
	if !seed.IncludeUserScope {
		return opencodeScopeConfig{}, false, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return opencodeScopeConfig{}, false, fmt.Errorf("resolve user home for OpenCode import: %w", err)
	}
	if err := rejectOpenCodeCompatSkillRoots(openCodeUserCompatSkillRoots(home)); err != nil {
		return opencodeScopeConfig{}, false, err
	}
	globalRoot := filepath.Join(home, ".config", "opencode")
	return opencodeScopeConfig{
		root:              globalRoot,
		displayConfigRoot: filepath.ToSlash(filepath.Join("~", ".config", "opencode")),
		workspaceRoot:     globalRoot,
		workspaceDisplay:  filepath.ToSlash(filepath.Join("~", ".config", "opencode")),
	}, true, nil
}

func openCodeUserCompatSkillRoots(home string) []openCodeCompatSkillRoot {
	return []openCodeCompatSkillRoot{
		{full: filepath.Join(home, ".agents", "skills"), display: filepath.ToSlash(filepath.Join("~", ".agents", "skills"))},
		{full: filepath.Join(home, ".claude", "skills"), display: filepath.ToSlash(filepath.Join("~", ".claude", "skills"))},
	}
}
