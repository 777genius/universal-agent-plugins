package platformexec

import "path/filepath"

func resolveOpenCodeProjectScopeConfig(root string) (opencodeScopeConfig, error) {
	if err := rejectOpenCodeCompatSkillRoots(openCodeProjectCompatSkillRoots(root)); err != nil {
		return opencodeScopeConfig{}, err
	}
	return opencodeScopeConfig{
		root:              root,
		displayConfigRoot: "",
		workspaceRoot:     filepath.Join(root, ".opencode"),
		workspaceDisplay:  ".opencode",
	}, nil
}

func openCodeProjectCompatSkillRoots(root string) []openCodeCompatSkillRoot {
	return []openCodeCompatSkillRoot{
		{full: filepath.Join(root, ".agents", "skills"), display: filepath.ToSlash(filepath.Join(".agents", "skills"))},
		{full: filepath.Join(root, ".claude", "skills"), display: filepath.ToSlash(filepath.Join(".claude", "skills"))},
	}
}
