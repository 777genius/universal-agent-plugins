package platformexec

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateClaudeLauncherContract(graph pluginmodel.PackageGraph, state pluginmodel.TargetState) []Diagnostic {
	if graph.Launcher != nil {
		return nil
	}
	authoredRoot := authoredRootHint(state, graph.Portable)
	if rel := claudePrimaryHookPath(state); rel != "" {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     rel,
			Target:   "claude",
			Message:  fmt.Sprintf("Claude hooks require %s/%s when %s/targets/claude/hooks/** is authored", authoredRoot, pluginmodel.LauncherFileName, authoredRoot),
		}}
	}
	if claudeHasPackageOnlySurface(graph, state) {
		return nil
	}
	return []Diagnostic{{
		Severity: SeverityFailure,
		Code:     CodeManifestInvalid,
		Path:     filepath.ToSlash(filepath.Join(authoredRoot, pluginmodel.FileName)),
		Target:   "claude",
		Message:  fmt.Sprintf("target claude without %s/launcher.yaml must author at least one package-only surface such as %s/mcp/servers.yaml, %s/skills/, %s/targets/claude/settings.json, %s/targets/claude/lsp.json, %s/targets/claude/user-config.json, %s/targets/claude/manifest.extra.json, or %s/targets/claude/commands/** and %s/targets/claude/agents/**", authoredRoot, authoredRoot, authoredRoot, authoredRoot, authoredRoot, authoredRoot, authoredRoot, authoredRoot, authoredRoot),
	}}
}

func validateClaudeHookComponents(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) []Diagnostic {
	if graph.Launcher == nil {
		return nil
	}
	var diagnostics []Diagnostic
	for _, rel := range state.ComponentPaths("hooks") {
		full := filepath.Join(root, rel)
		body, err := os.ReadFile(full)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "claude",
				Message:  fmt.Sprintf("Claude hooks file %s is not readable: %v", rel, err),
			})
			continue
		}
		mismatches, err := validateClaudeHookEntrypoints(body, graph.Launcher.Entrypoint)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "claude",
				Message:  fmt.Sprintf("Claude hooks file %s is invalid JSON: %v", rel, err),
			})
			continue
		}
		for _, mismatch := range mismatches {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeEntrypointMismatch,
				Path:     rel,
				Target:   "claude",
				Message:  mismatch,
			})
		}
	}
	return diagnostics
}
