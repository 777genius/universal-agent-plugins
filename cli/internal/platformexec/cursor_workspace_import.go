package platformexec

import (
	"fmt"
)

func (cursorWorkspaceAdapter) Import(root string, seed ImportSeed) (ImportResult, error) {
	result := ImportResult{Manifest: seed.Manifest}
	hasCursorState, err := appendCursorWorkspaceMCPArtifacts(root, seed.IncludeUserScope, &result)
	if err != nil {
		return ImportResult{}, err
	}

	ruleArtifacts, err := importCursorRuleArtifacts(root)
	if len(ruleArtifacts) > 0 {
		result.Artifacts = append(result.Artifacts, ruleArtifacts...)
		hasCursorState = true
	}
	if err != nil {
		return ImportResult{}, err
	}

	if err := ensureCursorRulesMigrationGuard(root); err != nil {
		return ImportResult{}, err
	}

	if agentsArtifact, ok, err := importCursorAgentsArtifact(root); err != nil {
		return ImportResult{}, err
	} else if ok {
		result.Artifacts = append(result.Artifacts, agentsArtifact)
		hasCursorState = true
	}

	if !hasCursorState {
		return ImportResult{}, fmt.Errorf("Cursor import requires .cursor/mcp.json, .cursor/rules/**, root AGENTS.md, or --include-user-scope with ~/.cursor/mcp.json")
	}
	result.Artifacts = compactArtifacts(result.Artifacts)
	return result, nil
}
