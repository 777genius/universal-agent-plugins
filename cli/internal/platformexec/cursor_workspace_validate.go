package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func (cursorWorkspaceAdapter) Validate(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]Diagnostic, error) {
	var diagnostics []Diagnostic
	authoredRoot := authoredRootHint(state, graph.Portable)
	if graph.Portable.MCP == nil && len(state.ComponentPaths("rules")) == 0 && strings.TrimSpace(state.DocPath("agents_markdown")) == "" {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     filepath.ToSlash(filepath.Join("targets", "cursor-workspace")),
			Target:   "cursor-workspace",
			Message:  fmt.Sprintf("Cursor workspace target requires at least one of %s/mcp/servers.yaml, %s/targets/cursor-workspace/rules/**, or %s/targets/cursor-workspace/AGENTS.md", authoredRoot, authoredRoot, authoredRoot),
		})
	}
	diagnostics = append(diagnostics, validateCursorRuleFiles(root, authoredRoot, state.ComponentPaths("rules"))...)
	diagnostics = append(diagnostics, validateCursorAgentsMarkdown(root, state.DocPath("agents_markdown"))...)
	return diagnostics, nil
}

func validateCursorRuleFiles(root, authoredRoot string, rels []string) []Diagnostic {
	if len(rels) == 0 {
		return nil
	}
	var diagnostics []Diagnostic
	seenCaseFolded := map[string]string{}
	for _, rel := range rels {
		clean := filepath.ToSlash(filepath.Clean(rel))
		if clean != rel || strings.Contains(clean, "..") {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "cursor-workspace",
				Message:  fmt.Sprintf("Cursor workspace rule file %s must stay within %s/targets/cursor-workspace/rules without path traversal", rel, authoredRoot),
			})
			continue
		}
		lower := strings.ToLower(clean)
		if prior, ok := seenCaseFolded[lower]; ok && prior != clean {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "cursor-workspace",
				Message:  fmt.Sprintf("Cursor workspace rule files %s and %s collide on case-insensitive filesystems", prior, rel),
			})
		} else {
			seenCaseFolded[lower] = clean
		}
		if strings.ToLower(filepath.Ext(rel)) != ".mdc" {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "cursor-workspace",
				Message:  fmt.Sprintf("Cursor workspace rule file %s must use the .mdc extension", rel),
			})
			continue
		}
		fullPath := filepath.Join(root, rel)
		info, err := os.Lstat(fullPath)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "cursor-workspace",
				Message:  fmt.Sprintf("Cursor workspace rule file %s is not readable: %v", rel, err),
			})
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "cursor-workspace",
				Message:  fmt.Sprintf("Cursor workspace rule file %s must not be a symlink", rel),
			})
			continue
		}
		body, err := os.ReadFile(fullPath)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "cursor-workspace",
				Message:  fmt.Sprintf("Cursor workspace rule file %s is not readable: %v", rel, err),
			})
			continue
		}
		if strings.TrimSpace(string(body)) == "" {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "cursor-workspace",
				Message:  fmt.Sprintf("Cursor workspace rule file %s must not be empty", rel),
			})
		}
	}
	return diagnostics
}

func validateCursorAgentsMarkdown(root string, rel string) []Diagnostic {
	if strings.TrimSpace(rel) == "" {
		return nil
	}
	full := filepath.Join(root, rel)
	info, err := os.Lstat(full)
	if err != nil {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     rel,
			Target:   "cursor-workspace",
			Message:  fmt.Sprintf("Cursor workspace AGENTS markdown %s is not readable: %v", rel, err),
		}}
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     rel,
			Target:   "cursor-workspace",
			Message:  fmt.Sprintf("Cursor workspace AGENTS markdown %s must not be a symlink", rel),
		}}
	}
	body, err := os.ReadFile(full)
	if err != nil {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     rel,
			Target:   "cursor-workspace",
			Message:  fmt.Sprintf("Cursor workspace AGENTS markdown %s is not readable: %v", rel, err),
		}}
	}
	if strings.TrimSpace(string(body)) == "" {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     rel,
			Target:   "cursor-workspace",
			Message:  fmt.Sprintf("Cursor workspace AGENTS markdown %s must not be empty", rel),
		}}
	}
	return nil
}
